package shared

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Only being called for windows
func CheckForFilesystemChanges() {
	initialPaths := []string{
		`C:\etc\config\settings`,
		`C:\etc\config\settings\prometheus`,
	}
	tokenFile := `C:\opt\genevamonitoringagent\datadirectory\mcs\metricsextension\TokenConfig.json`
	hashStore := `C:\last_config_hash.txt`
	logFile := `C:\filesystemwatcher.txt`
	debugLog := `C:\debug.txt`

	debug := func(format string, a ...any) {
		msg := fmt.Sprintf(format, a...)
		msg = time.Now().Format("2006-01-02 15:04:05") + " " + msg + "\n"

		f, err := os.OpenFile(debugLog, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			// Fallback to stdout if file can't be written
			fmt.Printf("Failed to write debug log: %v\n", err)
			return
		}
		defer f.Close()

		f.WriteString(msg)
	}

	debug("Starting CheckForFilesystemChanges")

	h := sha256.New()
	var allFiles []string

	// Collect all files from initial paths
	for _, dir := range initialPaths {
		debug("Walking directory: %s", dir)
		filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				debug("Skipping %s due to error: %v", path, err)
				return nil
			}
			if !d.IsDir() {
				allFiles = append(allFiles, path)
				debug("Queued file: %s", path)
			}
			return nil
		})
	}

	// If TokenConfig.json exists, add it to the list
	if info, err := os.Stat(tokenFile); err == nil && !info.IsDir() {
		allFiles = append(allFiles, tokenFile)
		debug("Token file added: %s", tokenFile)
	} else {
		debug("Token file not found or is a directory: %v", err)
	}

	// Sort the list to ensure deterministic hash input
	sort.Strings(allFiles)
	debug("Sorted %d files for hashing.", len(allFiles))

	// Hash file paths and contents
	for _, path := range allFiles {
		debug("Hashing file: %s", path)
		if err := hashFileContents(h, path); err != nil {
			debug("Failed to hash file %s: %v", path, err)
		}
	}

	// Generate final hash
	finalHash := hex.EncodeToString(h.Sum(nil))
	debug("Final combined hash: %s", finalHash)

	// Compare with last stored hash
	lastHashBytes, err := os.ReadFile(hashStore)
	lastHash := ""
	if err != nil {
		debug("Could not read last hash from %s: %v", hashStore, err)
	} else {
		lastHash = strings.TrimSpace(string(lastHashBytes))
	}
	debug("Previous hash: %s", lastHash)

	if lastHash == "" {
		debug("First run detected — storing initial hash.")
		os.WriteFile(hashStore, []byte(finalHash), 0644)
	} else if finalHash != lastHash {
		debug("Hash has changed — updating and logging.")
		os.WriteFile(hashStore, []byte(finalHash), 0644)
		now := time.Now().Format(time.RFC3339)
		msg := fmt.Sprintf("Configuration changed at %s\n", now)

		if f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644); err == nil {
			defer f.Close()
			f.WriteString(msg)
			debug("Wrote change log to %s", logFile)
		} else {
			debug("Failed to write to log file: %v", err)
		}
	} else {
		debug("No change in configuration.")
	}
}

func hashFileContents(h hash.Hash, path string) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("unable to open file %s: %w", path, err)
	}
	defer f.Close()

	io.WriteString(h, path) // Include path in hash
	_, err = io.Copy(h, f)
	if err != nil {
		return fmt.Errorf("error reading file %s: %w", path, err)
	}
	return nil
}
