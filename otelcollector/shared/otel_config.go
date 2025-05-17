package shared

import (
	"github.com/prometheus/common/model"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type PrometheusCRConfig struct {
	Enabled                         bool                  `yaml:"enabled,omitempty"`
	AllowNamespaces                 []string              `yaml:"allow_namespaces,omitempty"`
	DenyNamespaces                  []string              `yaml:"deny_namespaces,omitempty"`
	PodMonitorSelector              *metav1.LabelSelector `yaml:"pod_monitor_selector,omitempty"`
	PodMonitorNamespaceSelector     *metav1.LabelSelector `yaml:"pod_monitor_namespace_selector,omitempty"`
	ServiceMonitorSelector          *metav1.LabelSelector `yaml:"service_monitor_selector,omitempty"`
	ServiceMonitorNamespaceSelector *metav1.LabelSelector `yaml:"service_monitor_namespace_selector,omitempty"`
	ScrapeConfigSelector            *metav1.LabelSelector `yaml:"scrape_config_selector,omitempty"`
	ScrapeConfigNamespaceSelector   *metav1.LabelSelector `yaml:"scrape_config_namespace_selector,omitempty"`
	ProbeSelector                   *metav1.LabelSelector `yaml:"probe_selector,omitempty"`
	ProbeNamespaceSelector          *metav1.LabelSelector `yaml:"probe_namespace_selector,omitempty"`
	ScrapeInterval                  model.Duration        `yaml:"scrape_interval,omitempty"`
}

type Config struct {
	CollectorSelector  *metav1.LabelSelector  `yaml:"collector_selector,omitempty"`
	Config             map[string]interface{} `yaml:"config"`
	AllocationStrategy string                 `yaml:"allocation_strategy,omitempty"`
	PrometheusCR       PrometheusCRConfig     `yaml:"prometheus_cr,omitempty"`
	FilterStrategy     string                 `yaml:"filter_strategy,omitempty"`
}

type OtelConfig struct {
	Exporters  interface{} `yaml:"exporters"`
	Processors interface{} `yaml:"processors"`
	Extensions interface{} `yaml:"extensions"`
	Receivers  struct {
		Prometheus struct {
			Config          map[string]interface{} `yaml:"config"`
			TargetAllocator map[string]interface{} `yaml:"target_allocator"`
			APIServer       interface{}            `yaml:"api_server"`
		} `yaml:"prometheus"`
	} `yaml:"receivers"`
	Service struct {
		Extensions interface{} `yaml:"extensions"`
		Pipelines  struct {
			Metrics struct {
				Exporters  interface{} `yaml:"exporters"`
				Processors interface{} `yaml:"processors"`
				Receivers  interface{} `yaml:"receivers"`
			} `yaml:"metrics"`
			MetricsTelemetry struct {
				Exporters  interface{} `yaml:"exporters,omitempty"`
				Processors interface{} `yaml:"processors,omitempty"`
				Receivers  interface{} `yaml:"receivers,omitempty"`
			} `yaml:"metrics/telemetry,omitempty"`
		} `yaml:"pipelines"`
		Telemetry struct {
			Logs struct {
				Level    interface{} `yaml:"level"`
				Encoding interface{} `yaml:"encoding"`
			} `yaml:"logs"`
		} `yaml:"telemetry"`
	} `yaml:"service"`
}
