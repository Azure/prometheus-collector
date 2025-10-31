package testhelpers

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v2"
)

// CreateTempFile writes content to a file at dir/name, ensuring the directory exists.
func CreateTempFile(dir, name, content string) (string, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return "", fmt.Errorf("failed to write file %s: %w", path, err)
	}

	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("failed to stat file %s: %w", path, err)
	}
	if info.Size() != int64(len(content)) {
		return "", fmt.Errorf("file %s has incorrect size: expected %d, got %d", path, len(content), info.Size())
	}

	return path, nil
}

// MustCreateTempFile wraps CreateTempFile and panics if any error occurs.
// Intended for use in tests where panics fail the test immediately.
func MustCreateTempFile(dir, name, content string) string {
	path, err := CreateTempFile(dir, name, content)
	if err != nil {
		panic(err)
	}
	return path
}

// CopyDirectoryFiles copies all non-directory files from srcDir into dstDir.
func CopyDirectoryFiles(srcDir, dstDir string) error {
	if err := os.MkdirAll(dstDir, 0755); err != nil {
		return fmt.Errorf("failed to create destination directory %s: %w", dstDir, err)
	}

	entries, err := os.ReadDir(srcDir)
	if err != nil {
		return fmt.Errorf("failed to read source directory %s: %w", srcDir, err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		srcPath := filepath.Join(srcDir, entry.Name())
		dstPath := filepath.Join(dstDir, entry.Name())

		data, err := os.ReadFile(srcPath)
		if err != nil {
			return fmt.Errorf("failed to read file %s: %w", srcPath, err)
		}

		if err := os.WriteFile(dstPath, data, 0644); err != nil {
			return fmt.Errorf("failed to write file %s: %w", dstPath, err)
		}
	}

	return nil
}

// CheckEnvVars verifies that the environment matches the expected key/value pairs.
func CheckEnvVars(expected map[string]string) error {
	for key, value := range expected {
		if os.Getenv(key) != value {
			return fmt.Errorf("expected %s to be %s, but got %s", key, value, os.Getenv(key))
		}
	}
	return nil
}

// SetEnvVars applies the provided environment variables.
func SetEnvVars(values map[string]string) error {
	for key, value := range values {
		if err := os.Setenv(key, value); err != nil {
			return fmt.Errorf("failed setting %s: %w", key, err)
		}
	}
	return nil
}

// UnsetEnvVars removes the supplied environment variables.
func UnsetEnvVars(keys []string) {
	for _, key := range keys {
		_ = os.Unsetenv(key)
	}
}

// ReadYAMLStringMap loads a YAML file containing a string -> string map.
func ReadYAMLStringMap(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", path, err)
	}

	if len(data) == 0 {
		return map[string]string{}, nil
	}

	result := make(map[string]string)
	if err := yaml.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal YAML from %s: %w", path, err)
	}

	return result, nil
}
