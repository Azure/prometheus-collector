//go:build linux

package shared

import (
	"encoding/base64"
	"os"
	"testing"
)

func TestSetupTelemetryDelos(t *testing.T) {
	const key = "delos-test-key"
	encodedKey := base64.StdEncoding.EncodeToString([]byte(key))
	for _, cloud := range []string{"azuredeloscloud", "AzureDelosCloud"} {
		t.Run(cloud, func(t *testing.T) {
			t.Setenv("APPLICATIONINSIGHTS_AUTH_DELOS", encodedKey)
			t.Setenv("APPLICATIONINSIGHTS_AUTH_PUBLIC", base64.StdEncoding.EncodeToString([]byte("public-test-key")))
			t.Setenv("APPLICATIONINSIGHTS_AUTH", "")
			t.Setenv("APPLICATIONINSIGHTS_ENDPOINT", "")
			t.Setenv("TELEMETRY_APPLICATIONINSIGHTS_KEY", "")

			SetupTelemetry(cloud)

			for name, want := range map[string]string{
				"APPLICATIONINSIGHTS_AUTH":          encodedKey,
				"APPLICATIONINSIGHTS_ENDPOINT":      "https://deloscloudgermanycentral-0.in.applicationinsights.sovcloud-api.de/v2/track",
				"TELEMETRY_APPLICATIONINSIGHTS_KEY": key,
			} {
				if got := os.Getenv(name); got != want {
					t.Errorf("%s = %q, want %q", name, got, want)
				}
			}
		})
	}
}
