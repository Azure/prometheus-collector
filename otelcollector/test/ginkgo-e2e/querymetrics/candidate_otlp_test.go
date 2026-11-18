package querymetrics

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
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
	collectormetrics "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	common "go.opentelemetry.io/proto/otlp/common/v1"
	metrics "go.opentelemetry.io/proto/otlp/metrics/v1"
	"google.golang.org/protobuf/proto"

	"prometheus-collector/otelcollector/test/utils"
)

func sendCandidateOTLP(client *http.Client, endpoint, marker string, now time.Time) error {
	body, err := proto.Marshal(&collectormetrics.ExportMetricsServiceRequest{
		ResourceMetrics: []*metrics.ResourceMetrics{{
			ScopeMetrics: []*metrics.ScopeMetrics{{
				Metrics: []*metrics.Metric{{
					Name: "otlpapp.intgauge",
					Data: &metrics.Metric_Gauge{Gauge: &metrics.Gauge{
						DataPoints: []*metrics.NumberDataPoint{{
							Attributes: []*common.KeyValue{{
								Key: "scenario_run", Value: &common.AnyValue{Value: &common.AnyValue_StringValue{StringValue: marker}},
							}},
							TimeUnixNano: uint64(now.UnixNano()),
							Value:        &metrics.NumberDataPoint_AsDouble{AsDouble: 1},
						}},
					}},
				}},
			}},
		}},
	})
	if err != nil {
		return err
	}
	resp, err := client.Post(endpoint, "application/x-protobuf", bytes.NewReader(body))
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
	var result collectormetrics.ExportMetricsServiceResponse
	if err := proto.Unmarshal(response, &result); err != nil {
		return err
	}
	if rejected := result.GetPartialSuccess().GetRejectedDataPoints(); rejected != 0 {
		return fmt.Errorf("OTLP rejected data points: %d", rejected)
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
	status, response := http.StatusOK, []byte{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		var request collectormetrics.ExportMetricsServiceRequest
		if err != nil || r.Method != http.MethodPost || r.URL.Path != "/v1/metrics" ||
			r.Header.Get("Content-Type") != "application/x-protobuf" || proto.Unmarshal(body, &request) != nil {
			t.Error("invalid OTLP protobuf request")
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if len(request.ResourceMetrics) != 1 || len(request.ResourceMetrics[0].ScopeMetrics) != 1 ||
			len(request.ResourceMetrics[0].ScopeMetrics[0].Metrics) != 1 {
			t.Error("expected exactly one OTLP metric")
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		metric := request.ResourceMetrics[0].ScopeMetrics[0].Metrics[0]
		if metric.Name != "otlpapp.intgauge" || len(metric.GetGauge().GetDataPoints()) != 1 {
			t.Error("expected one otlpapp.intgauge gauge point")
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		point := metric.GetGauge().DataPoints[0]
		if point.TimeUnixNano != 1000000000 || point.GetAsDouble() != 1 || len(point.Attributes) != 1 ||
			point.Attributes[0].Key != "scenario_run" || point.Attributes[0].Value.GetStringValue() != "run123" {
			t.Errorf("invalid OTLP point: %v", point)
		}

		w.Header().Set("Content-Type", "application/x-protobuf")
		w.WriteHeader(status)
		w.Write(response)
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
	status = http.StatusOK
	response, _ = proto.Marshal(&collectormetrics.ExportMetricsServiceResponse{
		PartialSuccess: &collectormetrics.ExportMetricsPartialSuccess{RejectedDataPoints: 1},
	})
	if err := send(); err == nil {
		t.Fatal("partial rejection passed")
	}
	response = []byte{0xff}
	if err := send(); err == nil {
		t.Fatal("malformed protobuf response passed")
	}
}

func TestProbeCandidateOTLP(t *testing.T) {
	samples := `[]`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/metrics" {
			w.Header().Set("Content-Type", "application/x-protobuf")
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
