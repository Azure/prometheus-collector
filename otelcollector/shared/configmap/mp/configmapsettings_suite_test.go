package configmapsettings

import (
	"os"
	"slices"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestConfigmapSettings(t *testing.T) {
	environment := os.Environ()
	slices.Sort(environment)
	workingDirectory, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		currentEnvironment := os.Environ()
		slices.Sort(currentEnvironment)
		if !slices.Equal(currentEnvironment, environment) {
			t.Error("MP tests did not restore the process environment")
		}
		currentDirectory, err := os.Getwd()
		if err != nil {
			t.Fatal(err)
		}
		if currentDirectory != workingDirectory {
			t.Error("MP tests did not restore the working directory")
		}
	})
	RegisterFailHandler(Fail)
	RunSpecs(t, "Configmap Settings Suite")
}

func setTestPath(path *string, value string) {
	original := *path
	DeferCleanup(func() {
		*path = original
	})
	*path = value
}
