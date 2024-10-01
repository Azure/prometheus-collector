package shared

import (
	"encoding/base64"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func copyCAAnchors() {
	// Copy CA anchors from specified locations
	locations := []string{"/anchors/ubuntu/*", "/anchors/mariner/*", "/anchors/proxy/*"}
	for _, loc := range locations {
		matches, err := filepath.Glob(loc)
		if err != nil {
			log.Printf("Error matching pattern %s: %v", loc, err)
			continue
		}
		for _, match := range matches {
			if _, err := os.Stat(match); err == nil {
				cmd := exec.Command("cp", match, "/etc/pki/ca-trust/source/anchors")
				cmd.Stderr = os.Stderr
				if err := cmd.Run(); err != nil {
					log.Printf("Warning copying %s: %v", match, err)
				}
			} else if os.IsNotExist(err) {
				log.Printf("File %s does not exist", match)
			} else {
				log.Printf("Error checking file %s: %v", match, err)
			}
		}
	}

	// Update CA trust
	cmd := exec.Command("update-ca-trust")
	cmd.Stderr = os.Stderr
	cmd.Run()
}

func removeTrailingSlash(envVar string) string {
	if envVar != "" && strings.HasSuffix(envVar, "/") {
		return envVar[:len(envVar)-1]
	}
	return envVar
}

func addNoProxy(target string) {
	noProxy := os.Getenv("NO_PROXY")
	noProxy = strings.TrimSpace(noProxy)
	noProxy += "," + target
	SetEnvAndSourceBashrc("NO_PROXY", noProxy, true)
	SetEnvAndSourceBashrc("no_proxy", noProxy, true)
}

func setHTTPProxyEnabled() {
	httpProxyEnabled := "false"
	if os.Getenv("HTTP_PROXY") != "" {
		httpProxyEnabled = "true"
	}
	SetEnvAndSourceBashrc("HTTP_PROXY_ENABLED", httpProxyEnabled, true)
}

func ConfigureEnvironment() error {
	copyCAAnchors()

	// Remove trailing '/' character from HTTP_PROXY and HTTPS_PROXY
	proxyVariables := []string{"http_proxy", "HTTP_PROXY", "https_proxy", "HTTPS_PROXY"}
	for _, v := range proxyVariables {
		SetEnvAndSourceBashrc(v, removeTrailingSlash(os.Getenv(v)), true)
	}

	addNoProxy("ama-metrics-operator-targets.kube-system.svc.cluster.local")
	setHTTPProxyEnabled()

	// Process additional settings for Arc cluster with enabled HTTP proxy
	if os.Getenv("IS_ARC_CLUSTER") == "true" && os.Getenv("HTTP_PROXY_ENABLED") == "true" {
		proxyProtocol := strings.ToLower(strings.Split(os.Getenv("HTTPS_PROXY"), "://")[0])
		if proxyProtocol != "http" && proxyProtocol != "https" {
			fmt.Println("HTTP Proxy specified does not include http:// or https://")
		}

		urlParts := strings.SplitN(strings.TrimPrefix(os.Getenv("HTTPS_PROXY"), proxyProtocol+"://"), "@", 2)
		hostPort := urlParts[len(urlParts)-1]
		host := strings.Split(hostPort, "/")[0]
		if host == "" {
			fmt.Println("HTTP Proxy specified does not include a host")
		}

		password := base64.StdEncoding.EncodeToString([]byte(strings.SplitN(urlParts[0], ":", 2)[1]))
		os.WriteFile("/opt/microsoft/proxy_password", []byte(password), 0644)

		SetEnvAndSourceBashrc("MDSD_PROXY_MODE", "application", true)
		SetEnvAndSourceBashrc("MDSD_PROXY_ADDRESS", os.Getenv("HTTPS_PROXY"), true)
		if user := strings.SplitN(urlParts[0], ":", 2)[0]; user != "" {
			SetEnvAndSourceBashrc("MDSD_PROXY_USERNAME", user, true)
			SetEnvAndSourceBashrc("MDSD_PROXY_PASSWORD_FILE", "/opt/microsoft/proxy_password", true)
		}
	}

	return nil
}
