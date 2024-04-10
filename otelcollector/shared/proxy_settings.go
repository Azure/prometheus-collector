package shared

import (
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func copyCAAnchors() error {
	// Copy CA anchors from specified locations
	locations := []string{"/anchors/ubuntu/*", "/anchors/mariner/*", "/anchors/proxy/*"}
	for _, loc := range locations {
		cmd := exec.Command("cp", loc, "/etc/pki/ca-trust/source/anchors")
		cmd.Stderr = os.Stderr
		err := cmd.Run()
		if err != nil {
			return fmt.Errorf("error copying CA anchors: %w", err)
		}
	}

	// Update CA trust
	cmd := exec.Command("update-ca-trust")
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("error updating CA trust: %w", err)
	}

	return nil
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
	os.Setenv("NO_PROXY", noProxy)
	os.Setenv("no_proxy", noProxy)
}

func setHTTPProxyEnabled() {
	httpProxyEnabled := "false"
	if os.Getenv("HTTP_PROXY") != "" {
		httpProxyEnabled = "true"
	}
	os.Setenv("HTTP_PROXY_ENABLED", httpProxyEnabled)
}

func configureEnvironment() error {
	if err := copyCAAnchors(); err != nil {
		return err
	}

	// Remove trailing '/' character from HTTP_PROXY and HTTPS_PROXY
	proxyVariables := []string{"http_proxy", "HTTP_PROXY", "https_proxy", "HTTPS_PROXY"}
	for _, v := range proxyVariables {
		os.Setenv(v, removeTrailingSlash(os.Getenv(v)))
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

		os.Setenv("MDSD_PROXY_MODE", "application")
		os.Setenv("MDSD_PROXY_ADDRESS", os.Getenv("HTTPS_PROXY"))
		if user := strings.SplitN(urlParts[0], ":", 2)[0]; user != "" {
			os.Setenv("MDSD_PROXY_USERNAME", user)
			os.Setenv("MDSD_PROXY_PASSWORD_FILE", "/opt/microsoft/proxy_password")
		}
	}

	return nil
}
