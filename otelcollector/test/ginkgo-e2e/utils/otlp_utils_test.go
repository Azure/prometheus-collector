package utils_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"

	"prometheus-collector/otelcollector/test/utils"
)

func TestOTLPContext(t *testing.T) {
	for _, test := range []struct {
		name, cluster, runID string
		want                 bool
	}{
		{"missing run ID", "cluster", "", false},
		{"missing cluster", "", "run-id", false},
		{"configured", "cluster", "run-id", true},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := utils.HasOTLPContext(test.cluster, test.runID); got != test.want {
				t.Fatalf("OTLP context = %t, want %t", got, test.want)
			}
		})
	}
}

func TestSendOTLP(t *testing.T) {
	status, response := http.StatusOK, []byte(`{}`)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		var request struct {
			ResourceMetrics []struct {
				ScopeMetrics []struct {
					Metrics []struct {
						Name  string
						Gauge struct {
							DataPoints []struct {
								TimeUnixNano string
								AsDouble     float64
								Attributes   []struct {
									Key   string
									Value struct{ StringValue string }
								}
							}
						}
					}
				}
			}
		}
		if err != nil || r.Method != http.MethodPost || r.URL.Path != "/v1/metrics" ||
			r.Header.Get("Content-Type") != "application/json" || json.Unmarshal(body, &request) != nil {
			t.Error("invalid OTLP JSON request")
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
		if metric.Name != "otlpapp.intgauge" || len(metric.Gauge.DataPoints) != 1 {
			t.Error("expected one otlpapp.intgauge gauge point")
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		point := metric.Gauge.DataPoints[0]
		if point.TimeUnixNano != "1000000000" || point.AsDouble != 1 || len(point.Attributes) != 1 ||
			point.Attributes[0].Key != "scenario_run" || point.Attributes[0].Value.StringValue != "run123" {
			t.Errorf("invalid OTLP point: %+v", point)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		w.Write(response)
	}))
	defer server.Close()
	send := func() error {
		return utils.SendOTLP(server.Client(), server.URL+"/v1/metrics", "run123", time.Unix(1, 0))
	}
	if err := send(); err != nil {
		t.Fatal(err)
	}
	status = http.StatusBadRequest
	if err := send(); err == nil {
		t.Fatal("HTTP failure passed")
	}
	status = http.StatusOK
	response = []byte(`{"partialSuccess":{"rejectedDataPoints":"1"}}`)
	if err := send(); err == nil {
		t.Fatal("partial rejection passed")
	}
	response = []byte(`{`)
	if err := send(); err == nil {
		t.Fatal("malformed JSON response passed")
	}
}

func TestProbeOTLP(t *testing.T) {
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
		return utils.ProbeOTLP(server.Client(), server.URL+"/v1/metrics", v1.NewAPI(client),
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
