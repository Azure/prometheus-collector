package utils

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
)

func GetScopeFromEndpoint(amwQueryEndpoint string) (string, error) {
	u, err := url.Parse(amwQueryEndpoint)
	if err != nil {
		return "", fmt.Errorf("invalid AMW_QUERY_ENDPOINT: %w", err)
	}

	// Get the host (e.g., "aksdemocluster-amw-aedfdtdva5erevau.westus2.prometheus.monitor.azure.com")
	host := u.Host

	// Find "prometheus.monitor" and everything after it
	idx := strings.Index(host, "prometheus.monitor")
	if idx == -1 {
		return "", fmt.Errorf("invalid prometheus endpoint: missing 'prometheus.monitor' in host")
	}

	// Extract from "prometheus.monitor" onwards
	baseDomain := host[idx:]

	// Build the scope
	scope := fmt.Sprintf("https://%s/.default", baseDomain)
	return scope, nil
}

/*
 * Create a Prometheus API client to use with the Managed Prometheus AMW Query API.
 */
//TODO: Pass Token and RoundTripper into function??
func CreatePromApiManagedClient(amwQueryEndpoint string) (v1.API, error) {
	scope, err := GetScopeFromEndpoint(amwQueryEndpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to get scope from endpoint: %s", err.Error())
	}

	token, err := GetDefaultQueryAccessToken(scope)
	if err != nil {
		return nil, fmt.Errorf("failed to get query access token: %s", err.Error())
	}
	if token == "" {
		return nil, fmt.Errorf("failed to get query access token: token is empty")
	}

	// Use the secure transport instead of the basic one
	secureTransport := CreateSecureTransport(token)

	config := api.Config{
		Address:      amwQueryEndpoint,
		RoundTripper: secureTransport,
	}

	prometheusAPIClient, err := api.NewClient(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create Prometheus API client: %s", err.Error())
	}
	return v1.NewAPI(prometheusAPIClient), nil
}
