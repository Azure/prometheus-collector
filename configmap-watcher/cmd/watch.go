package cmd

import (
	"context"
	"fmt"
	"os"
	"path"
	"sync"

	"go.uber.org/zap"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
)

const (
	fileMode                = 0640
	watcherNotificationFile = "inotifysettingscreated"
)

// WatchForChanges watches a configmap for changes and updates the settings files
func WatchForChanges(ctx context.Context, clientSet kubernetes.Interface, logger *zap.Logger, info *ConfigMapSync) error {
	if exists, err := configMapExists(clientSet, info.namespace, info.configmapName); !exists {
		if err != nil {
			return fmt.Errorf("unable to read configmap %s. Error: %w", info.configmapName, err)
		}

		logger.Info("Configmap does not exist. Creating inotifysettingscreated file.")
		_, err := os.Create(path.Join(info.settingsVolume, watcherNotificationFile))
		if err != nil {
			return fmt.Errorf("unable to create inotifysettingscreated file. Error: %w", err)
		}
	}

	mutex := &sync.Mutex{}
	for {
		logger.Info("Watch for changes in configmap...")
		watcher, err := clientSet.CoreV1().ConfigMaps(info.namespace).Watch(ctx,
			metav1.SingleObject(metav1.ObjectMeta{Name: info.configmapName, Namespace: info.namespace}))
		if err != nil {
			logger.Error("Unable to create watcher. Error: %s", zap.Error(err))
			return err
		}

		err = handleConfigmapUpdate(watcher.ResultChan(), logger, mutex, info.settingsVolume)
		if err != nil {
			logger.Error("Processing configmap update.", zap.Error(err))
			return err
		}
	}
}

func handleConfigmapUpdate(eventChannel <-chan watch.Event, logger *zap.Logger, mutex *sync.Mutex, settingsVolume string) error {
	for {
		event, open := <-eventChannel
		if open {
			mutex.Lock()
			switch event.Type {
			case watch.Added:
				logger.Info("Adding configmap")
				err := updateSettingsFiles(settingsVolume, event, logger)
				if err != nil {
					logger.Error("Unable to create settings files", zap.Error(err))
					return err
				}
			case watch.Modified:
				logger.Info("Updating configmap")
				err := updateSettingsFiles(settingsVolume, event, logger)
				if err != nil {
					logger.Error("Unable to update settings files", zap.Error(err))
					return err
				}
			case watch.Deleted:
				logger.Info("Deleting configmap")
				err := deleteSettingsFiles(settingsVolume, event, logger)
				if err != nil {
					logger.Error("Unable to delete settings files. Error: %s", zap.Error(err))
					return err
				}
			default:
				// Do nothing
				logger.Error(fmt.Sprintf("Unsupported event type '%s'", event.Type))
			}
			mutex.Unlock()
		} else {
			// If eventChannel is closed, it means the server has closed the connection
			logger.Info("Channel closed. Server has closed the connection.")
			return nil
		}
	}
}

func updateSettingsFiles(volumePath string, event watch.Event, logger *zap.Logger) error {
	updatedConfigMap, ok := event.Object.(*corev1.ConfigMap)
	if !ok {
		return fmt.Errorf("unable to cast event settings to ConfigMap")
	}

	isCurrentVersion, err := isCurrentVersion(updatedConfigMap.ResourceVersion, volumePath)
	if err != nil {
		return err
	}
	if isCurrentVersion {
		logger.Info("Configmap resource version is current. Skipping update.")
		return nil
	}

	err = removeFileIfExists(path.Join(volumePath, watcherNotificationFile))
	if err != nil {
		return err
	}

	for settingKey, settingValue := range updatedConfigMap.Data {
		logger.Info(fmt.Sprintf("Creating/updating settings file: %s ", settingKey))
		filePath := path.Join(volumePath, settingKey)
		err = os.WriteFile(filePath, []byte(settingValue), fileMode)
		if err != nil {
			return fmt.Errorf("unable to create/update file '%s'. Error: %w", filePath, err)
		}
	}

	logger.Info("Creating inotifysettingscreated file with resourceVersion " + updatedConfigMap.ResourceVersion)
	err = os.WriteFile(path.Join(volumePath, watcherNotificationFile), []byte(updatedConfigMap.ResourceVersion), fileMode)
	if err != nil {
		return fmt.Errorf("unable to create inotifysettingscreated file. Error: %w", err)
	}

	return nil
}

func deleteSettingsFiles(volumePath string, event watch.Event, logger *zap.Logger) error {
	err := removeFileIfExists(path.Join(volumePath, watcherNotificationFile))
	if err != nil {
		return err
	}

	if updatedConfigMap, ok := event.Object.(*corev1.ConfigMap); ok {
		for settingKey := range updatedConfigMap.Data {
			logger.Info("Deleting settings file: " + settingKey)
			filePath := path.Join(volumePath, settingKey)
			err = os.Remove(filePath)
			if err != nil {
				return fmt.Errorf("unable to delete file '%s'. Error: %w", filePath, err)
			}
		}
	}

	_, err = os.Create(path.Join(volumePath, watcherNotificationFile))
	if err != nil {
		return fmt.Errorf("unable to create inotifysettingscreated file. Error: %w", err)
	}

	return nil
}

func configMapExists(clientSet kubernetes.Interface, namespace, configMapName string) (bool, error) {
	_, err := clientSet.CoreV1().ConfigMaps(namespace).Get(context.TODO(), configMapName, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	return true, nil
}

func removeFileIfExists(filePath string) error {
	if err := os.Remove(filePath); err != nil {
		// Check if the error is due to the file not existing, otherwise panic
		if !os.IsNotExist(err) {
			return fmt.Errorf("error deleting file '%s'. Error: %w", filePath, err)
		}
	}

	return nil
}

func isCurrentVersion(resourceVersion, settingsVolume string) (bool, error) {
	version, err := os.ReadFile(path.Join(settingsVolume, watcherNotificationFile))
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}

	return len(version) > 0 && string(version) == resourceVersion, nil
}
