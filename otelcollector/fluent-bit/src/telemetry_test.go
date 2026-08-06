package main

import (
	"testing"
)

func TestScrapeJobMetadataBearerTokenFileMonitorTypes(t *testing.T) {
	scrapeJobs := map[string]interface{}{
		"serviceMonitor/test/service/0": map[string]interface{}{
			"authorization": map[string]interface{}{
				"type":             "Bearer",
				"credentials_file": "/var/run/secrets/service-token",
			},
		},
		"podMonitor/test/pod/0": map[string]interface{}{
			"authorization": map[string]interface{}{
				"type":             "Bearer",
				"credentials_file": "/var/run/secrets/pod-token",
			},
		},
	}

	metadata := scrapeJobMetadata(scrapeJobs)

	assertTelemetryProperty(t, metadata, bearerTokenEnabledWithFile, "true")
	assertTelemetryProperty(t, metadata, serviceMonitorBearerTokenFileEnabled, "true")
	assertTelemetryProperty(t, metadata, podMonitorBearerTokenFileEnabled, "true")
	assertTelemetryProperty(t, metadata, serviceMonitorBearerTokenEnabled, "true")
	assertTelemetryProperty(t, metadata, podMonitorBearerTokenEnabled, "true")
}

func TestScrapeJobMetadataDoesNotAttributeNonMonitorBearerTokenFile(t *testing.T) {
	scrapeJobs := map[string]interface{}{
		"custom-scrape-job": map[string]interface{}{
			"authorization": map[string]interface{}{
				"type":             "Bearer",
				"credentials_file": "/var/run/secrets/custom-token",
			},
		},
	}

	metadata := scrapeJobMetadata(scrapeJobs)

	assertTelemetryProperty(t, metadata, bearerTokenEnabledWithFile, "true")
	assertTelemetryProperty(t, metadata, serviceMonitorBearerTokenFileEnabled, "false")
	assertTelemetryProperty(t, metadata, podMonitorBearerTokenFileEnabled, "false")
	assertTelemetryProperty(t, metadata, serviceMonitorBearerTokenEnabled, "false")
	assertTelemetryProperty(t, metadata, podMonitorBearerTokenEnabled, "false")
}

func TestScrapeJobMetadataBearerTokenSecretIsNotFileUsage(t *testing.T) {
	scrapeJobs := map[string]interface{}{
		"serviceMonitor/test/service/0": map[string]interface{}{
			"authorization": map[string]interface{}{
				"type":        "Bearer",
				"credentials": "token",
			},
		},
	}

	metadata := scrapeJobMetadata(scrapeJobs)

	assertTelemetryProperty(t, metadata, bearerTokenEnabledWithSecret, "true")
	assertTelemetryProperty(t, metadata, bearerTokenEnabledWithFile, "false")
	assertTelemetryProperty(t, metadata, serviceMonitorBearerTokenFileEnabled, "false")
	assertTelemetryProperty(t, metadata, serviceMonitorBearerTokenEnabled, "true")
}

func TestScrapeJobMetadataBasicAuthMonitorTypes(t *testing.T) {
	scrapeJobs := map[string]interface{}{
		"serviceMonitor/test/service/0": map[string]interface{}{
			"basic_auth": map[string]interface{}{
				"username": "service-user",
				"password": "service-password",
			},
		},
		"podMonitor/test/pod/0": map[string]interface{}{
			"basic_auth": map[string]interface{}{
				"username": "pod-user",
				"password": "pod-password",
			},
		},
	}

	metadata := scrapeJobMetadata(scrapeJobs)

	assertTelemetryProperty(t, metadata, basicAuthEnabled, "true")
	assertTelemetryProperty(t, metadata, serviceMonitorBasicAuthEnabled, "true")
	assertTelemetryProperty(t, metadata, podMonitorBasicAuthEnabled, "true")
}

func assertTelemetryProperty(t *testing.T, properties map[string]string, name, expected string) {
	t.Helper()
	if actual := properties[name]; actual != expected {
		t.Fatalf("property %q = %q, want %q", name, actual, expected)
	}
}
