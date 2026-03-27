# TSG: Proxy / Authenticated Proxy Issues

Run `tsg_errors`, look for HTTP proxy and AMCS connection errors. Then:

1. **Basic proxy** — ama-metrics supports unauthenticated HTTP proxies via AKS outbound proxy config. Check `tsg_config` → "HTTP Proxy Enabled"
2. **Authenticated proxy (NOT supported)** — ama-metrics does NOT currently support proxies that require authentication (username/password). If customer reports `ama-metrics cannot connect to AMCS when proxy has authentication`, confirm this is a known unsupported scenario
3. **Proxy bypass** — customer can configure `NO_PROXY` to bypass proxy for specific endpoints. AMCS and MDM endpoints should be in the bypass list if possible
4. **Escalation** — if this is a hard requirement for the customer, file a feature request on the prometheus-collector GitHub repo
5. **Customer documentation:**
   - [Network firewall requirements for monitoring Kubernetes](https://learn.microsoft.com/en-us/azure/azure-monitor/containers/kubernetes-monitoring-firewall)
   - [AKS outbound network and FQDN rules](https://learn.microsoft.com/en-us/azure/aks/outbound-rules-control-egress)
