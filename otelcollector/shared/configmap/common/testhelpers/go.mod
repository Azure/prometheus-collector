module github.com/prometheus-collector/shared/configmap/common/testhelpers

go 1.23.0

toolchain go1.23.2

replace github.com/prometheus-collector/shared => ../../../../shared

require (
	github.com/prometheus-collector/shared v0.0.0-00010101000000-000000000000
	gopkg.in/yaml.v2 v2.4.0
)
