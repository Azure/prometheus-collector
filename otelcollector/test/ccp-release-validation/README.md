# CCP Release Validation

End-to-end validation reports for the **CCP** (control-plane) variant of the
`prometheus-collector` agent — the `ama-metrics-ccp` deployment that runs in the AKS control-plane
namespace on an underlay cluster.

Each report validates one released `-ccp` image against a real AKS standalone environment, with
metrics ingested into and queried back from a real Azure Monitor Workspace.

## Reports

| Image | Release | Result |
|---|---|---|
| [`7.2.0-main-07-27-2026-2169504d-ccp`](7.2.0-main-07-27-2026-2169504d-ccp.md) | 07-27-2026 | ✅ PASS |

## What a report covers

1. **ConfigMap schema handling** — legacy `v1`, complete `v2`, and an **incomplete `v2`**
   (optional section omitted) that regression-tests
   [PR #1582](https://github.com/Azure/prometheus-collector/pull/1582).
2. **Parse-level evidence** — agent logs showing detected schema and final effective scrape settings.
3. **Data-level evidence** — PromQL queries against the AMW, ≥15 min after each ConfigMap is
   applied, confirming exactly the expected control-plane jobs ingest real samples.

## Test design principle

Every scenario sets values **flipped away from the agent's built-in defaults**
(`apiserver=true, etcd=true`, everything else `false`). This is what makes the results
falsifiable: "ConfigMap honoured" and "silently fell back to defaults" produce different,
observable outcomes in the AMW. A scenario that merely restates the defaults proves nothing.

## Assets

* `configmaps/` — the exact ConfigMaps applied.
  * `00-cx1-silence.yaml` → applied to the **cx-1 underlay's** `kube-system` (stops cx-1's *own*
    control plane from emitting `controlplane-*` series under the same `cluster` label).
  * `01-v1.yaml`, `02-v2-full.yaml`, `03-v2-incomplete.yaml` → applied to the **overlay
    (customer) cluster's** `kube-system`, which is where the CCP `configmap-watcher` reads from.
* `tools/Query-Amw.ps1` — run a raw PromQL query against an AMW Prometheus endpoint.
* `tools/Verify-Scenario.ps1` — query the AMW and attribute each returned series to *our* CCP
  agent vs cx-1's own control plane by matching the `instance` label against the live pod list.

## Related

* Skill: `.claude/skills/ccp-deployto-standalone` — how the environment is built and patched.
* Skill: `.claude/skills/ccp-update-prodimage` — how the CCP tag is rolled into `aks-rp`.
