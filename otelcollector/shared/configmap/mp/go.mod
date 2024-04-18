module github.com/prometheus-collector/shared/configmap/mp

replace github.com/prometheus-collector/shared => ../../../shared

go 1.21

require (
	github.com/pelletier/go-toml v1.9.5
	github.com/prometheus-collector/shared v0.0.0-00010101000000-000000000000
	gopkg.in/yaml.v2 v2.4.0
)
