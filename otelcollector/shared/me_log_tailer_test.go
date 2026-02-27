package shared

import (
	"os"
	"path/filepath"
	"testing"
	"time"
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

func TestTailMELogFile(t *testing.T) {
	// Save and restore globals
	TimeseriesVolumeMutex.Lock()
	origReceived := TimeseriesReceivedTotal
	origSent := TimeseriesSentTotal
	origBytes := BytesSentTotal
	TimeseriesReceivedTotal = 0
	TimeseriesSentTotal = 0
	BytesSentTotal = 0
	TimeseriesVolumeMutex.Unlock()

	defer func() {
		TimeseriesVolumeMutex.Lock()
		TimeseriesReceivedTotal = origReceived
		TimeseriesSentTotal = origSent
		BytesSentTotal = origBytes
		TimeseriesVolumeMutex.Unlock()
	}()

	// Create temp file to simulate ME log
	dir := t.TempDir()
	logFile := filepath.Join(dir, "MetricsExtensionConsoleDebugLog.log")
	f, err := os.Create(logFile)
	if err != nil {
		t.Fatal(err)
	}

	// Start the tailer in background
	go TailMELogFile(logFile)

	// Give tailer time to open and seek to end
	time.Sleep(200 * time.Millisecond)

	// Write ME log lines (these appear AFTER the tailer seeks to end)
	lines := []string{
		"2026-02-18 12:00:00 Starting MetricsExtension\n",
		"2026-02-18 12:00:10.000 Info DefaultAccount ReceivedCount: 500 ProcessedCount: 480 ProcessedBytes: 12345 SentToPublicationCount: 475 SentToPublicationBytes: 11234\n",
		"2026-02-18 12:00:15 Some other log line\n",
		"2026-02-18 12:00:20.000 Info EventsProcessedLastPeriod: 300 TotalEventsProcessed: 1000\n",
		"2026-02-18 12:00:30.000 Info Account2 ReceivedCount: 100 ProcessedCount: 100 ProcessedBytes: 2000 SentToPublicationCount: 95 SentToPublicationBytes: 1900\n",
		"2026-02-18 12:00:40.000 Info EventsProcessedLastPeriod: 150 TotalEventsProcessed: 1150\n",
	}
	for _, line := range lines {
		f.WriteString(line)
	}
	f.Sync()

	// Wait for tailer to process (polls every 1s)
	time.Sleep(3 * time.Second)

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
	f.Close()
}

func TestTailMELogFile_NoNewContent(t *testing.T) {
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

	// Create temp file with existing content (before tailer starts)
	dir := t.TempDir()
	logFile := filepath.Join(dir, "MetricsExtensionConsoleDebugLog.log")
	// Write some content BEFORE the tailer starts — it should seek past this
	err := os.WriteFile(logFile, []byte("preexisting content\n"), 0644)
	if err != nil {
		t.Fatal(err)
	}

	go TailMELogFile(logFile)

	// Wait — tailer should skip existing content, no new content arrives
	time.Sleep(2 * time.Second)

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
