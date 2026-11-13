# TestKube workflow setup regression tests

Run `python3 -m unittest discover -s otelcollector/test/testkube -p workflow_evidence_test.py` and `bash otelcollector/test/testkube/run-testkube-workflow-test.sh` from the repository root with Python/PyYAML, Bash, `jq`, and `envsubst` installed. Set `PYTHON` if the Python executable is not `python3`. These tests mock `kubectl` and require no cluster access. They reject setup errors, missing/mismatched execution IDs, nonpassed terminal statuses, empty or failed reports, and mutable candidate inputs. Pipeline 979 runs them before accessing the cluster.

## Immutable candidate execution

The existing TestKube TestWorkflow remains the executor; candidate mode does not run Ginkgo on the host. First verify the protected runner receipt and candidate deployment. The receipt must bind a full test-source commit to the runner digest and successful producer build/artifact, not free-form image fields. Candidate mode reuses the existing runner image, resolves the suite's Go toolchain and Ginkgo module versions, and runs that explicit CLI version with the same Go toolchain without modifying module manifests. The runner is scheduled on Linux/amd64. Toolchain/module downloads require outbound access from the TestKube namespace, independently of collector ingestion egress.

From `otelcollector/test`, after the caller applies the scenario and waits for readiness:

```bash
export CANDIDATE_TEST_SOURCE_SHA=<full-lowercase-40-hex-commit>
export CANDIDATE_RUNNER_IMAGE=<repository@sha256:64-hex-digest>
export SELECTED_WORKFLOW=<existing-TestWorkflow-metadata.name>
export BUILD_ARTIFACTSTAGINGDIRECTORY=<per-slot-absolute-output-directory>
# Optional: GINKGO_LABEL_FILTER overrides the existing workflow's labels.
bash ./testkube/run-testkube-workflow.sh "$AMW_QUERY_ENDPOINT" "$AZURE_CLIENT_ID" \
  testkube-test-crs.yaml candidate-health.yaml false 0 ConfigTests "$CANDIDATE_TEST_SOURCE_SHA"
```

Config CR filenames (`testkube-config-test-*-crs.yaml`) resolve under `config-processing-test-crs`; health templates resolve under `testkube`. Candidate mode requires all three provenance/selection variables, executes exactly the selected workflow, and never applies customer ConfigMaps/reference apps. The caller owns snapshot/apply/restore. Without candidate variables, existing workflow selection/setup remains available.

`testkube-results-ConfigTests.json` contains exact `test_source_sha`, `runner_image`, and `executions[]` records with `workflow`, `execution_id`, `status`, `specs`, `passed`, `skipped`, `failed`, and `artifact_dir`. Counts include only Ginkgo `It` specs, not suite setup; skipped specs are allowed but at least one must pass. Dry runs, failed setup/specs, and mismatched JUnit evidence fail. Each native absolute `artifact_dir` is rooted at `<staging>/testkube-executions/ConfigTests/<workflow>/<exact-id>/artifacts`; it must contain exactly one recursive `ginkgo.json` and one `junit.xml`. Its parent retains `submission.json`, `execution.json`, and `record.json`. Consumers must validate confinement, exact identity/status, provenance equality, and the applied workflow's Git revision/image; an adapter summary alone is not a trusted producer receipt.

Optional assertion expectations are `EXPECTED_CLUSTER`, `EXPECT_OPERATOR_HTTPS=true|false`, and `EXPECT_WINDOWS_EXPORTER=true|false`. Operator candidates require HTTPS by default; `false` is only for an intentional HTTP compatibility lane. Select querymetrics `cluster-identity` with `EXPECTED_CLUSTER=Customer_Config_POC` for the customer alias/annotation fixtures; it requires post-suite-start ingestion, not standard reference-app fixtures. `cluster-network` selects per-node ingestion for supported Linux/Windows lanes. `managed-acns` requires fresh managed Retina/Hubble targets and metrics after generated traffic; it does not depend on a standalone Retina release or assume a service name/port. These environment-specific labels do not establish AMPLS or proxy routing without independent host/network evidence. Windows exporter sentinels run only when explicitly enabled.

The ConfigTests pipeline always publishes JUnit and execution artifacts, and independently gates the complete expected scenario set against retained execution IDs, terminal passed statuses and nonempty matching reports. A Teams notification or a successful workflow with no executed specs cannot satisfy that gate.

### Same-invocation settings transitions

The existing configprocessing carriers select the seven settings variants below. Their fixtures must come from the same verified Git revision as the suite; the renderer includes `scenarios/settings`, passes the scoped AMW query identity, and labels the workflow pod for workload identity. The harness must configure that pod's service account and AMW role assignment. Each invocation restores its original ConfigMap/Secret on exit; the harness still owns outer resource cleanup.

| Label | Assertions |
|---|---|
| `config-processing-customer-transitions-v1` | v1 alias update/clear, annotation namespace update/removal, healthy canonical PM/SM targets and fresh ingested jobs, Windows kube-proxy per-node ingestion |
| `config-processing-customer-transitions-v2` | The same transitions with v2 settings |
| `config-processing-secret-transitions-v1` | Invalid dummy username to existing `admin`/`pwd` reload/auth recovery without collector/allocator/endpoint restarts; allowed to empty/omitted to restored Secret allowlist |
| `config-processing-secret-transitions-v2` | The same recovery/enforcement with v2 settings |
| `config-processing-windows-kube-proxy-ingestion` | Actual Windows node endpoint plus fresh `up` and `kubeproxy_sync_proxy_rules_duration_seconds_count`, exact Windows job/cluster/node |
| `config-processing-annotation-regex-edges-v1` | Absent/empty/invalid disabled fallback, two anchored alternatives, similar-prefix exclusion, wildcard inclusion, unannotated negative control, and invalid-after-valid normal reload |
| `config-processing-annotation-regex-edges-v2` | The same regex edge cases with v2 settings |

Regex edge cases verify that all declared endpoints are live, inspect every replica's generated annotation job, and require exact fixture target sets plus post-convergence AMW samples. Invalid phases must produce the diagnostic after a normal fresh collector/reader process restart; the spec never forces a restart. Missing/empty/invalid input disables annotation scraping on this lifecycle path, not a last-valid wildcard fallback. A separate fresh kubelet `up` control proves healthy fallback. Negative ingestion observations start after configuration/target convergence so historical or pre-reload samples do not become false continued-ingestion failures.

Scoped allowlist enforcement skips as unsupported below Kubernetes 1.37; auth recovery remains a separate spec. True two-valid-password credential rotation is **deferred by user**, not covered by recovery. A compile, label dry-run, or unsupported skip is not live candidate-image evidence.

### Durable candidate mode in the existing ConfigTests pipeline

Opt in with `candidateMode: true`, `candidateBuildId: <successful pipeline-440 build ID>`, and `candidateSourceSha: <full lowercase commit>`. The pipeline's own reviewed checkout supplies the trusted receipt verifier/scanner; PR execution is admitted only for an authorized SRE campaign head whose full SHA matches the candidate source. Same-build stage/job retries reuse a deterministic run identity only after verifying ownership, expiry, deployment outputs and live AKS state; new builds do not reuse old resources. The separate clean candidate checkout must match the requested SHA, and authenticated producer receipts are verified before invoking that checkout's existing scenario harness. The legacy shared-cluster job is disabled in this mode. The service connection must target the approved build subscription and retain its existing scoped resource, caller-object lookup and AMW role-assignment permissions.

`candidateScenario` selects `all` or one of five runnable matrix rows: `default`, `classic-azure-cni`, `acns`, `otlp`, and `operator-secret-watch`. The pipeline admits only approved SRE campaign PR heads, resolves the successful producer for that exact head, and requires aggregate regional capacity before its isolated candidate stages run in parallel. The first four rows run every customer-setting variant compatible with their declared OS/platform features; variants within a cluster remain sequential and restore the baseline before the next. The mixed default profile runs all 36 supported variants, so there is no duplicate settings-only cluster. The dedicated Linux secret-watch row selects a region-advertised stable official Kubernetes >=1.37 and runs all four empty/omitted v1/v2 namespace inputs; ordinary rows retain the regional default. Customer Helm remains blocked for its documented prerequisites. The harness budgets eight hours per row, with retained attempt evidence. Any later source change requires a matching protected producer and real rerun.

The harness owns the sole disposable environment and fixture lifecycle. An always-run ownership-guarded cleanup retry precedes scanning; cleanup failure remains a failed run. SRE's scanner separately authenticates the same producer build/source and retains the fixed-baseline comparison. The pipeline always stages filtered evidence, excludes kubeconfig/credentials/scanner caches, and publishes actual settings/health TestKube JUnit plus the evidence tree. `publication.json` binds producer/source, validation build, verified source and exact TestKube execution IDs, recording original versus published hashes when filtering changes bytes. Publication cannot turn missing tests, failed cleanup or a failed/blocked scan into success. Evidence is not committed back to Git.

# Current Tests
- Container Status
  - Each container on each pod that we deploy has status `Running`. Pods include:
    - ama-metrics replicaset
    - ama-metrics-node daemonset
    - ama-metrics-ksm replicaset
    - ama-metrics-targets-operator replicaset `label=operator`
    - prometheus-node-exporter daemonset `label=arc-extension`
  - All expected processes are running on the containers. Processes for the `prometheus-collector` replicaset and daemonset container are:
    - fluent-bit
    - telegraf
    - otelcollector
    - mdsd
    - metricsextension
    - inotify for configmap changes
    - inotify for DCR download changes
    - crond for rotating the log files
  - Each container on each pod that we deploy has no errors in the container logs. Pods include:
    - ama-metrics replicaset
    - ama-metrics-node daemonset
    - ama-metrics-ksm replicaset
    - ama-metrics-targets-operator replicaset `label=operator`
    - prometheus-node-exporter daemonset `label=arc-extension`
  - All Daemonset pods are scheduled in all the nodes. Nodes include:
    - FIPS
    - ARM64  
  - All Daemonset pods have all containers running in all nodes. Nodes include:
    - FIPS
    - ARM64
- Liveness Probe
  - When processes aren't running on the `prometheus-collector` replicaset container, the container should restart. Processes include:
    - otelcollector
    - metricsextension
    - mdsd
  - When processes aren't running on the `prometheus-collector` daemonset container, the container should restart. Processes include:
    - otelcollector
    - metricsextension
    - mdsd
  - When processes aren't running on the `prometheus-collector` windows daemonset container, the container should restart. Processes include: `label=windows`
    - otelcollector
    - MetricsExtension.Native
    - MonAgentLauncher
  - When the `ama-metrics-prometheus-config` configmap is updated, the `prometheus-collector` replicaset container restarts.
  - When the `ama-metrics-config-node` configmap is updated, the `prometheus-collector` daemonset container restarts. `label=linux-daemonset-custom-config`
  - When the `ama-metrics-prometheus-config-node-windows` configmap is updated, the `prometheus-collector` windows daemonset container restarts. `label=windows`
- Prometheus UI
  - The Prometheus UI API should return the expected scrape pools for both the `prometheus-collector` replicaset and daemonset containers.
  - The Prometheus UI API should return a valid Prometheus config for both the `prometheus-collector` replicaset and daemonset containers.
  - The Prometheus UI API should return targets for both the `prometheus-collector` replicaset and daemonset containers.
  - The Prometheus UI API should return the target metadata for both the `prometheus-collector` replicaset and daemonset containers.
  - The Prometheus UI should return a 200 for its UI pages for both the `prometheus-collector` replicaset and daemonset containers.
- Query Metrics from the AMW
  - Querying for the `up` metric returns data.
- Health Metrics (CCP)
  - **These tests cannot run in the standard CI pipeline.** CCP pods live on standalone underlay clusters, not CI clusters.
  - Run interactively via the [CCP Health Metrics Workflow](ccp/ccp-health-metrics-workflow.md) in a VS Code chat session.
  - Validates all 9 key health metrics: `me_metrics_received_per_minute`, `me_metrics_sent_per_minute`, `me_bytes_sent_per_minute`, `me_metrics_dropped_total`, `otelcol_metrics_received_per_minute`, `otelcol_metrics_sent_per_minute`, `otelcol_metrics_dropped_total`, `otelcol_export_failures_total`, `invalid_metrics_settings_config`.
  - Validates correct labels (computer, release, controller_type).
  - Validates CCP mode active in logs and fluent-bit NOT running.


## Current Labels for Tests
- Unlabeled: These tests should run on every basic cluster.
- `operator`: Tests that should only run when the target allocator is enabled.
- `arc-extension`: Tests that should only run on Arc clusters with the extension enabled.
- `windows`: Tests that should only run on clusters that have Windows nodes.
- `arm64`: Tests that should only run on clusters taht have ARM64 nodes.
- `linux-daemonset-custom-config`: Tests that should only run on clusters that have the ama-metrics-config-node configmap.
- `fips`: Tests that should only run on clusters taht have FIPS nodes.

> **Note:** CCP tests are no longer run as ginkgo-e2e tests. See [CCP Health Metrics Workflow](ccp/ccp-health-metrics-workflow.md).

# File Directory Structure
```
├── test                                                      - e2e test suites to run on clusters. Unit tests are included alongside the golang files.
│   ├── README.md                                             - Info about setting up, writing, and running the tests.
│   ├── ginkgo-e2e                                            - Each test suite is a golang package.
│   │   ├── <test suite package>                              - Each test suite is a golang package.
│   │   │   ├── <ginkgo test suite setup>                     - Ginkgo syntax to setup for any tests in the package.
│   │   │   ├── <ginkgo tests>                                - Actual Ginkgo tests.
│   │   │   ├── go.mod                                        - Used to import the local utils module (and any other packages).
│   │   │   ├── go.sum
│   │   ├── containerstatus                                   - Test container logs have no errors, containers are running, and all processes are running.
│   │   │   ├── suite_test.go                                 - Setup access to the Kubernetes cluster.
│   │   │   ├── container_status_test.go                      - Run the tests for each container that's part of our agent.
│   │   │   ├── go.mod
│   │   │   ├── go.sum
│   │   ├── livenessprobe                                     - Test that the pods detect and restart when a process is not running.
│   │   │   ├── suite_test.go                                 - Setup access to the Kubernetes cluster.
│   │   │   ├── process_liveness_test.go                      - Run the tests for each container that's part of our agent.
│   │   │   ├── go.mod
│   │   │   ├── go.sum
│   │   ├── prometheusui                                      - Test that the Prometheus UI paths are accessible and the API returns data.
│   │   │   ├── suite_test.go                                 - Setup access to the Kubernetes cluster.
│   │   │   ├── prometheus_ui_test.go
│   │   │   ├── go.mod    
│   │   │   ├── go.sum
│   │   ├── querymetrics                                      - Query the AMW and validate the data returned is expected.
│   │   │   ├── suite_test.go                                 - Setup access to the Kubernetes cluster.
│   │   │   ├── query_metrics_test.go
│   │   │   ├── go.mod
│   │   │   ├── go.sum
│   │   ├── utils                                             - Generalized utils functions for the test suites to use.
│   │   │   ├── amw_query_api_utils.go                        - Utils to query metrics from the AMW.
│   │   │   ├── kubernetes_api_utils.go                       - Utils that call the kubernetes API.
│   │   │   ├── prometheus_ui_api_utils.go                    - Utils that call the Prometheus UI.
│   │   │   ├── setup_utils.go                                - Setup functions for cluster access.
│   │   │   ├── constants.go                                  - Defined constants for test labels and transient errors to ignore.
│   │   │   ├── go.mod
│   │   │   ├── go.sum
│   ├── test-cluster-yamls                                    - YAMLs to deploy on your test cluster and CI/CD clusters.
│   │   ├── configmaps                                        - Configmaps for scrape jobs tested.
│   │   ├── customresources                                   - PodMonitor and ServiceMonitors for scrape jobs tested.
│   ├── testkube                                              - YAMLS to deploy on CI/CD clusters for TestKube.
│   │   ├── api-server-permissions.yaml                       - Permissions for the TestKube runner pods to call the API server.
│   │   ├── testkube-test-crs.yaml                            - CRs for TestKube test workflows for AKS CI/CD clusters.
│   │   ├── testkube-test-crs-arc.yaml                        - CRs for TestKube test workflows for Arc CI/CD clusters.
│   │   ├── testkube-test-crs-otel.yaml                       - CRs for TestKube test workflows for OTel CI/CD clusters.
│   │   ├── testkube-test-crs-otelcollector-upgrade.yaml      - CRs for TestKube test workflows for otel collector upgrade.
│   │   ├── run-testkube-workflow.sh                          - Script used to run the test workflows in build pipelines.
│   │   ├── send-testkube-summary.sh                          - Script used to share the test results to Teams channel in build pipelines.
│   ├── arc-conformance                                       - The same Ginkgo tests can be used with the Arc conformance infra, but with a different runner than TestKube.
│   │   ├── arc-conformance.yaml                              - The YAML for the Arc conformance test pod to give to the Arc conformance team to run in the Arc conformance infra.
│   │   ├── Dockerfile                                        - The Dockerfile for building the Arc conformance image that has the Ginkgo tests to run
│   │   ├── e2e_tests.sh                                      - The script to start the container to run the Ginkgo tests
│   │   ├── local-e2e-tests.yaml                              - The YAML to deploy the conformance test pod locally to test any changes before using in the Arc conformance infra.
│   │   ├── README.md
│   ├── ccp                                                   - CCP health metrics validation (standalone-based, see workflow).
│   │   ├── ccp-health-metrics-workflow.md                    - Interactive workflow for validating CCP health metrics.
│   │   ├── tools/                                            - Helper scripts for the CCP workflow.
│   │       ├── check_build.py                                - Check ADO build status for CCP image.
│   ├── ci-cd                                                 - Files related to our CI/CD clusters
│   │   ├── ci-cd-cluster.json                                - ARM template to deploy a new CI/CD cluster
│   │   ├── ci-cd-cluster-parameters.json                     - ARM template parameters
```

# Ginkgo
Tests are run using the [Ginkgo](https://onsi.github.io/ginkgo/) test framework. This is built upon the regular go test framework. It's advantages are that it:
- Has an easily readable test structure using the `Behavior-Driven Development` model that's used in many languages and is applicable outside of GoLang. This model follows a `Given..., When..., Then...` structure. This is implemented in Ginkgo using the `Describe()`, `Context()`, and `It()`/`Specify()` functions. The Ginkgo documentation on [Writing Specs](https://onsi.github.io/ginkgo/#writing-specs) has many examples of this.
- Utilizes the [Gomega assertion package](https://onsi.github.io/gomega/) for easily understandable test failure errors with the goal that the output will tell you exactly what failed.
- Has good support for parallelization and structuring which tests should be run in series and which can be run at the same time to speed up the tests.
- Has extensive documentation and examples from OSS community.

Ginkgo can be used for any tests written in golang, whether they are unit, integration, or e2e tests.

## Bootstrap a Dev Cluster to Run Ginkgo Tests
### Prerequisites
- Follow the [backdoor deployment instructions](../deploy/addon-chart/Readme.md). to deploy your ama-metrics chart onto the cluster.
- Deploy the following apps and configmaps on your cluster:
  - [Linux reference app](../../internal/referenceapp/prometheus-reference-app.yaml)
  - [Windows reference app](../../internal/referenceapp/win-prometheus-reference-app.yaml)
  - [Scraping configmaps](./test-cluster-yamls/configmaps)
  - [Pod and Service Monitor CRs](./test-cluster-yamls/customresources)

### Setup
- Get the query endpoint for your AMW by following [these instructions](https://learn.microsoft.com/en-us/azure/azure-monitor/essentials/prometheus-api-promql#query-endpoint).
- Setup your devbox environment by ensuring the following:
  - Kubectl access is pointed to your cluster.
  - You have cloned this repo and your current directory is pointed to the root.
  - You are connected to the corpnet VPN.
  - You have run `az login` from the terminal you will be running the tests in.

## Running the Tests
- Run the commands below by replacing the placeholders with the AMW query endpoint:
  ```
  (bash) export GOPROXY=https://proxy.golang.org / (powershell) $env:GOPROXY = "https://proxy.golang.org"
  sudo -E go install -v github.com/onsi/ginkgo/v2/ginkgo@latest

  cd otelcollector/test/ginkgo-e2e

  AMW_QUERY_ENDPOINT="<query endpoint>" \
  ginkgo -p -r --keep-going --label-filter='!/./' -ldflags="-s -X github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring.GroupName=azmonitoring.coreos.com" 
  ```
- `AMW_QUERY_ENDPOINT` points to the query API of the AMW connected to the cluster.
- `-ldflags="-s -X github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring.GroupName=azmonitoring.coreos.com"` allows use of the Prometheus Operator client package to get info about PodMonitors and ServiceMonitors under our group name instead of the OSS Prometheus group.
- You can customize which tests are run with `--label-filter`:
  - `--label-filter='!/./` is an expression that runs all tests that don't have a label.
  - `--label-filter='!/./ || LABELNAME` is an expression that runs all tests that don't have a label and tests that have the label `LABELNAME`.
  - `--label-filter='!(arc-extension,windows)'` is an expression that runs all tests, including those with labels, except for tests labeled `arc-extension` or `windows`.
- To run only one package of tests, add the path to the tests in the command. For example, to only run the livenessprobe tests on your cluster:
  ```
  ginkgo -p -r --keep-going ./livenessprobe
  ```
- For more uses of the Ginkgo CLI, refer to the [docs](https://onsi.github.io/ginkgo/#ginkgo-cli-overview).


## Writing Tests and Test Suites
- Each Ginkgo test suite has a function that handles the testing object and abstracts that away. It runs all Ginkgo tests in the same package.
- `BeforeSuite()` and `AfterSuite()` functions can be used for setup and tear-down. We use these for connecting to the cluster to get the kubeconfig and creating a kubernetes go-client.

  ```
  func TestE2E(t *testing.T) {
    // Connects failures to the Gomega assertions
    RegisterFailHandler(Fail)

    RunSpecs(t, "E2E Test Suite")
  }

  var _ = BeforeSuite(func() {
    // Get cluster context and create go-client
  })

  var _ = AfterSuite(func() {
    // Environment cleanup
  })
  ```
- Running `ginkgo bootstrap` in the directory with the golang files will create a starter test suite file for you.

### Running Tests in Parallel
- Ginkgo Test Suites are run one at a time.
- Ginkgo tests inside a suite are run parallely by default unless `Ordered` or `Serial` is specified as a parameter to a Describe function.

#### Example
- These two `DescribeTable()` tests will run at the same time. One tests a replica pod and the other tests a daemonset pod. Because `Ordered` is specified as a Ginkgo `Decorator`, each `Entry` in the table, however, is run one at a time.
- Testing the otelcollector is not running will run at around the same time on the replica pod and daemonset pod. However, MetricsExtension not running won't be tested on each pod until the otelcollector test finishes, so that there's no conflict.

  ```go
  var _ = DescribeTable("The liveness probe should restart the replica pod", Ordered,
    func(namespace, labelName, labelValue, containerName, terminatedMessage, processName string, ...) {
      err := utils.CheckContainerStatus(K8sClient, Cfg, labelName, labelValue, namespace, containerName, terminatedMessage, ...)
      Expect(err).NotTo(HaveOccurred())
    },
    Entry("when otelcollector is not running", ...),
    Entry("when MetricsExtension is not running", ...),
    Entry("when mdsd is not running", ...),
  )

  var _ = DescribeTable("The liveness probe should restart the daemonset pod", Ordered,
    func(namespace, labelName, labelValue, containerName, terminatedMessage, processName string, ...) {
      err := utils.CheckContainerStatus(K8sClient, Cfg, labelName, labelValue, namespace, containerName, terminatedMessage, ...)
      Expect(err).NotTo(HaveOccurred())
    },
    Entry("when otelcollector is not running", ...),
    Entry("when MetricsExtension is not running", ...),
    Entry("when mdsd is not running", ...),
  )
  ```

### Test Filtering for Different Environments
The `Label("labelName")` Ginkgo `Decorator` can be added to any test. This can be used when running the test to filter which tests should be run, depending on the environment or settings enabled.

For example, some tests have the labels ```"arc-extension"``` or ```"operator"``` that should only be run if the environment has the Arc extension or has the operator enabled. To exclude tests with these labels use `--label-filter='!(arc-extension,operator)'`.

To run only tests without labels, use `--label-filter` with the regex expression:
```
ginkgo -r -p --label-filter='!/./'
```
In TestKube, this extra command can be added to the test in `Settings` -> `Variables and Secrets` -> `Arguments`.

## What Kinds of Test Can Be Run?
- Unit tests for golang code.
- Any functionalities of the Kubernetes go-client package can be used for Kubernetes-specific tests. This includes:
  - Checking the status or spec of a Kubernetes resource (deployment, pod, configmap, container, CR, etc.)
  - Pulling the container logs.
  - Running exec commands on a container.
- Using the Query API to query an Azure Monitor Workspace to verify metrics are ingested.

### Unit Tests
An outline of tests for the prometheus-collector-settings section of the configmap is below. With this, we can have many configmap test files and ensure each combination is parsed and the correct prometheus config and environment variables are produced.

```golang
var _ = Describe("ConfigMapParser", func() {
  Describe("ParsePrometheusCollectorSettings", func() {
    Context("when the configmap settings file does not exist", func() {
      It("should return empty values and no error", func() {
        defaultMetricAccountName, clusterAlias, isOperatorEnabled, err := configmapparser.ParsePrometheusCollectorSettings()
        Expect(err).To(BeNil())
        Expect(defaultMetricAccountName).To(Equal(""))
        Expect(clusterAlias).To(Equal(""))
        Expect(isOperatorEnabled).To(Equal(""))
      })
    })

    Context("when the configmap settings file exists", func() {
      BeforeEach(func() {
        // Setup an example configmap settings file for testing
      })

      AfterEach(func() {
        // Remove the temporary configmap settings file
      })

      It("should parse the configmap settings file and return the values", func() {
        defaultMetricAccountName, clusterAlias, isOperatorEnabled, err := configmapparser.ParsePrometheusCollectorSettings()
        Expect(err).To(BeNil())

        // Assert the expected values
        Expect(defaultMetricAccountName).To(Equal("expected_default_metric_account_name"))
        Expect(clusterAlias).To(Equal("expected_cluster_alias"))
        Expect(isOperatorEnabled).To(Equal("expected_operator_enabled"))
      })

      It("should handle parsing errors and return an error", func() {
        // Create a temporary configmap settings file with invalid content for testing parsing errors

        // Call the ParsePrometheusCollectorSettings function
        _, _, _, err := configmapparser.ParsePrometheusCollectorSettings()

        // Assert that an error is returned
        Expect(err).To(HaveOccurred())
      })
    })
  })
})
```

### E2E Tests
These tests can be run on a dev cluster that you have kubeconfig/kubectl access to, or can be run directly inside CI/CD kubernetes clusters by using TestKube.

#### Packages
- [k8s.io/client-go/kubernetes](https://pkg.go.dev/k8s.io/client-go/kubernetes)
- [k8s.io/api/core/v1](https://pkg.go.dev/k8s.io/api/core/v1)
- [github.com/prometheus/client_golang/api](https://pkg.go.dev/github.com/prometheus/client_golang/api)
- [github.com/prometheus/client_golang/api/prometheus/v1](https://pkg.go.dev/github.com/prometheus/client_golang/api/prometheus/v1)
- [github.com/prometheus-operator/prometheus-operator/pkg/client/versioned](https://pkg.go.dev/github.com/prometheus-operator/prometheus-operator/pkg/client/versioned)

# TestKube
[Testkube](https://docs.testkube.io/) is an OSS runner framework for running the tests inside a Kubernetes cluster. It is deployed as a helm chart on the cluster. Ginkgo is included as one of the out-of-the-box executors supported.

Behind the scenes, tests and executors are custom resources. Running a test starts a job that deploys the test executor pod which runs the Ginkgo tests (or a different framework setup).

Some highlights are that:
- Includes test history, pass rate, and execution times.
- An easy Golang integration with out-of-the-box Ginkgo runner.
- A [Teams channel notification](https://docs.testkube.io/articles/webhooks#microsoft-teams) can integrated with testkube for notifying if a test failed. These tests can be run after every merge to main or scheduled to be run on an interval.
- There are many other test framework integrations including curl and postman for testing Kubernetes services and their APIs. There is also a k6 and jmeter integration for performance testing Kubernetes services.

## Getting Started
- Install the CLI on linux/WSL:
  ```bash
    wget -qO - https://repo.testkube.io/key.pub | sudo apt-key add -
    echo "deb https://repo.testkube.io/linux linux main" | sudo tee -a /etc/apt/sources.list
    sudo apt-get update
    sudo apt-get install -y testkube=1.14.2
  ```
  Other OS installation instructions are [here](https://docs.testkube.io/articles/install-cli/).
- Install the [helm chart](https://docs.testkube.io/articles/helm-chart/) on your cluster:
  ```bash
  helm repo add kubeshop https://kubeshop.github.io/helm-charts
  helm upgrade testkube kubeshop/testkube --install --create-namespace -f values.yaml -n testkube
  ```
- The helm chart will install in the namespace `testkube`.
- Run `testkube dashboard` to port-forward the dashboard.
- Create a test connected to the Github repository and branch. Tests are a custom resource behind the scenes and can be created with the UX, CLI, or applying a CR. Tests can be run through the UX or CLI.
- Apply the yaml [api-server-permissions.yaml](./testkube/api-server-permissions.yaml) to update the permissions needed for the Ginkgo executor to be able to make calls to the API server:
  ```
  cd ./testkube
  kubectl apply -f api-server-permissions.yaml
  ```

## Bootstrap a CI/CD Cluster to Run TestKube Tests
- Create a new cluster using the [ARM template](./ci-cd) as a starting point with the nodepool type matrix. This template does the following and can be edited to create a private cluster or http(s) proxy cluster:
  - Creates an AMW in the subscription and resource group the ARM template is deployed in.
  - Creates an AKS cluster in the subscription and resource group the ARM template is deployed in with the following nodepools:
    - AMD64 Ubuntu Linux
    - FIPS-Enabled AMD64 Ubuntu Linux
    - ARM64 Ubuntu Linux
    - AMD64 Mariner Linux
    - ARM64 Mariner Linux
    - Windows 2019
    - Windows 2022
  - Creates the DCE, DCR, and DCRA for the AMW and AKS cluster.
  - Creates the recording rules for Linux and Windows.
  - [Optional] The alert rule group for CI/CD ICM alerting can be changed from `enabled: false` to `enabled: true`.
- Install the ama-metrics agent helm chart through the [backdoor deployment](../deploy/addon-chart/Readme.md#step-3-go-to-addon-chart-directory) starting at Step 3.
- Deploy the following apps and configmaps on the cluster:
  - [Linux reference app](../../internal/referenceapp/prometheus-reference-app.yaml)
  - [Windows reference app](../../internal/referenceapp/win-prometheus-reference-app.yaml)
  - [Scraping configmaps](./test-cluster-yamls/configmaps)
  - [Pod and Service Monitor CRs](./test-cluster-yamls/customresources)
- Follow the steps in the above `Getting Started` section to install TestKube on the cluster and give permissions to the Ginkgo executor to call the API server.
- Run the following to add the existing tests to the cluster:
  ```
  cd ./testkube
  kubectl apply -f testkube-test-crs.yaml
  ```
- Get the full resource ID of your AMW and the client ID of the AKS cluster kubelet managed identity. Run the following command to allow query access from the cluster. Due to access policies, this may need to be run from Cloud Shell in the Portal:

  ```
  az role assignment create --assignee <client ID>  --role "Monitoring Data Reader" --scope <AMW resource ID>
  ```
- The file `testkube-test-crs.yaml` will also be applied through the build pipeline for every merge to main right before the tests are run. This is so that any updates can be checked in, consistent between CI/CD clusters, and applied to all clusters at once.
- Add to the `Deploy_AKS_Chart` job in the pipeline yaml to deploy the chart to another cluster. Replace the `azureResourceGroup` and `kubernetesCluster` with the corresponding values.
  ```
  - task: HelmDeploy@0
    displayName: "Deploy: <cluster-name> cluster"
    inputs:
      connectionType: 'Azure Resource Manager'
      azureSubscription: 'ContainerInsights_Build_Subscription(9b96ebbd-c57a-42d1-bbe9-b69296e4c7fb)'
      azureResourceGroup: 'cluster-resource-group'
      kubernetesCluster: 'cluster-name'
      namespace: 'default'
      command: 'upgrade'
      chartType: 'FilePath'
      chartPath: '$(Build.SourcesDirectory)/otelcollector/deploy/addon-chart/azure-monitor-metrics-addon/'
      releaseName: 'ama-metrics'
      waitForExecution: false
      arguments: --dependency-update --values $(Build.SourcesDirectory)/otelcollector/deploy/addon-chart/azure-monitor-metrics-addon/values.yaml
  ```
- Add running the tests on the cluster to the build pipeline yaml. Replace the `az aks get-credentials` command with the corresponding cluster name and resource group. Also, copy the [run-testkube-workflow.sh](./testkube/run-testkube-workflow.sh) in the respective folder.
  ```
  - deployment: Testkube
    displayName: "Test: AKS testkube tests"
    environment: Prometheus-Collector
    dependsOn: Deploy_AKS_Chart
    condition: and(succeeded(), and(eq(variables.IS_PR, false), eq(variables.IS_MAIN_BRANCH, true)))
    variables:
      HELM_CHART_NAME: $[ stageDependencies.Build.Image_Tags_and_Ev2_Artifacts.outputs['setup.HELM_CHART_NAME'] ]
      HELM_SEMVER: $[ stageDependencies.Build.Image_Tags_and_Ev2_Artifacts.outputs['setup.SEMVER'] ]
      IMAGE_TAG: $[ stageDependencies.Build.Image_Tags_and_Ev2_Artifacts.outputs['setup.SEMVER'] ]
      IMAGE_TAG_WINDOWS: $[ stageDependencies.Build.Image_Tags_and_Ev2_Artifacts.outputs['setup.WINDOWS_IMAGE_TAG'] ]
      HELM_FULL_IMAGE_NAME: $[ stageDependencies.Build.Image_Tags_and_Ev2_Artifacts.outputs['setup.HELM_FULL_IMAGE_NAME'] ]
      skipComponentGovernanceDetection: true
    templateContext:
      type: releaseJob
      isProduction: false
      inputs:
      - input: pipelineArtifact
        artifactName: testkube-test-files
        targetPath: $(Pipeline.Workspace)
    strategy:
      runOnce:
        deploy:
          steps:
          - task: AzureCLI@1
            displayName: Get kubeconfig
            inputs:
              azureSubscription: 'ContainerInsights_Build_Subscription(9b96ebbd-c57a-42d1-bbe9-b69296e4c7fb)'
              scriptLocation: 'inlineScript'
              inlineScript: 'az aks get-credentials -g cluster-resource-group -n cluster-name' 
          - bash: |
              export BUILD_ARTIFACTSTAGINGDIRECTORY="$(Build.ArtifactStagingDirectory)"
              export BUILD_BUILDID="$(Build.BuildId)"
              export SYSTEM_JOBID="$(System.JobId)"
              export SYSTEM_TASKINSTANCEID="$(System.TaskInstanceId)"

              chmod +x ./testkube/run-testkube-workflow.sh
              ./testkube/run-testkube-workflow.sh \
                <amw_query_endpoint> \
                "<cluster managed identity client ID>" \
                "testkube-test-crs.yaml" \
                "testkube-test-crs-<cluster-name>.yaml" \
                "" \
                "" \
                <env-name>
            workingDirectory: $(Pipeline.Workspace)
            displayName: "Run TestKube workflow"
  ```

# Processes
## When to Run Each Test
- During development of a feature or fix, run the e2e tests following the instructions to bootstrap your cluster to run Ginkgo tests.
- In your PR, use the PR checklist to include results of the e2e tests on your cluster.
- After merging the PR into main, the new main build will be deployed on the CI/CD clusters. The e2e tests will be run on the cluster through TestKube. The pipeline is locked to deploy a new chart and run tests sequentially for only one merge at a time, so that there is no conflict between PRs merged around the same time. The tests in the Testkube test suite `e2e-tests-merge` will be run.
- The TestKube tests in the test suite `e2e-tests-nightly` will be run every night. This includes longer-running tests such as the liveness probe tests.

## Viewing TestKube Results
All CI/CD cluster results will be sent to our [TestKube teams channel](https://teams.microsoft.com/l/channel/19%3Aef162826eb094f25885b8c02392b7b6f%40thread.tacv2/TestKube?groupId=992de6aa-c74c-430e-9bec-3ead89525bcd&tenantId=72f988bf-86f1-41af-91ab-2d7cd011db47).

### View Results from Merge Test Suite
The merge tests are run on the dev CI/CD cluster as part of the `Deploy` stage of the [build pipeline](https://github-private.visualstudio.com/azure/_build?definitionId=440&_a=summary). The job will be named `Test: AKS testkube tests`. Scroll to the bottom of the logs to view the detailed results.

### View Results from Nightly Test Suite
1. Connect to the CI/CD cluster to have your kubeconfig pointing to it in your terminal.
2. Have the [TestKube CLI](https://docs.testkube.io/articles/install/cli) installed in your terminal.
3. Run `testkube get testsuite` to get results similar to below:
    ```
    Context:  (2.1.95)   Namespace: testkube
    ----------------------------------------

      NAME              | DESCRIPTION | STEPS | LABELS | SCHEDULE  | STATUS | EXECUTION ID
    --------------------+-------------+-------+--------+-----------+--------+---------------------------
      e2e-tests-merge   |             |     3 |        |           | failed | 67b8d6507b2c2db2fb150f79
      e2e-tests-nightly |             |     4 |        | 0 7 * * * | failed | 67b82471355d19c08cf7865b
    ```
4. Use the `EXECUTION ID` for `e2e-tests-nightly` to run `testkube get testsuiteexecution <execution ID>` to get results similar to below:

    ```
    Context:  (2.1.95)   Namespace: testkube
    ----------------------------------------
    Id:       67b82471355d19c08cf7865b
    Name:     ts-e2e-tests-nightly-313
    Status:   failed
    Duration: 1h17m8.542s

    Labels:
    Disabled webhooks: false

    Test Suite URI: /test-suites/e2e-tests-nightly
    Test Suite Execution URI: /test-suites/e2e-tests-nightly/executions/67b82471355d19c08cf7865b

      STATUSES               | STEP                           | IDS                            | ERRORS


    -------------------------+--------------------------------+--------------------------------+-------------------
      passed, passed, passed | containerstatus, prometheusui, | 67b82471355d19c08cf7865c,      | "", "", ""


                            | operator                       | 67b82471355d19c08cf7865d,      |


                            |                                | 67b82471355d19c08cf7865e       |


      passed                 | 2m0s                           | ""                             | ""


      failed                 | querymetrics                   | 67b82471355d19c08cf78660       | <error messages>
    ```
5. View an easier-to-read error message for any tests that failed by using the `ID` of the test that failed to run `testkube get execution <ID>` to get results similar to below:
    ```
    Running Suite: Query Metrics Test Suite - /data/repo/otelcollector/test/ginkgo-e2e/querymetrics
    ===============================================================================================
    Random Seed: 1740122572 - will randomize all specs

    Will run 13 of 13 specs
    Running in parallel across 4 processes
    •
    ------------------------------
    • [FAILED] [0.369 seconds]
    Query Metrics Test Suite When querying metrics [It] should return the expected results for up=1 for all default jobs
    /data/repo/otelcollector/test/ginkgo-e2e/querymetrics/query_metrics_test.go:307

      [FAILED] Expected
          <string>: UP
      to equal
          <string>: up
      In [It] at: /data/repo/otelcollector/test/ginkgo-e2e/querymetrics/query_metrics_test.go:326 @ 02/21/25 07:24:27.073

      Full Stack Trace
        prometheus-collector/otelcollector/test/querymetrics.glob..func1.3.1()
            /data/repo/otelcollector/test/ginkgo-e2e/querymetrics/query_metrics_test.go:326 +0x42d
    ------------------------------
    •
    ------------------------------
    • [FAILED] [0.104 seconds]
    Query Metrics Test Suite should return the expected labels for specified metrics in each job [It] External labels are applied from DaemonSet Configmap
    /data/repo/otelcollector/test/ginkgo-e2e/querymetrics/query_metrics_test.go:296

      [FAILED] Expected label "external_label_1" not found in metric "up" for the job node-configmap
      Expected
          <bool>: false
      to be true
      In [It] at: /data/repo/otelcollector/test/ginkgo-e2e/querymetrics/query_metrics_test.go:265 @ 02/21/25 07:24:27.205

      Full Stack Trace
        prometheus-collector/otelcollector/test/querymetrics.glob..func1.2({0x177f0b5, 0xe}, {0x1775779, 0x2}, 0x0?)
            /data/repo/otelcollector/test/ginkgo-e2e/querymetrics/query_metrics_test.go:265 +0x658
        reflect.Value.call({0x156afe0?, 0x18723d0?, 0x13?}, {0x1776068, 0x4}, {0xc000140230, 0x3, 0x3?})
            /usr/local/go/src/reflect/value.go:596 +0xce7
        reflect.Value.Call({0x156afe0?, 0x18723d0?, 0x1a04528?}, {0xc000140230?, 0x0?, 0x0?})
            /usr/local/go/src/reflect/value.go:380 +0xb9
    ------------------------------
    •••••••••

    Summarizing 2 Failures:
      [FAIL] Query Metrics Test Suite should return the expected labels for specified metrics in each job [It] External labels are applied from DaemonSet Configmap
      /data/repo/otelcollector/test/ginkgo-e2e/querymetrics/query_metrics_test.go:265
      [FAIL] Query Metrics Test Suite When querying metrics [It] should return the expected results for up=1 for all default jobs
      /data/repo/otelcollector/test/ginkgo-e2e/querymetrics/query_metrics_test.go:326

    Ran 13 of 13 Specs in 12.489 seconds
    FAIL! -- 11 Passed | 2 Failed | 0 Pending | 0 Skipped


    Ginkgo ran 1 suite in 1m46.495464269s
    ```

## Upgrading
### Upgrade Testkube version
1. Connect to the CI/CD cluster to have your kubeconfig pointing to it in your terminal.
2. Have the lasted version of the [TestKube CLI](https://docs.testkube.io/articles/install/cli) installed in your terminal.
3. Run `testkube upgrade` to upgrade the TestKube helm chart to the latest version.

### Upgrade Golang Version
1. The required Golang version in the `go.mod` files in the `ginkgo-e2e` directory will always need to be `<=` the Golang version of the TestKube Ginkgo runner.  
2. Check the Golang version of the TestKube Ginkgo runner in the [Dockerfile](https://github.com/kubeshop/testkube/blob/main/contrib/executor/ginkgo/build/agent/Dockerfile) of the TestKube repo.
3. Update the version in the `go.mod` files and the `TESTKUBE_GOLANG_VERSION` variable in the [build pipeline YAML](../../.pipelines/azure-pipeline-build.yml).

### Upgrade Golang Package Dependencies
1. From the `ginkgo-e2e` directory, run `./update-go-packages.sh` to upgrade all package dependencies to the latest.

## Creating a New Test or Test Suite
- Any test added inside a test suite will automatically be picked up to run after merging to main.
- Any test suite added should be included in [testkube-test-crs.yaml](./testkube/testkube-test-crs.yaml) that will be applied on the CI/CD clusters.
- Any additional permissions needed for access to the API server should be added to [api-server-permissions.yaml](./testkube/api-server-permissions.yaml).
- If a new scrape job is required for a test, add the scrape job to the correct configmap or add a custom resource under the folder [test-cluster-yamls](./test-cluster-yamls/).
- If you add a new label:
  - Use a constant for the string in the constants.go file.
  - Add the label and description in the labels section of this README.
  - Add the label to the PR checklist file.
  - Add the label where needed in [testkube-test-crs.yaml](./testkube/testkube-test-crs.yaml).

## PR Checklist
Test processes for a PR are covered in the [PR checklist](/.github/pull_request_template.md).
