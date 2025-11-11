package utils

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
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

// GetAzureCredential returns an Azure credential, preferring federated identity when available.
func GetAzureCredential() (azcore.TokenCredential, error) {
	cred, err := newFederatedCredential()
	if err != nil {
		fmt.Printf("failed to initialize federated credential, falling back to default: %v\n", err)
	} else if cred != nil {
		fmt.Printf("received federated credential\n")
		return cred, nil
	}

	defaultCred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create default azure credential: %w", err)
	}
	fmt.Printf("received default credential\n")

	return defaultCred, nil
}

func newFederatedCredential() (azcore.TokenCredential, error) {
	clientID := os.Getenv("FED_CLIENT_ID")
	tenantID := os.Getenv("TENANT_ID")

	if clientID != "" && tenantID != "" {
		cred, err := azidentity.NewClientAssertionCredential(tenantID, clientID, requestOIDCToken, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create federated client assertion credential: %w", err)
		}

		return cred, nil
	}

	return nil, fmt.Errorf("failed to get federated creds as either clientID or tenantID is empty")
}

func requestOIDCToken(ctx context.Context) (string, error) {
	systemAccessToken := os.Getenv("SYSTEM_ACCESSTOKEN")
	serviceConnectionID := os.Getenv("SERVICE_CONNECTION_ID")
	oidcRequestURI := os.Getenv("SYSTEM_OIDCREQUESTURI")

	if systemAccessToken == "" || serviceConnectionID == "" || oidcRequestURI == "" {
		return "", errors.New("missing OIDC environment variables for federated identity flow")
	}

	requestURL := fmt.Sprintf("%s?api-version=7.1&serviceConnectionId=%s", oidcRequestURI, serviceConnectionID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, requestURL, http.NoBody)
	if err != nil {
		return "", fmt.Errorf("failed to build OIDC token request: %w", err)
	}

	req.Header.Set("Content-Length", "0")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", systemAccessToken))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to request federated token: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read federated token response: %w", err)
	}

	if resp.StatusCode >= http.StatusBadRequest {
		return "", fmt.Errorf("federated token request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var payload struct {
		OIDCToken string `json:"oidcToken"`
	}

	if err := json.Unmarshal(body, &payload); err != nil {
		return "", fmt.Errorf("failed to decode federated token response: %w", err)
	}

	token := strings.TrimSpace(payload.OIDCToken)
	if token == "" {
		return "", errors.New("received empty federated token from OIDC endpoint")
	}

	return token, nil
}

/*
 * Get the access token to the AMW query API
 */
func GetQueryAccessToken() (string, error) {
	cred, err := GetAzureCredential()
	if err != nil {
		return "", fmt.Errorf("failed to create identity credential: %w", err)
	}

	opts := policy.TokenRequestOptions{
		Scopes: []string{"https://prometheus.monitor.azure.com/.default"},
	}

	accessToken, err := cred.GetToken(context.Background(), opts)
	if err != nil {
		return "", fmt.Errorf("failed to get access token: %w", err)
	}

	if accessToken.Token == "" {
		return "", errors.New("federated identity credential returned an empty access token")
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
		return nil, fmt.Errorf("failed to get query access token: %w", err)
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
