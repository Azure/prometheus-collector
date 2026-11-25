# Build E2E pipeline

Pipeline 440 builds and pushes images, then scans the collector from that build
and runs the cluster scenarios in
[`otelcollector/test/scenarios`](../../otelcollector/test/scenarios/README.md).
The SRE branch stages are appended through `templates/azure-template-e2e-stages.yml`;
config-tests is not involved. The template's `scenarios` parameter lists the
stage IDs and must match the runnable rows in `matrix.json`.

## Layout

| Folder | Contents |
|---|---|
| `build/` | Candidate evidence filtering. |
| `scan/` | Image resolution, Trivy/Qualys scans, and baseline comparisons. |
| `templates/` | Appended SRE stages and cluster scenario steps. |

## How it works

1. Pipeline 440 pushes the collector, target allocator and config-reader images.
   On SRE branches, the collector's HIGH/CRITICAL/MEDIUM, fixed-only Trivy scan
   emits JSON once, converts that same result to the table output, and retains the
   full package inventory and DB snapshot for comparison. Other images keep the
   ordinary filtered scan. Retries publish separate artifacts; the scan job's
   output variable selects its artifact through a native pipeline-artifact input.
2. The SRE-only stages run after `Build` succeeds, using the same build ID and commit.
   Stage dependencies enforce Build success; no ADO API lookup or build receipt is
   needed. Cluster jobs use the existing Build-stage image-tag outputs and resolve
   them to digests. Reused tags can resolve to images from a later build. Windows
   CoMET uses each Windows producer job's immutable push digest as the child input.
3. `scan_trivy.py` compares filtered build-image Trivy findings with the released
   baseline using the same scanner version and DB. Open and new findings are
   reported and fail the comparison; missing evidence and scanner errors also
   block it. Scenario stages depend on `Build` and the runner import, not this
   scan stage, so the findings do not prevent scenario execution.
   Findings in collector binaries owned by the separate OpenTelemetry Collector
   upgrade are reported as deferred. `scan_qualys.py` runs
   CoMET on the same build image and baseline and compares Qualys XML QIDs. Open and
   new QIDs are reported; missing evaluations block, and closure requires an explicit
   not-found QID. Separate Windows CoMET jobs compare the WS2019 and WS2022 build
   images with the corresponding released images. Trivy covers Linux/amd64 only.
   Neither scanner proves full image clearance: ME and mdsd package inventory is not
   copied into the final image, and Qualys language coverage requires validation.
4. `run-scenario.py` deploys the resolved images to test clusters and runs the
   Ginkgo suites through TestKube. Before those stages, the pipeline mirrors the
   latest Ginkgo runner into its ACR and passes its resolved digest to TestKube.
   Network probe ServiceMonitors use per-run names; cleanup removes only the
   monitor created by that execution.
   Published evidence excludes YAML and
   credential files and filters known secret patterns; it may not detect every
   secret.

Comparison reports retain `candidate` and `baseline` keys. Existing cluster
installation modes and fixture resource names, including `candidate-addon`
and `candidate-*` resources, and the runner ACR repository path remain unchanged.

## Run locally

```powershell
$env:PYTHONPATH = ".pipelines\pr-e2e\build;.pipelines\pr-e2e\scan"
python .pipelines\pr-e2e\scan\scan_trivy.py --producer-scan <downloaded-scan-artifact> --baseline-image <repo@sha256:...> --platform linux/amd64 --output <dir>
```

The offline checks also need the scenario requirements:

```powershell
python -m pip install -r otelcollector\test\scenarios\requirements.txt
python .pipelines\pr-e2e\test_pr_e2e.py
```

Local scan comparison uses an already downloaded artifact; it does not query ADO.
