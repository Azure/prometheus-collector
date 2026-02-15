module github.com/prometheus-collector/test/healthmetrics

go 1.23

require (
	github.com/onsi/ginkgo/v2 v2.21.0
	github.com/onsi/gomega v1.35.1
	github.com/prometheus-collector/test/utils v0.0.0
	k8s.io/api v0.32.0
	k8s.io/apimachinery v0.32.0
)

replace github.com/prometheus-collector/test/utils => ../utils
