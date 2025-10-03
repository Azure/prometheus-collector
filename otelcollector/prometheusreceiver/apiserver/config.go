// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package apiserver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver/apiserver"

import (
	"errors"

	"go.opentelemetry.io/collector/config/confighttp"
)

type Config struct {
	ServerConfig confighttp.ServerConfig `mapstructure:"server_config"`
}

func (cfg *Config) Validate() error {
	if cfg.ServerConfig.Endpoint == "" {
		return errors.New("if api_server is enabled, it requires a non-empty server_config endpoint")
	}

	return nil
}
