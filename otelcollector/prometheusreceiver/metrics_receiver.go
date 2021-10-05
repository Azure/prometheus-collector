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

package prometheusreceiver

import (
	"context"
	"os"
	"fmt"
	"time"
	"net/url"

	"github.com/prometheus/prometheus/discovery"
	"github.com/prometheus/prometheus/scrape"
	"github.com/prometheus/common/version"
  //"github.com/prometheus/prometheus/web"
	"go.uber.org/zap"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/obsreport"
	//"go.opentelemetry.io/collector/receiver/prometheusreceiver/internal"
	"github.com/gracewehner/prometheusreceiver/internal"
	"github.com/gracewehner/prometheusreceiver/web"
	//"github.com/gracewehner/web"
	"github.com/go-kit/log"
)

const transport = "http"

// pReceiver is the type that provides Prometheus scraper/receiver functionality.
type pReceiver struct {
	cfg        *Config
	consumer   consumer.Metrics
	cancelFunc context.CancelFunc

	logger *zap.Logger
}

// New creates a new prometheus.Receiver reference.
func newPrometheusReceiver(logger *zap.Logger, cfg *Config, next consumer.Metrics) *pReceiver {
	pr := &pReceiver{
		cfg:      cfg,
		consumer: next,
		logger:   logger,
	}
	return pr
}

// Start is the method that starts Prometheus scraping and it
// is controlled by having previously defined a Configuration using perhaps New.
func (r *pReceiver) Start(_ context.Context, host component.Host) error {
	discoveryCtx, cancel := context.WithCancel(context.Background())
	r.cancelFunc = cancel

	logger := internal.NewZapToGokitLogAdapter(r.logger)

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
			r.logger.Error("Discovery manager failed", zap.Error(err))
			host.ReportFatalError(err)
		}
	}()

	var jobsMap *internal.JobsMap
	if !r.cfg.UseStartTimeMetric {
		jobsMap = internal.NewJobsMap(2 * time.Minute)
	}
	// Per component.Component Start instructions, for async operations we should not use the
	// incoming context, it may get cancelled.
	receiverCtx := obsreport.ReceiverContext(context.Background(), r.cfg.ID(), transport)
	ocaStore := internal.NewOcaStore(
		receiverCtx,
		r.consumer,
		r.logger,
		jobsMap,
		r.cfg.UseStartTimeMetric,
		r.cfg.StartTimeMetricRegex,
		r.cfg.ID(),
		r.cfg.PrometheusConfig.GlobalConfig.ExternalLabels,
	)
	scrapeManager := scrape.NewManager(logger, ocaStore)
	ocaStore.SetScrapeManager(scrapeManager)
	if err := scrapeManager.ApplyConfig(r.cfg.PrometheusConfig); err != nil {
		return err
	}
	go func() {
		if err := scrapeManager.Run(discoveryManager.SyncCh()); err != nil {
			r.logger.Error("Scrape manager failed", zap.Error(err))
			host.ReportFatalError(err)
		}
	}()

	ctxWeb, _ := context.WithCancel(context.Background())
	webOptions := web.Options{
		ScrapeManager: scrapeManager,
		Context: ctxWeb,
		ListenAddress: ":9091",
		ExternalURL: &url.URL{
			Scheme: "http",
			Host:   "localhost:9091",
			Path:   "",
		},
		RoutePrefix:    "/",
		ReadTimeout: time.Second * 10,
		PageTitle: "Prometheus Receiver",
		Version: &web.PrometheusVersion{
			Version:   version.Version,
			Revision:  version.Revision,
			Branch:    version.Branch,
			BuildUser: version.BuildUser,
			BuildDate: version.BuildDate,
			GoVersion: version.GoVersion,
		},
		MaxConnections: 3,
	} 
	w := log.NewSyncWriter(os.Stderr)
  go_kit_logger := log.NewLogfmtLogger(w)
	// Depends on cfg.web.ScrapeManager so needs to be after cfg.web.ScrapeManager = scrapeManager.
	webHandler := web.New(go_kit_logger, &webOptions)
	listener, err := webHandler.Listener()
	if err != nil {
		os.Exit(1)
	}
	webHandler.ApplyConfig(r.cfg.PrometheusConfig)
	//webHandler.Ready()
	go func() {
		if err := webHandler.Run(ctxWeb, listener, ""); err != nil {
			return //errors.Wrapf(err, "error starting web server")
		}
	}()

	return nil
}

// Shutdown stops and cancels the underlying Prometheus scrapers.
func (r *pReceiver) Shutdown(context.Context) error {
	r.cancelFunc()
	return nil
}
