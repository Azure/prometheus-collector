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
	"errors"
	"fmt"
	"os"
	"time"

	//"github.com/prometheus/common/config"
	config_util "github.com/prometheus/common/config"
	promconfig "github.com/prometheus/prometheus/config"
	"github.com/spf13/cast"
	"gopkg.in/yaml.v2"

	//"go.uber.org/zap"
	"go.opentelemetry.io/collector/config"
	//"github.com/gracewehner/prometheusreceiver/internal"
)

const (
	// The key for Prometheus scraping configs.
	prometheusConfigKey = "config"
)

// Config defines configuration for Prometheus receiver.
type Config struct {
	config.ReceiverSettings `mapstructure:",squash"` // squash ensures fields are correctly decoded in embedded struct
	PrometheusConfig        *promconfig.Config       `mapstructure:"-"`
	BufferPeriod            time.Duration            `mapstructure:"buffer_period"`
	BufferCount             int                      `mapstructure:"buffer_count"`
	UseStartTimeMetric      bool                     `mapstructure:"use_start_time_metric"`
	StartTimeMetricRegex    string                   `mapstructure:"start_time_metric_regex"`

	// ConfigPlaceholder is just an entry to make the configuration pass a check
	// that requires that all keys present in the config actually exist on the
	// structure, ie.: it will error if an unknown key is present.
	ConfigPlaceholder interface{} `mapstructure:"config"`
	//logger  *zap.Logger
}

var _ config.Receiver = (*Config)(nil)
var _ config.CustomUnmarshable = (*Config)(nil)

func checkFileExists(fn string) error {
	// Nothing set, nothing to error on.
	if fn == "" {
		return nil
	}
	_, err := os.Stat(fn)
	// fmt.Printf("response from os stat - %v... \n", resp)
	return err
}

func checkTLSConfig(tlsConfig config_util.TLSConfig) error {
	if err := checkFileExists(tlsConfig.CertFile); err != nil {

		fmt.Printf("error checking client cert file %q - &v", tlsConfig.CertFile, err)
		return errors.New("error checking client cert file %q", tlsConfig.CertFile)
	}
	if err := checkFileExists(tlsConfig.KeyFile); err != nil {
		fmt.Printf("error checking client key file %q - &v", tlsConfig.KeyFile, err)
		return errors.New("error checking client key file %q", tlsConfig.KeyFile)
	}

	if len(tlsConfig.CertFile) > 0 && len(tlsConfig.KeyFile) == 0 {
		return errors.New("client cert file %q specified without client key file", tlsConfig.CertFile)
	}
	if len(tlsConfig.KeyFile) > 0 && len(tlsConfig.CertFile) == 0 {
		return errors.New("client key file %q specified without client cert file", tlsConfig.KeyFile)
	}

	return nil
}

// Validate checks the receiver configuration is valid
func (cfg *Config) Validate() error {
	//cfg.logger = internal.NewZapToGokitLogAdapter(cfg.logger)
	if cfg.PrometheusConfig == nil {
		return nil // noop receiver
	}
	if len(cfg.PrometheusConfig.ScrapeConfigs) == 0 {
		return errors.New("no Prometheus scrape_configs")
	}
	//cfg.logger.Info("Starting custom validation...\n")
	fmt.Printf("Starting custom validation...\n")
	for _, scfg := range cfg.PrometheusConfig.ScrapeConfigs {
		fmt.Printf(".................................\n")
		// fmt.Printf("scrape config- HTTPClientConfig - %v...\n", scfg.HTTPClientConfig)
		// fmt.Printf("in file validation-Authorization- %v...\n", scfg.HTTPClientConfig.Authorization)
		if scfg.HTTPClientConfig.Authorization != nil {
			// fmt.Printf("in file validation-Authorization-credentials file- %v...\n", scfg.HTTPClientConfig.Authorization.CredentialsFile)
			if err := checkFileExists(scfg.HTTPClientConfig.Authorization.CredentialsFile); err != nil {
				fmt.Printf("error checking authorization credentials file %q - %s", scfg.HTTPClientConfig.Authorization, err)
				return errors.New("error checking authorization credentials file")
			}
		}

		if err := checkTLSConfig(scfg.HTTPClientConfig.TLSConfig); err != nil {
			return err
		}
	}
	return nil
}

// Unmarshal a config.Parser into the config struct.
func (cfg *Config) Unmarshal(componentParser *config.Parser) error {
	if componentParser == nil {
		return nil
	}
	// We need custom unmarshaling because prometheus "config" subkey defines its own
	// YAML unmarshaling routines so we need to do it explicitly.

	err := componentParser.UnmarshalExact(cfg)
	if err != nil {
		return fmt.Errorf("prometheus receiver failed to parse config: %s", err)
	}

	// Unmarshal prometheus's config values. Since prometheus uses `yaml` tags, so use `yaml`.
	promCfgMap := cast.ToStringMap(componentParser.Get(prometheusConfigKey))
	if len(promCfgMap) == 0 {
		return nil
	}
	out, err := yaml.Marshal(promCfgMap)
	if err != nil {
		return fmt.Errorf("prometheus receiver failed to marshal config to yaml: %s", err)
	}

	err = yaml.UnmarshalStrict(out, &cfg.PrometheusConfig)
	if err != nil {
		return fmt.Errorf("prometheus receiver failed to unmarshal yaml to prometheus config: %s", err)
	}
	return nil
}
