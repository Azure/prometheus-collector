package shared

import "testing"

func TestValidatePrometheusConfigProviders(t *testing.T) {
	tests := []struct {
		name string
		uri  string
		raw  any
		want bool
	}{
		{
			name: "rejects file provider in prometheus receiver",
			uri:  "file:/config.yaml",
			raw: map[string]any{
				"receivers": map[string]any{
					"prometheus": map[string]any{"config": "${file:/var/run/secrets/token}"},
				},
			},
			want: true,
		},
		{
			name: "rejects arbitrary env provider in named prometheus receiver",
			uri:  "file:/config.yaml",
			raw: map[string]any{
				"receivers": map[string]any{
					"prometheus/custom": map[string]any{"config": "${env:SECRET}"},
				},
			},
			want: true,
		},
		{
			name: "rejects provider replacing receiver tree",
			uri:  "file:/config.yaml",
			raw:  map[string]any{"receivers": "${file:/tenant-receivers.yaml}"},
			want: true,
		},
		{
			name: "rejects provider in flattened config key",
			uri:  "file:/config.yaml",
			raw:  map[string]any{"receivers::prometheus::config::scrape_configs": "${env:SECRET}"},
			want: true,
		},
		{
			name: "allows node variables and relabel references",
			uri:  "file:/config.yaml",
			raw: map[string]any{
				"receivers": map[string]any{
					"prometheus": map[string]any{
						"config": map[string]any{
							"scrape_configs": []any{
								map[string]any{
									"relabel_configs": []any{
										map[string]any{"replacement": "$1-${1}-${env:NODE_NAME}-${env:NODE_IP}"},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "allows escaped provider-looking text",
			uri:  "file:/config.yaml",
			raw: map[string]any{
				"receivers": map[string]any{
					"prometheus": map[string]any{"config": "$${env:SECRET}"},
				},
			},
		},
		{
			name: "rejects dollar in node environment value",
			uri:  "env:NODE_NAME",
			raw:  "node-$name",
			want: true,
		},
		{
			name: "ignores providers in other receivers",
			uri:  "file:/config.yaml",
			raw: map[string]any{
				"receivers": map[string]any{
					"otlp": map[string]any{"endpoint": "${env:OTLP_ENDPOINT}"},
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := ValidatePrometheusConfigProviders(test.uri, test.raw)
			if got := err != nil; got != test.want {
				t.Fatalf("ValidatePrometheusConfigProviders() error = %v, want error %t", err, test.want)
			}
		})
	}
}
