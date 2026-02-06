package shared

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestCCPLogWriter(t *testing.T) {
	tests := []struct {
		name        string
		pod         string
		containerID string
		input       string
		wantPod     string
		wantCID     string
		wantMsg     string
	}{
		{
			name:        "basic log message",
			pod:         "test-pod",
			containerID: "test-container-id",
			input:       "test message\n",
			wantPod:     "test-pod",
			wantCID:     "test-container-id",
			wantMsg:     "test message",
		},
		{
			name:        "log message without trailing newline",
			pod:         "pod-123",
			containerID: "cid-456",
			input:       "no newline message",
			wantPod:     "pod-123",
			wantCID:     "cid-456",
			wantMsg:     "no newline message",
		},
		{
			name:        "empty pod and containerID",
			pod:         "",
			containerID: "",
			input:       "message with empty fields\n",
			wantPod:     "",
			wantCID:     "",
			wantMsg:     "message with empty fields",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer

			// Create writer with specific pod/containerID
			writer := &CCPLogWriter{
				dest:        &buf,
				pod:         tc.pod,
				containerID: tc.containerID,
			}

			n, err := writer.Write([]byte(tc.input))
			if err != nil {
				t.Fatalf("Write() error = %v", err)
			}
			if n != len(tc.input) {
				t.Errorf("Write() returned %d, want %d", n, len(tc.input))
			}

			// Parse the output JSON
			output := strings.TrimSuffix(buf.String(), "\n")
			var result map[string]interface{}
			if err := json.Unmarshal([]byte(output), &result); err != nil {
				t.Fatalf("Failed to parse JSON output: %v, output was: %s", err, output)
			}

			if result["pod"] != tc.wantPod {
				t.Errorf("pod = %v, want %v", result["pod"], tc.wantPod)
			}
			if result["containerID"] != tc.wantCID {
				t.Errorf("containerID = %v, want %v", result["containerID"], tc.wantCID)
			}
			if result["message"] != tc.wantMsg {
				t.Errorf("message = %v, want %v", result["message"], tc.wantMsg)
			}
			if _, ok := result["time"]; !ok {
				t.Error("time field is missing from output")
			}
		})
	}
}

func TestNewCCPLogWriter(t *testing.T) {
	// Save and restore env vars
	origPodName := os.Getenv("POD_NAME")
	origHostname := os.Getenv("HOSTNAME")
	origContainerID := os.Getenv("CONTAINER_ID")
	defer func() {
		os.Setenv("POD_NAME", origPodName)
		os.Setenv("HOSTNAME", origHostname)
		os.Setenv("CONTAINER_ID", origContainerID)
	}()

	t.Run("uses POD_NAME when set", func(t *testing.T) {
		os.Setenv("POD_NAME", "my-pod")
		os.Setenv("HOSTNAME", "my-hostname")
		os.Setenv("CONTAINER_ID", "my-container")

		var buf bytes.Buffer
		writer := NewCCPLogWriter(&buf)

		if writer.pod != "my-pod" {
			t.Errorf("pod = %v, want my-pod", writer.pod)
		}
		if writer.containerID != "my-container" {
			t.Errorf("containerID = %v, want my-container", writer.containerID)
		}
	})

	t.Run("falls back to HOSTNAME when POD_NAME is empty", func(t *testing.T) {
		os.Setenv("POD_NAME", "")
		os.Setenv("HOSTNAME", "fallback-hostname")
		os.Setenv("CONTAINER_ID", "cid")

		var buf bytes.Buffer
		writer := NewCCPLogWriter(&buf)

		if writer.pod != "fallback-hostname" {
			t.Errorf("pod = %v, want fallback-hostname", writer.pod)
		}
	})
}
