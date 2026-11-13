# Repeatable ama-metrics cluster scenarios

`matrix.json` assigns many settings variants to one compatible host. Linux and
Windows tests share that host; variants modifying cluster-wide ConfigMaps run
sequentially with reset and verified restoration, not on separate clusters.
Separate cluster configurations are needed for incompatible dataplanes,
identity baselines, or installation ownership.

## Status and boundaries

The runner is a candidate-image adapter, not an approval/merge bot. A successful
plan, Bicep build, mock test or Azure preflight is **not** a live test pass.
`result.json` reports a pass only after terminal-passed TestKube executions with
nonempty Ginkgo JSON/JUnit, candidate runtime
image verification before and after the tests, and a recorded retained-resource
inventory. Missing
images/settings/adapters and incompatible profiles fail rather than use a released
addon or silently substitute another configuration.

Live candidate tests must execute through the existing TestKube workflows, using
the coverage owner's pinned workflow adapter. The harness invokes only the selected
workflow and rejects missing provenance, artifacts or executed cases. **No live
candidate pass is recorded by the offline evidence in this repository; a matching
protected producer and full candidate validation are required for a live pass.
It never falls back to direct local Ginkgo or treats installation as a pass. It captures
the exact submitted execution IDs, immutable test-source SHA and runner-image
digest, terminal passed status and nonempty JUnit/artifacts. Legacy `main`/`latest`
defaults are not candidate provenance.
`regionTests` is a separate region/rule validation, not a cluster-health substitute.
The run-specific health monitors keep `myapp_temperature` as well as `up`, so
the fresh-ingestion query can verify a marked sample from this run.

For proxy-enabled AKS clusters, TestKube bootstrap copies the existing
`httpProxyConfig` into inline global workflow container defaults so Git-clone init
containers receive it even when admission only injects regular containers.
Both environment-variable casings retain the complete `effectiveNoProxy` list,
including AKS-injected defaults, and bypass the internal TestKube gRPC endpoint
`testkube-api-server:8089` for artifact uploads. If core `az aks show` omits this
field, bootstrap privately reads the same cluster through stable ARM API `2026-09-01` without
requiring `aks-preview`; a missing effective list still fails closed.
The cluster read and Helm stdin values are private; no proxy values enter command
arguments or logs. Inline values are persisted in the API Deployment, so
credential-bearing proxy URLs are rejected rather than stored there; AKS does not
support proxy username/password authentication. Absent or explicitly disabled
proxy configurations add no defaults.

## Prerequisites

Use Python 3.10+ and the packages pinned in `clusters/requirements.txt`, Azure CLI
with Bicep, kubectl, Helm, Git Bash, jq, envsubst and the TestKube CLI compatible
with the explicitly selected chart version. The pinned runner image supplies Go;
tests use each suite's module-pinned Ginkgo CLI. Install dependencies only when missing:

```powershell
python -m pip install -r otelcollector\test\scenarios\clusters\requirements.txt
python otelcollector\test\scenarios\clusters\test_runner.py --mutations
```

Provide the settings owner's `settings/catalog.json`, `manifest.schema.json`, and
variant manifests in the checkout, or pass `--settings-root` to the delivered
catalog in another checkout. Fixture paths resolve relative to each manifest,
remain inside its checkout, and reuse existing fixtures without copying them.
Catalog reset/restore metadata must match the existing ConfigTests lifecycle.
Optional `helmValues` is accepted for validation but requires a separately owned
`helmChartInstall` target; the addon candidate adapter refuses it. `addonParams`
must never be mistaken for customer Helm values or silently ignored.

```powershell
python otelcollector\test\scenarios\run-scenario.py validate --settings-root C:\path\to\settings --evidence C:\temp\ama-validation
python otelcollector\test\scenarios\run-scenario.py plan --scenario default --settings-root C:\path\to\settings --evidence C:\temp\ama-plan
python otelcollector\test\scenarios\run-scenario.py preflight --scenario default --settings-root C:\path\to\settings --location eastus2 --evidence C:\temp\ama-preflight
```

The build subscription is explicitly restricted to
`9b96ebbd-c57a-42d1-bbe9-b69296e4c7fb`. No shared CI, settings-session, stopped, or
failed cluster is an implicit target. The runner creates a separately owned
resource group and validates its recorded ownership tags before recording the
parent and AKS-managed resource groups.
Each campaign profile uses one deterministic persistent resource group. The first
run provisions it; later builds reuse it only after verifying ownership, deployment,
location, topology and live AKS state. Persistent resources have no expiry tag and
the runner refuses to clean them up. Older run-owned groups are not adopted.
Pipeline 979 uses a sequential stage-level exclusive lock per profile. Direct
execution outside the corresponding locked 979 stage is refused; each attempt has
a separate execution ID and ingestion marker.

The retained `operator-secret-watch` AKS cluster and Azure Monitor workspace
have resource-scoped `scenario-retain` `CanNotDelete` locks. After verifying
ownership, the runner requires an effective `CanNotDelete` or `ReadOnly` lock on
each resource before using the cluster. It does not create locks; if protection
is missing, an authorized operator must add a resource-scoped lock before reuse.
These management locks are separate from the pipeline execution lock and can
block deletion of child resources. They do not survive resource recreation.

Before provisioning any host in a full campaign, run the read-only aggregate
admission once against its clean, exact-source checkout:

```powershell
python otelcollector\test\scenarios\run-scenario.py campaign-preflight --scenario all --build-id 4401 --source-root C:\path\to\candidate-checkout --test-source-sha <test-commit> --evidence C:\temp\ama-campaign-capacity
```

`--scenario four-lane` selects default, classic Azure CNI, ACNS and OTLP without
the isolated secret-watch and Cilium rows; it still checks their combined regional peak.
`--scenario` also accepts one of the six runnable campaign rows. The
`campaign-preflight.json` success receipt binds `sourceSha`, `testSourceSha` and the supplied
producer `buildId` to a `placements` map and regional `compute`/`quota` evidence.
The producer ID is a correlation field here; the existing authenticated candidate
verifier remains mandatory before deployment. `acns` and `acns-cilium` run in `eastus2`;
`default`, `classic-azure-cni` and `otlp` run in `centralus`.
`acns-cilium` is included in `all` after its successful live proof. Its resource group
has a `CanNotDelete` lock and retains the same narrow bootstrap grants for reuse.
The managed-addon handoff uses the AKS resource API, not the CLI disable path
that deletes the fixture's DCR/DCRAs. It restores and verifies the owned DCR
association while leaving the lock intact. A partial handoff is resumed on the
same retained cluster, never by recreating it.
The Cilium proof also installs and reads back a DNS inspection policy limited to
the TestKube namespace's workload-identity pods. DNS requests are inspected with
a wildcard allow rule; all non-DNS ports remain allowed. Cilium requires this
policy to emit DNS metrics, as documented in the
[ACNS metrics prerequisites](https://learn.microsoft.com/azure/aks/container-network-observability-how-to#prerequisites).
New pools' autoscaler maximum **and** declared surge count toward the simultaneous
regional and VM-family peak. Verified existing pools are already in current usage;
only their declared surge is added. An admitted existing host that disappears fails
closed rather than being recreated without admission.

The pipeline must require successful admission and use its per-row location
outputs. Failure emits no success receipt. This is a point-in-time check, not a
quota/capacity reservation: other workloads can consume quota afterward. Existing
per-row live SKU/version/quota checks still run before each deployment. Do not
repeat full-campaign admission after creating its hosts and count those hosts
twice, lower pool/surge limits, or delete retained hosts to fit quota.

## Candidate receipt and execution

Supply `--build-id` for a successful pipeline-440 build. The harness invokes the
SRE-owned `.pipelines/pr-e2e/fetch_candidate.py` authenticated verifier before
provisioning. It checks the build's full source SHA and downloads all five
same-build protected image/runner artifacts, retaining the entire
`verified-candidate` evidence directory. Local JSON, a URL or a bare digest is
not accepted as a substitute.
Only after authenticated verification, the adapter maps producer `config-reader`
to its internal `cfg` component and `indexRef` to `manifestRef`. Downloaded receipts
remain unchanged; runtime image IDs must match the verified index or platform child digest.

`--source-root C:\path\to\candidate-checkout` uses an existing clean
candidate checkout without changing this worktree's branch; by default the source
is this checkout. `--test-source-root` defaults to that checkout and must match
the expected immutable `--test-source-sha`; the pipeline uses its own checked-out
`Build.SourceVersion`. The producer receipt still binds the reusable toolchain to
the product build; execution evidence separately records the actual test commit.
Test packages, workflow scripts,
chart and reference fixtures must be clean and committed. Selected settings metadata/resource hashes are
recorded separately; a content hash is not a trusted build-source attestation.
Each image is immutable and records `name` (`collector`, `targetallocator`, `cfg`),
`os`, `arch`, and `ref` (`registry/repository@sha256:<64 hex digits>`).
Windows collector entries also need `osVersion`, for example `10.0.20348.0`;
the host and image must match the Windows build family, not necessarily patch.

Multi-architecture components need their common digest-pinned `manifestRef`
and an entry for each actual runtime child digest. ARM64 profiles require
ARM64 candidate entries before provisioning. The downloaded `runner.json` bytes must match `testRunner.receiptSha256`,
and its source/image must match the verified nested runner fields. The runner is
a reused, digest-pinned toolchain; its test source is fetched separately, not
claimed to have been baked into that image.

The calling protected runner must pin/control the verifier checkout.
`--receipt-tools-root` supports the SRE tools checkout before integration; it must
never be selected by untrusted PR inputs.

```powershell
python otelcollector\test\scenarios\run-scenario.py run --scenario default --build-id 123456 --receipt-tools-root C:\path\to\trusted-sre-tools --source-root C:\path\to\candidate-checkout --test-source-sha <test-commit> --testkube-version 2.15.0 --location centralus --evidence C:\temp\ama-candidate-run
```

The adapter initializes the managed addon's token secret, disables the addon,
waits for its workloads to disappear, then installs a generated copy of the
existing addon chart with candidate workload images pinned. It does not edit the
tracked chart or keep two controllers owning the same workloads.
Kubeconfig is explicitly supplied to kubectl and the TestKube adapter; the user's
default kubeconfig is never switched. Use `--kubectl C:\path\to\kubectl.exe` when
the default executable is unavailable. `--tools-bin` prepends an explicitly chosen
scoped TestKube CLI/jq directory to the child process's PATH, not global config.
The runner never silently chooses another executable.

The owned host also installs an explicitly versioned TestKube chart, records its
package hash, sets MinIO's volume group to 1001, and places MongoDB on the system
pool with a Recreate rollout for its single-writer volume, and schedules the
Linux-only built-in templates hook on Linux nodes. After dependency rollouts it
restarts and waits for the API, replaying bucket initialization if MinIO's earlier
unavailability exhausted its retries; API readiness alone does not ensure buckets.
It creates a query-only managed identity/federation for the tests-job
service account, scoped to the **new AMW only**. The calling identity receives the
same read-only role because the fresh-query probe forwards its token over stdin
into the collector pod. This requires role-assignment permission and directory
lookup of the caller's object ID; Graph access for a CI service principal is not
assumed. Neither identity gets a subscription-wide data role. Installation/
federation is not evidence that a workflow ran or exchanged a token. The existing
adapter receives the verified source/image, selected workflow and label override,
with `false 0 ConfigTests` to prohibit a second fixture lifecycle. Evidence must
contain exactly one submitted execution per invocation with native absolute artifact paths and
exactly one recursively located `ginkgo.json` and `junit.xml`.

Every settings slot snapshots the four catalog ConfigMaps and all affected fixture
resources in memory, removes only those ConfigMaps, applies the variant, waits for
propagation, runs its OR-combined existing configprocessing labels, and restores
and reads back the baseline even after failure. Restoration failure aborts the
batch. Negative config cases run their existing expected-error assertions; a
generic unhealthy cluster does not count as success.
The six default/custom-config combinations in
`internal/docs/CONFIGPROCESSINGTESTS.md` now have settings-catalog variants,
including custom valid/invalid configs with default targets disabled. The full
candidate matrix selects every compatible variant for PR validation. The shipped
addon chart hard-codes `MODE` and `WINMODE` to `advanced` and documents it as the
only supported mode; the historical simple-mode request is not a supported
candidate deployment.
Fixture-declared namespaces must not already exist. Transition specs may change
initially declared resource identities within the selected workflow; no separate
phase executor or manifest fields are supported.
Customer v1/v2 alias slots also run `querymetrics` / `cluster-identity` with
`EXPECTED_CLUSTER=Customer_Config_POC` **before restoration**. This proves fresh
alias/annotation-namespace ingestion; configprocessing alone does not.
The Cilium lane also requires `managed-acns-cilium` in each of these slots.
Catalog `overrideWorkflowLabels: true` declares intentional source-filter
replacement and is retained in `selection.json`; filters are replaced, not appended.

Health suites use existing reference workloads and scrape fixtures. Kubelet checks
do not require lazily created runtime-error counters from error-free nodes. JUnit
records executed/failed/skipped counts; skipped tests are not claimed as executed.
Pool-specific runtime image evidence is separate from the suite verdict.
One passing Windows spec does not imply every Windows OS/pool was checked.

Evidence includes command timings/logs, deployment outputs, per-suite JUnit and
Ginkgo JSON, per-node/component `imageID` observations, results and
`retained-resources.json` with resource IDs and tags.
Secret-bearing command output is redacted; snapshots remain in memory.
Failed deployments retain `deployment-error.txt` with secret-bearing environment
values redacted. Command history survives a separate explicit cleanup invocation.
Private role-assignment listing failures expose only allowlisted Azure error codes
in the raised error and `result.json`; unknown errors and all other output stay private.
New provisioning requires a ServicePrincipal so the independently authorized
bootstrap can cover managed groups even if the runner times out.
Service-principal runs create their owned resource group, then wait up to 20 minutes
for an administrator bootstrap before deploying. The runner prints the exact
`az role assignment create` command in `rbac-bootstrap.ps1` plus
`rbac-bootstrap.json`. An authorized administrator runs the script in their own
Azure session; it retries only `AuthorizationFailed` on
`Microsoft.Authorization/roleAssignments/write` for at most 10 minutes. No token
or credential is transferred to the runner.
Keep that script running while ARM creates the cluster: it discovers the sibling
groups through the owned AKS and AMW provider associations, grants their narrow
delegations and SRE Readers, then exits after both are confirmed. Missing groups
at terminal deployment state fail explicitly; the script is independent of the
runner's timeout.
The parent-group delegation is **Role Based Access Control Administrator**,
with condition version 2.0 restricting both assignment creation and
deletion to Network Contributor and Monitoring Data Reader for service principals.
It additionally permits Reader creation only for the SRE managed identity;
the custom-CA profile retains its existing Reader allowance.
Managed-group delegations permit only Reader creation for that same SRE principal.
None can delegate Owner, User Access Administrator or itself.
Only groups matching the persistent ownership contract can be reused.
The gate requires the exact principal, group, role and condition; missing or wider
delegations do not satisfy it. Timeout fails and retains any created resources for
inspection. Actual ARM deployment remains the permission-propagation check, not
the presence of a grant. Service-principal deployments and Monitoring Data Reader
service-principal writes share one retry implementation: only role-assignment
write authorization failures (including wrapped ARM errors) retry, for at most
10 minutes or the remaining execution budget, with 30-second exponential backoff
capped at 5 minutes. Direct writes recheck the exact unconditional assignment
after a denial; credentials are refreshed only when the error requests it.
User writes remain single-attempt, and unrelated failures are not retried.
Bootstrap prechecks skip existing exact workspace grants on reused hosts.

Bootstrap retains Reader on the owned parent, AKS node and AMW-managed resource
groups, plus Monitoring Data Reader on the AMW itself, for SRE principal
`e2bd6e9c-e75e-438b-8f57-2bb62327f346`. Existing exact, unconditional
ServicePrincipal assignments are reused, including manually assigned GUIDs.
These read grants are never revoked and require no separate per-run SRE approval.
Legacy pipeline delegations without the principal-bound Reader permission require
an authorized, one-time migration through this script at their next bootstrap.
Only the exact known legacy condition is updated in place on the same assignment,
preserving its scope, principal and roles; the approved condition is read back.
Unknown or wider conditions fail without deletion or replacement. Partial
read-grant completion is a failure, not a successful run.

Persistent hosts retain their conditional administrator delegation and narrower
workload grants for future runs. The runner refuses bootstrap revocation for
persistent groups, and the pipeline does not revoke them automatically.
For older **nonpersistent** retained runs only, an authorized user can revoke the
exact recorded administrator assignment:

```powershell
python otelcollector\test\scenarios\run-scenario.py revoke-bootstrap --evidence C:\temp\ama-candidate-run
```

It validates the exact recorded scope, principal, role and condition, deletes
only the recorded assignment ID, verifies its absence, and confirms the
narrower role assignments are unchanged. It does not delete infrastructure.
Explicit cleanup waits for the recorded AKS node and AMW managed resource groups
to disappear as well as the parent. Incomplete receipts discover parent-linked managed groups,
including retries after the parent has disappeared. Failed discovery is an
explicit incomplete cleanup, not a pass. It never directly
deletes an unrelated managed group. The execution-local kubeconfig is removed even
when cloud cleanup fails. CI publication must unconditionally exclude credential
files in case the process was killed before `finally`.
When an administrator bootstrap was recorded, cleanup also verifies its role
assignment disappeared after deleting the owned group.
A four-hour execution budget bounds commands; retention recording and kubeconfig
removal still run after the budget expires. `--max-run-seconds` changes that budget.
The cleanup action applies only to earlier nonpersistent executions:

```powershell
python otelcollector\test\scenarios\run-scenario.py cleanup --evidence C:\temp\ama-candidate-run
```

No automatic image push, ADO pipeline queue, review approval, commit or merge is
performed. A CI caller must publish the evidence directory (including failed-run
JUnit and `result.json`) as an artifact even when the runner exits nonzero; local
files alone are not a published CI result.

## Contributing another real scenario

1. Copy the nearest `clusters/*.bicepparam`, changing only the infrastructure
   dimension being exercised. Reuse the AKS module; combine compatible pools
   rather than cloning a host for each settings variant.
2. Add one entry to `matrix.json` using the existing schema: declare the parameter
   path, actual cluster features/platforms, compatible catalog variant IDs, Ginkgo
   labels and tier. Features are extensible strings, not a closed
   enum. Unsupported installation/proof adapters need an explicit `blockedReason`;
   a new feature name is not proof of implementation.
3. Use the existing settings catalog for customer settings, keeping resource paths
   manifest-relative and preserving its reset/restore lifecycle. Helm values belong
   to an isolated customer-chart target, never `addonParams`. Empty Ginkgo labels
   must not select all suites.
4. Run `test_runner.py --mutations`, matrix `validate`, Bicep module/parameter
   compilation and the scenario's read-only `preflight`. Add a focused rejecting
   case when introducing an incompatibility, and confirm it fails when that guard
   is removed. Then run the same candidate lifecycle with authorized resources;
   retain JUnit, imageIDs, timing and retained-resource evidence before claiming coverage.

No separate per-scenario runner, fixture copies, or additional scenario schema is
needed. The existing profiles exercise this extension path; planned axes remain
coverage gaps until their real adapter and live evidence exist.

## Auto-upgrade and combination semantics

There is no Kubernetes-version-history matrix. Ordinary runs resolve the currently
supported stable **KubernetesOfficial** regional default and every provisioned AKS
host must report effective `autoUpgradeProfile.upgradeChannel=stable` from Azure.
The observation is retained in `effective-upgrade.json`; template intent alone is
not enough. An unavailable default or incompatible Windows OS fails instead of
pinning older Kubernetes or opting into LTS/preview.

Preflight compiles the chosen parameters and uses their literal `nodePools`
inventory, including explicit `maxSurge`, not a duplicate topology in Python.
It checks each exact SKU's regional subscription restrictions, advertised CPU
architecture, regional/family vCPU quota at autoscaler maximum **plus surge**, and
the separately declared proxy VM count. Capacity is checked, not reserved.
Actual pool properties and node OS/architecture are checked after provisioning.
The mixed profile additionally requires both Windows build families and ARM64
candidate images before any resource creation.

The runnable candidate matrix has five rows. Four rows use `settingsVariants: ["*"]` to
select every catalog variant compatible with its declared cluster features and
platforms. The mixed `default` profile runs all 36 supported variants and the
FIPS/ARM64 scheduling assertions; the duplicate settings-only cluster is removed.
Classic CNI and managed non-Cilium ACNS also run all 36 compatible variants.
The standalone autoscaler, Cilium and kubenet candidate profiles are removed;
shared metric assertions remain, but the autoscaler scale-up spec is no longer
selected by this campaign. Four empty/omitted-secret inputs share a Linux-only
`operator-secret-watch` lane in eastus2: only this row selects the highest explicitly
advertised AKS Kubernetes patch at or above 1.37, even when it is not the
regional default. Stable and preview versions are eligible; 1.38 is selected when
AKS advertises it in this region. Availability is resolved at execution, not pinned.
If no eligible version exists, preflight fails before provisioning.
The retained cluster is upgraded
with `az aks upgrade` in place when it is behind the selected version; the runner
never recreates it, and verifies its existing deletion-protection locks before an
upgrade. A retained version newer than the advertised target is rejected, not
downgraded. Its deployment uses the `linux` profile tag while its resource group
retains the scenario's own ownership tags. Other rows retain the regional default;
aggregate admission records each row's version and any retained-cluster upgrade
plan separately. Customer Helm is separately blocked, giving 41 catalog inputs
(40 runnable) without claiming live passes. Deferred true password rotation is
metadata, not an executable settings assignment.

The `collector-controls` slot also runs the `ksm-allowlist` querymetrics spec on
profiles with the required Windows/operator features. It creates a namespace/pod and requires fresh ingested allowlisted
pod-label/namespace-annotation metrics while rejecting non-allowlisted dimensions;
rendered KSM arguments alone are not coverage.

### Executable behavior by candidate row

| Row | Executable assertions | Boundary |
|---|---|---|
| `default` | All 36 compatible settings variants; FIPS/ARM64 scheduling; common health, fresh Linux/Windows kubelet ingestion and node network-counter increases after DNS/service traffic. | Mixed-pool inventory alone is not proof; labeled scheduling and traffic assertions provide behavior evidence. |
| `classic-azure-cni` | All 36 compatible settings variants; common health, per-node ingestion and node network-counter increases after DNS/service traffic. | Traffic traverses the actual profile; no claim is made about CNI-specific policy or flow records. |
| `acns` | All 36 compatible settings variants; `querymetrics` requires healthy managed Retina/Hubble scrape targets, fresh `networkobservability_forward_bytes` from every Linux collector node, a positive five-minute increase after generated DNS requests, and fresh Linux `hubble_dns_queries_total` with source/destination pod labels and a positive increase. | This verifies managed non-Cilium basic node and advanced Linux pod-level ingestion; the standalone Retina Helm release is not used. |
| `acns-cilium` | Customer-settings v1/v2; independently verified managed ACNS/Cilium, fresh per-node `cilium_forward_bytes_total` and pod-level `hubble_dns_queries_total`, and positive traffic increases after controlled DNS/HTTP requests in both slots. | Retained Linux-only Cilium cluster; the real deletion lock and owned DCR association remain intact. |
| `otlp` | All compatible settings variants; candidate chart enables the node OTLP receiver, an owned DCR routes OTLP metrics to AMW, and TestKube sends a run-unique metric and queries it from AMW. | Linux-only OTLP/HTTP POC; gRPC and delta-temporality behavior are not asserted, and live candidate execution remains required. |
| `operator-secret-watch` | Four empty/omitted v1/v2 inputs keep secret-backed ServiceMonitor and PodMonitor credentials unchanged after a secret update, then verify updates when the namespace is allowlisted. SubjectAccessReview confirms the fixture grants secret get/list/watch. | The allowlist scopes watches, not direct secret GETs. Requires Kubernetes 1.37 and real passing TestKube cases; unsupported/skipped is not coverage. No live candidate result is recorded. |

### PR and scenario-source audit

These are executable assertions, not claims inferred from fixture names. A passing
local test or a prior release-image run is not a candidate-image pass.

| Source | Current assertion or execution boundary |
|---|---|
| #930 and the user-supplied #1097 OneNote list | The catalog selects ConfigTests workflows for absent/default/partial settings, disabled and enabled targets, RS-only, DS-only, all targets, custom/all-actions, global, invalid settings/custom/global and custom-node cases. The `windows` manifest recreates the explicit combined row: default targets, the RS all-actions ConfigMap, and Linux/Windows node ConfigMaps. Its configprocessing assertions inspect the live Prometheus config and node external labels; its candidate slot now also checks fresh Linux/Windows node-configmap ingestion. The pasted table's remaining blank-cell pairings cannot be recovered, so no additional combinations are inferred. |
| #1305 MIP follow-up promise | Configprocessing checks the MIP log value. Querymetrics now also requires `node_filefd_allocated` to be absent while the `node` target remains up with MIP enabled, and present with MIP disabled. This is an executable candidate check; it still needs a compliant live run. |
| #1536 and #1493 secret-access follow-up | The v1/v2 secret-transition specs test invalid-to-valid reload, fresh scrape recovery and scoped watches for empty/omitted allowlists where supported. Fixtures alone are not coverage; no live candidate result is recorded. |
| #875, #1788 and #1590 | ARM64/FIPS placement is asserted by node labels and collector-pod scheduling. Windows/Linux metrics are queried per node. The managed `acns` row creates traffic and checks fresh node-level Retina plus pod-level Hubble metrics; it does not install standalone Retina. |
| #1391/#1417, #1232/#1239 and #1393 | DCGM has config/job/interval/regex assertions but no GPU pool or GPU-metric ingestion proof. OTLP has a candidate HTTP ingestion scenario awaiting live execution. `controlplane-istio` validates config generation with a fake member FQDN, not real target discovery or scraping. |
| #1687 | Operator HTTPS code checks cover loopback plaintext refusal, liveness and mTLS behavior. The PR's enabled/disabled deployment checks remain unverified in a live candidate run. |
| #1029/#1032 region document | `regionTests` is separate and partial: it checks diagnostics, a nonempty `up` result, a recording-rule result and usage/limit response shape, but not the full cloud-parameter, AMCS/DCR/DCE, always/never-alert, alert-evaluation or actionable failure/request-ID contract. Fairfax, Mooncake, USSec and USNat access is not established. |
| #1027/#1446/#1457 CCP; #896/#1338 Arc | CCP config and health scenarios require the standalone underlay documented under `test/ccp`; Arc uses Sonobuoy/conformance. Neither is exercised by ordinary AKS/TestKube, and neither has an adapter in this candidate runner. |
| #1795/#1799 and #1809 | Open/dry-run-only proposals and author-reported runs with image skew are not exact-source candidate evidence. They are not adopted as new scenario requirements. |

`ccp-health` remains blocked because CCP requires its standalone underlay
workflow, which ordinary AKS/TestKube cannot reach. Arc conformance is a
separate Arc/Sonobuoy workflow and is not executed by this AKS candidate matrix.
Neither is inferred from a passing AKS row.

The default 360-second settle alone costs 216 minutes for a 36-slot profile;
provisioning and TestKube execution are additional. Each candidate row allows
eight hours, but no measured Azure duration or live pass is claimed yet.

### Observed CI/CD examples

Read-only Azure inventory, not scenario declarations, established these examples.
All five report stable auto-upgrade and system-assigned identity; none showed
proxy/private API. ARM-indicative pool SKUs are not independent runtime architecture
proof because the Azure architecture field was null.

| Existing target | Observed distinguishing configuration |
|---|---|
| `ci-dev-aks-mac-eus` / eastus | Classic Azure CNI; Ubuntu, Azure Linux, Windows 2022 and ARM-indicative pool; WI/OIDC enabled |
| `ci-dev-aks-tests` / centralus | Azure CNI overlay; Ubuntu, Windows 2022 and ARM-indicative Azure Linux; WI/OIDC enabled |
| `ciprom-dev-aks-otlp` / westus3 | Overlay/mixed pools; WI disabled, OIDC enabled; target name alone does not prove OTLP routing |
| `ciprom-upgrade-bot` / westus3 | Overlay/mixed pools; WI/OIDC enabled |
| `ci-prod-aks-mac-weu` / westeurope | Classic CNI; Ubuntu/Windows 2022 FIPS pools; WI/OIDC fields null, not asserted disabled |

Pipeline target references: `.pipelines/azure-pipeline-build.yml` (dev 2461-2673,
upgrade 2732+, prod 207-216), `.pipelines/azure-pipeline-release.yml` (190-202)
and `.pipelines/azure-pipeline-nightly-tests.yml` (42-54/130-142).
CI Retina is a separate Helm release, not evidence of managed ACNS.
No AMPLS/query endpoints were observed **in those resource groups**; absence
elsewhere was not established.

Live dev TestKube inventory found chart/app 2.14.1, ready API/MongoDB/MinIO/NATS,
nonempty passed workflow history and bound 8Gi/10Gi/10Gi backing storage.
No separate dashboard/ingress was observed; external dashboard, server execution
mode and actual artifact utilization remain unverified. These historical records
are not candidate tests, and no shared cluster was mutated.

| Dimensions | Sharing/constraint |
|---|---|
| Linux + Windows settings | Same mixed host, sequential complete variants |
| Ubuntu/Azure Linux, AMD64/ARM64, FIPS | Compatible distinct pools on one host; ARM64 FIPS requires Azure Linux 3+ |
| Windows 2022/2025 | Separate host pools; the protected Windows 2022 collector supports both through process isolation |
| Windows 2019 | Unsupported on 1.33+; no substitution with Windows 2022 |
| Standalone autoscaler, Cilium, kubenet | Removed from the candidate campaign; shared collector metrics/assertions remain. The separate managed `acns-cilium` lane verifies the AKS Cilium/ACNS profile and ready Cilium DaemonSet, then requires fresh per-node Cilium forwarding and Hubble DNS traffic increases in both customer-settings v1/v2 slots before restoration. |
| AMPLS/private ingestion + private query | Can share mixed-pool infrastructure; require distinct endpoint/DNS and data-path assertions |
| Managed ACNS | The runnable `acns` row uses managed non-Cilium Retina; it requires fresh per-node `networkobservability_forward_bytes` and Linux pod-level `hubble_dns_queries_total` after generated DNS traffic. This is distinct from the standalone Retina Helm release and from Cilium ACNS. |
| Customer Helm, candidate addon, Arc, CCP | Distinct installation ownership/adapters, not namespaces on the same managed addon |
| Public/private access, proxy CA, deny/recovery | Isolated owned states with readback and restoration before reuse |

[Windows container compatibility](https://learn.microsoft.com/en-us/virtualization/windowscontainers/deploy-containers/version-compatibility#windows-server-host-os-compatibility)
explicitly permits Windows 2022 containers on Windows 2025 hosts under process
isolation, which is the [AKS-supported isolation mode](https://learn.microsoft.com/en-us/azure/aks/windows-best-practices#windows-os-version).
The chart uses ordinary Windows containers, not HostProcess or a custom runtime.
Admission still checks the protected image's architecture, four-part OS version,
source and digest; documented OS compatibility does not replace live readiness
and ingestion evidence.

Inventory additional interactions before declaring matrix completeness:
taints/DaemonSet and replica placement; autoscaling/mixed pools; private
API/AMPLS/query DNS; proxy/noProxy/private endpoints; Cilium/ACNS/network
policies; BYO subnet/private DNS; restricted egress/private image pulls;
OTLP/network/identity; service mesh; customer Helm advanced mode/Windows
fixtures. CCP remains blocked from ordinary AKS/TestKube: its required standalone
underlay workflow is not part of this matrix.

AMPLS must include configuration/ingestion DCE scope and association, and the AMW
needs a **separate** `prometheusMetrics` query private endpoint. Private AKS API
alone proves neither. Private-path proof needs approved endpoint/DNS readback,
fresh run-specific metrics, public-bypass denial and a genuinely reachable private
runner. Proxy proof needs actual route observations, `noProxy` behavior and CA
trust/failure/recovery; ordinary connectivity is not equivalent. ACNS proof needs
real enablement, targets and fresh metrics from controlled traffic.

The matrix lists only concrete compiled profiles; `coverageGaps` preserves omitted,
blocked or unsupported outcomes. A compiled template without a live adapter is not
an executed scenario. Nightly/weekly tier suggestions are not permission to reduce
an already-required full patch-test suite. Report provisioned host counts, per-slot
time, pool SKU/count and network-resource costs separately; do not multiply cluster
cost by the number of compatible settings variants.
