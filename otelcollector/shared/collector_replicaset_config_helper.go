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

// httpsCheckConfig holds configurable parameters for the HTTPS connectivity check.
type httpsCheckConfig struct {
	caCertPath      string
	clientCertPath  string
	clientKeyPath   string
	taEndpoint      string
	maxRetries      int
	certRetryDelay  time.Duration
	httpsRetryDelay time.Duration
}

func CollectorTAHttpsCheck(collectorConfig string) error {
	return collectorTAHttpsCheckWithConfig(httpsCheckConfig{
		caCertPath:      "/etc/operator-targets/client/certs/ca.crt",
		clientCertPath:  "/etc/operator-targets/client/certs/client.crt",
		clientKeyPath:   "/etc/operator-targets/client/certs/client.key",
		taEndpoint:      "https://ama-metrics-operator-targets.kube-system.svc.cluster.local:443/scrape_configs",
		maxRetries:      3,
		certRetryDelay:  10 * time.Second,
		httpsRetryDelay: 10 * time.Second,
	}, collectorConfig)
}

func collectorTAHttpsCheckWithConfig(cfg httpsCheckConfig, collectorConfig string) error {
	removeHttps := false
	var resp *http.Response
	certRetryDelay := cfg.certRetryDelay

	for i := 0; i < cfg.maxRetries; i++ {
		if _, err := os.Stat(cfg.caCertPath); os.IsNotExist(err) {
			if i == cfg.maxRetries-1 {
				log.Printf("ca.crt file does not exist at path: %s after %d retries, exiting\n", cfg.caCertPath, cfg.maxRetries)
				removeHttps = true
				break
			}
			log.Printf("ca.crt file does not exist at path: %s, retrying in %v (%d/%d)\n", cfg.caCertPath, certRetryDelay, i+1, cfg.maxRetries)
			time.Sleep(certRetryDelay)
			certRetryDelay *= 2
		} else {
			log.Printf("ca.crt file exists at path: %s\n", cfg.caCertPath)
			break
		}
	}

	// Checking for HTTPS connection with exponential backoff
	if !removeHttps {
		httpsRetryDelay := cfg.httpsRetryDelay
		log.Printf("HTTPS connection check between Collector and TargetAllocator\n")
		for i := 0; i < cfg.maxRetries; i++ {
			certPEM, err := os.ReadFile(cfg.caCertPath)
			if err != nil {
				log.Printf("Failed to read CA cert file from path: %s - (%d/%d): %v\n", cfg.caCertPath, i+1, cfg.maxRetries, err)
				removeHttps = true
			} else {
				rootCAs := x509.NewCertPool()
				if ok := rootCAs.AppendCertsFromPEM(certPEM); !ok {
					log.Printf("Failed to append %s to RootCAs- (%d/%d): %v\n", cfg.caCertPath, i+1, cfg.maxRetries, err)
					removeHttps = true
				} else {
					log.Printf("[%s] Pinging Target Allocator endpoint with HTTPS\n", time.Now().Format(time.RFC3339))
					clientCert, err := tls.LoadX509KeyPair(cfg.clientCertPath, cfg.clientKeyPath)
					if err != nil {
						log.Printf("Unable to load client certs - %s\n", cfg.clientCertPath)
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
					resp, err = client.Get(cfg.taEndpoint)
					if err != nil || resp.StatusCode != http.StatusOK {
						if i == cfg.maxRetries-1 {
							log.Printf("Failed to reach Target Allocator endpoint with HTTPS after %d retries, exiting - %v\n", cfg.maxRetries, err)
							removeHttps = true
							break
						}
						log.Printf("Failed to reach Target Allocator endpoint with HTTPS, retrying in %v (%d/%d) - %v\n", httpsRetryDelay, i+1, cfg.maxRetries, err)
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
