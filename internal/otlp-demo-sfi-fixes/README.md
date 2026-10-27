# rashmi-otlp-test: OpenTelemetry demo Brotli (CVE-2026-111497) fixes

Record of the changes made on 2026-10-27 (UTC) to clear the Red Flag findings for the `rashmi-otlp-test` dev cluster in IcM 892876797 (out-of-bounds write in the Brotli decoder, CVE-2026-111497 and CVE-2026-114148), **without deleting the OpenTelemetry demo workload**. Use it to check what changed and to roll back if something goes wrong.

| | |
|---|---|
| Cluster | `rashmi-otlp-test` (resource group `rashmi-otlp-test`, australiaeast, subscription ContainerInsights_Dev_Rashmi `0e4773a2-8221-441a-a06f-17db16ab16d4`) |
| Namespace | `otel-demo` |
| Flagged node | `aks-userpool-26036444-vmss000000` |
| Registry | `rashmiotlp0e4773a2.azurecr.io` |

```powershell
az aks get-credentials -g rashmi-otlp-test -n rashmi-otlp-test --subscription 0e4773a2-8221-441a-a06f-17db16ab16d4
```

## Files

| File | Purpose |
|---|---|
| [`grafana-patch.json`](grafana-patch.json) | Strategic merge patch applied to `deploy/grafana` |
| [`fraud-detection/Dockerfile`](fraud-detection/Dockerfile) | Rebuilds the fraud-detection image on the MCR OpenJDK 17 distroless base |
| [`fraud-detection-patch.json`](fraud-detection-patch.json) | Strategic merge patch applied to `deploy/fraud-detection` |
| [`imagelist.yaml`](imagelist.yaml) | AKS Image Cleaner (Eraser) `ImageList` used to purge the old images from the node caches |

## What was flagged

| Workload | Image (as deployed) | Flagged linux/amd64 digest | Vulnerable package |
|---|---|---|---|
| `fraud-detection` | `rashmiotlp0e4773a2.azurecr.io/open-telemetry/demo:3.0.0-fraud-detection` | `sha256:d4810e29cf7677d9e2a683dcfa5071b8aa647be24eac8355e3b62d1a8d71ad44` | `libbrotli1 1.0.9-2+b6` (Debian 12, from `gcr.io/distroless/java17-debian12`) |
| `grafana` | `rashmiotlp0e4773a2.azurecr.io/grafana/grafana:13.1.0` | `sha256:6ea068891652aa6a65ca9065c26b89de939653803c836426970305c11fd00534` | `brotli-libs 1.2.0-r1` (Alpine 3.24) |

When the fix was made, the latest `gcr.io/distroless/java17-debian12` still shipped `libbrotli1 1.0.9-2+b6`, so moving to a newer upstream base would not have helped. Both images were replaced with MCR content instead.

## What changed

### 1. Grafana: switched to MCR Grafana

New image: `mcr.microsoft.com/oss/v2/grafana/grafana:v13.2.2-1` (Azure Linux 3.0, pulled as `sha256:0580681f337858e7085a400b2efdb4a98c46b1f952ca1781cca08fdef48584f9`).

- It contains no brotli OS package or shared library. The only brotli code in the image is the pure-Go `github.com/andybalholm/brotli` module compiled into the `grafana` binary, which upstream Grafana also has. The CVE is in the C library, not this module.
- The MCR entrypoint (`/usr/bin/grafana-entrypoint`) is equivalent to upstream `run.sh`, and the image runs as the same user (`472:0`).
- Grafana storage is an `emptyDir`, so the 13.1.0 → 13.2.2 upgrade has no persistent database to migrate.

The patch also adds two environment variables:

| Env var | Why |
|---|---|
| `GF_PLUGINS_PREINSTALL_DISABLED=false` | The MCR image defaults this to `true`, which turns off both the default preinstall list and `preinstall_sync`. The demo needs `GF_PLUGINS_PREINSTALL_SYNC=grafana-opensearch-datasource` (configmap `grafana`, key `plugins`) for its OpenSearch datasource. Its Jaeger datasource also needs the `jaeger` plugin, which Grafana 13.2 installs from the plugin catalog. |
| `GF_PLUGINS_PREINSTALL_AUTO_UPDATE=false` | MCR bundles its own unsigned build of the `prometheus` datasource plugin (13.1.7, allowed through `GF_PLUGINS_ALLOW_LOADING_UNSIGNED_PLUGINS`) in the root-owned `/usr/share/grafana/data/plugins-bundled`. With auto-update on (the default), the background installer tries to update it to the catalog version (13.2.4). It unloads the plugin first, then fails to delete the files (`unlinkat .../plugins-bundled/prometheus/3186.js.map: permission denied`), which leaves the Prometheus datasource with `Plugin not registered`. With auto-update off, plugins that are already present are left alone and missing ones are still installed. Plugin storage is an `emptyDir`, so every pod start still installs the latest catalog versions. |

Where this behavior comes from in the Grafana v13.2.2 source:

- `pkg/setting/setting_plugins.go`: the default preinstall list includes `prometheus`, and `preinstall_disabled` turns off both preinstall lists.
- `pkg/services/pluginsintegration/plugininstaller/service.go`: `installPlugins` skips plugins that are already present when `PreinstallAutoUpdate` is false.
- `pkg/plugins/manager/installer.go`: `Remove` unloads a plugin before deleting its files.

```powershell
kubectl patch deploy grafana -n otel-demo --type=strategic --patch-file grafana-patch.json --dry-run=server
kubectl patch deploy grafana -n otel-demo --type=strategic --patch-file grafana-patch.json
kubectl rollout status deploy/grafana -n otel-demo
```

The first rollout (revision 3, 00:21 UTC) didn't include `GF_PLUGINS_PREINSTALL_AUTO_UPDATE=false`. Its background installer hit the failed update described above, and the Prometheus datasource reported `Plugin not registered` until revision 4 rolled out at 00:28:49 UTC. Revision 4 is the current state; `prometheus` registered at 00:28:55.

### 2. fraud-detection: rebuilt on MCR OpenJDK 17 distroless

[`fraud-detection/Dockerfile`](fraud-detection/Dockerfile) copies `/usr/src/app/` (the application jar and the OTel Java agent) unchanged from the original image onto `mcr.microsoft.com/openjdk/jdk:17-distroless`. That base is the Microsoft Build of OpenJDK 17 on Azure Linux 3.0 and has no brotli.

- The original `WORKDIR`, `JAVA_TOOL_OPTIONS`, and `java -jar fraud-detection-1.0-all.jar` entrypoint are kept.
- It runs as the base image's non-root `app` user (uid 101) instead of distroless `nonroot` (uid 65532).

Built with ACR task run `cr1` (1m8s). This is the equivalent command:

```powershell
az acr build -r rashmiotlp0e4773a2 --platform linux/amd64 `
  -t open-telemetry/demo:3.0.0-fraud-detection-mcr-openjdk17-20261026 `
  internal/otlp-demo-sfi-fixes/fraud-detection
```

| | |
|---|---|
| Result | `rashmiotlp0e4773a2.azurecr.io/open-telemetry/demo:3.0.0-fraud-detection-mcr-openjdk17-20261026`, digest `sha256:7d2da6c62a913fe932b58e5a5b764f1176a621f86ee96616a70036987927aaa4` (linux/amd64 only) |
| Runtime base | `mcr.microsoft.com/openjdk/jdk:17-distroless@sha256:c1f579232d45eb76a3a53a935a149cacf07b9521cf1a31f8ac87a793e251f666` |
| Source image | `rashmiotlp0e4773a2.azurecr.io/open-telemetry/demo:3.0.0-fraud-detection@sha256:1cdfd1bcf476914c39afff9d91144fe7a6bffd6b897b4e29b20485791c8ec6a5` |

```powershell
kubectl patch deploy fraud-detection -n otel-demo --type=strategic --patch-file fraud-detection-patch.json --dry-run=server
kubectl patch deploy fraud-detection -n otel-demo --type=strategic --patch-file fraud-detection-patch.json
kubectl rollout status deploy/fraud-detection -n otel-demo
```

The new pod started at 00:24 UTC (revision 2).

### 3. Old images purged from the node caches

The cluster has AKS Image Cleaner (Eraser) enabled, so the old images were removed with a manual `ImageList`. The list also includes `docker.io/grafana/grafana:13.1.0`, the reference Grafana revision 1 used.

```powershell
kubectl apply -f imagelist.yaml --dry-run=server
kubectl apply -f imagelist.yaml
kubectl get imagelist imagelist -o jsonpath='{.status}'
```

- Status: `{"failed":0,"skipped":1,"success":4}`. That's the 4 Linux nodes; the Windows node is skipped.
- Remover logs on `aks-userpool-26036444-vmss000000` show `removed image` for `grafana/grafana:13.1.0` (image ID `sha256:e76fd1761e3c…`) and `demo:3.0.0-fraud-detection` (image ID `sha256:3e154e99b004…`).
- Every other node logged `image is not on node`.

Eraser ran one removal job when the `ImageList` was created (00:33 UTC) and deleted the finished job 10 minutes later (`delayOnSuccess: 10m`), which is when the status timestamp (00:43 UTC) was written. The `ImageList` itself was left in place. To delete it, run `kubectl delete imagelist imagelist`.

## Verification (2026-10-27, re-checked at 00:45 UTC)

| Check | Result |
|---|---|
| Grafana pod | All 4 containers ready, 0 restarts, image `mcr.microsoft.com/oss/v2/grafana/grafana@sha256:0580681f…` |
| Grafana `/grafana/api/health` | `version 13.2.2`, `database ok` |
| Grafana plugins | 18 `Plugin successfully installed`, no `Failed to install plugin`, and no update attempts. `prometheus` 13.1.7 (MCR-bundled, unsigned and allowed) is registered; `jaeger` 13.0.5 and `grafana-opensearch-datasource` 2.34.6 are installed from the catalog. |
| Jaeger datasource | Health `OK` (`Data source is working`) |
| OpenSearch datasource | A count query over the last hour returned 25,100 log documents (00:35 UTC). The health check reports `Index not found: otel-logs-*`; see the known issues below. |
| Prometheus datasource | The plugin works. The health check fails with `connection refused` because the in-cluster Prometheus is crash-looping, which predates this change (see below). |
| Grafana sidecars | Dashboards were written to `/tmp/dashboards` and datasources to `/etc/grafana/provisioning/datasources`; the datasources reload returned `200 OK`. |
| fraud-detection pod | 1/1 ready, 0 restarts, image `…/open-telemetry/demo@sha256:7d2da6c6…`. Microsoft OpenJDK 17.0.21 and the App Insights Java agent 3.7.7 started. It's consuming orders: the `total count` reached 75 by 00:44 UTC (the pod started at 00:24 UTC). |
| Brotli in the new images | Neither image contains a brotli OS package or `libbrotli*` shared library. |
| Node caches | The old images are gone from every node, per the Eraser logs and node `status.images`. |

To re-check:

```powershell
kubectl get pods -n otel-demo -l 'app.kubernetes.io/name in (grafana,fraud-detection)' -o wide
kubectl logs deploy/grafana -n otel-demo -c grafana | Select-String 'Plugin successfully installed|Failed to install'
kubectl get --raw /api/v1/namespaces/otel-demo/services/grafana:80/proxy/grafana/api/plugins/prometheus/settings | ConvertFrom-Json | Select-Object id, signature, @{n='version';e={$_.info.version}}
kubectl logs deploy/fraud-detection -n otel-demo --tail=5
kubectl get --raw /api/v1/namespaces/otel-demo/services/grafana:80/proxy/grafana/api/health
```

The plugin settings call fails with a 404 if the `prometheus` plugin isn't registered. To scan the images independently, use any SBOM or vulnerability scanner, for example `trivy image <ref>`.

## Known issues (not caused by the image changes)

| Issue | Details |
|---|---|
| In-cluster Prometheus crash-looping | `deploy/prometheus` has been crash-looping since 2026-10-25 07:00 UTC, `OOMKilled` at its `400Mi` memory limit (482 restarts as of 2026-10-27 00:45 UTC). Until it's fixed (for example, by raising the memory limit), the Grafana Prometheus datasource fails with `connection refused`. It wasn't changed here. |
| fraud-detection OOM restarts | The pod had 6 `OOMKilled` restarts at its `300Mi` limit before this change. In both the old and new image, the App Insights auto-instrumentation webhook sets the pod's `JAVA_TOOL_OPTIONS`, which replaces the image's own value (`-Xmx180m` and the OTel agent). Watch the restart count. |
| OpenSearch datasource health check | It returns `Index not found: otel-logs-*`, although the daily `otel-logs-2026-10-25`, `-26`, and `-27` indices exist and log queries return data. It isn't tied to the image: the plugin is downloaded from the Grafana catalog at every pod start through the same `preinstall_sync` setting as before, so any restart picks up the latest version (2.34.6, released 2026-10-21). |
| `Errno 98 Address in use` in `grafana-sc-dashboard` and `grafana-sc-alerts` | The three k8s-sidecar containers share the pod network, and none of them sets a health port, so all three try to bind the same default port. The containers stay ready and keep syncing. The sidecar images weren't changed. |
| App Insights Java agent warning `Unrecognized field "instrumentationOptions"` | The unchanged `ad` and `kafka` pods log it too. It comes from the injected agent configuration, and the agent starts normally. |

## Follow-ups

- **Update the source manifests.** The demo was deployed with `kubectl apply`, so re-applying the original manifests reverts both deployments to the flagged images. Update the image references and the two Grafana env vars in the source manifests, or re-apply these patches afterwards.
- **Delete the old ACR tags when you no longer need them.** They were kept on purpose for rollback, and registry scans can still report them. Deleting the fraud-detection tag also removes the `SOURCE_IMAGE` that [`fraud-detection/Dockerfile`](fraud-detection/Dockerfile) builds from, so retag it first if you may need to rebuild.

  ```powershell
  az acr repository delete -n rashmiotlp0e4773a2 --image grafana/grafana:13.1.0 --yes
  az acr repository delete -n rashmiotlp0e4773a2 --image open-telemetry/demo:3.0.0-fraud-detection --yes
  ```

- **Expect more demo images to be flagged.** The IcM also lists Node.js and the .NET runtime as affected, so the demo's Node.js and .NET services may show up in later scans. Rebuild them on patched runtimes once those are available.
- **Tag IcM 892876797.** Add the `AzRF.SDPInProgress` tag, as the Red Flag instructions ask. The finding clears after the next Red Flag scan, which can lag.

## Rollback

The old images are still in ACR, so rolling back re-pulls them, and they'll be flagged again.

| Deployment | Revisions | Roll back to the pre-change state |
|---|---|---|
| `grafana` | 1: original `docker.io` and `quay.io` references<br>2: ACR mirror of `grafana:13.1.0` (the state before this change)<br>3: first MCR rollout (had the Prometheus plugin issue)<br>4: current | `kubectl rollout undo deploy/grafana -n otel-demo --to-revision=2` |
| `fraud-detection` | 1: `demo:3.0.0-fraud-detection` (before)<br>2: current | `kubectl rollout undo deploy/fraud-detection -n otel-demo --to-revision=1` |

Revision numbers change after an undo, so run `kubectl rollout history deploy/<name> -n otel-demo` first. Backups of the pre-change YAML are kept locally, not in this repo, because they contain the App Insights connection string.
