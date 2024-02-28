package utils

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"

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
func GetQueryAccessToken(clientID, clientSecret string) (string, error) {
	if clientID == "" || clientSecret == "" {
		return "", fmt.Errorf("Client ID or Client Secret is empty")
	}

	apiUrl := "https://login.microsoftonline.com/72f988bf-86f1-41af-91ab-2d7cd011db47/oauth2/token"
	data := url.Values{}
	data.Set("grant_type", "client_credentials")
	data.Set("client_id", clientID)
	data.Set("client_secret", clientSecret)
	data.Set("resource", "https://prometheus.monitor.azure.com")

	client := &http.Client{}
	r, err := http.NewRequest(http.MethodPost, apiUrl, strings.NewReader(data.Encode()))
	if err != nil {
		return "", fmt.Errorf("Failed create request for authorization token: %s", err.Error())
	}
	r.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	resp, err := client.Do(r)
	if err != nil {
		return "", fmt.Errorf("Failed to request authorization token: %s", err.Error())
	}
	defer resp.Body.Close()
  body, err := ioutil.ReadAll(resp.Body)
  if err != nil {
		return "", fmt.Errorf("Failed to read body of auth token response: %s", err.Error())
	}

	if resp.StatusCode != http.StatusOK {
    return "", fmt.Errorf("Request for token returned status code: %s. Error Message: %s\n", resp.StatusCode, string(body))
	}

	var tokenResponse TokenResponse
	err = json.Unmarshal([]byte(body), &tokenResponse)
	if err != nil {
		return "", fmt.Errorf("Failed to unmarshal the token response: %s", err.Error())
	}

	return tokenResponse.AccessToken, nil
}

/*
 * The custom Prometheus API transport with the bearer token.
 */
type transport struct {
	underlyingTransport http.RoundTripper
	apiToken string
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
func CreatePrometheusAPIClient(amwQueryEndpoint, clientId, clientSecret string) (v1.API, error) {
	token, err := GetQueryAccessToken(clientId, clientSecret)
	if err != nil {
		return nil, fmt.Errorf("Failed to get query access token: %s", err.Error())
	}
	if token == "" {
		return nil, fmt.Errorf("Failed to get query access token: token is empty")
	}
	config := api.Config{
		Address: amwQueryEndpoint,
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
func InstantQuery(api v1.API, query string) (v1.Warnings, error) {
	result, warnings, err := api.Query(context.Background(), query, time.Now())
	if err != nil {
		return warnings, fmt.Errorf("Failed to run query: %s", err.Error())
	}	
	for _, sample := range result.(model.Vector) {
		fmt.Printf("Metric: %s\n", sample.Metric)
		fmt.Printf("Metric Name: %s\n", sample.Metric["__name__"])
		fmt.Printf("Cluster: %s\n", sample.Metric["cluster"])
		fmt.Printf("Job: %s\n", sample.Metric["job"])
		fmt.Printf("Instance: %s\n", sample.Metric["instance"])
		fmt.Printf("external_label_1: %s\n", sample.Metric["external_label_1"])
		fmt.Printf("external_label_123: %s\n", sample.Metric["external_label_123"])
		fmt.Printf("Value: %s\n", sample.Value)
		fmt.Printf("Timestamp: %s\n", sample.Timestamp)
		fmt.Printf("Histogram: %s\n", sample.Histogram)
	}

	return warnings, nil
}
