package shared

import (
	"crypto/tls"
	"crypto/x509"
	"log"
	"net/http"
	"os"
	"time"

	yaml "gopkg.in/yaml.v2"
)

func RemoveHTTPSSettingsInCollectorConfig(configpath string) error {
	configFileContents, err := os.ReadFile(configpath)
	if err != nil {
		log.Printf("Unable to read file contents from: %s - %v\n", configpath, err)
		return err
	}
	var otelConfig OtelConfig
	err = yaml.Unmarshal([]byte(configFileContents), &otelConfig)
	if err != nil {
		log.Printf("Unable to unmarshal merged otel configuration from: %s - %v\n", configFileContents, err)
		return err
	}

	targetAllocatorConfig := otelConfig.Receivers.Prometheus.TargetAllocator
	tlsSettings := targetAllocatorConfig["tls"]
	if tlsSettings != nil {
		delete(targetAllocatorConfig, "tls")
	}
	targetAllocatorConfig["endpoint"] = "http://ama-metrics-operator-targets.kube-system.svc.cluster.local"

	updatedConfigYaml, err := yaml.Marshal(otelConfig)
	if err != nil {
		log.Printf("Unable to marshal updated otel configuration - %v\n", err)
		return err
	}
	if err := os.WriteFile(configpath, updatedConfigYaml, 0644); err != nil {
		log.Printf("Unable to write updated configuration to: %s - %v\n", configpath, err)
		return err
	}
	if err := os.Setenv("COLLECTOR_CONFIG_HTTPS_REMOVED", "true"); err != nil {
		log.Printf("Unable to set environment variable COLLECTOR_CONFIG_HTTPS_REMOVED - %v\n", err)
		return err
	}
	log.Println("Updated HTTPS configuration written to", configpath)
	return nil
}

func CollectorTAHttpsCheck(collectorConfig string) error {
	caCertPath := "/etc/operator-targets/client/certs/ca.crt"
	removeHttps := false
	// Checking for file existence with exponential backoff before proceeding.
	maxRetries := 3
	var resp *http.Response
	certRetryDelay := 10 * time.Second

	for i := 0; i < maxRetries; i++ {
		if _, err := os.Stat(caCertPath); os.IsNotExist(err) {
			if i == maxRetries-1 {
				log.Printf("ca.crt file does not exist at path: %s after %d retries, exiting\n", caCertPath, maxRetries)
				removeHttps = true
				break
			}
			log.Printf("ca.crt file does not exist at path: %s, retrying in %v (%d/%d)\n", caCertPath, certRetryDelay, i+1, maxRetries)
			time.Sleep(certRetryDelay)
			certRetryDelay *= 2
		} else {
			log.Printf("ca.crt file exists at path: %s\n", caCertPath)
			break
		}
	}

	// Checking for HTTPS connection with exponential backoff
	if !removeHttps {
		httpsRetryDelay := 10 * time.Second
		log.Printf("HTTPS connection check between Collector and TargetAllocator\n")
		for i := 0; i < maxRetries; i++ {
			certPEM, err := os.ReadFile(caCertPath)
			if err != nil {
				log.Printf("Failed to read CA cert file from path: %s - (%d/%d): %v\n", caCertPath, i+1, maxRetries, err)
				removeHttps = true
			} else {
				rootCAs := x509.NewCertPool()
				if ok := rootCAs.AppendCertsFromPEM(certPEM); !ok {
					log.Printf("Failed to append %s to RootCAs- (%d/%d): %v\n", caCertPath, i+1, maxRetries, err)
					removeHttps = true
				} else {
					log.Printf("[%s] Pinging Target Allocator endpoint with HTTPS\n", time.Now().Format(time.RFC3339))
					certPath := "/etc/operator-targets/client/certs/client.crt"
					keyPath := "/etc/operator-targets/client/certs/client.key"
					clientCert, err := tls.LoadX509KeyPair(certPath, keyPath)
					if err != nil {
						log.Printf("Unable to load client certs - %s\n", certPath)
						removeHttps = true
						break
					}

					client := &http.Client{
						Transport: &http.Transport{
							TLSClientConfig: &tls.Config{
								RootCAs:      rootCAs,
								Certificates: []tls.Certificate{clientCert},
							},
						},
					}
					resp, err = client.Get("https://ama-metrics-operator-targets.kube-system.svc.cluster.local:443/scrape_configs")
					if err != nil || resp.StatusCode != http.StatusOK {
						if i == maxRetries-1 {
							log.Printf("Failed to reach Target Allocator endpoint with HTTPS after %d retries, exiting - %v\n", maxRetries, err)
							removeHttps = true
							break
						}
						log.Printf("Failed to reach Target Allocator endpoint with HTTPS, retrying in %v (%d/%d) - %v\n", httpsRetryDelay, i+1, maxRetries, err)
						time.Sleep(httpsRetryDelay)
						httpsRetryDelay *= 2
					} else {
						log.Printf("Target Allocator endpoint is reachable with HTTPS\n")
						removeHttps = false
						break
					}
				}
			}
		}
	}

	if resp != nil && resp.Body != nil {
		defer resp.Body.Close()
	}

	if removeHttps {
		// Fallback to starting without HTTPS
		_ = RemoveHTTPSSettingsInCollectorConfig(collectorConfig)
	} else {
		if err := os.Setenv("COLLECTOR_CONFIG_WITH_HTTPS", "true"); err != nil {
			log.Printf("Unable to set environment variable COLLECTOR_CONFIG_WITH_HTTPS - %v\n", err)
			return err
		}
	}

	return nil
}
