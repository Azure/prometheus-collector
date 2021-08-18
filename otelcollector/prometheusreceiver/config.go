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
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	//"github.com/prometheus/common/config"
	config_util "github.com/prometheus/common/config"
	promconfig "github.com/prometheus/prometheus/config"
	"github.com/spf13/cast"
	yaml "gopkg.in/yaml.v2"

	//"go.uber.org/zap"
	"github.com/prometheus/prometheus/discovery/file"
	"github.com/prometheus/prometheus/discovery/kubernetes"
	"github.com/prometheus/prometheus/discovery/targetgroup"
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
	fmt.Printf("in checkTLSConfig %v.....\n", tlsConfig)
	fmt.Printf("tlsConfig.CertFile - %v\n", tlsConfig.CertFile)

	if err := checkFileExists(tlsConfig.CertFile); err != nil {
		fmt.Errorf("error checking client cert file %q - &v", tlsConfig.CertFile, err)
		return errors.New("error checking client cert file")
	}
	fmt.Printf("tlsConfig.KeyFile - %v\n", tlsConfig.KeyFile)

	if err := checkFileExists(tlsConfig.KeyFile); err != nil {
		fmt.Errorf("error checking client key file %q - &v", tlsConfig.KeyFile, err)
		return errors.New("error checking client key file")
	}

	if len(tlsConfig.CertFile) > 0 && len(tlsConfig.KeyFile) == 0 {
		fmt.Errorf("client cert file %q specified without client key file", tlsConfig.CertFile)
		return errors.New("client cert file specified without client key file")
	}
	if len(tlsConfig.KeyFile) > 0 && len(tlsConfig.CertFile) == 0 {
		fmt.Errorf("client key file %q specified without client cert file", tlsConfig.CertFile)
		return errors.New("client key file specified without client cert file")
	}

	return nil
}

func checkSDFile(filename string) error {
	fmt.Printf("In CheckSDFile...")
	fd, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer fd.Close()

	content, err := ioutil.ReadAll(fd)
	if err != nil {
		return err
	}

	var targetGroups []*targetgroup.Group

	switch ext := filepath.Ext(filename); strings.ToLower(ext) {
	case ".json":
		if err := json.Unmarshal(content, &targetGroups); err != nil {
			fmt.Errorf("Error in unmarshaling json file extension - %v", err)
			return err
		}
	case ".yml", ".yaml":
		if err := yaml.UnmarshalStrict(content, &targetGroups); err != nil {
			fmt.Errorf("Error in unmarshaling yaml file extension - %v", err)
			return err
		}
	default:
		fmt.Errorf("invalid file extension: %q", ext)
		return errors.New("invalid file extension")
	}

	for i, tg := range targetGroups {
		if tg == nil {
			fmt.Errorf("nil target group item found (index %d)", i)
			return errors.New("nil target group item found")
		}
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

		// Providing support for older version on prometheus config
		if err := checkFileExists(scfg.HTTPClientConfig.BearerTokenFile); err != nil {
			fmt.Errorf("error checking bearer token file %q - %s", scfg.HTTPClientConfig.BearerTokenFile, err)
			return errors.New("error checking bearer token file")
		}

		if scfg.HTTPClientConfig.Authorization != nil {
			// fmt.Printf("in file validation-Authorization-credentials file- %v...\n", scfg.HTTPClientConfig.Authorization.CredentialsFile)
			if err := checkFileExists(scfg.HTTPClientConfig.Authorization.CredentialsFile); err != nil {
				fmt.Errorf("error checking authorization credentials file %q - %s", scfg.HTTPClientConfig.Authorization, err)
				return errors.New("error checking authorization credentials file")
			}
		}

		fmt.Printf("Checking TLS config %v", scfg.HTTPClientConfig.TLSConfig)
		if err := checkTLSConfig(scfg.HTTPClientConfig.TLSConfig); err != nil {
			return err
		}

		for _, c := range scfg.ServiceDiscoveryConfigs {
			switch c := c.(type) {
			case *kubernetes.SDConfig:
				fmt.Printf("In kubernetes sd config...%v", c.HTTPClientConfig.TLSConfig)
				if err := checkTLSConfig(c.HTTPClientConfig.TLSConfig); err != nil {
					return err
				}
			case *file.SDConfig:
				fmt.Printf("In file sd config...")
				for _, file := range c.Files {
					files, err := filepath.Glob(file)
					if err != nil {
						return err
					}
					if len(files) != 0 {
						for _, f := range files {
							err = checkSDFile(f)
							if err != nil {
								fmt.Errorf("checking SD file %q: %v", file, err)
								return errors.New("Error in SD file check")
							}
						}
						continue
					}
					fmt.Printf("  WARNING: file %q for file_sd in scrape job %q does not exist\n", file, scfg.JobName)
				}
			}
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
