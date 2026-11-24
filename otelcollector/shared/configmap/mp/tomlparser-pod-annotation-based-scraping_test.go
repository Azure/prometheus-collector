package configmapsettings

import (
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func podAnnotationSections(regex string) map[string]map[string]string {
	return map[string]map[string]string{"pod-annotation-based-scraping": {"podannotationnamespaceregex": regex}}
}

func TestConfigurePodAnnotationSettings(t *testing.T) {
	tests := []struct {
		name     string
		sections map[string]map[string]string
		wantErr  string
		wantFile string
	}{
		{name: "unmounted", wantErr: "configmap section not mounted"},
		{name: "absent", sections: map[string]map[string]string{}, wantErr: "configuration not found"},
		{name: "empty", sections: podAnnotationSections(""), wantErr: "does not have a value"},
		{name: "invalid", sections: podAnnotationSections("["), wantErr: "Invalid namespace regex for pod annotations: ["},
		{name: "anchored", sections: podAnnotationSections("^ama-settings-included$"), wantFile: "AZMON_PROMETHEUS_POD_ANNOTATION_NAMESPACES_REGEX=^ama-settings-included$\nAZMON_PROMETHEUS_POD_ANNOTATION_SCRAPING_ENABLED=true\n"},
		{name: "alternation", sections: podAnnotationSections("^(ama-settings-included|ama-settings-excluded)$"), wantFile: "AZMON_PROMETHEUS_POD_ANNOTATION_NAMESPACES_REGEX=^(ama-settings-included|ama-settings-excluded)$\nAZMON_PROMETHEUS_POD_ANNOTATION_SCRAPING_ENABLED=true\n"},
		{name: "wildcard", sections: podAnnotationSections(".*"), wantFile: "AZMON_PROMETHEUS_POD_ANNOTATION_NAMESPACES_REGEX=.*\nAZMON_PROMETHEUS_POD_ANNOTATION_SCRAPING_ENABLED=true\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			original := podAnnotationEnvVarPath
			t.Cleanup(func() { podAnnotationEnvVarPath = original })
			podAnnotationEnvVarPath = filepath.Join(t.TempDir(), "pod-annotation-env")

			var output strings.Builder
			originalOutput := log.Writer()
			log.SetOutput(&output)
			t.Cleanup(func() { log.SetOutput(originalOutput) })

			err := configurePodAnnotationSettings(tt.sections)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error = %v, want containing %q", err, tt.wantErr)
				}
				if _, statErr := os.Stat(podAnnotationEnvVarPath); !os.IsNotExist(statErr) {
					t.Fatalf("pod annotation env file was written for a rejected regex")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			wantLog := "Using configmap namespace regex for pod annotations: " + tt.sections["pod-annotation-based-scraping"]["podannotationnamespaceregex"]
			if !strings.Contains(output.String(), wantLog) {
				t.Fatalf("log = %q, want %q", output.String(), wantLog)
			}
			got, err := os.ReadFile(podAnnotationEnvVarPath)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != tt.wantFile {
				t.Fatalf("env file = %q, want %q", got, tt.wantFile)
			}
		})
	}
}

func TestConfigurePodAnnotationSettingsUnwritableOutput(t *testing.T) {
	original := podAnnotationEnvVarPath
	t.Cleanup(func() { podAnnotationEnvVarPath = original })
	podAnnotationEnvVarPath = filepath.Join(t.TempDir(), "missing", "pod-annotation-env")

	err := configurePodAnnotationSettings(podAnnotationSections(".*"))
	if err == nil || !strings.Contains(err.Error(), "error opening file") {
		t.Fatalf("error = %v, want error opening file", err)
	}
}
