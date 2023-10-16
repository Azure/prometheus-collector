package cmd_test

import (
	"context"
	"errors"
	"os"
	"path"
	"testing"
	"time"

	"go.goms.io/aks/configmap-watcher/cmd"
	"k8s.io/client-go/kubernetes"

	"github.com/stretchr/testify/assert"
)

func TestWatchForChangesSuccessConfigmapAdded(t *testing.T) {
	t.Logf("Case 1: Watch Create")
	data := loadConfigmapFromFile(t, "../tests/settings-configmap-create.yaml")
	fakeClient, watch := createFakeClient()
	// the command runs indefinitely in a loop, therefore we need to cancel it after a while
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// Simulate watch event
	tmpDir := executeConfigmapWatch(ctx, t, fakeClient)
	data.ResourceVersion = "abc"
	watch.Add(data)
	time.Sleep(1 * time.Second)

	// Wait for the context to be done
	<-ctx.Done()

	// Assert result
	files, _ := os.ReadDir(tmpDir)
	checkSettingFilesAreCreated(t, files)
	version, _ := os.ReadFile(path.Join(tmpDir, "inotifysettingscreated"))
	assert.Equal(t, data.ResourceVersion, string(version))
}

func TestWatchForChangesSuccessConfigmapUpdated(t *testing.T) {
	data := loadConfigmapFromFile(t, "../tests/settings-configmap-create.yaml")
	fakeClient, watchInterface := createFakeClient()
	// the command runs indefinitely in a loop, therefore we need to cancel it after a while
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Simulate watch event
	tmpDir := executeConfigmapWatch(ctx, t, fakeClient)
	data.ResourceVersion = "123"
	watchInterface.Add(data)
	time.Sleep(1 * time.Second)

	files, _ := os.ReadDir(tmpDir)
	checkSettingFilesAreCreated(t, files)
	version, _ := os.ReadFile(path.Join(tmpDir, "inotifysettingscreated"))
	assert.Equal(t, "123", string(version))

	dataUpdated := loadConfigmapFromFile(t, "../tests/settings-configmap-update.yaml")
	dataUpdated.ResourceVersion = "1234"
	watchInterface.Modify(dataUpdated)
	time.Sleep(1 * time.Second)

	// Wait for the context to be done
	<-ctx.Done()

	// Assert final result
	files, _ = os.ReadDir(tmpDir)
	checkSettingFilesAreCreated(t, files)
	version, _ = os.ReadFile(path.Join(tmpDir, "inotifysettingscreated"))
	assert.Equal(t, "1234", string(version))
}

func TestWatchForChangesSuccessConfigmapDeleted(t *testing.T) {
	data := loadConfigmapFromFile(t, "../tests/settings-configmap-create.yaml")
	fakeClient, watchInterface := createFakeClient()
	// the command runs indefinitely in a loop, therefore we need to cancel it after a while
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Simulate watch event
	tmpDir := executeConfigmapWatch(ctx, t, fakeClient)
	data.ResourceVersion = "123"
	watchInterface.Add(data)
	time.Sleep(1 * time.Second)

	files, _ := os.ReadDir(tmpDir)
	checkSettingFilesAreCreated(t, files)
	version, _ := os.ReadFile(path.Join(tmpDir, "inotifysettingscreated"))
	assert.Equal(t, "123", string(version))

	watchInterface.Delete(data)
	time.Sleep(1 * time.Second)

	// Wait for the context to be done
	<-ctx.Done()

	// Assert final result
	files, _ = os.ReadDir(tmpDir)
	assert.Equal(t, 1, len(files))
	assert.True(t, fileExists(files, "inotifysettingscreated"))
	version, _ = os.ReadFile(path.Join(tmpDir, "inotifysettingscreated"))
	assert.Equal(t, "", string(version))
}

func fileExists(files []os.DirEntry, fileName string) bool {
	for _, file := range files {
		if file.Name() == fileName {
			return true
		}
	}

	return false
}

func checkSettingFilesAreCreated(t *testing.T, files []os.DirEntry) {
	assert.Equal(t, 9, len(files))
	assert.True(t, fileExists(files, "inotifysettingscreated"))
	assert.True(t, fileExists(files, "default-targets-scrape-interval-settings"))
	assert.True(t, fileExists(files, "pod-annotation-based-scraping"))
	assert.True(t, fileExists(files, "prometheus-collector-settings"))
	assert.True(t, fileExists(files, "schema-version"))
	assert.True(t, fileExists(files, "config-version"))
	assert.True(t, fileExists(files, "debug-mode"))
	assert.True(t, fileExists(files, "default-scrape-settings-enabled"))
	assert.True(t, fileExists(files, "default-targets-metrics-keep-list"))
}

func executeConfigmapWatch(ctx context.Context, t *testing.T, fakeClient kubernetes.Interface) (settingsVolume string) {
	// Implement the specific behavior for this test case
	var cli cmd.KubeClient = &KubectlMock{
		clientSet: fakeClient,
	}
	tmpDir := t.TempDir()
	rootCmd := cmd.NewKubeCommand(cli)
	rootCmd.SetArgs([]string{
		"--kubeconfig-file=/config/fake/kubeconfig",
		"--settings-volume=" + tmpDir,
		"--configmap-name=ama-metrics-settings-configmap",
		"--configmap-namespace=kube-system",
	})

	go func() {
		if err := rootCmd.ExecuteContext(ctx); err != nil && !errors.Is(ctx.Err(), context.DeadlineExceeded) {
			t.Error("Command execution failed: %w", err)
			return
		}
	}()

	return tmpDir
}
