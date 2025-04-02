module github.com/prometheus-collector/shared/configmap/ccp

replace github.com/prometheus-collector/shared => ../../../shared

go 1.21

require (
	github.com/pelletier/go-toml v1.9.5
	github.com/prometheus-collector/shared v0.0.0-00010101000000-000000000000
	github.com/stretchr/testify v1.9.0
	gopkg.in/yaml.v2 v2.4.0
)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
