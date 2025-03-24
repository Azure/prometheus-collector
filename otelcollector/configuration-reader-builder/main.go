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

	configmapsettings "github.com/prometheus-collector/shared/configmap/mp"

	allocatorconfig "github.com/open-telemetry/opentelemetry-operator/cmd/otel-allocator/config"
	certCreator "github.com/prometheus-collector/certcreator"
	certGenerator "github.com/prometheus-collector/certgenerator"
	certOperator "github.com/prometheus-collector/certoperator"
	yaml "gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	// DefaultValidityYears is the duration for regular certificates, SSL etc. 2 years.
	ServerValidityYears = 2

	// CaValidityYears is the duration for CA certificates. 30 years.
	CaValidityYears = 30

	// ClockSkewDuration is the allowed clock skews.
	ClockSkewDuration = time.Minute * 10

	// KeyRetryCount is the number of retries for certificate generation.
	KeyRetryCount    = 3
	KeyRetryInterval = time.Microsecond * 5
	KeyRetryTimeout  = time.Second * 10
)

type Config struct {
	CollectorSelector  *metav1.LabelSelector              `yaml:"collector_selector,omitempty"`
	Config             map[string]interface{}             `yaml:"config"`
	AllocationStrategy string                             `yaml:"allocation_strategy,omitempty"`
	PrometheusCR       allocatorconfig.PrometheusCRConfig `yaml:"prometheus_cr,omitempty"`
	FilterStrategy     string                             `yaml:"filter_strategy,omitempty"`
	HTTPS              allocatorconfig.HTTPSServerConfig  `yaml:"https,omitempty"`
}

type OtelConfig struct {
	Exporters  interface{} `yaml:"exporters"`
	Processors interface{} `yaml:"processors"`
	Extensions interface{} `yaml:"extensions"`
	Receivers  struct {
		Prometheus struct {
			Config          map[string]interface{} `yaml:"config"`
			TargetAllocator interface{}            `yaml:"target_allocator"`
		} `yaml:"prometheus"`
	} `yaml:"receivers"`
	Service struct {
		Extensions interface{} `yaml:"extensions"`
		Pipelines  struct {
			Metrics struct {
				Exporters  interface{} `yaml:"exporters"`
				Processors interface{} `yaml:"processors"`
				Receivers  interface{} `yaml:"receivers"`
			} `yaml:"metrics"`
			MetricsTelemetry struct {
				Exporters  interface{} `yaml:"exporters,omitempty"`
				Processors interface{} `yaml:"processors,omitempty"`
				Receivers  interface{} `yaml:"receivers,omitempty"`
			} `yaml:"metrics/telemetry,omitempty"`
		} `yaml:"pipelines"`
		Telemetry struct {
			Logs struct {
				Level    interface{} `yaml:"level"`
				Encoding interface{} `yaml:"encoding"`
			} `yaml:"logs"`
		} `yaml:"telemetry"`
	} `yaml:"service"`
}

var RESET = "\033[0m"
var RED = "\033[31m"

var taConfigFilePath = "/ta-configuration/targetallocator.yaml"
var taConfigUpdated = false
var taLivenessCounter = 0
var taLivenessStartTime = time.Time{}

func logFatalError(message string) {
	// Always log the full message
	log.Fatalf("%s%s%s", RED, message, RESET)
}

func updateTAConfigFile(configFilePath string) {
	defaultsMergedConfigFileContents, err := os.ReadFile(configFilePath)
	if err != nil {
		logFatalError(fmt.Sprintf("config-reader::Unable to read file contents from: %s - %v\n", configFilePath, err))
		os.Exit(1)
	}
	var promScrapeConfig map[string]interface{}
	var otelConfig OtelConfig
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

	targetAllocatorConfig := Config{
		AllocationStrategy: "consistent-hashing",
		FilterStrategy:     "relabel-config",
		CollectorSelector: &metav1.LabelSelector{
			MatchLabels: map[string]string{
				"rsName":                         "ama-metrics",
				"kubernetes.azure.com/managedby": "aks",
			},
		},
		Config: promScrapeConfig,
		PrometheusCR: allocatorconfig.PrometheusCRConfig{
			ServiceMonitorSelector: &metav1.LabelSelector{},
			PodMonitorSelector:     &metav1.LabelSelector{},
		},
		HTTPS: allocatorconfig.HTTPSServerConfig{
			Enabled:    true,
			ListenAddr: ":8443",
			// CAFilePath:      "/etc/prometheus/certs/client-ca.crt",
			// TLSCertFilePath: "/etc/prometheus/certs/server.crt",
			// TLSKeyFilePath:  "/etc/prometheus/certs/server.key",
			// CAFilePath:      "/etc/prometheus/certs/client-ca.crt",
			TLSCertFilePath: "/etc/operator-targets/certs/server.crt",
			TLSKeyFilePath:  "/etc/operator-targets/certs/server.key",
		},
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
		message += "\ninotifyoutput.txt has been updated - config-reader-config changed"
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
	notBefore := now.Add(ClockSkewDuration)
	notAfter := now.AddDate(CaValidityYears, 0, 0)

	caCSR := &x509.Certificate{
		Subject:               pkix.Name{CommonName: "ca"},
		NotBefore:             notBefore,
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
	log.Println("CA certificate is generated successfully")
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
	notBefore := now.Add(ClockSkewDuration)
	notAfter := now.AddDate(CaValidityYears, 0, 0)

	csr := &x509.Certificate{
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		Subject:               pkix.Name{CommonName: "apiserver"},
		DNSNames:              dnsNames,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}

	serverCertPem, serverKeyPem, rerr := co.CreateCertificateKeyPair(csr, caCert, caKey)
	if rerr != nil {
		log.Println("CreateCertificateKeyPair for api server failed: %s", rerr)
		return "", "", rerr
	}
	log.Println("Server certificate is generated successfully")
	return serverCertPem, serverKeyPem, nil
}

func writeServerCertAndKeyToFile(serverCertPem string, serverKeyPem string) error {
	log.Println("Writing server cert and key to file")
	if err := os.WriteFile("/etc/operator-targets/certs/server.crt", []byte(serverCertPem), fs.FileMode(0644)); err != nil {
		log.Println("Error writing server cert to file: %v\n", err)
		return err
	}
	if err := os.WriteFile("/etc/operator-targets/certs/server.key", []byte(serverKeyPem), fs.FileMode(0644)); err != nil {
		log.Println("Error writing server key to file: %v\n", err)
		return err
	}
	log.Println("Server cert and key written to file successfully")
	return nil
}

func generateSecretWithCACert(caCertPem string) error {
	log.Println("Generating secret with CA cert")
	// Code to create secret from the ca cert, server cert and server key
	secretName := "ama-metrics-operator-targets-tls-secret"
	namespace := "kube-system"

	// Create the secret data
	secretData := map[string][]byte{
		"ca.crt": []byte(caCertPem),
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
		logFatalError(fmt.Sprintf("Unable to create in-cluster config: %v", err))
		return err
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		logFatalError(fmt.Sprintf("Unable to create Kubernetes client: %v", err))
		return err
	}

	// Create or update the secret in the kube-system namespace
	_, err = clientset.CoreV1().Secrets(namespace).Create(context.TODO(), secret, metav1.CreateOptions{})
	if err != nil {
		if apierrors.IsAlreadyExists(err) {
			_, err = clientset.CoreV1().Secrets(namespace).Update(context.TODO(), secret, metav1.UpdateOptions{})
			if err != nil {
				logFatalError(fmt.Sprintf("Unable to update secret %s in namespace %s: %v", secretName, namespace, err))
				return err
			}
		} else {
			logFatalError(fmt.Sprintf("Unable to create secret %s in namespace %s: %v", secretName, namespace, err))
			return err
		}
	}

	log.Printf("Secret %s created/updated successfully in namespace %s", secretName, namespace)
	return nil
}

func createTLSCertificatesAndSecret() (error, error, error, error) {
	log.Println("Generating TLS certificates and secret")
	certCreator := certCreator.NewCertCreator()
	certGenerator := certGenerator.NewCertGenerator(certCreator)
	certOperator := certOperator.NewCertOperator(certGenerator)
	// Create CA cert, server cert and server key
	caCert, caCertPem, caKey, _, caErr := createCACertificate(certOperator)
	if caErr != nil {
		log.Println("Error creating CA certificate: %v\n", caErr)
	}
	serverCertPem, serverKeyPem, serErr := createServerCertificate(certOperator, caCert, caKey)
	if serErr != nil {
		log.Println("Error creating server certificate: %v\n", serErr)
	}

	writeErr := writeServerCertAndKeyToFile(serverCertPem, serverKeyPem)
	if writeErr != nil {
		log.Println("Error writing server cert and key to file: %v\n", writeErr)
	}

	gErr := generateSecretWithCACert(caCertPem)
	if gErr != nil {
		log.Println("Error generating secret with CA cert: %v\n", gErr)
	}
	log.Println("TLS certificates and secret generated successfully")
	return caErr, serErr, writeErr, gErr
}

func main() {
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

	configmapsettings.Configmapparser()
	if os.Getenv("AZMON_USE_DEFAULT_PROMETHEUS_CONFIG") == "true" {
		if _, err = os.Stat("/opt/microsoft/otelcollector/collector-config-default.yml"); err == nil {
			updateTAConfigFile("/opt/microsoft/otelcollector/collector-config-default.yml")
		}
	} else if _, err = os.Stat("/opt/microsoft/otelcollector/collector-config.yml"); err == nil {
		updateTAConfigFile("/opt/microsoft/otelcollector/collector-config.yml")
	} else {
		log.Println("No configs found via configmap, not running config reader")
	}

	http.HandleFunc("/health", healthHandler)
	http.HandleFunc("/health-ta", taHealthHandler)

	http.ListenAndServe(":8081", nil)

	caErr, serErr, writeErr, gErr := createTLSCertificatesAndSecret()

	if caErr != nil || serErr != nil || writeErr != nil || gErr != nil {
		log.Println("Error creating TLS certificates and secret, retrying in 5 seconds")
		time.Sleep(5 * time.Second)
		caErr1, serErr1, writeErr1, gErr1 := createTLSCertificatesAndSecret()
		if caErr1 != nil || serErr1 != nil || writeErr1 != nil || gErr1 != nil {
			log.Println("Error creating TLS certificates and secret, during retry, not trying again")
			if caErr1 != nil {
				log.Println("Error during ca cert creation: %v\n", caErr1)
			}
			if serErr1 != nil {
				log.Println("Error during server cert creation: %v\n", serErr1)
			}
			if writeErr1 != nil {
				log.Println("Error writing server cert and key to file: %v\n", writeErr1)
			}
			if gErr1 != nil {
				log.Println("Error generating secret with CA cert: %v\n", gErr1)
			}
		}
	}

}
