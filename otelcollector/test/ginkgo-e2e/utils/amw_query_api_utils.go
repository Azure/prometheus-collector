package utils

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	. "github.com/onsi/gomega"
	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"
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
	result, warnings, _, err := api.Query(context.Background(), query, time.Now())
	if err != nil {
		return warnings, nil, fmt.Errorf("Failed to run query: %s", err.Error())
	}

	return warnings, result, nil
}

func MetricQuery(metric, job, cluster string) string {
	return fmt.Sprintf("%s{job=%q,cluster=%q}", metric, job, cluster)
}

func FreshSamples(g Gomega, api v1.API, selector string, started time.Time) model.Vector {
	warnings, result, err := InstantQuery(api, FreshMetricQuery(selector, started))
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(warnings).To(BeEmpty())
	samples, ok := result.(model.Vector)
	g.Expect(ok).To(BeTrue())
	return samples
}

func CheckMinimalProfile(g Gomega, api v1.API, query, metric string) {
	warnings, result, err := InstantQuery(api, query)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(warnings).To(BeEmpty())
	samples, ok := result.(model.Vector)
	g.Expect(ok).To(BeTrue())
	g.Expect(samples).To(BeEmpty(), "non-minimal metric %s was ingested with MIP enabled", metric)
}

func HasOTLPContext(cluster, runID string) bool {
	return cluster != "" && runID != ""
}

// SendOTLP posts one gauge point using the OTLP/HTTP JSON encoding.
func SendOTLP(client *http.Client, endpoint, marker string, now time.Time) error {
	point := map[string]any{
		"attributes":   []any{map[string]any{"key": "scenario_run", "value": map[string]any{"stringValue": marker}}},
		"timeUnixNano": fmt.Sprint(now.UnixNano()),
		"asDouble":     1,
	}
	metric := map[string]any{"name": "otlpapp.intgauge", "gauge": map[string]any{"dataPoints": []any{point}}}
	body, err := json.Marshal(map[string]any{"resourceMetrics": []any{map[string]any{
		"scopeMetrics": []any{map[string]any{"metrics": []any{metric}}},
	}}})
	if err != nil {
		return err
	}
	resp, err := client.Post(endpoint, "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	response, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("OTLP HTTP status %d: %s", resp.StatusCode, response)
	}
	var result struct {
		PartialSuccess struct {
			RejectedDataPoints json.Number `json:"rejectedDataPoints"`
		} `json:"partialSuccess"`
	}
	if err := json.Unmarshal(response, &result); err != nil {
		return err
	}
	if rejected := result.PartialSuccess.RejectedDataPoints; rejected != "" && rejected != "0" {
		return fmt.Errorf("OTLP rejected data points: %s", rejected)
	}
	return nil
}

func ProbeOTLP(client *http.Client, endpoint string, queryClient v1.API, cluster, marker string, started time.Time) error {
	if err := SendOTLP(client, endpoint, marker, time.Now()); err != nil {
		return err
	}
	selector := fmt.Sprintf(`{"otlpapp.intgauge",cluster=%q,scenario_run=%q}`, cluster, marker)
	warnings, result, err := InstantQuery(queryClient, FreshMetricQuery(selector, started))
	if err != nil {
		return err
	}
	samples, ok := result.(model.Vector)
	if len(warnings) != 0 || !ok || len(samples) == 0 {
		return fmt.Errorf("no fresh run-specific OTLP samples: warnings=%v", warnings)
	}
	return nil
}

func TransitionAPI() v1.API {
	endpoint := os.Getenv("AMW_QUERY_ENDPOINT")
	Expect(endpoint).NotTo(BeEmpty(), "selected workflow requires the existing AMW query endpoint")
	api, err := CreatePrometheusAPIClient(endpoint)
	Expect(err).NotTo(HaveOccurred())
	return api
}

func FreshTransitionSamples(api v1.API, selector string, after time.Time) (model.Vector, error) {
	warnings, result, err := InstantQuery(api, FreshMetricQuery(selector, after))
	if err != nil {
		return nil, err
	}
	if len(warnings) != 0 {
		return nil, fmt.Errorf("AMW returned query warnings: %v", warnings)
	}
	vector, ok := result.(model.Vector)
	if !ok {
		return nil, fmt.Errorf("AMW query did not return an instant vector")
	}
	return vector, nil
}

func ExpectTransitionMetric(api v1.API, metric, job, cluster, node string, after time.Time) {
	selector := fmt.Sprintf("%s{job=%q,cluster=%q}", metric, job, cluster)
	if node != "" {
		selector = fmt.Sprintf("%s{job=%q,cluster=%q,instance=%q}", metric, job, cluster, node)
	}
	Eventually(func(g Gomega) {
		samples, err := FreshTransitionSamples(api, selector, after)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(ValidateMetricSamples(samples, metric, job, cluster, time.Now(), metric == "up")).To(Succeed())
	}, 8*time.Minute, 15*time.Second).Should(Succeed())
}

func ExpectWindowsKubeProxy(api v1.API, cluster string, after time.Time) {
	pods, err := GetPodsWithLabel(K8sClient, "kube-system", "dsName", "ama-metrics-win-node")
	Expect(err).NotTo(HaveOccurred(), "actual Windows collectors are required; do not substitute Linux metrics")
	const metric = "kubeproxy_sync_proxy_rules_duration_seconds_count"
	present := regexp.MustCompile(`(?m)^` + metric + `(?:\{|[ \t])`)
	for _, pod := range pods {
		Expect(pod.Spec.NodeName).NotTo(BeEmpty())
		Expect(pod.Status.HostIP).NotTo(BeEmpty())
		url := "http://" + pod.Status.HostIP + ":10249/metrics"
		command := []string{"powershell", "-NoProfile", "-Command",
			fmt.Sprintf("(Invoke-WebRequest -UseBasicParsing -TimeoutSec 30 '%s').Content", url)}
		out, _, err := ExecCmd(K8sClient, Cfg, pod.Name, "prometheus-collector", "kube-system", command)
		Expect(err).NotTo(HaveOccurred(), "unsupported/unreachable Windows kube-proxy metrics endpoint %s", url)
		Expect(present.MatchString(out)).To(BeTrue(), "unsupported Windows kube-proxy metric %s on node %s", metric, pod.Spec.NodeName)
		ExpectTransitionMetric(api, "up", "kube-proxy-windows", cluster, pod.Spec.NodeName, after)
		ExpectTransitionMetric(api, metric, "kube-proxy-windows", cluster, pod.Spec.NodeName, after)
	}
}
