# Same-invocation settings transitions

These are fixture contracts for the existing selected ConfigTests workflow,
not another executor. Coverage owns the configprocessing specs and runtime
assertions; the scenario runner owns initial apply and final restoration.
No phase fields or expectation environment variables are added to manifests.
Paths below are relative to this directory in the verified candidate checkout.

## Lifecycle and evidence

Before any reset, refuse a pre-existing fixture Namespace. Save the original
data/absence of the four ConfigMaps in `catalog.json` and every declared initial
resource. Apply the manifest once, then run its selected configprocessing spec.
The spec applies the phase files sequentially, awaiting observable convergence
and querying fresh samples after each phase's start time. A fixed sleep or old
AMW sample is not convergence. Do not restart collectors to make reload pass.

Specs must defer restoration of their initial ConfigMap/Secret inputs even on
failure. The outer runner then restores the pre-slot baseline and verifies its
original data/absence. Delete only initially declared, newly created resources;
never delete a pre-existing Namespace. Keep snapshots containing Secrets in
memory, not logs, reports or artifacts.

Candidate TestKube source must include `otelcollector/test/scenarios/settings`
as well as the existing Ginkgo source. Resolve later fixture paths from that
verified checkout; do not download them from `main` or another revision.
The committed inputs and metadata checks are not runtime evidence.

## Alias and annotation namespace transitions

Run `customer-transitions-v1` or `customer-transitions-v2`, selected by
`config-processing-customer-transitions-v1/v2`. The existing schema-matched
customer-settings workflow is a carrier with explicit label replacement.
All five phase files live in the variant's directory.

| Phase input | Expected `cluster` | Annotation namespace with fresh samples |
|---|---|---|
| `initial.yaml` | `Customer_Config_POC` | `ama-settings-included` only |
| `alias-updated.yaml` | `Customer_Config_Updated` | `ama-settings-included` only |
| `alias-cleared.yaml` | actual non-alias cluster identity | `ama-settings-included` only |
| `namespace-updated.yaml` | actual non-alias cluster identity | `ama-settings-excluded` only |
| `namespace-removed.yaml` | actual non-alias cluster identity | neither; annotation scrape job removed |

Use complete replacement of ConfigMap `data`, not a merge that retains removed
keys. In v1, namespace removal deletes `pod-annotation-based-scraping`; in v2 it
removes that nested section from the complete `cluster-metrics` string.
The clear phase sets `cluster_alias = ""`. Derive the expected fallback
independently from actual collector `CLUSTER`: for `MAC=true`, use its last
slash-separated segment; otherwise use `CLUSTER` unchanged. Do not infer the
expected fallback by reading `AZMON_CLUSTER_LABEL` after clearing.

For each alias phase require new ingested samples from both
`podMonitor/ama-settings-included/settings-alias/0` and
`serviceMonitor/ama-settings-included/settings-alias/0` with the expected
`cluster`, plus the Windows kube-proxy metric below for every Windows node.
Check the `cluster` label on ingested metrics, not only generated config or
collector environment. PM/SM target the included pod regardless of annotation
namespace selection; they must remain healthy when annotations move or stop.
For namespace absence use the annotation job's target discovery/removal and
fresh sample timestamps, not historical-series existence or absence alone.

Initial resource inventory from `annotation-targets.yaml` + `monitor-targets.yaml`:
Namespaces `ama-settings-included` and `ama-settings-excluded`; Pod
`annotation-target` in each; Service `settings-monitor-target`, PodMonitor
`settings-alias`, ServiceMonitor `settings-alias` in `ama-settings-included`.
Only `kube-system/ama-metrics-settings-configmap` changes in later phases.

## Credential reload and allowlist transitions

Run `secret-transitions-v1/v2`, selected by
`config-processing-secret-transitions-v1/v2`, through the corresponding
basic-auth-smon workflow with label replacement.

1. Apply `allowed.yaml` and the initial `auth-targets.yaml`. Confirm the live
   endpoint rejects the deliberately invalid test username and both monitor
   scrape targets report authentication failure.
2. Replace Secret `ama-settings-auth/settings-auth-creds` with
   `auth-recovered.yaml`. Without restarting the collector or endpoint, await
   both generated auth configurations and authenticated target recovery.
   Require fresh ingested `my_metric` samples from both monitor jobs:
   `podMonitor/ama-settings-auth/settings-auth/0` and
   `serviceMonitor/ama-settings-auth/settings-auth/0`.
3. On the supported scoped-secret path only (Kubernetes >=1.37), replace the
   settings ConfigMap with the variant's `empty.yaml`, then `omitted.yaml`.
   After each transition, prove both authenticated monitors lose secret access;
   retaining cached credentials must not count as enforcement. Mutate the same
   declared dummy Secret if needed to distinguish a stale watch from a fresh
   resolution; never add an undeclared Secret. Reapply `allowed.yaml` and
   `auth-recovered.yaml`, then require fresh authenticated recovery again.
4. On lower current versions, report only the enforcement subcase unsupported,
   not the entire reload/recovery case and not a passing enforcement result.

The existing reference endpoint accepts fixed public test credentials
`admin`/`pwd`. This is invalid-credential -> valid-credential reload/recovery,
**not** successful rotation between two valid server passwords. Real server
credential rotation is explicitly user-deferred and excluded from coverage.
Do not add a different auth server or extend the reference application for it.

Initial resource inventory in `auth-targets.yaml`: Namespace `ama-settings-auth`;
Secret `settings-auth-creds`, Deployment `settings-auth-target`, Service
`settings-auth-target`, PodMonitor `settings-auth`, ServiceMonitor `settings-auth`
inside it. Later inputs mutate only that Secret and the settings ConfigMap.
All credentials are public dummy fixture values, never production credentials.

## Dedicated Windows kube-proxy ingestion

Run `windows-kube-proxy` with configprocessing selector
`config-processing-windows-kube-proxy-ingestion`, using the existing custom-node
workflow only as a label-replaced carrier. Its configprocessing wrapper calls
the existing metrics query helper; a querymetrics label alone would select
nothing in that carrier.

Require at least one actual Windows node and a healthy default job named
`kube-proxy-windows` for each one. The input enables that target, sets a 15s
interval and keeps `kubeproxy_sync_proxy_rules_duration_seconds.*`.
Require a newly sampled ingested
`kubeproxy_sync_proxy_rules_duration_seconds_count{job="kube-proxy-windows"}`
for each Windows node, matching the job's `instance` node identity. Prove the
metric really exists at the current node endpoint `:10249/metrics`; report an
unsupported endpoint/metric explicitly rather than substitute Windows exporter
or a Linux kube-proxy series. No Azure resource or nodepool provisioning occurs.
