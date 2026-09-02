package configmapsettings

import (
	"bytes"
	"os"
	"strconv"
	"strings"
)

const (
	windowsExporterDefaultPort         = "19182"
	windowsExporterPortEnv             = "AZMON_WINDOWS_EXPORTER_PORT"
	windowsExporterPlaceholder         = "$$WINDOWS_EXPORTER_PORT$$"
	windowsExporterMinimalMetricsRegex = "windows_system_boot_time_timestamp_seconds|windows_system_boot_time_timestamp|windows_system_system_up_time|windows_cpu_time_total|windows_memory_available_bytes|windows_os_visible_memory_bytes|windows_memory_physical_total_bytes|windows_memory_cache_bytes|windows_memory_modified_page_list_bytes|windows_memory_standby_cache_core_bytes|windows_memory_standby_cache_normal_priority_bytes|windows_memory_standby_cache_reserve_bytes|windows_memory_swap_page_operations_total|windows_logical_disk_read_seconds_total|windows_logical_disk_write_seconds_total|windows_logical_disk_size_bytes|windows_logical_disk_free_bytes|windows_net_bytes_total|windows_net_packets_received_discarded_total|windows_net_packets_outbound_discarded_total|windows_container_available|windows_container_cpu_usage_seconds_total|windows_container_memory_usage_commit_bytes|windows_container_memory_usage_private_working_set_bytes|windows_container_network_receive_bytes_total|windows_container_network_transmit_bytes_total"
)

func normalizeWindowsExporterPort(value string) (string, bool) {
	port := strings.TrimSpace(value)
	portNumber, err := strconv.Atoi(port)
	if err != nil || portNumber < 1 || portNumber > 65535 {
		return "", false
	}
	return port, true
}

func configuredWindowsExporterPort() string {
	if port, ok := normalizeWindowsExporterPort(os.Getenv(windowsExporterPortEnv)); ok {
		return port
	}
	return windowsExporterDefaultPort
}

func configureWindowsExporterTarget(contents []byte) []byte {
	return bytes.ReplaceAll(contents, []byte(windowsExporterPlaceholder), []byte(configuredWindowsExporterPort()))
}
