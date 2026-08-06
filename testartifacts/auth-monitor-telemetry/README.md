# Authentication monitor telemetry test

These manifests create one metrics endpoint and five monitors:

| Monitor | Authentication |
|---|---|
| ServiceMonitor | Basic Auth |
| ServiceMonitor | Bearer token from a Kubernetes Secret |
| ServiceMonitor | Bearer token from a file |
| PodMonitor | Basic Auth |
| PodMonitor | Bearer token from a Kubernetes Secret |

The `azmonitoring.coreos.com/v1` PodMonitor CRD does not support
`bearerTokenFile`, so there is no valid PodMonitor example for that field.

The test endpoint does not enforce authentication. The purpose of these
artifacts is to exercise generated scrape configuration and telemetry, not to
validate authentication at the HTTP server.

## Deploy

```powershell
kubectl apply -f .\testartifacts\auth-monitor-telemetry
```

If scoped Secret access is enabled, ensure the target allocator is configured
to read Secrets from the `ama-metrics-auth-telemetry-test` namespace.

The file-based ServiceMonitor uses the collector's standard service-account
token path:

```text
/var/run/secrets/kubernetes.io/serviceaccount/token
```

Run this only on a test cluster. When `DenyFSAccessThroughSMs` is enabled, the
file-based ServiceMonitor scrape job is intentionally dropped and its
file-specific telemetry remains false.

## Verify resources

```powershell
kubectl get servicemonitors.azmonitoring.coreos.com,podmonitors.azmonitoring.coreos.com -n ama-metrics-auth-telemetry-test
```

The following `ClusterCoreCapacity` dimensions should become true after the
next telemetry interval:

```text
ServiceMonitorBasicAuthEnabled
ServiceMonitorBearerTokenEnabled
ServiceMonitorBearerTokenFileEnabled
PodMonitorBasicAuthEnabled
PodMonitorBearerTokenEnabled
```

## Remove

```powershell
kubectl delete namespace ama-metrics-auth-telemetry-test
```
