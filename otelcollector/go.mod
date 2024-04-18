module prometheus-collector

go 1.21

toolchain go1.22.0

replace github.com/prometheus-collector/shared => ./shared

replace github.com/prometheus-collector/shared/configmap/mp => ./shared/configmap/mp

require (
	github.com/prometheus-collector/shared v0.0.0-00010101000000-000000000000
	github.com/prometheus-collector/shared/configmap/mp v0.0.0-00010101000000-000000000000
)

require (
	github.com/pelletier/go-toml v1.9.5 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
)
