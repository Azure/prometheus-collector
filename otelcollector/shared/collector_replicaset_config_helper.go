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

type OtelConfig struct {
	Exporters  interface{} `yaml:"exporters"`
	Processors interface{} `yaml:"processors"`
	Extensions interface{} `yaml:"extensions"`
	Receivers  struct {
		Prometheus struct {
			Config          map[string]interface{} `yaml:"config"`
			TargetAllocator map[string]interface{} `yaml:"target_allocator"`
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

func SetInsecureInCollectorConfig(configpath string) {
	configFileContents, err := os.ReadFile(configpath)
	if err != nil {
		fmt.Printf("Unable to read file contents from: %s - %v\n", configpath, err)
		return
	}
	var otelConfig OtelConfig
	err = yaml.Unmarshal([]byte(configFileContents), &otelConfig)
	if err != nil {
		fmt.Printf("Unable to unmarshal merged otel configuration from: %s - %v\n", configFileContents, err)
		return
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
		return
	}
	if err := os.WriteFile(configpath, updatedConfigYaml, 0644); err != nil {
		fmt.Printf("Unable to write updated configuration to: %s - %v\n", configpath, err)
		return
	}
	if err := os.Setenv("TARGETALLOCATOR_INSECURE", "true"); err != nil {
		fmt.Printf("Unable to set environment variable TARGETALLOCATOR_INSECURE - %v\n", err)
		return
	}
	fmt.Println("Updated configuration written to", configpath)
}

func CollectorTAHttpsCheck(collectorConfig string) {
	caCertPath := "/etc/operator-targets/certs/ca.crt"

	// Checking for file existence before proceeding.
	retries := 2
	for i := 0; i <= retries; i++ {
		if _, err := os.Stat(caCertPath); os.IsNotExist(err) {
			if i == retries {
				fmt.Printf("ca.crt file does not exist at path: %s after %d retries, exiting\n", caCertPath, retries)
				return
			}
			fmt.Printf("ca.crt file does not exist at path: %s, retrying in 30s (%d/%d)\n", caCertPath, i+1, retries)
			time.Sleep(30 * time.Second)
		} else {
			fmt.Printf("ca.crt file exists at path: %s\n", caCertPath)
			return
		}
	}

	// Checking for HTTPS connection with retries
	setInsecure := false
	retries_https := 2
	for i := 0; i <= retries_https; i++ {
		certPEM, err := ioutil.ReadFile(caCertPath)
		if err != nil {
			fmt.Printf("Failed to read CA cert file from path: %s - (%d/%d): %v\n", caCertPath, i+1, retries_https)
			setInsecure = true
			break
		} else {
			// Create a new cert pool
			rootCAs := x509.NewCertPool()
			// Append CA cert to the new pool
			if ok := rootCAs.AppendCertsFromPEM(certPEM); !ok {
				fmt.Printf("Failed to append %s to RootCAs- (%d/%d): %v\n", caCertPath, i+1, retries_https, err)
				setInsecure = true
				break
			} else {
				fmt.Printf("[%s] Pinging Target Allocator endpoint with HTTPS\n", time.Now().Format(time.RFC3339))
				client := &http.Client{
					Transport: &http.Transport{
						TLSClientConfig: &tls.Config{
							RootCAs: rootCAs,
						},
					},
				}
				resp, err := client.Get("https://ama-metrics-operator-targets.kube-system.svc.cluster.local:443/scrape_configs")
				if err != nil || resp.StatusCode != http.StatusOK {
					if i == retries_https {
						fmt.Printf("Failed to reach Target Allocator endpoint with HTTPS after %d retries, exiting - %v\n", retries_https, err)
						setInsecure = true
						return
					}
					fmt.Printf("Failed to reach Target Allocator endpoint with HTTPS, retrying in 30s (%d/%d) - %v\n", i+1, retries_https, err)
					time.Sleep(30 * time.Second)
				} else {
					fmt.Printf("Target Allocator endpoint is reachable with HTTPS\n")
					setInsecure = false
					return
				}
			}
		}
	}

	if setInsecure {
		// Fallback to starting without HTTPS
		SetInsecureInCollectorConfig(collectorConfig)
	}

	// certPEM, err := ioutil.ReadFile(caCertPath)
	// // fmt.Printf("Contents of CA cert file: %s\n", string(certPEM))
	// if err != nil {
	// 	fmt.Printf("Failed to read CA cert file from path: %s\n", caCertPath)
	// 	// Fallback to start the collector without TLS
	// 	SetInsecureInCollectorConfig(collectorConfig)
	// } else {
	// 	// Create a new cert pool
	// 	rootCAs := x509.NewCertPool()
	// 	// Append CA cert to the new pool
	// 	if ok := rootCAs.AppendCertsFromPEM(certPEM); !ok {
	// 		fmt.Printf("Failed to append %q to RootCAs: %v\n", caCertPath, err)
	// 		// Fallback to starting without HTTPS
	// 		SetInsecureInCollectorConfig(collectorConfig)
	// 	} else {
	// 		fmt.Printf("[%s] Pinging Target Allocator endpoint with HTTPS\n", time.Now().Format(time.RFC3339))
	// 		client := &http.Client{
	// 			Transport: &http.Transport{
	// 				TLSClientConfig: &tls.Config{
	// 					RootCAs: rootCAs,
	// 				},
	// 			},
	// 		}
	// 		resp, err := client.Get("https://ama-metrics-operator-targets.kube-system.svc.cluster.local:443/scrape_configs")
	// 		if err != nil || resp.StatusCode != http.StatusOK {
	// 			fmt.Printf("Failed to reach Target Allocator endpoint with HTTPS, retrying: %v\n", err)
	// 			resp, err = client.Get("https://ama-metrics-operator-targets.kube-system.svc.cluster.local:443/scrape_configs")
	// 			if err != nil || resp.StatusCode != http.StatusOK {
	// 				fmt.Printf("Failed to reach Target Allocator endpoint with HTTPS, after retry, setting insecure: %v\n", err)
	// 				// Fallback to start the collector without HTTPS
	// 				SetInsecureInCollectorConfig(collectorConfig)
	// 			}
	// 		} else {
	// 			fmt.Printf("Target Allocator endpoint is reachable with HTTPS\n")
	// 		}

	// 	}
	// }
}
