# Customer settings scenarios

These are customer ConfigMap/monitor-CR and customer Helm-value scenarios.
AKS infrastructure is supplied by the cluster matrix, not configured here.
The existing [ConfigTests](../../CONFIG-TESTS-README.md), Ginkgo suites and
TestKube runner remain the execution path.

## Run

From the repository root (Python 3.11+ and the packages in `requirements.txt`):

```powershell
python otelcollector\test\scenarios\settings\test_catalog.py -v
```

The [coverage table](coverage.json) maps published v1/v2 keys, parsed collector
controls and customer-chart values to real input variants. The guard fails on a
new unmapped key, missing fixture, schema error or workflow-label mismatch.
Management-plane target switches must also have both enabled and disabled
inputs in each schema; the v2 variants reuse the existing job assertions.
`assertions` records field-specific checks separately: an input present with its
default value is **not** proof that its behavior is verified.
Use `--require-runtime` to fail on remaining assertion gaps; it is deliberately
not a blanket claim of complete runtime coverage.

From `otelcollector/test/ginkgo-e2e/configprocessing`:

```powershell
go test -v ..\..\scenarios\settings\customer_helm_test.go
```

This renders the **customer chart**, not the AKS addon chart. It does not install
a Helm release or prove ingestion. It renders selected real templates offline,
not optional subcharts, authentication setup or full Kubernetes admission.
The Windows template currently does not propagate the Linux pod
label/annotation/priority customizations; those checks are Linux-only.

For cluster-backed cases, select a manifest through the shared scenario runner.
Use an explicitly approved disposable cluster. `configmaps`, `customResources`
and optional `helmValues` paths are relative to the manifest's directory.
`ginkgoLabels` are combined with **OR**, matching existing TestKube workflows.
An empty Ginkgo label list is allowed only with Helm values; it must not select
every Ginkgo test. Helm variants require an isolated customer-chart adapter.

The four ConfigMaps in `catalog.json` must be saved and reset before applying a
variant, then restored even when a test fails. Absent original ConfigMaps must
remain absent afterward. Shared cluster variants run sequentially.
The new v1/v2 cases also create two explicitly named namespaces; execution must
refuse to overwrite pre-existing namespaces and remove only its own fixtures.
The empty/omitted-secret variants require the operator and the
`config-processing-empty-secret-namespaces` spec in the combined candidate.
They run on the current cluster; a version-gated skip is not enforcement proof.
The dedicated `operator-secret-watch` lane selects an explicitly supported regional
stable official Kubernetes >=1.37 without changing ordinary/default clusters.
Its owned `ama-secret-watch-test` namespace grants the collector secret get/list/watch.
The spec verifies the permissions, observes unchanged credentials after a secret update
with no watch, then verifies a subsequent update with the namespace allowlisted. The
allowlist does not deny direct GETs. Each slot restores/deletes only its owned resources.

The candidate `collector-controls` slot also runs the `ksm-allowlist`
querymetrics spec. It verifies fresh ingestion of allowlisted pod labels and
namespace annotations and absence of non-allowlisted dimensions; rendered KSM
arguments alone are not runtime coverage.

`expectedOutcome: config-error` means the collector should report the expected
configuration error and use its fallback. It does not mean skip assertions or
expect unhealthy pods.

## Add a scenario

For existing same-invocation alias, namespace and secret transitions, see
[the precise phase/apply/reset contracts](TRANSITIONS.md). These reuse the
selected ConfigTests spec and declare all mutable identities initially.
The credential cases prove reload/recovery only. True two-valid-password
rotation is explicitly deferred by the user and excluded from coverage claims.

1. Check existing ConfigTests fixtures, workflow label expressions and assertions.
   Reuse a case when it already proves the requested behavior.
2. Add `<id>/manifest.json` and only the missing input YAML. Add its entry and
   PR/nightly/weekly schedule to `catalog.json`. Use `requires.clusterFeatures`
   for prerequisites, never provision infrastructure in settings fixtures.
   Every Ginkgo variant needs an existing `workflow` source CR filename.
   If its source selectors differ, set catalog `overrideWorkflowLabels: true`;
   execution replaces them with the manifest's OR labels, never appends them.
3. Map each exercised customer setting in the coverage table, keeping configured
   inputs distinct from runtime-asserted behavior.
4. For missing behavior, add a labeled spec to the existing `configprocessing`
   suite, a TestKube CR, a new runner case mapping and pipeline setup/result/
   cleanup steps following [the existing recipe](../../CONFIG-TESTS-README.md).
   The `customer-settings-v1` and `customer-settings-v2` entries demonstrate this.
5. Run metadata checks and the exact labels against a disposable cluster. Break
   each new setting and show the assertion fails, restore it, then rerun.
   Preserve logs/JUnit and verify cleanup, not just Kubernetes acceptance.

For Helm cases, add customer values (not internal RP/addon values) and extend the
existing `customer_helm_test.go` render assertions. Record render-only evidence
as such until an isolated live deployment is verified.

## Evidence boundaries

The existing ConfigTests workflows retain their existing `$BRANCH_NAME` source
and runner image conventions. They are **not** candidate-provenance evidence
unless a separate candidate path supplies and verifies an exact source SHA and
runner digest. A local Ginkgo run against a released addon tests those customer
settings on that released image, not a future candidate image.

The inventory is bounded to the published settings schemas, explicitly read
collector controls and customer-chart values. It does not enumerate every
upstream Prometheus scrape or PodMonitor/ServiceMonitor API property. The DCGM
variant verifies generated configuration without requiring a GPU; it is not
proof that DCGM metrics were scraped or ingested.
