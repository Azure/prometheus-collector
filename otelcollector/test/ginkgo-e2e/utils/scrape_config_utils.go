package utils

import (
	"errors"

	"gopkg.in/yaml.v2"
)

func ParseScrapeJobs[T any](text string) ([]T, error) {
	var cfg *struct {
		ScrapeConfigs []T `yaml:"scrape_configs"`
	}
	if err := yaml.Unmarshal([]byte(text), &cfg); err != nil {
		return nil, err
	}
	if cfg == nil {
		return nil, errors.New("empty Prometheus configuration")
	}
	return cfg.ScrapeConfigs, nil
}
