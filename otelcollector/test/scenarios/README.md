# Cluster scenarios

`run-scenario.py` resolves Build-stage ama-metrics image tags to digests, deploys
them to dedicated AKS clusters, and runs existing Ginkgo suites through TestKube. Pipeline 440
calls it once per scenario after its Build stage succeeds on `sre/*` branches.

## Layout

| Path | Purpose |
|---|---|
| `matrix.json` | Scenario rows plus build image tags, container roles and runtime requirements. |
| `clusters/` | Bicep for the AKS host (`aks.bicep`, `pools.bicep`, `proxy.bicep`, `orchestrator.bicep`) and one `.bicepparam` per profile. |
| `settings/` | Customer settings fixtures; see [settings/README.md](settings/README.md). |

## Scenarios

| Scenario | What it covers |
|---|---|
| `default` | Mixed Linux/Windows/ARM64 host; all compatible settings variants and health checks. |
| `acns`, `acns-cilium` | Managed ACNS (and Cilium) network metrics. |
| `otlp` | OTLP receiver ingestion into the Azure Monitor workspace. |
| `operator-secret-watch` | Secret-backed monitors and credential transitions on Kubernetes 1.37+. |
| `proxy`, `proxy-custom-ca` | Collector behind an HTTP proxy, with and without a custom CA. |
| `ampls`, `ampls-proxy` | Private ingestion through Azure Monitor Private Link Scope. |

Settings variants that change cluster-wide ConfigMaps run one at a time on the same
host; the runner saves and restores the original ConfigMaps around each one.
Reset ConfigMaps come from `settings/catalog.json`; general health suites are
discovered from `testkube-test-crs.yaml`. Pipeline runs use the active Azure CLI
subscription selected by the service connection.

## Run

Requires Python 3.11+, Azure CLI with Bicep, kubectl, Helm and the TestKube CLI.

```powershell
python -m pip install -r otelcollector\test\scenarios\requirements.txt
python otelcollector\test\scenarios\run-scenario.py validate --evidence <dir>
python otelcollector\test\scenarios\run-scenario.py plan --scenario default --evidence <dir>
python otelcollector\test\scenarios\run-scenario.py preflight --scenario default --evidence <dir>
```

`validate` and `plan` are offline; `preflight` reads Azure but creates nothing.
Preflight and run use the scenario's matrix location unless `--location` overrides it.
The [pipeline stages](../../../.pipelines/pr-e2e/README.md) require Build success
and serialize access to shared hosts. Cluster runs require the
`Prometheus-Collector` deployment environment. The runner resolves the Build
image tags, reuses or creates the scenario's resource group, and runs the selected tests.
TestKube clones the pipeline commit to run those tests.
The pipeline supplies the pinned TestKube runner as `TEST_RUNNER_IMAGE`; the
runner passes `SCENARIO_RUN_ID` to the tests for run-scoped monitor cleanup.
The evidence directory gets logs, JUnit/Ginkgo results and `result.json`.

Clusters are kept between runs and reused only after ownership checks; the runner
never deletes them. Only one run may use a cluster at a time
(`kube-system/ama-e2e-workflow-hold`). If a run is interrupted, check that its
TestKube execution has finished and settings were restored before clearing the hold.

## Add a scenario

1. Copy the closest `clusters/*.bicepparam` and change only the dimension you need.
2. Add a row to `matrix.json` with its location, features, settings variants and Ginkgo labels.
3. Add its ID to the `scenarios` parameter in the [stages template](../../../.pipelines/pr-e2e/templates/azure-template-e2e-stages.yml).
4. Run `validate`, `plan` and `preflight`, then a real `run` before relying on it.