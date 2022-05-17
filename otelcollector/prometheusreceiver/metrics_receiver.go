// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package prometheusreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver"

import (
	"context"
	"os"
	"time"
	"net/url"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/prometheus/config"
	"github.com/prometheus/prometheus/discovery"
	"github.com/prometheus/prometheus/scrape"
	"github.com/prometheus/common/version"
	"github.com/prometheus/prometheus/web"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.uber.org/zap"
	"github.com/go-kit/log"

	//"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver/internal"
	"github.com/gracewehner/prometheusreceiver/internal"
)

const (
	defaultGCInterval = 2 * time.Minute
	gcIntervalDelta   = 1 * time.Minute

		// Use same settings as Prometheus web server
		maxConnections = 512
		readTimeoutMinutes = 10
)

// pReceiver is the type that provides Prometheus scraper/receiver functionality.
type pReceiver struct {
	cfg        *Config
	consumer   consumer.Metrics
	cancelFunc context.CancelFunc

	settings      component.ReceiverCreateSettings
	scrapeManager *scrape.Manager
}

// New creates a new prometheus.Receiver reference.
func newPrometheusReceiver(set component.ReceiverCreateSettings, cfg *Config, next consumer.Metrics) *pReceiver {
	pr := &pReceiver{
		cfg:      cfg,
		consumer: next,
		settings: set,
	}
	return pr
}

// Start is the method that starts Prometheus scraping and it
// is controlled by having previously defined a Configuration using perhaps New.
func (r *pReceiver) Start(_ context.Context, host component.Host) error {
	discoveryCtx, cancel := context.WithCancel(context.Background())
	r.cancelFunc = cancel

	logger := internal.NewZapToGokitLogAdapter(r.settings.Logger)

	discoveryManager := discovery.NewManager(discoveryCtx, logger)
	discoveryCfg := make(map[string]discovery.Configs)
	for _, scrapeConfig := range r.cfg.PrometheusConfig.ScrapeConfigs {
		discoveryCfg[scrapeConfig.JobName] = scrapeConfig.ServiceDiscoveryConfigs
	}
	if err := discoveryManager.ApplyConfig(discoveryCfg); err != nil {
		return err
	}
	go func() {
		if err := discoveryManager.Run(); err != nil {
			r.settings.Logger.Error("Discovery manager failed", zap.Error(err))
			host.ReportFatalError(err)
		}
	}()

	store := internal.NewAppendable(
		r.consumer,
		r.settings,
		gcInterval(r.cfg.PrometheusConfig),
		r.cfg.UseStartTimeMetric,
		r.cfg.StartTimeMetricRegex,
		r.cfg.ID(),
		r.cfg.PrometheusConfig.GlobalConfig.ExternalLabels,
	)
	r.scrapeManager = scrape.NewManager(&scrape.Options{PassMetadataInContext: true}, logger, store)
	if err := r.scrapeManager.ApplyConfig(r.cfg.PrometheusConfig); err != nil {
		return err
	}
	go func() {
		if err := r.scrapeManager.Run(discoveryManager.SyncCh()); err != nil {
			r.settings.Logger.Error("Scrape manager failed", zap.Error(err))
			host.ReportFatalError(err)
		}
	}()

	// Setup settings and logger and create Prometheus web handler
	webOptions := web.Options{
		ScrapeManager: r.scrapeManager,
		Context: discoveryCtx,
		ListenAddress: ":9090",
		ExternalURL: &url.URL{
			Scheme: "http",
			Host:   "localhost:9090",
			Path:   "",
		},
		RoutePrefix:    "/",
		ReadTimeout: time.Minute * readTimeoutMinutes,
		PageTitle: "Prometheus Receiver",
		Version: &web.PrometheusVersion{
			Version:   version.Version,
			Revision:  version.Revision,
			Branch:    version.Branch,
			BuildUser: version.BuildUser,
			BuildDate: version.BuildDate,
			GoVersion: version.GoVersion,
		},
		Flags: make(map[string]string),
		MaxConnections: maxConnections,
		IsAgent: true,
		Gatherer:   prometheus.DefaultGatherer,
	}
	go_kit_logger := log.NewLogfmtLogger(log.NewSyncWriter(os.Stderr))
	webHandler := web.New(go_kit_logger, &webOptions)

	listener, err := webHandler.Listener()
	if err != nil {
		return err
	}

	// Pass config and let the web handler know the config is ready.
	// These are needed because Prometheus allows reloading the config without restarting.
	webHandler.ApplyConfig(r.cfg.PrometheusConfig)
	webHandler.Ready()

	// Uses the same context as the discovery and scrape managers for shutting down
	go func() {
		if err := webHandler.Run(discoveryCtx, listener, ""); err != nil {
			r.settings.Logger.Error("Web handler failed", zap.Error(err))
			host.ReportFatalError(err)
		}
	}()

	return nil
}

// gcInterval returns the longest scrape interval used by a scrape config,
// plus a delta to prevent race conditions.
// This ensures jobs are not garbage collected between scrapes.
func gcInterval(cfg *config.Config) time.Duration {
	gcInterval := defaultGCInterval
	if time.Duration(cfg.GlobalConfig.ScrapeInterval)+gcIntervalDelta > gcInterval {
		gcInterval = time.Duration(cfg.GlobalConfig.ScrapeInterval) + gcIntervalDelta
	}
	for _, scrapeConfig := range cfg.ScrapeConfigs {
		if time.Duration(scrapeConfig.ScrapeInterval)+gcIntervalDelta > gcInterval {
			gcInterval = time.Duration(scrapeConfig.ScrapeInterval) + gcIntervalDelta
		}
	}
	return gcInterval
}

// Shutdown stops and cancels the underlying Prometheus scrapers.
func (r *pReceiver) Shutdown(context.Context) error {
	r.cancelFunc()
	r.scrapeManager.Stop()
	return nil
}
