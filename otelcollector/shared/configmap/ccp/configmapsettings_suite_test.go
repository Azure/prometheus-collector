package ccpconfigmapsettings

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestConfigmapSettings(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Configmap Settings Suite")
}
