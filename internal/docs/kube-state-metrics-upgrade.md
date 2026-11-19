# Upgrading kube-state-metrics

This document describes how the `kube-state-metrics` (KSM) image shipped with the
Azure Monitor Metrics addon is versioned, how to upgrade it, and it records the
most recent upgrade (**v2.19.1-2 → v2.20.0-4**).

## TL;DR — what this upgrade changed

| | Before | After |
|---|---|---|
| Image tag | `v2.19.1-2` | `v2.20.0-4` |
| Upstream KSM version | `v2.19.1` | `v2.20.0` |
| dalec build revision | `-2` | `-4` |
| prometheus-community Helm chart | `8.0.0` | `8.4.0` (latest `8.4.1`) |
| Go toolchain (upstream) | — | `v1.26.6` |
| `k8s.io/client-go` (upstream) | — | `v0.36.3` |

Image: `mcr.microsoft.com/oss/v2/kubernetes/kube-state-metrics:v2.20.0-4`

## How KSM is versioned here

The addon consumes the Microsoft-built (dalec) KSM image from MCR:
`mcr.microsoft.com/oss/v2/kubernetes/kube-state-metrics:<tag>`.

The tag is `v<upstreamVersion>-<dalecRevision>`:

- **`<upstreamVersion>`** — the upstream [kubernetes/kube-state-metrics](https://github.com/kubernetes/kube-state-metrics/releases) release (e.g. `v2.20.0`).
- **`<dalecRevision>`** — the Microsoft build revision from [Azure/dalec-build-defs](https://github.com/Azure/dalec-build-defs/tree/main/specs/kube-state-metrics). The revision is bumped independently of the upstream version, usually to pick up CVE fixes in the Go toolchain and Go module dependencies (the same upstream source, rebuilt). Always pick the **highest** revision published to MCR for the chosen upstream version.

## Where the tag lives in this repo

Two files reference the KSM image tag and must be kept in sync:

| File | Field |
|------|-------|
| `.pipelines/azure-pipeline-build.yml` | `KUBE_STATE_METRICS_IMAGE` |
| `otelcollector/deploy/addon-chart/azure-monitor-metrics-addon/values-template.yaml` | `KubeStateMetrics.ImageTag` (and the `# ... corresponds to chart version` comment above it) |

In addition, the addon's hand-maintained KSM manifests carry an `app.kubernetes.io/version`
label that should track the **upstream** version (no `v`, no dalec revision):

| File | Field |
|------|-------|
| `otelcollector/deploy/addon-chart/azure-monitor-metrics-addon/templates/ama-metrics-ksm-*.yaml` | `app.kubernetes.io/version` (e.g. `2.20.0`) — 5 manifests; the deployment carries it twice |

> Note: `values-rashmi-operator-cfg.yaml` is a local developer override and is **not** part of the shipped tag; do not treat it as the source of truth.

## Upgrade procedure

1. **Find the latest upstream release** in [kubernetes/kube-state-metrics/releases](https://github.com/kubernetes/kube-state-metrics/releases).
2. **Confirm dalec has a spec for it** in [Azure/dalec-build-defs `specs/kube-state-metrics`](https://github.com/Azure/dalec-build-defs/tree/main/specs/kube-state-metrics). Read the version's `kube-state-metrics-<version>.yml` (and `matrix.yml`) to get the `REVISION`, `COMMIT`, and any dependency/CVE patches in the `changelog`.
3. **Verify the image + revision exist in MCR** (source of truth for what is actually pullable):
   ```powershell
   (Invoke-RestMethod "https://mcr.microsoft.com/v2/oss/v2/kubernetes/kube-state-metrics/tags/list").tags |
     Where-Object { $_ -like "v2.20*" } | Sort-Object
   ```
   Pick the highest `v<version>-<revision>` tag (here: `v2.20.0-4`).
4. **Map the upstream version to the prometheus-community Helm chart** version for the comment in `values-template.yaml` (see [helm-charts/charts/kube-state-metrics](https://github.com/prometheus-community/helm-charts/tree/main/charts/kube-state-metrics) — `Chart.yaml` `appVersion`). KSM `2.20.0` first shipped in chart `8.4.0`.
5. **Update both files** above to the new tag, and bump the `app.kubernetes.io/version` label in the `ama-metrics-ksm-*.yaml` manifests to the new upstream version.
6. **Collect the upstream changelog** (`git compare <old>...<new>`) for the PR description — see below.
7. Open the PR; CI will build/push the addon image referencing the new KSM tag.

Handy commands for steps 2 & 6:
```powershell
# dalec spec + revision for a version
gh api repos/Azure/dalec-build-defs/contents/specs/kube-state-metrics/kube-state-metrics-2.20.0.yml `
  --jq '.content' | % { [Text.Encoding]::UTF8.GetString([Convert]::FromBase64String($_)) }

# upstream changes between the current and next version
gh api repos/kubernetes/kube-state-metrics/compare/v2.19.1...v2.20.0 `
  --jq '{commits: .total_commits, files: (.files|length)}'
gh api repos/kubernetes/kube-state-metrics/releases/tags/v2.20.0 --jq '.body'
```

---

## Upstream changes: v2.19.1 → v2.20.0 (for the PR description)

Source: [v2.19.1...v2.20.0](https://github.com/kubernetes/kube-state-metrics/compare/v2.19.1...v2.20.0)
(200 commits, 122 files) and the [v2.20.0 release notes](https://github.com/kubernetes/kube-state-metrics/releases/tag/v2.20.0).

**Notes**
- Custom Resource State (CRS) metrics are now **feature-frozen**; no new CRS features will be accepted ([#3050](https://github.com/kubernetes/kube-state-metrics/pull/3050)). Use resource-state-metrics going forward.
- Builds with Go `v1.26.6`.
- Builds with `k8s.io/client-go` `v0.36.3`.

**[CHANGE]**
- `kube_pod_status_reason` now emits a row only for the reason that is actually set, instead of one zero/one row per known reason for every pod ([#3089](https://github.com/kubernetes/kube-state-metrics/pull/3089)). This reduces cardinality and can change existing queries/alerts that relied on the always-present zero rows.

**[FEATURE]**
- Add `kube_pod_status_disruption_reason` metric for pods evicted via the Eviction API ([#3088](https://github.com/kubernetes/kube-state-metrics/pull/3088))
- Add `MutatingAdmissionPolicy` and `MutatingAdmissionPolicyBinding` metrics ([#3019](https://github.com/kubernetes/kube-state-metrics/pull/3019))
- Add `ValidatingAdmissionPolicy` and `ValidatingAdmissionPolicyBinding` metrics ([#3014](https://github.com/kubernetes/kube-state-metrics/pull/3014))
- Add HPA scale up/down behavior tolerance metrics ([#3015](https://github.com/kubernetes/kube-state-metrics/pull/3015))
- Add `kube_node_spec_pod_cidrs` metric ([#3011](https://github.com/kubernetes/kube-state-metrics/pull/3011))
- Add `kube_pod_resourceclaim_info` metric for DRA ResourceClaim references ([#3005](https://github.com/kubernetes/kube-state-metrics/pull/3005))
- Add `kube_pod_init_container_status_last_terminated_exitcode` metric ([#3000](https://github.com/kubernetes/kube-state-metrics/pull/3000))
- Add init container `state_started` and `last_terminated_timestamp` metrics ([#2997](https://github.com/kubernetes/kube-state-metrics/pull/2997))
- Shard only resource mutation watch events ([#2993](https://github.com/kubernetes/kube-state-metrics/pull/2993))
- Add label generation support for ephemeral volumes in pods ([#2891](https://github.com/kubernetes/kube-state-metrics/pull/2891))
- Allow wildcards in annotation and label allowlists ([#2873](https://github.com/kubernetes/kube-state-metrics/pull/2873))
- Add access mode metric for PersistentVolumes ([#2823](https://github.com/kubernetes/kube-state-metrics/pull/2823))

**[ENHANCEMENT]**
- Reduce allocations in metric generation and header sanitization ([#3060](https://github.com/kubernetes/kube-state-metrics/pull/3060))
- Implement `cache.Store` Bookmark and `LastStoreSyncResourceVersion` in MetricsStore for client-go 0.36 compatibility ([#2965](https://github.com/kubernetes/kube-state-metrics/pull/2965))

**[BUGFIX]** (selection — full list in the release notes)
- Don't panic on a Deployment with an unset `rollingUpdate` value ([#3083](https://github.com/kubernetes/kube-state-metrics/pull/3083))
- Serialize config reloads so one does not strand another instance ([#3070](https://github.com/kubernetes/kube-state-metrics/pull/3070))
- Rebuild clients from the current kubeconfig on reload ([#3080](https://github.com/kubernetes/kube-state-metrics/pull/3080))
- Several discovery hardening / race fixes: read the GVK cache under the lock ([#3074](https://github.com/kubernetes/kube-state-metrics/pull/3074)), guard the store builder state shared with discovery ([#3075](https://github.com/kubernetes/kube-state-metrics/pull/3075)), check CRD fields instead of asserting them ([#3078](https://github.com/kubernetes/kube-state-metrics/pull/3078)), ignore non-served CRD versions ([#2940](https://github.com/kubernetes/kube-state-metrics/pull/2940)), stop the discoverer when its context is canceled ([#2977](https://github.com/kubernetes/kube-state-metrics/pull/2977))
- Fix three issues in the server request and shutdown paths ([#3068](https://github.com/kubernetes/kube-state-metrics/pull/3068))
- Escape reserved characters in metric help text ([#3067](https://github.com/kubernetes/kube-state-metrics/pull/3067))
- Don't panic on a custom resource metric that fails to compile ([#3066](https://github.com/kubernetes/kube-state-metrics/pull/3066))
- Preserve pagination metadata when filtering a sharded list ([#3065](https://github.com/kubernetes/kube-state-metrics/pull/3065))
- Never emit the same label name twice ([#3064](https://github.com/kubernetes/kube-state-metrics/pull/3064))
- Don't dereference an unset EndpointSlice port number ([#3063](https://github.com/kubernetes/kube-state-metrics/pull/3063)) / build each hint's labels independently ([#3076](https://github.com/kubernetes/kube-state-metrics/pull/3076)) / fix wrong labels applied to endpointslices ([#3058](https://github.com/kubernetes/kube-state-metrics/pull/3058))
- Write headers when only a later store has metrics ([#3061](https://github.com/kubernetes/kube-state-metrics/pull/3061))
- Don't emit nil metrics for unparseable pod IPs ([#3059](https://github.com/kubernetes/kube-state-metrics/pull/3059))
- Embed tzdata so named CronJob `spec.timeZone` values resolve ([#3022](https://github.com/kubernetes/kube-state-metrics/pull/3022)); emit the default suspend metric value ([#2999](https://github.com/kubernetes/kube-state-metrics/pull/2999)); guard nil suspend in next-schedule metric ([#2970](https://github.com/kubernetes/kube-state-metrics/pull/2970))
- Prevent leaked goroutines on CRD updates ([#3007](https://github.com/kubernetes/kube-state-metrics/pull/3007)); start the zombie-process reaper only once ([#3079](https://github.com/kubernetes/kube-state-metrics/pull/3079))
- Prevent duplicate cluster-scoped metrics when using `--namespaces` ([#2998](https://github.com/kubernetes/kube-state-metrics/pull/2998))
- Handle omitted `minReplicas` in HPA metrics ([#2991](https://github.com/kubernetes/kube-state-metrics/pull/2991)); skip an HPA status metric whose source is not set ([#3071](https://github.com/kubernetes/kube-state-metrics/pull/3071))
- Guard nil `Spec.Replicas` before dereferencing on Deployments ([#2971](https://github.com/kubernetes/kube-state-metrics/pull/2971))
- Skip landing page registration on `NewLandingPage` error ([#2937](https://github.com/kubernetes/kube-state-metrics/pull/2937))
- Handle nil path gracefully in StateSet metrics ([#2884](https://github.com/kubernetes/kube-state-metrics/pull/2884))

### Microsoft (dalec) build patches in `-4`

Beyond the upstream `v2.20.0` source, the dalec build revision `-4` rebuilds the
same source with CVE-patched Go toolchain and pinned module replacements
(from the dalec spec changelog / `gomod` edits):

- `golang.org/x/crypto` → `v0.55.0` (addresses CVE-2026-56854)
- `google.golang.org/grpc` → `v1.82.1` (addresses GHSA-hrxh-6v49-42gf)
- `github.com/google/cel-go` → `v0.29.0` (addresses GHSA-gcjh-h69q-9w9g)
- Go stdlib CVE fixes via the msft-golang toolchain bump (multiple HIGH stdlib advisories)

## Upstream Helm chart changes (chart 8.3.1 → 8.4.0)

The addon does **not** vendor the prometheus-community chart — it maintains its own
KSM manifests (`otelcollector/deploy/addon-chart/azure-monitor-metrics-addon/templates/ama-metrics-ksm-*.yaml`),
which are a customized fork of that chart. On every upgrade, diff the upstream chart
across the app-version boundary (last chart at the old appVersion → first chart at the
new one) and port anything relevant: **RBAC (`role.yaml`), container args/flags, probes,
ports, securityContext**.

For **2.19.1 → 2.20.0** the boundary is chart `8.3.1` → `8.4.0`, and the delta is
**version-only** — verified with:

```powershell
gh api "repos/prometheus-community/helm-charts/compare/kube-state-metrics-8.3.1...kube-state-metrics-8.4.0" `
  --jq '.files[] | select(.filename|startswith("charts/kube-state-metrics/")) | "\(.status)  \(.filename)"'
# => modified  charts/kube-state-metrics/Chart.yaml   (only the version/appVersion bump)
```

- `templates/role.yaml` (RBAC): **unchanged** between 8.3.1 and 8.4.0.
- `templates/deployment.yaml`, `service.yaml`, `serviceaccount.yaml`, etc.: **unchanged**.
- No new required args, flags, probes, or ports.

**Conclusion:** no RBAC/args/probe changes need to be ported into the addon's `ama-metrics-ksm-*.yaml`
for 2.20.0. The manifest change there is limited to the `app.kubernetes.io/version` label bump to
`2.20.0`; the functional changes are the image tag and the chart-version comment.

### New 2.20.0 metrics vs. RBAC / collectors

| New metric(s) | KSM collector | Enabled by default in the addon? | New RBAC needed? |
|---|---|---|---|
| PersistentVolume access mode | `persistentvolumes` | yes | no (already granted) |
| `kube_node_spec_pod_cidrs` | `nodes` | yes | no |
| init-container `state_started` / `last_terminated_*`, `kube_pod_status_disruption_reason`, `kube_pod_resourceclaim_info`, ephemeral-volume labels | `pods` | yes | no |
| HPA scale up/down behavior tolerance | `horizontalpodautoscalers` | yes | no |
| ValidatingAdmissionPolicy(+Binding), MutatingAdmissionPolicy(+Binding) | `validatingadmissionpolicies` / `mutatingadmissionpolicies` (new, **opt-in**) | no | **yes** if enabled |

Most new metrics extend collectors the addon already enables, so they light up as soon
as the image is bumped — no RBAC or config change required. The admission-policy
collectors are opt-in (upstream leaves them off by default too); to scrape them, add the
collector name to `KubeStateMetrics.Collectors` in `values-template.yaml` **and** add a
matching `admissionregistration.k8s.io` `list`/`watch` rule in `ama-metrics-ksm-role.yaml`.

## Impact / things to watch

- **Cardinality change**: `kube_pod_status_reason` now emits only the set reason ([#3089](https://github.com/kubernetes/kube-state-metrics/pull/3089)). Review any recording rules/alerts that assumed the previous always-present zero rows.
- **New metrics** (admission policies, DRA ResourceClaims, PV access modes, node pod CIDRs, init-container timestamps/exit codes, pod disruption reason) are additive; enable the corresponding collectors in `values-template.yaml` if you want them scraped.
- **CRS feature freeze**: no functional break, but new Custom Resource State features won't land upstream.

## References

- Microsoft dalec build definitions (source of the MCR image + revision/CVE patches): <https://github.com/Azure/dalec-build-defs/tree/main/specs/kube-state-metrics>
- MCR image repository: `mcr.microsoft.com/oss/v2/kubernetes/kube-state-metrics` — tags: <https://mcr.microsoft.com/v2/oss/v2/kubernetes/kube-state-metrics/tags/list>
- Upstream releases: <https://github.com/kubernetes/kube-state-metrics/releases>
- Upstream v2.19.1…v2.20.0 diff: <https://github.com/kubernetes/kube-state-metrics/compare/v2.19.1...v2.20.0>
- prometheus-community Helm chart (version ↔ appVersion mapping): <https://github.com/prometheus-community/helm-charts/tree/main/charts/kube-state-metrics>
- prometheus-community chart diff for this upgrade: <https://github.com/prometheus-community/helm-charts/compare/kube-state-metrics-8.3.1...kube-state-metrics-8.4.0>
