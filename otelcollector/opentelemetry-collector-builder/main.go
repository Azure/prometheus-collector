package main

import (
	"context"
	"log"
	"os"

	"github.com/prometheus-collector/shared"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/confmap/provider/envprovider"
	"go.opentelemetry.io/collector/confmap/provider/fileprovider"
	"go.opentelemetry.io/collector/otelcol"
)

func main() {
	info := component.BuildInfo{
		Command:     "custom-collector-distro",
		Description: "Custom OpenTelemetry Collector distribution",
		Version:     "0.162.0",
	}

	set := otelcol.CollectorSettings{
		BuildInfo: info,
		Factories: components,
		ConfigProviderSettings: otelcol.ConfigProviderSettings{
			ResolverSettings: confmap.ResolverSettings{
				ProviderFactories: []confmap.ProviderFactory{
					restrictProvider(envprovider.NewFactory()),
					restrictProvider(fileprovider.NewFactory()),
				},
			},
		},
	}

	app := otelcol.NewCommand(set)
	err := app.Execute()
	if err != nil {
		log.Fatal("collector server run finished with error: %w", err)
	}
}

func restrictProvider(factory confmap.ProviderFactory) confmap.ProviderFactory {
	if os.Getenv(shared.RestrictPrometheusConfigProvidersEnv) != "true" {
		return factory
	}
	return confmap.NewProviderFactory(func(settings confmap.ProviderSettings) confmap.Provider {
		return &restrictedProvider{Provider: factory.Create(settings)}
	})
}

type restrictedProvider struct {
	confmap.Provider
}

func (p *restrictedProvider) Retrieve(ctx context.Context, uri string, watcher confmap.WatcherFunc) (*confmap.Retrieved, error) {
	retrieved, err := p.Provider.Retrieve(ctx, uri, watcher)
	if err != nil {
		return nil, err
	}
	raw, err := retrieved.AsRaw()
	if err != nil {
		retrieved.Close(ctx)
		return nil, err
	}
	if config, ok := raw.(map[string]any); ok {
		raw = confmap.NewFromStringMap(config).ToStringMap()
	}
	if err := shared.ValidatePrometheusConfigProviders(uri, raw); err != nil {
		retrieved.Close(ctx)
		return nil, err
	}
	return retrieved, nil
}
