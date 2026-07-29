// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWaitForConfigFile(t *testing.T) {
	t.Run("returns immediately when the file already exists", func(t *testing.T) {
		file := filepath.Join(t.TempDir(), "targetallocator.yaml")
		require.NoError(t, os.WriteFile(file, []byte("config: value\n"), 0600))

		require.NoError(t, waitForConfigFile(file, 5*time.Second))
	})

	t.Run("does not wait when the timeout is disabled", func(t *testing.T) {
		file := filepath.Join(t.TempDir(), "missing.yaml")

		start := time.Now()
		require.NoError(t, waitForConfigFile(file, 0))
		assert.Less(t, time.Since(start), 500*time.Millisecond)
	})

	t.Run("waits for a file written after startup", func(t *testing.T) {
		file := filepath.Join(t.TempDir(), "targetallocator.yaml")
		go func() {
			time.Sleep(500 * time.Millisecond)
			_ = os.WriteFile(file, []byte("config: value\n"), 0600)
		}()

		require.NoError(t, waitForConfigFile(file, 5*time.Second))
	})

	t.Run("treats an empty file as not ready and times out", func(t *testing.T) {
		file := filepath.Join(t.TempDir(), "targetallocator.yaml")
		require.NoError(t, os.WriteFile(file, []byte{}, 0600))

		err := waitForConfigFile(file, 500*time.Millisecond)
		require.Error(t, err)
		assert.ErrorIs(t, err, fs.ErrNotExist)
	})

	t.Run("times out when the file never appears", func(t *testing.T) {
		file := filepath.Join(t.TempDir(), "missing.yaml")

		err := waitForConfigFile(file, 500*time.Millisecond)
		require.Error(t, err)
		assert.ErrorIs(t, err, fs.ErrNotExist)
	})

	t.Run("fails when the path is a directory", func(t *testing.T) {
		err := waitForConfigFile(t.TempDir(), time.Second)
		require.Error(t, err)
	})
}

func TestLoadWaitsForLateConfigFile(t *testing.T) {
	contents, err := os.ReadFile(filepath.Join("testdata", "config_test.yaml"))
	require.NoError(t, err)

	file := filepath.Join(t.TempDir(), "targetallocator.yaml")
	go func() {
		time.Sleep(500 * time.Millisecond)
		_ = os.WriteFile(file, contents, 0600)
	}()

	cfg, err := Load([]string{"--config-file=" + file, "--config-file-wait-timeout=5s"})
	require.NoError(t, err)
	require.NotNil(t, cfg)
}

func TestLoadFailsFastOnMissingConfigFileByDefault(t *testing.T) {
	file := filepath.Join(t.TempDir(), "missing.yaml")

	_, err := Load([]string{"--config-file=" + file})
	require.Error(t, err)
	assert.ErrorIs(t, err, fs.ErrNotExist)
}
