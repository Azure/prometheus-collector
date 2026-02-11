package shared

import (
	"encoding/json"
	"io"
	"log"
	"os"
	"strings"
	"time"
)

// CCPLogWriter wraps an io.Writer to add pod and containerID fields to each log line
type CCPLogWriter struct {
	dest        io.Writer
	pod         string
	containerID string
}

// NewCCPLogWriter creates a new CCPLogWriter that adds pod and containerID to each log line
func NewCCPLogWriter(dest io.Writer) *CCPLogWriter {
	pod := os.Getenv("POD_NAME")
	if pod == "" {
		pod = os.Getenv("HOSTNAME")
	}
	containerID := os.Getenv("CONTAINER_ID")

	return &CCPLogWriter{
		dest:        dest,
		pod:         pod,
		containerID: containerID,
	}
}

// Write implements io.Writer and wraps each log line with JSON containing pod and containerID
func (w *CCPLogWriter) Write(p []byte) (n int, err error) {
	msg := strings.TrimSuffix(string(p), "\n")

	logEntry := map[string]interface{}{
		"time":        time.Now().UTC().Format(time.RFC3339Nano),
		"pod":         w.pod,
		"containerID": w.containerID,
		"message":     msg,
	}

	jsonBytes, err := json.Marshal(logEntry)
	if err != nil {
		// Fall back to writing the original message if JSON marshaling fails
		return w.dest.Write(p)
	}

	jsonBytes = append(jsonBytes, '\n')
	_, err = w.dest.Write(jsonBytes)
	if err != nil {
		return 0, err
	}

	// Return original length to satisfy the interface contract
	return len(p), nil
}

// SetupCCPLogging configures the global logger to output JSON with pod and containerID fields
// This should be called early in main() when CCP_METRICS_ENABLED is true
func SetupCCPLogging() {
	ccpWriter := NewCCPLogWriter(os.Stdout)
	log.SetOutput(ccpWriter)
	log.SetFlags(0) // Disable default timestamp since we include it in JSON
}
