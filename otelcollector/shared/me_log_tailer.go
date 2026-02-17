package shared

import (
	"bufio"
	"io"
	"log"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// ME log line regexes — ported from fluent-bit telemetry.go
var (
	// ProcessedCount log line from ME:
	// <timestamp> <level> <account> ... ProcessedCount: 123 ... ProcessedBytes: 456 ... SentToPublicationCount: 789 ... SentToPublicationBytes: 012
	meProcessedCountRegex = regexp.MustCompile(`\s*([^\s]+)\s*([^\s]+)\s*([^\s]+).*ProcessedCount: ([\d]+).*ProcessedBytes: ([\d]+).*SentToPublicationCount: ([\d]+).*SentToPublicationBytes: ([\d]+)`)

	// EventsProcessedLastPeriod log line from ME:
	// ... EventsProcessedLastPeriod: 123 ...
	meEventsProcessedRegex = regexp.MustCompile(`EventsProcessedLastPeriod: (\d+)`)
)

// TailMELogs reads ME stdout line by line, copies each line to os.Stdout,
// and parses ProcessedCount/EventsProcessedLastPeriod lines to feed the
// shared health metric globals (TimeseriesReceivedTotal, TimeseriesSentTotal, BytesSentTotal).
// This replaces the fluent-bit ME log parsing pipeline for CCP mode.
func TailMELogs(reader io.Reader) {
	scanner := bufio.NewScanner(reader)
	// ME can emit long lines; increase buffer to 1MB
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()

		// Always echo to stdout so container logs are preserved
		os.Stdout.WriteString(line + "\n")

		// Parse ProcessedCount lines for SentToPublicationCount / SentToPublicationBytes
		parseMEProcessedCountLine(line)

		// Parse EventsProcessedLastPeriod lines for received count
		parseMEEventsProcessedLine(line)
	}

	if err := scanner.Err(); err != nil {
		log.Printf("Error reading ME stdout: %v", err)
	}
	log.Println("ME stdout reader exited")
}

// parseMEProcessedCountLine extracts SentToPublicationCount and SentToPublicationBytes
// from an ME log line and feeds them into the shared globals.
func parseMEProcessedCountLine(line string) {
	if !strings.Contains(line, "ProcessedCount:") {
		return
	}

	matches := meProcessedCountRegex.FindStringSubmatch(line)
	if len(matches) < 8 {
		return
	}

	sentCount, err := strconv.ParseFloat(matches[6], 64)
	if err != nil {
		return
	}

	sentBytes, err := strconv.ParseFloat(matches[7], 64)
	if err != nil {
		sentBytes = 0
	}

	TimeseriesVolumeMutex.Lock()
	TimeseriesSentTotal += sentCount
	BytesSentTotal += sentBytes
	TimeseriesVolumeMutex.Unlock()
}

// parseMEEventsProcessedLine extracts EventsProcessedLastPeriod from an ME log line
// and feeds it into TimeseriesReceivedTotal.
func parseMEEventsProcessedLine(line string) {
	if !strings.Contains(line, "EventsProcessedLastPeriod:") {
		return
	}

	matches := meEventsProcessedRegex.FindStringSubmatch(line)
	if len(matches) < 2 {
		return
	}

	receivedCount, err := strconv.ParseFloat(matches[1], 64)
	if err != nil {
		return
	}

	TimeseriesVolumeMutex.Lock()
	TimeseriesReceivedTotal += receivedCount
	TimeseriesVolumeMutex.Unlock()
}

// TailOtelCollectorLogFile tails the otelcollector log file watching for
// "Exporting failed" messages. This replaces the fluent-bit rewrite_tag rule
// that filtered otelcollector logs for export failures.
func TailOtelCollectorLogFile(filePath string) {
	log.Printf("Waiting for otelcollector log file: %s", filePath)

	// Wait for the file to exist
	for {
		if _, err := os.Stat(filePath); err == nil {
			break
		}
		time.Sleep(5 * time.Second)
	}

	log.Printf("Starting otelcollector log tailer: %s", filePath)

	file, err := os.Open(filePath)
	if err != nil {
		log.Printf("Error opening otelcollector log file: %v", err)
		return
	}
	defer file.Close()

	// Seek to end — we only care about new lines
	file.Seek(0, io.SeekEnd)

	reader := bufio.NewReader(file)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			// EOF — wait and retry (file is being appended to)
			time.Sleep(1 * time.Second)
			continue
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Check for "Exporting failed" — same pattern fluent-bit used
		if strings.Contains(line, "Exporting failed") {
			ExportingFailedMutex.Lock()
			OtelCollectorExportingFailedCount += 1
			ExportingFailedMutex.Unlock()
		}
	}
}
