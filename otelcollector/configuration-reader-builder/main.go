package main

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os/exec"
	"strings"
	"time"

	"os"

	certCreator "github.com/prometheus-collector/certcreator"
	certGenerator "github.com/prometheus-collector/certgenerator"
	certOperator "github.com/prometheus-collector/certoperator"
	shared "github.com/prometheus-collector/shared"
	configmapsettings "github.com/prometheus-collector/shared/configmap/mp"
	yaml "gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	// DefaultValidityYears is the duration for regular certificates, SSL etc. 2 years.
	ServerValidityMonths = 8

	ClientValidityMonths = 8

	// CaValidityYears is the duration for CA certificates. 30 years.
	CaValidityYears = 2

	// KeyRetryCount is the number of retries for certificate generation.
	KeyRetryCount    = 3
	KeyRetryInterval = time.Microsecond * 5
	KeyRetryTimeout  = time.Second * 10
)

var RESET = "\033[0m"
var RED = "\033[31m"

var taConfigFilePath = "/ta-configuration/targetallocator.yaml"
var taConfigUpdated = false
var taLivenessCounter = 0
var taLivenessStartTime = time.Time{}
var cfgReaderContainerStartTime = time.Time{}

func logFatalError(message string) {
	// Always log the full message
	log.Fatalf("%s%s%s", RED, message, RESET)
}

func updateTAConfigFile(configFilePath string, httpsEnabled bool) {
	defaultsMergedConfigFileContents, err := os.ReadFile(configFilePath)
	if err != nil {
		logFatalError(fmt.Sprintf("config-reader::Unable to read file contents from: %s - %v\n", configFilePath, err))
		os.Exit(1)
	}
	var promScrapeConfig map[string]interface{}
	var otelConfig shared.OtelConfig
	err = yaml.Unmarshal([]byte(defaultsMergedConfigFileContents), &otelConfig)
	if err != nil {
		logFatalError(fmt.Sprintf("config-reader::Unable to unmarshal merged otel configuration from: %s - %v\n", configFilePath, err))
		os.Exit(1)
	}

	promScrapeConfig = otelConfig.Receivers.Prometheus.Config
	// Removing $$ added for regex and replacement in relabel_config and metric_relabel_config added by promconfigvalidator.
	// The $$ are required by the validator's otel get method, but the TA doesnt do env substitution and hence needs to be removed, else TA crashes.
	scrapeConfigs := promScrapeConfig["scrape_configs"]
	if scrapeConfigs != nil {
		var sc = scrapeConfigs.([]interface{})
		for _, scrapeConfig := range sc {
			scrapeConfig := scrapeConfig.(map[interface{}]interface{})
			if scrapeConfig["relabel_configs"] != nil {
				relabelConfigs := scrapeConfig["relabel_configs"].([]interface{})
				for _, relabelConfig := range relabelConfigs {
					relabelConfig := relabelConfig.(map[interface{}]interface{})
					//replace $$ with $ for regex field
					if relabelConfig["regex"] != nil {
						// Adding this check here since regex can be boolean and the conversion will fail
						if _, isString := relabelConfig["regex"].(string); isString {
							regexString := relabelConfig["regex"].(string)
							modifiedRegexString := strings.ReplaceAll(regexString, "$$", "$")
							relabelConfig["regex"] = modifiedRegexString
						}
					}
					//replace $$ with $ for replacement field
					if relabelConfig["replacement"] != nil {
						replacement := relabelConfig["replacement"].(string)
						modifiedReplacementString := strings.ReplaceAll(replacement, "$$", "$")
						relabelConfig["replacement"] = modifiedReplacementString
					}
				}
			}

			if scrapeConfig["metric_relabel_configs"] != nil {
				metricRelabelConfigs := scrapeConfig["metric_relabel_configs"].([]interface{})
				for _, metricRelabelConfig := range metricRelabelConfigs {
					metricRelabelConfig := metricRelabelConfig.(map[interface{}]interface{})
					//replace $$ with $ for regex field
					if metricRelabelConfig["regex"] != nil {
						// Adding this check here since regex can be boolean and the conversion will fail
						if _, isString := metricRelabelConfig["regex"].(string); isString {
							regexString := metricRelabelConfig["regex"].(string)
							modifiedRegexString := strings.ReplaceAll(regexString, "$$", "$")
							metricRelabelConfig["regex"] = modifiedRegexString
						}
					}

					//replace $$ with $ for replacement field
					if metricRelabelConfig["replacement"] != nil {
						replacement := metricRelabelConfig["replacement"].(string)
						modifiedReplacementString := strings.ReplaceAll(replacement, "$$", "$")
						metricRelabelConfig["replacement"] = modifiedReplacementString
					}
				}
			}
		}
	}

	var targetAllocatorConfig shared.Config

	if os.Getenv("AZMON_OPERATOR_HTTPS_ENABLED") == "true" && httpsEnabled {
		fmt.Println("AZMON_OPERATOR_HTTPS_ENABLED is true, setting tls config in TargetAllocator")
		targetAllocatorConfig = shared.Config{
			AllocationStrategy: "consistent-hashing",
			FilterStrategy:     "relabel-config",
			CollectorSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"rsName":                         "ama-metrics",
					"kubernetes.azure.com/managedby": "aks",
				},
			},
			Config: promScrapeConfig,
			PrometheusCR: shared.PrometheusCRConfig{
				ServiceMonitorSelector: &metav1.LabelSelector{},
				PodMonitorSelector:     &metav1.LabelSelector{},
			},
			HTTPS: shared.HTTPSServerConfig{
				Enabled:         true,
				ListenAddr:      ":8443",
				TLSCertFilePath: "/etc/operator-targets/server/certs/server.crt",
				TLSKeyFilePath:  "/etc/operator-targets/server/certs/server.key",
				CAFilePath:      "/etc/operator-targets/server/certs/ca.crt",
			},
		}
	} else {
		fmt.Println("AZMON_OPERATOR_HTTPS_ENABLED is not set/false or error in cert creation, not setting tls config in TargetAllocator")
		targetAllocatorConfig = shared.Config{
			AllocationStrategy: "consistent-hashing",
			FilterStrategy:     "relabel-config",
			CollectorSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"rsName":                         "ama-metrics",
					"kubernetes.azure.com/managedby": "aks",
				},
			},
			Config: promScrapeConfig,
			PrometheusCR: shared.PrometheusCRConfig{
				ServiceMonitorSelector: &metav1.LabelSelector{},
				PodMonitorSelector:     &metav1.LabelSelector{},
			},
		}
	}

	targetAllocatorConfigYaml, _ := yaml.Marshal(targetAllocatorConfig)
	if err := os.WriteFile(taConfigFilePath, targetAllocatorConfigYaml, 0644); err != nil {
		logFatalError(fmt.Sprintf("config-reader::Unable to write to: %s - %v\n", taConfigFilePath, err))
		os.Exit(1)
	}

	log.Println("Updated file - targetallocator.yaml for the TargetAllocator to pick up new config changes")
	taConfigUpdated = true
	taLivenessStartTime = time.Now()
}

func hasConfigChanged(filePath string) bool {
	if _, err := os.Stat(filePath); err == nil {
		fileInfo, err := os.Stat(filePath)
		if err != nil {
			fmt.Println("Error getting file info:", err)
			os.Exit(1)
		}

		return fileInfo.Size() > 0
	}
	return false
}

func taHealthHandler(w http.ResponseWriter, r *http.Request) {
	status := http.StatusOK
	message := "\ntargetallocator is running."

	client := &http.Client{Timeout: time.Duration(2) * time.Second}

	req, err := http.NewRequest("GET", "http://localhost:8080/metrics", nil)
	if err == nil {
		resp, _ := client.Do(req)
		if resp != nil && resp.StatusCode == http.StatusOK {
			if taConfigUpdated {
				if !taLivenessStartTime.IsZero() {
					duration := time.Since(taLivenessStartTime)
					// Serve the response of ServiceUnavailable for 60s and then reset
					if duration.Seconds() < 60 {
						status = http.StatusServiceUnavailable
						message += "targetallocator-config changed"
					} else {
						taConfigUpdated = false
						taLivenessStartTime = time.Time{}
					}
				}
			}

			if hasConfigChanged("/opt/inotifyoutput-ta-server-cert-secret.txt") {
				status = http.StatusServiceUnavailable
				message = "\ninotifyoutput-ta-server-cert-secret.txt has been updated"
				// Resetting contents of inotifyoutput-ta-server-cert-secret.txt file after detecting changes to secret
				if err := os.WriteFile("/opt/inotifyoutput-ta-server-cert-secret.txt", []byte{}, 0644); err != nil {
					log.Printf("Error clearing inotifyoutput-ta-server-cert-secret.txt: %v", err)
				}
			}

			if status != http.StatusOK {
				fmt.Printf(message)
			}
			w.WriteHeader(status)
			fmt.Fprintln(w, message)
		}
		if resp != nil && resp.Body != nil {
			defer resp.Body.Close()
		}
	} else {
		message = "\ncall to get TA metrics failed"
		status = http.StatusServiceUnavailable
		fmt.Printf(message)
		w.WriteHeader(status)
		fmt.Fprintln(w, message)
	}
}

func writeTerminationLog(message string) {
	if err := os.WriteFile("/dev/termination-log", []byte(message), fs.FileMode(0644)); err != nil {
		log.Printf("Error writing to termination log: %v", err)
	}
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	status := http.StatusOK
	message := "\nconfig-reader is running."

	if hasConfigChanged("/opt/inotifyoutput.txt") {
		status = http.StatusServiceUnavailable
		message = "\ninotifyoutput.txt has been updated - config-reader-config changed"
	}

	if hasConfigChanged("/opt/inotifyoutput-server-cert-secret.txt") {
		status = http.StatusServiceUnavailable
		message = "\ninotifyoutput-server-cert-secret.txt has been updated"
	}

	if hasConfigChanged("/opt/inotifyoutput-ca-cert-secret.txt") {
		status = http.StatusServiceUnavailable
		message = "\ninotifyoutput-ca-cert-secret.txt has been updated"
	}

	if os.Getenv("AZMON_OPERATOR_HTTPS_ENABLED") == "true" {
		duration := time.Since(cfgReaderContainerStartTime)
		// Server certificate validity is for 8 months, so if the container is running for more than 5 months, then restart the container
		if duration.Hours() > (5 * 30 * 24) {
			status = http.StatusServiceUnavailable
			message = "\nconfig-reader container is running for more than 5 months, restart the container"
		}
	}

	w.WriteHeader(status)
	fmt.Fprintln(w, message)
	if status != http.StatusOK {
		fmt.Printf(message)
		writeTerminationLog(message)
	}
}

func createCACertificate(co certOperator.CertOperator) (*x509.Certificate, string, *rsa.PrivateKey, string, error) {
	log.Println("Creating CA certificate")
	now := time.Now()
	notAfter := now.AddDate(CaValidityYears, 0, 0)

	caCSR := &x509.Certificate{
		Subject:               pkix.Name{CommonName: "ama-metrics-operator-targets-CA"},
		NotBefore:             now,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	caCert, caCertPem, caKey, caKeyPem, err := co.CreateSelfSignedCertificateKeyPair(caCSR)

	if err != nil {
		log.Println("CreateSelfSignedCertificateKeyPair for ca failed: %s", err)
		return nil, "", nil, "", err
	}
	log.Println("CA certificate is generated successfully.")
	return caCert, caCertPem, caKey, caKeyPem, nil
}

func createServerCertificate(co certOperator.CertOperator, caCert *x509.Certificate,
	caKey *rsa.PrivateKey) (string, string, error) {
	log.Println("Creating server certificate")
	dnsNames := []string{
		"localhost",
		"ama-metrics-operator-targets.kube-system.svc.cluster.local",
	}
	now := time.Now()
	notAfter := now.AddDate(0, ServerValidityMonths, 0)

	csr := &x509.Certificate{
		NotBefore:             now,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		Subject:               pkix.Name{CommonName: "ama-metrics-operator-targets"},
		DNSNames:              dnsNames,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}

	serverCertPem, serverKeyPem, rerr := co.CreateCertificateKeyPair(csr, caCert, caKey)
	if rerr != nil {
		log.Println("CreateCertificateKeyPair for targetallocator failed: %s", rerr)
		return "", "", rerr
	}
	log.Println("Server certificate is generated successfully")
	return serverCertPem, serverKeyPem, nil
}

func createClientCertificate(co certOperator.CertOperator, caCert *x509.Certificate,
	caKey *rsa.PrivateKey) (string, string, error) {
	log.Println("Creating client certificate")
	now := time.Now()
	notAfter := now.AddDate(0, ClientValidityMonths, 0)

	csr := &x509.Certificate{
		NotBefore:             now,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		Subject:               pkix.Name{CommonName: "ama-metrics"},
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}

	clientCertPem, clientKeyPem, rerr := co.CreateCertificateKeyPair(csr, caCert, caKey)
	if rerr != nil {
		log.Println("CreateCertificateKeyPair for replicaset client failed: %s", rerr)
		return "", "", rerr
	}
	log.Println("Client certificate is generated successfully")
	return clientCertPem, clientKeyPem, nil
}

func generateSecretWithServerCertsForTA(serverCertPem string, serverKeyPem string, caCertPem string) error {
	log.Println("Generating secret with server cert, server key and CA cert")
	// Create secret from the ca cert, server cert and server key
	secretName := "ama-metrics-operator-targets-server-tls-secret"
	namespace := "kube-system"

	// Create the secret data
	secretData := map[string][]byte{
		"ca.crt":     []byte(caCertPem),
		"server.crt": []byte(serverCertPem),
		"server.key": []byte(serverKeyPem),
	}

	// Create the secret object
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: namespace,
		},
		Data: secretData,
		Type: corev1.SecretTypeOpaque,
	}

	// Create the Kubernetes client
	config, err := rest.InClusterConfig()
	if err != nil {
		log.Printf("Unable to create in-cluster config: %v", err)
		return err
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Printf("Unable to create Kubernetes client: %v", err)
		return err
	}

	// Create or update the secret in the kube-system namespace
	_, err = clientset.CoreV1().Secrets(namespace).Create(context.TODO(), secret, metav1.CreateOptions{})
	if err != nil {
		if apierrors.IsAlreadyExists(err) {
			_, err = clientset.CoreV1().Secrets(namespace).Update(context.TODO(), secret, metav1.UpdateOptions{})
			if err != nil {
				log.Printf("Unable to update secret %s in namespace %s: %v", secretName, namespace, err)
				return err
			}
		} else {
			log.Printf("Unable to create secret %s in namespace %s: %v", secretName, namespace, err)
			return err
		}
	}

	log.Printf("Secret %s created/updated successfully in namespace %s", secretName, namespace)
	return nil
}

func generateSecretWithClientCertForRs(clientCertPem string, clientKeyPem string, caCertPem string) error {
	log.Println("Generating secret with CA cert")
	// Create secret from the ca cert, server cert and server key
	secretName := "ama-metrics-operator-targets-client-tls-secret"
	namespace := "kube-system"

	// Create the secret data
	secretData := map[string][]byte{
		"ca.crt":     []byte(caCertPem),
		"client.crt": []byte(clientCertPem),
		"client.key": []byte(clientKeyPem),
	}

	// Create the secret object
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: namespace,
		},
		Data: secretData,
		Type: corev1.SecretTypeOpaque,
	}

	// Create the Kubernetes client
	config, err := rest.InClusterConfig()
	if err != nil {
		log.Printf("Unable to create in-cluster config: %v", err)
		return err
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Printf("Unable to create Kubernetes client: %v", err)
		return err
	}

	// Create or update the secret in the kube-system namespace
	_, err = clientset.CoreV1().Secrets(namespace).Create(context.TODO(), secret, metav1.CreateOptions{})
	if err != nil {
		if apierrors.IsAlreadyExists(err) {
			_, err = clientset.CoreV1().Secrets(namespace).Update(context.TODO(), secret, metav1.UpdateOptions{})
			if err != nil {
				log.Printf("Unable to update secret %s in namespace %s: %v", secretName, namespace, err)
				return err
			}
		} else {
			log.Printf("Unable to create secret %s in namespace %s: %v", secretName, namespace, err)
			return err
		}
	}

	log.Printf("Secret %s created/updated successfully in namespace %s", secretName, namespace)
	return nil
}

func createTLSCertificatesAndSecret() (error, error, error, error, error) {
	log.Println("Generating TLS certificates and secret")
	certCreator := certCreator.NewCertCreator()
	certGenerator := certGenerator.NewCertGenerator(certCreator)
	certOperator := certOperator.NewCertOperator(certGenerator)
	// Create CA cert, server cert and server key
	caCert, caCertPem, caKey, _, caErr := createCACertificate(certOperator)
	if caErr != nil {
		log.Println("Error creating CA certificate: %v\n", caErr)
	}
	// TODO: add delay for TA start
	serverCertPem, serverKeyPem, serErr := createServerCertificate(certOperator, caCert, caKey)
	if serErr != nil {
		log.Println("Error creating server certificate: %v\n", serErr)
	}

	clientCertPem, clientKeyPem, cliErr := createClientCertificate(certOperator, caCert, caKey)
	if cliErr != nil {
		log.Println("Error creating client certificate: %v\n", cliErr)
	}

	var serverSecretErr error
	serverSecretErr = nil
	if caErr == nil && serErr == nil && cliErr == nil {
		log.Println("Generating secret so that targetallocator pod can get the certs and key")
		serverSecretErr := generateSecretWithServerCertsForTA(serverCertPem, serverKeyPem, caCertPem)
		if serverSecretErr != nil {
			log.Println("Error generating secret for targetallocator: %v\n", serverSecretErr)
		}
	}

	var clientSecretErr error
	clientSecretErr = nil
	if caErr == nil && serErr == nil && cliErr == nil {
		log.Println("Generating secret so that replicaset pod can get the certs and key")
		clientSecretErr := generateSecretWithClientCertForRs(clientCertPem, clientKeyPem, caCertPem)
		if clientSecretErr != nil {
			log.Println("Error generating secret for replciaset: %v\n", clientSecretErr)
		}
	}
	log.Println("TLS certificates and secret generated successfully")
	return caErr, serErr, cliErr, serverSecretErr, clientSecretErr
}

func main() {
	cfgReaderContainerStartTime = time.Now()
	_, err := os.Create("/opt/inotifyoutput.txt")
	if err != nil {
		log.Fatalf("Error creating output file: %v\n", err)
		log.Println("Error creating inotify output file:", err)
	}

	// Define the command to start inotify for config reader's liveness probe
	inotifyCommandCfg := exec.Command(
		"inotifywait",
		"/etc/config/settings",
		"--daemon",
		"--recursive",
		"--outfile", "/opt/inotifyoutput.txt",
		"--event", "create",
		"--event", "delete",
		"--format", "%e : %T",
		"--timefmt", "+%s",
	)

	// Start the inotify process
	err = inotifyCommandCfg.Start()
	if err != nil {
		log.Fatalf("Error starting inotify process for config reader's liveness probe: %v\n", err)
		fmt.Println("Error starting inotify process:", err)
	}
	httpsEnabled := true
	caErr, serErr, cliErr, serverSecretErr, clientSecretErr := createTLSCertificatesAndSecret()

	if caErr != nil || serErr != nil || cliErr != nil || serverSecretErr != nil || clientSecretErr != nil {
		log.Println("Error creating TLS certificates and secret, retrying in 5 seconds")
		time.Sleep(5 * time.Second)
		caErr1, serErr1, cliErr1, serverSecretErr1, clientSecretErr1 := createTLSCertificatesAndSecret()
		if caErr1 != nil || serErr1 != nil || cliErr1 != nil || serverSecretErr1 != nil || clientSecretErr1 != nil {
			log.Println("Error creating TLS certificates and secret, during retry, not trying again")
			if caErr1 != nil {
				log.Println("Error during ca cert creation: %v\n", caErr1)
				httpsEnabled = false
			}
			if serErr1 != nil {
				log.Println("Error during server cert creation: %v\n", serErr1)
				httpsEnabled = false
			}

			if cliErr1 != nil {
				log.Println("Error during client cert creation: %v\n", serErr1)
				httpsEnabled = false
			}
			if serverSecretErr1 != nil {
				log.Println("Error generating secret for targetallocator: %v\n", serverSecretErr1)
				httpsEnabled = false
			}
			if clientSecretErr1 != nil {
				log.Println("Error generating secret for replicaset: %v\n", clientSecretErr1)
				httpsEnabled = false
			}
		}
	}

	configmapsettings.Configmapparser()
	if os.Getenv("AZMON_USE_DEFAULT_PROMETHEUS_CONFIG") == "true" {
		if _, err = os.Stat("/opt/microsoft/otelcollector/collector-config-default.yml"); err == nil {
			updateTAConfigFile("/opt/microsoft/otelcollector/collector-config-default.yml", httpsEnabled)
		}
	} else if _, err = os.Stat("/opt/microsoft/otelcollector/collector-config.yml"); err == nil {
		updateTAConfigFile("/opt/microsoft/otelcollector/collector-config.yml", httpsEnabled)
	} else {
		log.Println("No configs found via configmap, not running config reader")
	}

	if os.Getenv("AZMON_OPERATOR_HTTPS_ENABLED") == "true" {
		log.Println("AZMON_OPERATOR_HTTPS_ENABLED is true, starting inotify for server certs and ca certs")
		// Start inotify for server certs and ca certs for TargetAllocator container since the server has to
		// pick up the latest server certs and ca certs after the secret to configmap propogation happens
		outputFile := "/opt/inotifyoutput-ta-server-cert-secret.txt"
		log.Println("Starting inotify for server certs")
		if err = shared.Inotify(outputFile, "/etc/operator-targets/server/certs"); err != nil {
			log.Println("Error starting inotify for watching targetallocator server certs: %v\n", err)
		}

		// Wait for 10 seconds before starting inotify for server certs and ca certs
		// This is to ensure that the server certs and ca certs are generated before starting inotify
		time.Sleep(90 * time.Second)
		outputFile = "/opt/inotifyoutput-server-cert-secret.txt"
		log.Println("Starting inotify for server certs")
		if err = shared.Inotify(outputFile, "/etc/operator-targets/server/certs"); err != nil {
			log.Println("Error starting inotify for watching targetallocator server certs: %v\n", err)
		}
		outputFile = "/opt/inotifyoutput-ca-cert-secret.txt"
		log.Println("Starting inotify for ca certs")
		if err = shared.Inotify(outputFile, "/etc/operator-targets/client/certs"); err != nil {
			log.Println("Error starting inotify for watching targetallocator client certs: %v\n", err)
		}
	}

	http.HandleFunc("/health", healthHandler)
	http.HandleFunc("/health-ta", taHealthHandler)

	http.ListenAndServe(":8081", nil)

}
