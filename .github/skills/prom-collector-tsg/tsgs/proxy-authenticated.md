# TSG: Proxy / Authenticated Proxy Issues

Run `tsg_errors`, look for HTTP proxy and AMCS connection errors. Then:

1. **Basic proxy** — ama-metrics supports unauthenticated HTTP proxies via AKS outbound proxy config. Check `tsg_config` → "HTTP Proxy Enabled"
2. **Authenticated proxy (NOT supported)** — ama-metrics does NOT currently support proxies that require authentication (username/password). If customer reports `ama-metrics cannot connect to AMCS when proxy has authentication`, confirm this is a known unsupported scenario
3. **Proxy bypass** — do not assume `NO_PROXY` covers both configuration and ingestion. In the checked Linux mdsd/MCS application-proxy path, AMCS requests use an explicit proxy without applying `NO_PROXY`. DCE exclusions alone are not a reliable fix while mdsd handles configuration. See the OTel prerequisite below
4. **Escalation** — if this is a hard requirement for the customer, file a feature request on the prometheus-collector GitHub repo
5. **Customer documentation:**
   - [Network firewall requirements for monitoring Kubernetes](https://learn.microsoft.com/en-us/azure/azure-monitor/containers/kubernetes-monitoring-firewall)
   - [AKS outbound network and FQDN rules](https://learn.microsoft.com/en-us/azure/aks/outbound-rules-control-egress)

## Arc + AMPLS with selective proxy bypass

**Prerequisite: enable OTel to avoid the mdsd configuration path.** Merge this key into the existing `ama-metrics-settings-configmap` in `kube-system`; preserve its other settings:

```yaml
data:
  opentelemetry-metrics: |-
    enabled = true
```

This sets `AZMON_FULL_OTLP_ENABLED=true`. Startup skips mdsd and starts MetricsExtension without `-LocalControlChannel`, retaining `-TokenSource AMCS`. It does not disable the proxy for all extension traffic or remove the mdsd package.

- [ConfigMap example](../../../../otelcollector/configmaps/ama-metrics-settings-configmap-otel.yaml)
- [ConfigMap parser](../../../../otelcollector/shared/configmap/mp/tomlparser-opentelemetry-metrics.go)
- [Startup gate](../../../../otelcollector/main/main.go)
- [MetricsExtension launch](../../../../otelcollector/shared/process_utilities_linux.go)

Use the actual DCE configuration and metrics ingestion URL hostnames in the bypass list, not just their DNS CNAME/`privatelink` aliases. Configuration and ingestion can use different DCEs. Confirm private DNS resolves the destinations to reachable private IPs and that the required DCEs are associated with AMPLS. AMCS also uses `global.handler.control.monitor.azure.com` for bootstrap/redirects; DCE bypass does not eliminate that dependency.

For Arc, `az connectedk8s update --proxy-skip-range` can update the bypass list. Preserve existing exclusions. `az connectedk8s show` did not expose the list in the inspected ARM response; inspect `global.noProxy` in the `azure-arc` Helm release in `azure-arc-release`, then verify the running metrics pods received `NO_PROXY`/`no_proxy`.

**MetricsExtension honors `NO_PROXY` for configuration and ingestion in this OTel path.** Enable OTel to remove the mdsd blocker and configure the endpoint exclusions to retain the proxy for other traffic. Linux MetricsExtension documents environment-proxy support with `proxyDefinitionMode=1`. When troubleshooting a customer's deployment, verify the image supports this setting, mdsd is not running after settings take effect, and both fresh configuration retrieval and ingestion succeed over the intended routes. Do not treat continued ingestion from cached configuration as proof of configuration connectivity.

The public network requirements also state that a combination of HTTPS proxy and private cluster is not currently supported. Confirm the support boundary for the customer's Arc + AMPLS topology before recommending this as a supported workaround. This is not a way to enable authenticated-proxy support.

### References

- [Reference incident: IcM 699230150 (internal)](https://portal.microsofticm.com/imp/v5/incidents/details/699230150/summary). Customer-identifying details are intentionally omitted.
- [Arc outbound proxy configuration](https://learn.microsoft.com/en-us/azure/azure-arc/kubernetes/quickstart-connect-cluster#connect-using-an-outbound-proxy-server)
- [Arc proxy update command](https://learn.microsoft.com/en-us/cli/azure/connectedk8s?view=azure-cli-latest#az-connectedk8s-update)
- [DCE configuration and ingestion endpoints](https://learn.microsoft.com/en-us/azure/azure-monitor/data-collection/data-collection-endpoint-overview)
- [Linux MetricsExtension proxy behavior (internal)](https://eng.ms/docs/products/geneva/metrics/metricsagentsdk/webproxy)
- [mdsd application-proxy initialization, release/1.36 (internal)](https://dev.azure.com/msazure/One/_git/Compute-Runtime-Tux?path=/mdsdutil/HttpProxySetup.cc&version=GBrelease/1.36)
- [MCS HTTP client proxy configuration, LinuxAgent (internal)](https://dev.azure.com/msazure/One/_git/EngSys-MDA-CloudAgent?path=/src/shared/mcsmanager/lib/src/McsManager.h&version=GBLinuxAgent)

The native-source finding is from the inspected source branches, not a runtime test of the customer's exact mdsd binary.
