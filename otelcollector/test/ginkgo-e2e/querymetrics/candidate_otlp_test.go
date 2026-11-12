package querymetrics

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"

	"prometheus-collector/otelcollector/test/utils"
)

func sendCandidateOTLP(client *http.Client, endpoint, marker string, now time.Time) error {
	body := fmt.Sprintf(`{"resourceMetrics":[{"scopeMetrics":[{"metrics":[{"name":"otlpapp.intgauge","gauge":{"dataPoints":[{"attributes":[{"key":"scenario_run","value":{"stringValue":%q}}],"timeUnixNano":"%d","asDouble":1}]}}]}]}]}`,
		marker, now.UnixNano())
	resp, err := client.Post(endpoint, "application/json", strings.NewReader(body))
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
			Rejected json.Number `json:"rejectedDataPoints"`
		} `json:"partialSuccess"`
	}
	if err := json.Unmarshal(response, &result); err != nil {
		return err
	}
	if result.PartialSuccess.Rejected != "" && result.PartialSuccess.Rejected != "0" {
		return fmt.Errorf("OTLP rejected data points: %s", result.PartialSuccess.Rejected)
	}
	return nil
}

func probeCandidateOTLP(client *http.Client, endpoint string, queryClient v1.API, cluster, marker string, started time.Time) error {
	if err := sendCandidateOTLP(client, endpoint, marker, time.Now()); err != nil {
		return err
	}
	selector := fmt.Sprintf(`{"otlpapp.intgauge",cluster=%q,scenario_run=%q}`, cluster, marker)
	warnings, result, err := utils.InstantQuery(queryClient, utils.FreshMetricQuery(selector, started))
	if err != nil {
		return err
	}
	samples, ok := result.(model.Vector)
	if len(warnings) != 0 || !ok || len(samples) == 0 {
		return fmt.Errorf("no fresh run-specific OTLP samples: warnings=%v", warnings)
	}
	return nil
}

var _ = Describe("Candidate OTLP ingestion", Label("candidate-otlp"), func() {
	It("ingests a fresh run-specific OTLP metric", func() {
		id := make([]byte, 16)
		_, err := rand.Read(id)
		Expect(err).NotTo(HaveOccurred())
		marker := hex.EncodeToString(id)
		started := time.Now()
		client := &http.Client{Timeout: 10 * time.Second}
		Eventually(func(g Gomega) {
			g.Expect(probeCandidateOTLP(client, "http://candidate-otlp.kube-system.svc:56681/v1/metrics",
				PrometheusQueryClient, ExpectedCluster, marker, started)).To(Succeed())
		}, 10*time.Minute, 15*time.Second).Should(Succeed())
	})
})

func TestSendCandidateOTLP(t *testing.T) {
	status, response := http.StatusOK, `{}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil || r.URL.Path != "/v1/metrics" || r.Header.Get("Content-Type") != "application/json" ||
			!json.Valid(body) || !bytes.Contains(body, []byte(`"stringValue":"run123"`)) ||
			!bytes.Contains(body, []byte(`"timeUnixNano":"1000000000"`)) {
			t.Errorf("invalid OTLP request: %s", body)
		}

		w.WriteHeader(status)
		fmt.Fprint(w, response)
	}))
	defer server.Close()
	send := func() error {
		return sendCandidateOTLP(server.Client(), server.URL+"/v1/metrics", "run123", time.Unix(1, 0))
	}
	if err := send(); err != nil {
		t.Fatal(err)
	}
	status = http.StatusBadRequest
	if err := send(); err == nil {
		t.Fatal("HTTP failure passed")
	}
	status, response = http.StatusOK, `{"partialSuccess":{"rejectedDataPoints":"1"}}`
	if err := send(); err == nil {
		t.Fatal("partial rejection passed")
	}
}

func TestProbeCandidateOTLP(t *testing.T) {
	samples := `[]`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/metrics" {
			fmt.Fprint(w, `{}`)
			return
		}
		query := r.FormValue("query")
		if !strings.Contains(query, `scenario_run="run123"`) ||
			!strings.Contains(query, `cluster="cluster"`) || !strings.Contains(query, ">= 1.000") {
			t.Errorf("query lacks run marker or start time: %s", query)
		}
		fmt.Fprintf(w, `{"status":"success","data":{"resultType":"vector","result":%s}}`, samples)
	}))
	defer server.Close()
	client, err := api.NewClient(api.Config{Address: server.URL})
	if err != nil {
		t.Fatal(err)
	}
	check := func() error {
		return probeCandidateOTLP(server.Client(), server.URL+"/v1/metrics", v1.NewAPI(client),
			"cluster", "run123", time.Unix(1, 0))
	}
	if err := check(); err == nil {
		t.Fatal("empty AMW result passed")
	}
	samples = `[{"metric":{"scenario_run":"run123"},"value":[2,"1"]}]`
	if err := check(); err != nil {
		t.Fatal(err)
	}
}
