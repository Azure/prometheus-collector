# Repeatable ama-metrics cluster scenarios

Service-principal deployments and Monitoring Data Reader assignments retry only
authorization denials naming `Microsoft.Authorization/roleAssignments/write`.
The shared retry uses capped backoff for up to 10 minutes within the run budget;
direct writes first check for the exact existing principal/type/role/scope.
User-principal calls remain single-attempt. Token reacquisition uses the existing
Azure CLI login only when the error recommends refreshing credentials.

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
candidate pass is recorded:** a real protected candidate build is still required.
It never falls back to direct local Ginkgo or treats installation as a pass. It captures
the exact submitted execution IDs, immutable test-source SHA and runner-image
digest, terminal passed status and nonempty JUnit/artifacts. Legacy `main`/`latest`
defaults are not candidate provenance.
`regionTests` is a separate region/rule validation, not a cluster-health substitute.

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
Infrastructure remains provisioned after every run, including failed runs, until
the explicit `cleanup` action is requested. Its generated names are derived from
the recorded execution identity; it never takes over an existing group. Do not
start the same execution/evidence directory concurrently on multiple machines;
the calling protected runner must serialize that execution. Separate fresh
executions do not share clusters.

## Candidate receipt and execution

Supply `--build-id` for a successful pipeline-440 build. The harness invokes the
SRE-owned `.pipelines/pr-e2e/fetch_candidate.py` authenticated verifier before
provisioning. It checks the build's full source SHA and downloads all five
same-build protected image/runner artifacts, retaining the entire
`verified-candidate` evidence directory. Local JSON, a URL or a bare digest is
not accepted as a substitute.
Only after authenticated verification, the adapter maps producer `config-reader`
to its internal `cfg` component and `indexRef` to `manifestRef`. Downloaded receipts
remain unchanged; the common index is deployed and child digests verify runtime images.

`--source-root C:\path\to\candidate-checkout` uses an existing clean
candidate checkout without changing this worktree's branch; by default the source
is this checkout. `--test-source-root` defaults to that checkout and must match
the verified `testRunner.testSource.commit`. Test packages, workflow scripts,
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
python otelcollector\test\scenarios\run-scenario.py run --scenario default --settings-root C:\path\to\settings --build-id 123456 --receipt-tools-root C:\path\to\trusted-sre-tools --source-root C:\path\to\candidate-checkout --testkube-version 2.15.0 --location eastus2 --evidence C:\temp\ama-candidate-run
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
package hash, and creates a query-only managed identity/federation for the tests-job
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
Fixture-declared namespaces must not already exist. Transition specs may change
initially declared resource identities within the selected workflow; no separate
phase executor or manifest fields are supported.
Customer v1/v2 alias slots also run only `querymetrics` / `cluster-identity` with
`EXPECTED_CLUSTER=Customer_Config_POC` **before restoration**. This proves fresh
alias/annotation-namespace ingestion; configprocessing alone does not.
Catalog `overrideWorkflowLabels: true` declares intentional source-filter
replacement and is retained in `selection.json`; filters are replaced, not appended.

Health suites use existing reference workloads and scrape fixtures. JUnit
records executed/failed/skipped counts; skipped tests are not claimed as executed.
Pool-specific runtime image evidence is separate from the suite verdict.
One passing Windows spec does not imply every Windows OS/pool was checked.

Evidence includes command timings/logs, deployment outputs, per-suite JUnit and
Ginkgo JSON, per-node/component `imageID` observations, results and
`retained-resources.json` with resource IDs and tags.
Secret-bearing command output is redacted; snapshots remain in memory.
Failed deployments retain `deployment-error.txt` with secret-bearing environment
values redacted. Command history survives a separate explicit cleanup invocation.
Service-principal runs create their owned resource group, then wait up to 20 minutes
for an administrator bootstrap before deploying. The runner prints the exact
`az role assignment create` command and saves `rbac-bootstrap.ps1` plus
`rbac-bootstrap.json`. An authorized administrator runs that command in their own
Azure session; no token or credential is transferred to the runner.
The delegation is **Role Based Access Control Administrator**, scoped only to this
new group, with condition version 2.0 restricting both assignment creation and
deletion to Network Contributor and Monitoring Data Reader for service principals.
Only the custom-CA profile additionally permits Reader. It cannot delegate Owner,
User Access Administrator or itself. A precreated group is still refused.
The gate requires the exact principal, group, role and condition; missing or wider
delegations do not satisfy it. Timeout fails and retains any created resources for
inspection. Actual ARM deployment remains the permission-propagation check, not
the presence of a grant. Persistent-cluster runs retain the same scoped,
conditional administrator grant for repairing missing workload assignments or
retrying provisioning; successful runs do not revoke it. Healthy reuse skips
existing exact Monitoring Data Reader assignments and needs no new role write.
The narrower workload roles also remain with the retained cluster. The pipeline
identity cannot delete its own conditional administrator assignment.

For a non-persistent execution that no longer needs bootstrap, run revocation
from the authorized user Azure session:

```powershell
python otelcollector\test\scenarios\run-scenario.py revoke-bootstrap --evidence C:\temp\ama-candidate-run
```

It validates the exact recorded scope, principal, role and condition, deletes
only the recorded assignment ID, verifies its absence, and confirms the
narrower role assignments are unchanged. Persistent groups are refused before
any Azure call. It does not delete infrastructure.
Explicit cleanup waits for the recorded AKS node and AMW managed resource groups
to disappear as well as the parent. Incomplete receipts discover parent-linked managed groups,
including retries after the parent has disappeared. Failed discovery is an
explicit incomplete cleanup, not a pass. It never directly
deletes an unrelated managed group. The execution-local kubeconfig is removed even
when cloud cleanup fails. CI publication must unconditionally exclude credential
files in case the process was killed before `finally`.
When an administrator bootstrap was recorded, cleanup also verifies its role
assignment disappeared after deleting the owned group.
A four-hour execution
budget bounds commands; retention recording and kubeconfig removal still run after
the budget expires. Explicit cleanup remains available after the budget expires.
`--max-run-seconds` changes that explicit budget. Expiry tags include cleanup grace,
but tags alone do not automatically delete Azure resources. To explicitly remove a
retained execution, use:

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
The [private-runner finalizer contract](clusters/private-runner/CONTRACT.md)
provides tested cleanup code, not a commissioned runner or private-path proof.

## Auto-upgrade and combination semantics

There is no Kubernetes-version-history matrix. Every run resolves the currently
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

The matrix contains 22 cases sharing 20 infrastructure profiles. `default` is the
small PR lane; `settings` runs all 34 version-independent Ginkgo settings on one
mixed host. Four empty/omitted-secret inputs share a Linux-only
`operator-secret-watch` lane: the current regional default must be at least 1.37,
otherwise preflight reports unsupported without provisioning. No historical
version is selected. Customer Helm is separately blocked, giving 39 catalog inputs
without claiming 39 live passes. Deferred true password rotation is metadata,
not an executable settings assignment.

The default 360-second settle alone costs 204 minutes for 34 settings slots;
provisioning and TestKube execution are additional. Review the explicit
`--max-run-seconds` budget before a full-settings campaign. No measured Azure cost
or duration is claimed yet.

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
| Windows 2022/2025 | Compatible separate pools only with eligible AKS version, VM/image and candidate support |
| Windows 2019 | Unsupported on 1.33+; no substitution with Windows 2022 |
| Cilium, kubenet, AKS HTTP proxy | Separate host configurations; these profiles cannot include Windows |
| AMPLS/private ingestion + private query | Can share mixed-pool infrastructure; require distinct endpoint/DNS and data-path assertions |
| ACNS | Not implied by Cilium; non-Cilium Retina and Cilium/Hubble/security are different cases |
| Customer Helm, candidate addon, Arc, CCP | Distinct installation ownership/adapters, not namespaces on the same managed addon |
| Public/private access, proxy CA, deny/recovery | Isolated owned states with readback and restoration before reuse |

Inventory additional interactions before declaring matrix completeness:
ARM64/Azure Linux/FIPS; taints/DaemonSet and replica placement; autoscaling/mixed
pools; private API/AMPLS/query DNS; proxy/noProxy/private endpoints;
ACNS/Windows; Cilium/ACNS/network policies; UAI/BYO subnet/private DNS;
restricted egress/private image pulls; OTLP/network/identity; service mesh;
customer Helm advanced mode/Windows fixtures.

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
