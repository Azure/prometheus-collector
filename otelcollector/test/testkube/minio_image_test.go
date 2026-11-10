package testkube

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"sigs.k8s.io/yaml"
)

func TestMinioImageUsesChartPublishedDigest(t *testing.T) {
	_, source, _, _ := runtime.Caller(0)
	valuesPath := filepath.Join(filepath.Dir(source), "values.yaml")
	data, err := os.ReadFile(valuesPath)
	if err != nil {
		t.Fatal(err)
	}
	var values struct {
		TestkubeAPI struct {
			Minio struct {
				Image struct {
					Registry   string `json:"registry"`
					Repository string `json:"repository"`
					Tag        string `json:"tag"`
					Digest     string `json:"digest"`
				} `json:"image"`
			} `json:"minio"`
		} `json:"testkube-api"`
	}
	if err := yaml.Unmarshal(data, &values); err != nil {
		t.Fatal(err)
	}
	image := values.TestkubeAPI.Minio.Image
	if image.Registry != "docker.io" || image.Repository != "kubeshop/testkube-minio" ||
		image.Tag != "2025.10" || image.Digest != "sha256:d8e1af6aca995ec6170742fb2b910d6214fea6aa0b32ed509fee17a88ae605c8" {
		t.Fatalf("unexpected MinIO image configuration: %+v", image)
	}
}
