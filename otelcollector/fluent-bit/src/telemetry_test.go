package main

import (
	"encoding/base64"
	"testing"
)

func TestInitializeTelemetryClientDenyFSAccessThroughSMs(t *testing.T) {
	originalClient, originalProperties := TelemetryClient, CommonProperties
	t.Cleanup(func() {
		TelemetryClient, CommonProperties = originalClient, originalProperties
	})
	t.Setenv(envAppInsightsAuth, base64.StdEncoding.EncodeToString([]byte("00000000-0000-0000-0000-000000000000")))
	t.Setenv(envTelemetryOffSwitch, "true")
	t.Setenv("MAC", "false")

	for _, tc := range []struct {
		name string
		env  string
		want string
	}{
		{"default", "", "false"},
		{"disabled", "false", "false"},
		{"enabled", "true", "true"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("AZMON_DENY_FS_ACCESS_THROUGH_SMS", tc.env)
			if status, err := InitializeTelemetryClient(""); err != nil || status != 0 {
				t.Fatalf("InitializeTelemetryClient() = %d, %v", status, err)
			}
			assertTelemetryProperty(t, TelemetryClient.Context().CommonProperties, "denyFSAccessThroughSMs", tc.want)
		})
	}
}

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
