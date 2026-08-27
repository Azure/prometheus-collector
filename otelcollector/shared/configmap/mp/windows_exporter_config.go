package configmapsettings

import (
	"bytes"
	"os"
)

const windowsExporterDefaultPort = "19182"

func configureWindowsExporterTarget(contents []byte) []byte {
	targetPort := os.Getenv("NODE_EXPORTER_TARGETPORT")
	if targetPort == "" {
		targetPort = windowsExporterDefaultPort
	}

	return bytes.ReplaceAll(contents, []byte("$$NODE_EXPORTER_TARGETPORT$$"), []byte(targetPort))
}
