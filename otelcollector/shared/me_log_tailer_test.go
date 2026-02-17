package shared

import (
	"strings"
	"testing"
)

func TestParseMEProcessedCountLine(t *testing.T) {
	// Save and restore globals
	TimeseriesVolumeMutex.Lock()
	origSent := TimeseriesSentTotal
	origBytes := BytesSentTotal
	TimeseriesSentTotal = 0
	BytesSentTotal = 0
	TimeseriesVolumeMutex.Unlock()
	defer func() {
		TimeseriesVolumeMutex.Lock()
		TimeseriesSentTotal = origSent
		BytesSentTotal = origBytes
		TimeseriesVolumeMutex.Unlock()
	}()

	// Real ME ProcessedCount log line format
	line := `2026-02-18 12:00:00.000 Info DefaultMetricAccount ReceivedCount: 500 ProcessedCount: 480 ProcessedBytes: 12345 SentToPublicationCount: 475 SentToPublicationBytes: 11234`

	parseMEProcessedCountLine(line)

	TimeseriesVolumeMutex.Lock()
	defer TimeseriesVolumeMutex.Unlock()

	if TimeseriesSentTotal != 475 {
		t.Errorf("TimeseriesSentTotal = %f, want 475", TimeseriesSentTotal)
	}
	if BytesSentTotal != 11234 {
		t.Errorf("BytesSentTotal = %f, want 11234", BytesSentTotal)
	}
}

func TestParseMEProcessedCountLine_NoMatch(t *testing.T) {
	TimeseriesVolumeMutex.Lock()
	origSent := TimeseriesSentTotal
	origBytes := BytesSentTotal
	TimeseriesSentTotal = 0
	BytesSentTotal = 0
	TimeseriesVolumeMutex.Unlock()
	defer func() {
		TimeseriesVolumeMutex.Lock()
		TimeseriesSentTotal = origSent
		BytesSentTotal = origBytes
		TimeseriesVolumeMutex.Unlock()
	}()

	// Line that doesn't match the ProcessedCount pattern
	parseMEProcessedCountLine("some random log line")

	TimeseriesVolumeMutex.Lock()
	defer TimeseriesVolumeMutex.Unlock()

	if TimeseriesSentTotal != 0 {
		t.Errorf("TimeseriesSentTotal = %f, want 0", TimeseriesSentTotal)
	}
	if BytesSentTotal != 0 {
		t.Errorf("BytesSentTotal = %f, want 0", BytesSentTotal)
	}
}

func TestParseMEEventsProcessedLine(t *testing.T) {
	TimeseriesVolumeMutex.Lock()
	origReceived := TimeseriesReceivedTotal
	TimeseriesReceivedTotal = 0
	TimeseriesVolumeMutex.Unlock()
	defer func() {
		TimeseriesVolumeMutex.Lock()
		TimeseriesReceivedTotal = origReceived
		TimeseriesVolumeMutex.Unlock()
	}()

	line := `2026-02-18 12:00:00.000 Info EventsProcessedLastPeriod: 1234 TotalEventsProcessed: 5678`

	parseMEEventsProcessedLine(line)

	TimeseriesVolumeMutex.Lock()
	defer TimeseriesVolumeMutex.Unlock()

	if TimeseriesReceivedTotal != 1234 {
		t.Errorf("TimeseriesReceivedTotal = %f, want 1234", TimeseriesReceivedTotal)
	}
}

func TestParseMEEventsProcessedLine_NoMatch(t *testing.T) {
	TimeseriesVolumeMutex.Lock()
	origReceived := TimeseriesReceivedTotal
	TimeseriesReceivedTotal = 0
	TimeseriesVolumeMutex.Unlock()
	defer func() {
		TimeseriesVolumeMutex.Lock()
		TimeseriesReceivedTotal = origReceived
		TimeseriesVolumeMutex.Unlock()
	}()

	parseMEEventsProcessedLine("some random log line")

	TimeseriesVolumeMutex.Lock()
	defer TimeseriesVolumeMutex.Unlock()

	if TimeseriesReceivedTotal != 0 {
		t.Errorf("TimeseriesReceivedTotal = %f, want 0", TimeseriesReceivedTotal)
	}
}

func TestParseMEProcessedCountLine_MultipleLines(t *testing.T) {
	TimeseriesVolumeMutex.Lock()
	origSent := TimeseriesSentTotal
	origBytes := BytesSentTotal
	TimeseriesSentTotal = 0
	BytesSentTotal = 0
	TimeseriesVolumeMutex.Unlock()
	defer func() {
		TimeseriesVolumeMutex.Lock()
		TimeseriesSentTotal = origSent
		BytesSentTotal = origBytes
		TimeseriesVolumeMutex.Unlock()
	}()

	lines := []string{
		`2026-02-18 12:00:00.000 Info Account1 ReceivedCount: 100 ProcessedCount: 100 ProcessedBytes: 5000 SentToPublicationCount: 90 SentToPublicationBytes: 4500`,
		`2026-02-18 12:01:00.000 Info Account2 ReceivedCount: 200 ProcessedCount: 200 ProcessedBytes: 10000 SentToPublicationCount: 180 SentToPublicationBytes: 9000`,
	}

	for _, line := range lines {
		parseMEProcessedCountLine(line)
	}

	TimeseriesVolumeMutex.Lock()
	defer TimeseriesVolumeMutex.Unlock()

	if TimeseriesSentTotal != 270 { // 90 + 180
		t.Errorf("TimeseriesSentTotal = %f, want 270", TimeseriesSentTotal)
	}
	if BytesSentTotal != 13500 { // 4500 + 9000
		t.Errorf("BytesSentTotal = %f, want 13500", BytesSentTotal)
	}
}

func TestTailMELogs(t *testing.T) {
	// Save and restore globals
	TimeseriesVolumeMutex.Lock()
	origReceived := TimeseriesReceivedTotal
	origSent := TimeseriesSentTotal
	origBytes := BytesSentTotal
	TimeseriesReceivedTotal = 0
	TimeseriesSentTotal = 0
	BytesSentTotal = 0
	TimeseriesVolumeMutex.Unlock()

	ExportingFailedMutex.Lock()
	origFailed := OtelCollectorExportingFailedCount
	OtelCollectorExportingFailedCount = 0
	ExportingFailedMutex.Unlock()

	defer func() {
		TimeseriesVolumeMutex.Lock()
		TimeseriesReceivedTotal = origReceived
		TimeseriesSentTotal = origSent
		BytesSentTotal = origBytes
		TimeseriesVolumeMutex.Unlock()
		ExportingFailedMutex.Lock()
		OtelCollectorExportingFailedCount = origFailed
		ExportingFailedMutex.Unlock()
	}()

	// Simulate ME stdout with mixed log lines
	input := strings.Join([]string{
		`2026-02-18 12:00:00 Starting MetricsExtension`,
		`2026-02-18 12:00:10.000 Info DefaultAccount ReceivedCount: 500 ProcessedCount: 480 ProcessedBytes: 12345 SentToPublicationCount: 475 SentToPublicationBytes: 11234`,
		`2026-02-18 12:00:15 Some other log line`,
		`2026-02-18 12:00:20.000 Info EventsProcessedLastPeriod: 300 TotalEventsProcessed: 1000`,
		`2026-02-18 12:00:30.000 Info Account2 ReceivedCount: 100 ProcessedCount: 100 ProcessedBytes: 2000 SentToPublicationCount: 95 SentToPublicationBytes: 1900`,
		`2026-02-18 12:00:40.000 Info EventsProcessedLastPeriod: 150 TotalEventsProcessed: 1150`,
	}, "\n")

	reader := strings.NewReader(input)
	TailMELogs(reader)

	TimeseriesVolumeMutex.Lock()
	defer TimeseriesVolumeMutex.Unlock()

	// SentToPublicationCount: 475 + 95 = 570
	if TimeseriesSentTotal != 570 {
		t.Errorf("TimeseriesSentTotal = %f, want 570", TimeseriesSentTotal)
	}

	// SentToPublicationBytes: 11234 + 1900 = 13134
	if BytesSentTotal != 13134 {
		t.Errorf("BytesSentTotal = %f, want 13134", BytesSentTotal)
	}

	// EventsProcessedLastPeriod: 300 + 150 = 450
	if TimeseriesReceivedTotal != 450 {
		t.Errorf("TimeseriesReceivedTotal = %f, want 450", TimeseriesReceivedTotal)
	}
}

func TestTailMELogs_EmptyInput(t *testing.T) {
	TimeseriesVolumeMutex.Lock()
	origReceived := TimeseriesReceivedTotal
	origSent := TimeseriesSentTotal
	TimeseriesReceivedTotal = 0
	TimeseriesSentTotal = 0
	TimeseriesVolumeMutex.Unlock()
	defer func() {
		TimeseriesVolumeMutex.Lock()
		TimeseriesReceivedTotal = origReceived
		TimeseriesSentTotal = origSent
		TimeseriesVolumeMutex.Unlock()
	}()

	reader := strings.NewReader("")
	TailMELogs(reader)

	TimeseriesVolumeMutex.Lock()
	defer TimeseriesVolumeMutex.Unlock()

	if TimeseriesReceivedTotal != 0 {
		t.Errorf("TimeseriesReceivedTotal = %f, want 0", TimeseriesReceivedTotal)
	}
	if TimeseriesSentTotal != 0 {
		t.Errorf("TimeseriesSentTotal = %f, want 0", TimeseriesSentTotal)
	}
}

func TestParseMEProcessedCountLine_PartialMatch(t *testing.T) {
	// Line has ProcessedCount but not SentToPublicationCount
	TimeseriesVolumeMutex.Lock()
	origSent := TimeseriesSentTotal
	TimeseriesSentTotal = 0
	TimeseriesVolumeMutex.Unlock()
	defer func() {
		TimeseriesVolumeMutex.Lock()
		TimeseriesSentTotal = origSent
		TimeseriesVolumeMutex.Unlock()
	}()

	// Truncated line — contains "ProcessedCount:" but not the full pattern
	line := `ProcessedCount: 500`
	parseMEProcessedCountLine(line)

	TimeseriesVolumeMutex.Lock()
	defer TimeseriesVolumeMutex.Unlock()

	if TimeseriesSentTotal != 0 {
		t.Errorf("TimeseriesSentTotal = %f, want 0 for partial match", TimeseriesSentTotal)
	}
}
