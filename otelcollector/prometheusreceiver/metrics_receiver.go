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
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/go-kit/log"
	"github.com/mitchellh/hashstructure/v2"
	"github.com/prometheus/common/version"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/prometheus/config"
	"github.com/prometheus/prometheus/discovery"
	promHTTP "github.com/prometheus/prometheus/discovery/http"
	"github.com/prometheus/prometheus/scrape"
	"github.com/prometheus/prometheus/web"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.uber.org/zap"

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

	settings                      component.ReceiverCreateSettings
	scrapeManager                 *scrape.Manager
	discoveryManager              *discovery.Manager
	targetAllocatorIntervalTicker *time.Ticker
}

type linkJSON struct {
	Link string `json:"_link"`
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

	// add scrape configs defined by the collector configs
	baseCfg := r.cfg.PrometheusConfig

	err := r.initPrometheusComponents(discoveryCtx, host, logger)
	if err != nil {
		r.settings.Logger.Error("Failed to initPrometheusComponents Prometheus components", zap.Error(err))
		return err
	}

	err = r.applyCfg(baseCfg)
	if err != nil {
		r.settings.Logger.Error("Failed to apply new scrape configuration", zap.Error(err))
		return err
	}

	allocConf := r.cfg.TargetAllocator
	if allocConf != nil {
		go func() {
			// immediately sync jobs and not wait for the first tick
			savedHash, _ := r.syncTargetAllocator(uint64(0), allocConf, baseCfg)
			r.targetAllocatorIntervalTicker = time.NewTicker(allocConf.Interval)
			for {
				<-r.targetAllocatorIntervalTicker.C
				hash, err := r.syncTargetAllocator(savedHash, allocConf, baseCfg)
				if err != nil {
					r.settings.Logger.Error(err.Error())
					continue
				}
				savedHash = hash
			}
		}()
	}

	return nil
}

// syncTargetAllocator request jobs from targetAllocator and update underlying receiver, if the response does not match the provided compareHash.
// baseDiscoveryCfg can be used to provide additional ScrapeConfigs which will be added to the retrieved jobs.
func (r *pReceiver) syncTargetAllocator(compareHash uint64, allocConf *targetAllocator, baseCfg *config.Config) (uint64, error) {
	r.settings.Logger.Debug("Syncing target allocator jobs")
	jobObject, err := getJobResponse(allocConf.Endpoint)
	if err != nil {
		r.settings.Logger.Error("Failed to retrieve job list", zap.Error(err))
		return 0, err
	}

	hash, err := hashstructure.Hash(jobObject, hashstructure.FormatV2, nil)
	if err != nil {
		r.settings.Logger.Error("Failed to hash job list", zap.Error(err))
		return 0, err
	}
	if hash == compareHash {
		// no update needed
		return hash, nil
	}

	cfg := *baseCfg

	for _, linkJSON := range *jobObject {
		var httpSD promHTTP.SDConfig
		if allocConf.HTTPSDConfig == nil {
			httpSD = promHTTP.SDConfig{}
		} else {
			httpSD = *allocConf.HTTPSDConfig
		}

		httpSD.URL = fmt.Sprintf("%s%s?collector_id=%s", allocConf.Endpoint, linkJSON.Link, allocConf.CollectorID)

		scrapeCfg := &config.ScrapeConfig{
			JobName: linkJSON.Link,
			ServiceDiscoveryConfigs: discovery.Configs{
				&httpSD,
			},
		}

		cfg.ScrapeConfigs = append(cfg.ScrapeConfigs, scrapeCfg)
	}

	err = r.applyCfg(&cfg)
	if err != nil {
		r.settings.Logger.Error("Failed to apply new scrape configuration", zap.Error(err))
		return 0, err
	}

	return hash, nil
}

func getJobResponse(baseURL string) (*map[string]linkJSON, error) {
	jobURLString := fmt.Sprintf("%s/jobs", baseURL)
	_, err := url.Parse(jobURLString) // check if valid
	if err != nil {
		return nil, err
	}

	resp, err := http.Get(jobURLString) //nolint
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	jobObject := &map[string]linkJSON{}
	err = json.NewDecoder(resp.Body).Decode(jobObject)
	if err != nil {
		return nil, err
	}
	return jobObject, nil
}

func (r *pReceiver) applyCfg(cfg *config.Config) error {
	if err := r.scrapeManager.ApplyConfig(cfg); err != nil {
		return err
	}

	discoveryCfg := make(map[string]discovery.Configs)
	for _, scrapeConfig := range cfg.ScrapeConfigs {
		discoveryCfg[scrapeConfig.JobName] = scrapeConfig.ServiceDiscoveryConfigs
	}
	if err := r.discoveryManager.ApplyConfig(discoveryCfg); err != nil {
		return err
	}
	return nil
}

func (r *pReceiver) initPrometheusComponents(ctx context.Context, host component.Host, logger log.Logger) error {
	r.discoveryManager = discovery.NewManager(ctx, logger)

	go func() {
		if err := r.discoveryManager.Run(); err != nil {
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
	go func() {
		if err := r.scrapeManager.Run(r.discoveryManager.SyncCh()); err != nil {
			r.settings.Logger.Error("Scrape manager failed", zap.Error(err))
			host.ReportFatalError(err)
		}
	}()

	// Setup settings and logger and create Prometheus web handler
	webOptions := web.Options{
		ScrapeManager: r.scrapeManager,
		Context: ctx,
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
	webHandler.SetReady(true)
	
	// Uses the same context as the discovery and scrape managers for shutting down
	go func() {
		if err := webHandler.Run(ctx, listener, ""); err != nil {
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
	if r.targetAllocatorIntervalTicker != nil {
		r.targetAllocatorIntervalTicker.Stop()
	}
	return nil
}
