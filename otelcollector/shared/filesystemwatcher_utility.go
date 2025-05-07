package shared

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"os"
	"path/filepath"
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
	logFile := `C:\opt\microsoft\scripts\filesystemwatcher.txt`

	h := sha256.New()

	tokenFileExists := false
	if info, err := os.Stat(tokenFile); err == nil && !info.IsDir() {
		tokenFileExists = true
	}

	// Hash all files in initial paths
	for _, dir := range initialPaths {
		filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			return hashFileContents(h, path)
		})
	}

	// If TokenConfig.json exists, hash it
	if tokenFileExists {
		hashFileContents(h, tokenFile)
	}

	// Generate final combined hash
	finalHash := hex.EncodeToString(h.Sum(nil))

	// Compare to last stored hash
	lastHashBytes, _ := os.ReadFile(hashStore)
	lastHash := string(lastHashBytes)

	// Check if the hash has changed
	if finalHash != lastHash {
		// Update stored hash only if it changed
		os.WriteFile(hashStore, []byte(finalHash), 0644)

		// Log the change
		now := time.Now().Format(time.RFC3339)
		msg := fmt.Sprintf("Configuration changed at %s\n", now)

		os.MkdirAll(filepath.Dir(logFile), os.ModePerm)
		f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err == nil {
			defer f.Close()
			f.WriteString(msg)
		} else {
			fmt.Println("Failed to write to log file:", err)
		}
	}
}

func hashFileContents(h hash.Hash, path string) error {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	io.WriteString(h, path) // Include path in hash for stability
	_, err = io.Copy(h, f)
	return err
}
