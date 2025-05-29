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

func SetInsecureInCollectorConfig(configpath string) error {
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
		if tlsMap, ok := tlsSettings.(map[interface{}]interface{}); ok {
			tlsMap["insecure_skip_verify"] = true
			targetAllocatorConfig["tls"] = tlsMap
		} else {
			fmt.Println("TLS settings are not in the expected format")
		}
	} else {
		fmt.Println("TLS settings are nil, not adding insecure")
	}

	updatedConfigYaml, err := yaml.Marshal(otelConfig)
	if err != nil {
		fmt.Printf("Unable to marshal updated otel configuration - %v\n", err)
		return err
	}
	if err := os.WriteFile(configpath, updatedConfigYaml, 0644); err != nil {
		fmt.Printf("Unable to write updated configuration to: %s - %v\n", configpath, err)
		return err
	}
	if err := os.Setenv("COLLECTOR_CONFIG_INSECURE", "true"); err != nil {
		fmt.Printf("Unable to set environment variable COLLECTOR_CONFIG_INSECURE - %v\n", err)
		return err
	}
	fmt.Println("Updated configuration written to", configpath)
	return nil
}

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
	if err := os.Setenv("COLLECTOR_CONFIG_HTTPS_REMOVED", "true"); err != nil {
		fmt.Printf("Unable to set environment variable COLLECTOR_CONFIG_HTTPS_REMOVED - %v\n", err)
		return err
	}
	fmt.Println("Updated HTTPS configuration written to", configpath)
	return nil
}

func CollectorTAHttpsCheck(collectorConfig string) error {
	caCertPath := "/etc/operator-targets/client/certs/ca.crt"
	removeHttps := false
	setInsecure := false
	// Checking for file existence before proceeding.
	retries := 2
	var resp *http.Response

	for i := 0; i <= retries; i++ {
		if _, err := os.Stat(caCertPath); os.IsNotExist(err) {
			if i == retries {
				fmt.Printf("ca.crt file does not exist at path: %s after %d retries, exiting\n", caCertPath, retries)
				removeHttps = true
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
	if !removeHttps {
		retries_https := 2
		fmt.Printf("HTTPS connection check between Collector and TargetAllocator\n")
		for i := 0; i <= retries_https; i++ {
			certPEM, err := ioutil.ReadFile(caCertPath)
			if err != nil {
				fmt.Printf("Failed to read CA cert file from path: %s - (%d/%d): %v\n", caCertPath, i+1, retries_https)
				removeHttps = true
				// break
			} else {
				// Create a new cert pool
				rootCAs := x509.NewCertPool()
				// Append CA cert to the new pool
				if ok := rootCAs.AppendCertsFromPEM(certPEM); !ok {
					fmt.Printf("Failed to append %s to RootCAs- (%d/%d): %v\n", caCertPath, i+1, retries_https, err)
					removeHttps = true
					// break
				} else {
					fmt.Printf("[%s] Pinging Target Allocator endpoint with HTTPS\n", time.Now().Format(time.RFC3339))
					// Load client certificate and key
					certPath := "/etc/operator-targets/client/certs/client.crt"
					keyPath := "/etc/operator-targets/client/certs/client.key"
					clientCert, err := tls.LoadX509KeyPair(certPath, keyPath)
					if err != nil {
						fmt.Printf("Unable to load client certs - %s\n", certPath)
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
						if i == retries_https {
							fmt.Printf("Failed to reach Target Allocator endpoint with HTTPS after %d retries - %v\n", retries_https, err)
							fmt.Printf("Trying insecure mode\n")
							client = &http.Client{
								Transport: &http.Transport{
									TLSClientConfig: &tls.Config{
										//RootCAs:            rootCAs,
										//Certificates:       []tls.Certificate{clientCert},
										InsecureSkipVerify: true,
									},
								},
							}
							resp, err = client.Get("https://ama-metrics-operator-targets.kube-system.svc.cluster.local:443/scrape_configs")
							if err != nil || resp.StatusCode != http.StatusOK {
								fmt.Printf("Failed to reach Target Allocator endpoint with HTTPS after %d retries, even in insecure mode - %v\n", retries_https, err)
								fmt.Printf("Removing https config for collector\n")
								removeHttps = true
							} else {
								fmt.Printf("Target Allocator endpoint is reachable with HTTPS in insecure mode\n")
								fmt.Printf("Setting targetallocator insecure to true\n")
								setInsecure = true
							}
							//removeHttps = true
							break
						}
						fmt.Printf("Failed to reach Target Allocator endpoint with HTTPS, retrying in 30s (%d/%d) - %v\n", i+1, retries_https, err)
						time.Sleep(30 * time.Second)
					} else {
						fmt.Printf("Target Allocator endpoint is reachable with HTTPS\n")
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

	if setInsecure {
		_ = SetInsecureInCollectorConfig(collectorConfig)
	} else if removeHttps {
		// Fallback to starting without HTTPS
		_ = RemoveHTTPSSettingsInCollectorConfig(collectorConfig)
	} else {
		if err := os.Setenv("COLLECTOR_CONFIG_WITH_HTTPS", "true"); err != nil {
			fmt.Printf("Unable to set environment variable COLLECTOR_CONFIG_WITH_HTTPS - %v\n", err)
			return err
		}
	}

	return nil
}
