package shared

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"time"

	yaml "gopkg.in/yaml.v2"
)

func RemoveHTTPSSettingsInCollectorConfig(configpath string) error {
	configFileContents, err := os.ReadFile(configpath)
	if err != nil {
		fmt.Printf("Unable to read file contents from: %s - %v\n", configpath, err)
		return err
	}
	var otelConfig OtelConfig
	err = yaml.Unmarshal([]byte(configFileContents), &otelConfig)
	if err != nil {
		fmt.Printf("Unable to unmarshal merged otel configuration from: %s - %v\n", configFileContents, err)
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
		fmt.Printf("Unable to marshal updated otel configuration - %v\n", err)
		return err
	}
	if err := os.WriteFile(configpath, updatedConfigYaml, 0644); err != nil {
		fmt.Printf("Unable to write updated configuration to: %s - %v\n", configpath, err)
		return err
	}
	if err := os.Setenv("TARGETALLOCATOR_INSECURE", "true"); err != nil {
		fmt.Printf("Unable to set environment variable TARGETALLOCATOR_INSECURE - %v\n", err)
		return err
	}
	fmt.Println("Updated HTTPS configuration written to", configpath)
	return nil
}

func CollectorTAHttpsCheck(collectorConfig string) error {
	caCertPath := "/etc/operator-targets/client/certs/ca.crt"
	setInsecure := false
	// Checking for file existence before proceeding.
	retries := 2
	var resp *http.Response

	for i := 0; i <= retries; i++ {
		if _, err := os.Stat(caCertPath); os.IsNotExist(err) {
			if i == retries {
				fmt.Printf("ca.crt file does not exist at path: %s after %d retries, exiting\n", caCertPath, retries)
				setInsecure = true
				break
			}
			fmt.Printf("ca.crt file does not exist at path: %s, retrying in 30s (%d/%d)\n", caCertPath, i+1, retries)
			time.Sleep(30 * time.Second)
		} else {
			fmt.Printf("ca.crt file exists at path: %s\n", caCertPath)
			break
		}
	}

	// Checking for HTTPS connection with retries
	if !setInsecure {
		retries_https := 2
		fmt.Printf("HTTPS connection check between Collector and TargetAllocator\n")
		for i := 0; i <= retries_https; i++ {
			certPEM, err := ioutil.ReadFile(caCertPath)
			if err != nil {
				fmt.Printf("Failed to read CA cert file from path: %s - (%d/%d): %v\n", caCertPath, i+1, retries_https)
				setInsecure = true
				// break
			} else {
				// Create a new cert pool
				rootCAs := x509.NewCertPool()
				// Append CA cert to the new pool
				if ok := rootCAs.AppendCertsFromPEM(certPEM); !ok {
					fmt.Printf("Failed to append %s to RootCAs- (%d/%d): %v\n", caCertPath, i+1, retries_https, err)
					setInsecure = true
					// break
				} else {
					fmt.Printf("[%s] Pinging Target Allocator endpoint with HTTPS\n", time.Now().Format(time.RFC3339))
					// Load client certificate and key
					certPath := "/etc/operator-targets/client/certs/client.crt"
					keyPath := "/etc/operator-targets/client/certs/client.key"
					clientCert, err := tls.LoadX509KeyPair(certPath, keyPath)
					if err != nil {
						fmt.Printf("Unable to load client certs - %s\n", certPath)
						setInsecure = true
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
						if i == retries_https {
							fmt.Printf("Failed to reach Target Allocator endpoint with HTTPS after %d retries, exiting - %v\n", retries_https, err)
							setInsecure = true
							break
						}
						fmt.Printf("Failed to reach Target Allocator endpoint with HTTPS, retrying in 30s (%d/%d) - %v\n", i+1, retries_https, err)
						time.Sleep(30 * time.Second)
					} else {
						fmt.Printf("Target Allocator endpoint is reachable with HTTPS\n")
						setInsecure = false
						break
					}

				}
			}
		}
	}

	if resp != nil && resp.Body != nil {
		defer resp.Body.Close()
	}

	if setInsecure {
		// Fallback to starting without HTTPS
		// _ = SetInsecureInCollectorConfig(collectorConfig)
		_ = RemoveHTTPSSettingsInCollectorConfig(collectorConfig)
	} else {
		if err := os.Setenv("TARGETALLOCATOR_INSECURE", "false"); err != nil {
			fmt.Printf("Unable to set environment variable TARGETALLOCATOR_INSECURE - %v\n", err)
			return err
		}
	}

	return nil
}
