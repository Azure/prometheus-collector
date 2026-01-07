// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package apiserver

import (
	"fmt"

	"go.opentelemetry.io/collector/config/confighttp"
)

// Config holds the settings for the optional embedded Prometheus API server.
type Config struct {
	ServerConfig confighttp.ServerConfig `mapstructure:"server_config"`
}

// Validate ensures the API server configuration is usable.
func (cfg *Config) Validate() error {
	if cfg == nil {
		return fmt.Errorf("api server config must be provided")
	}
	if cfg.ServerConfig.Endpoint == "" {
		return fmt.Errorf("server_config.endpoint must be specified")
	}
	return nil
}
