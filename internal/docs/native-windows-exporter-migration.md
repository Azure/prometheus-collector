# AKS Native Windows Exporter Support

## Table of Contents

- [Overview](#overview)
- [What Changed](#what-changed)
- [For New Clusters](#for-new-clusters-no-action-needed)
- [For Existing Clusters](#for-existing-clusters)
- [ConfigMap Settings](#configmap-settings)
- [Troubleshooting](#troubleshooting)
- [Metrics Reference](#metrics-reference)

---

## Overview

The **AKS native Windows exporter** is a Windows metrics exporter that comes pre-installed on AKS Windows nodes (port 19182). The existing `windowsexporter` scrape target now supports both the native exporter and the manually-installed `windows_exporter` DaemonSet (port 9182) through **smart port auto-selection**.

There is no new target — the same `windowsexporter` target, same `job="windows-exporter"` job name, same keeplist, same recording rules, and same dashboards are used regardless of which exporter is running.

### Why use the native exporter?

| | Old (manual DaemonSet) | Native (pre-installed) |
|---|---|---|
| **Deployment** | Manual DaemonSet + firewall rules | Pre-installed on AKS Windows nodes |
| **Firewall rules** | Required (port 9182) | Not required |
| **Image management** | Customer-managed | Managed by AKS |
| **Port** | 9182 | 19182 |
| **Setup** | ConfigMap required to enable | Works automatically for new clusters |

Both exporters produce the same `windows_*` metrics, so dashboards and alerts work without modification.

---

## What Changed

### Default enabled for new deployments

The `windowsexporter` default changed from `false` to `true` in the no-ConfigMap code path (`SetDefaultScrapeSettings`). This means new clusters that don't have a ConfigMap will automatically scrape the native Windows exporter.

Existing clusters with a ConfigMap are unaffected — the ConfigMap value takes precedence.

### Port default

The port defaults to **19182** (the AKS native exporter port). If you were previously using the manually-installed `windows_exporter` DaemonSet on port 9182, you need to explicitly set the port in your ConfigMap:

```yaml
prometheus-collector-settings: |-
  windowsexporter_port = "9182"
```

### Same job name

The job name remains `windows-exporter`. No changes to recording rules or dashboards are needed.

### Metric name changes handled automatically

The native exporter (newer windows_exporter version) renamed some metrics. We've updated all recording rules and dashboards to handle both names automatically using PromQL `or` fallback, so **no customer action is required**:

| Old metric (port 9182) | New metric (port 19182) | Impact |
|---|---|---|
| `windows_system_boot_time_timestamp_seconds` | `windows_system_boot_time_timestamp` | Used in node count rules and dashboard variables |
| `windows_os_visible_memory_bytes` | `windows_memory_physical_total_bytes` | Used in memory utilization rules and dashboards |
| `windows_system_system_up_time` | _(no equivalent)_ | Only available on old exporter — not critical |

If you have **custom** dashboards or alerts referencing these metrics, you may need to update them to use the new names when migrating to the native exporter.

---

## For New Clusters (No Action Needed)

For new AKS clusters with Windows node pools, the native Windows exporter is scraped automatically:

- ✅ `windowsexporter` defaults to `true` (no ConfigMap needed)
- ✅ Port defaults to `19182` (native exporter)
- ✅ No DaemonSet to deploy
- ✅ No firewall rules to configure
- ✅ Metrics flow automatically under `job="windows-exporter"`

---

## For Existing Clusters

### If you were NOT using windowsexporter before

No action needed — when you enable `windowsexporter = true` in your ConfigMap, it will automatically use port 19182 (native exporter).

### If you ARE using the old manual DaemonSet (port 9182)

You have two options:

#### Option A: Keep using the old DaemonSet

Add the explicit port to your ConfigMap to preserve existing behavior:

```yaml
prometheus-collector-settings: |-
  windowsexporter_port = "9182"
```

#### Option B: Migrate to the native exporter

##### Step 1: Verify the native exporter is available

Ensure your AKS cluster version supports the native Windows exporter:

```bash
kubectl get nodes -l kubernetes.io/os=windows
```

##### Step 2: Remove the port override (or don't set one)

The default port (19182) points to the native exporter. Simply remove any `windowsexporter_port` setting from your ConfigMap, or don't add one.

##### Step 3: Confirm metrics are flowing

```promql
up{job="windows-exporter"}
windows_cpu_time_total{job="windows-exporter"}
```

##### Step 4: Remove the old DaemonSet

Once metrics are confirmed, remove the manually deployed DaemonSet:

```bash
kubectl delete daemonset windows-exporter -n monitoring
kubectl delete configmap windows-exporter-config -n monitoring
```

---

## ConfigMap Settings

### Enable/disable scraping

```yaml
default-scrape-settings-enabled: |-
  windowsexporter = true    # Enabled by default for new clusters
```

### Port override (for old DaemonSet users)

If you're using the manually-installed Windows exporter DaemonSet on port 9182, explicitly set the port:

```yaml
prometheus-collector-settings: |-
  windowsexporter_port = "9182"    # Keep using old exporter port
```

If you're using the native exporter (default), no port setting is needed.

### Full example (keeping old exporter)

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: ama-metrics-settings-configmap
  namespace: kube-system
data:
  default-scrape-settings-enabled: |-
    windowsexporter = true

  default-targets-scrape-interval-settings: |-
    windowsexporter = "30s"

  prometheus-collector-settings: |-
    windowsexporter_port = "9182"    # Explicit port for old DaemonSet
```

---

## Troubleshooting

### No metrics from the native exporter

**Symptoms**: `up{job="windows-exporter"}` returns no results or shows `0`.

**Solutions**:
1. **Check AKS version**: Ensure your AKS cluster version supports the native Windows exporter.
2. **Verify port**: Confirm that `windowsexporter_port` is set to `"19182"` in your ConfigMap (or that no ConfigMap exists for the default).
3. **Check scraping is enabled**: Ensure `windowsexporter = true` in your ConfigMap.
4. **Verify node OS label**: The scrape config filters by `kubernetes.io/os: windows`:
   ```bash
   kubectl get nodes --show-labels | findstr windows
   ```
5. **Check network policies**: Unlike the old exporter, no firewall rules should be needed, but network policies may block access to port 19182.

### Wrong port being used

**Symptoms**: Scraping targets show port 9182 when you expected 19182 (or vice versa).

**Explanation**: The port auto-selects based on ConfigMap presence:
- Default is always 19182 (native exporter)
- If you need port 9182, set `windowsexporter_port = "9182"` in your ConfigMap

**Solution**: Set the port explicitly in your ConfigMap:

```yaml
prometheus-collector-settings: |-
  windowsexporter_port = "9182"    # For old DaemonSet
```

### Dashboards show no data after migration

**Symptoms**: Grafana dashboards that previously showed Windows metrics are empty.

**Solutions**:
1. **Verify metrics**: Query raw metrics to confirm data collection:
   ```promql
   windows_cpu_time_total{job="windows-exporter"}
   ```
2. **Check job selector**: The job name is still `windows-exporter`. If you have custom dashboards, ensure they use this job name.
3. **Verify recording rules**: Ensure recording rules under `mixins/kubernetes/rules/windows.libsonnet` are deployed and evaluating.

### Scrape interval too slow

**Symptoms**: Metrics are stale or dashboards show gaps.

**Solution**: Reduce the scrape interval:

```yaml
default-targets-scrape-interval-settings: |-
  windowsexporter = "15s"
```

---

## Metrics Reference

The following 24 metrics are included in the minimal ingestion profile for the Windows exporter. These metrics are identical for both the manually-installed and native exporters.

### System metrics

| Metric | Description |
|--------|-------------|
| `windows_system_boot_time_timestamp_seconds` | System boot time as a Unix timestamp |
| `windows_system_system_up_time` | System uptime in seconds |

### CPU metrics

| Metric | Description |
|--------|-------------|
| `windows_cpu_time_total` | Total CPU time spent in each mode (user, idle, etc.) |

### Memory metrics

| Metric | Description |
|--------|-------------|
| `windows_memory_available_bytes` | Available memory in bytes |
| `windows_os_visible_memory_bytes` | Total visible (physical) memory in bytes |
| `windows_memory_cache_bytes` | Memory used by the file system cache |
| `windows_memory_modified_page_list_bytes` | Memory in the modified page list |
| `windows_memory_standby_cache_core_bytes` | Standby cache core bytes |
| `windows_memory_standby_cache_normal_priority_bytes` | Standby cache normal priority bytes |
| `windows_memory_standby_cache_reserve_bytes` | Standby cache reserve bytes |
| `windows_memory_swap_page_operations_total` | Total swap page operations |

### Disk metrics

| Metric | Description |
|--------|-------------|
| `windows_logical_disk_read_seconds_total` | Total disk read time in seconds |
| `windows_logical_disk_write_seconds_total` | Total disk write time in seconds |
| `windows_logical_disk_size_bytes` | Total disk size in bytes |
| `windows_logical_disk_free_bytes` | Free disk space in bytes |

### Network metrics

| Metric | Description |
|--------|-------------|
| `windows_net_bytes_total` | Total network bytes (sent + received) |
| `windows_net_packets_received_discarded_total` | Total inbound packets discarded |
| `windows_net_packets_outbound_discarded_total` | Total outbound packets discarded |

### Container metrics

| Metric | Description |
|--------|-------------|
| `windows_container_available` | Whether a container is available |
| `windows_container_cpu_usage_seconds_total` | Total container CPU usage in seconds |
| `windows_container_memory_usage_commit_bytes` | Container memory commit in bytes |
| `windows_container_memory_usage_private_working_set_bytes` | Container private working set memory |
| `windows_container_network_receive_bytes_total` | Total container network bytes received |
| `windows_container_network_transmit_bytes_total` | Total container network bytes transmitted |
