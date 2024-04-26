package utils

import (
	"context"
	"net/http"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"

	"fmt"
)

/*
 * The format of the response from getting the access token.
 */
type TokenResponse struct {
	TokenType    string `json:"token_type"`
	ExpiresIn    string `json:"expires_in"`
	ExtExpiresIn string `json:"ext_expires_in"`
	ExpiresOn    string `json:"expires_on"`
	NotBefore    string `json:"not_before"`
	Resource     string `json:"resource"`
	AccessToken  string `json:"access_token"`
}

/*
 * Get the access token to the AMW query API
 */
func GetQueryAccessToken() (string, error) {
	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		return "", fmt.Errorf("Failed to create identity credential: %s", err.Error())
	}

	opts := policy.TokenRequestOptions{
		Scopes: []string{"https://prometheus.monitor.azure.com"},
	}

	accessToken, err := cred.GetToken(context.Background(), opts)
	if err != nil {
		return "", fmt.Errorf("failed to get accesstoken: %s", err.Error())
	}

	return accessToken.Token, nil
}

/*
 * The custom Prometheus API transport with the bearer token.
 */
type transport struct {
	underlyingTransport http.RoundTripper
	apiToken            string
}

/*
 * The custom RoundTrip with the bearer token added to the request header.
 */
func (t *transport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", t.apiToken))
	return t.underlyingTransport.RoundTrip(req)
}

/*
 * Create a Prometheus API client to use with the Managed Prometheus AMW Query API.
 */
func CreatePrometheusAPIClient(amwQueryEndpoint string) (v1.API, error) {
	token, err := GetQueryAccessToken()
	if err != nil {
		return nil, fmt.Errorf("Failed to get query access token: %s", err.Error())
	}
	if token == "" {
		return nil, fmt.Errorf("Failed to get query access token: token is empty")
	}
	config := api.Config{
		Address:      amwQueryEndpoint,
		RoundTripper: &transport{underlyingTransport: http.DefaultTransport, apiToken: token},
	}
	prometheusAPIClient, err := api.NewClient(config)
	if err != nil {
		return nil, fmt.Errorf("Failed to create Prometheus API client: %s", err.Error())
	}
	return v1.NewAPI(prometheusAPIClient), nil
}

/*
 * Example parsing of the instant query response.
 */
func InstantQuery(api v1.API, query string) (v1.Warnings, interface{}, error) {
	result, warnings, err := api.Query(context.Background(), query, time.Now())
	if err != nil {
		return warnings, nil, fmt.Errorf("Failed to run query: %s", err.Error())
	}

	return warnings, result, nil
}
