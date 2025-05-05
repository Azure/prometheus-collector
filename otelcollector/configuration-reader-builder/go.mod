module github.com/configurationreader

go 1.23.0

toolchain go1.23.6

replace github.com/prometheus-collector/shared => ../shared

replace github.com/prometheus-collector/shared/configmap/mp => ../shared/configmap/mp

replace github.com/prometheus-collector/defaultscrapeconfigs => ../defaultscrapeconfigs

require (
	github.com/prometheus-collector/shared/configmap/mp v0.0.0-00010101000000-000000000000
	github.com/prometheus/common v0.63.0
	gopkg.in/yaml.v2 v2.4.0
	k8s.io/apimachinery v0.32.3
)

require (
	github.com/fxamacker/cbor/v2 v2.7.0 // indirect
	github.com/go-logr/logr v1.4.2 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/google/gofuzz v1.2.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/kr/pretty v0.3.1 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/onsi/ginkgo/v2 v2.22.2 // indirect
	github.com/onsi/gomega v1.36.2 // indirect
	github.com/pelletier/go-toml v1.9.5 // indirect
	github.com/prometheus-collector/shared v0.0.0-00010101000000-000000000000 // indirect
	github.com/prometheus/client_model v0.6.1 // indirect
	github.com/rogpeppe/go-internal v1.13.1 // indirect
	github.com/spf13/pflag v1.0.6 // indirect
	github.com/x448/float16 v0.8.4 // indirect
	golang.org/x/net v0.37.0 // indirect
	golang.org/x/text v0.23.0 // indirect
	golang.org/x/tools v0.29.0 // indirect
	google.golang.org/protobuf v1.36.5 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	k8s.io/klog/v2 v2.130.1 // indirect
	k8s.io/utils v0.0.0-20241210054802-24370beab758 // indirect
	sigs.k8s.io/json v0.0.0-20241014173422-cfa47c3a1cc8 // indirect
	sigs.k8s.io/structured-merge-diff/v4 v4.5.0 // indirect
	sigs.k8s.io/yaml v1.4.0 // indirect
)
