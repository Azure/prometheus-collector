module main

go 1.19

require (
	github.com/pelletier/go-toml v1.9.5
	gopkg.in/yaml.v2 v2.4.0
	github.com/configmapparser/shared v0.0.0-20220119101345-3b3b3b3b3b3b
)

replace github.com/configmapparser/shared => ../shared
