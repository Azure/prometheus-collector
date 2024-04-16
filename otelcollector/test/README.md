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

## Current Labels for Tests
- Unlabeled: These tests should run on every basic cluster.
- `operator`: Tests that should only run when the target allocator is enabled.
- `arc-extension`: Tests that should only run on Arc clusters with the extension enabled.
- `windows`: Tests that should only run on clusters that have Windows nodes.
- `arm64`: Tests that should only run on clusters taht have ARM64 nodes.
- `linux-daemonset-custom-config`: Tests that should only run on clusters that have the ama-metrics-config-node configmap.

# File Directory Structure
```
├── test                                 - e2e test suites to run on clusters. Unit tests are included alongside the golang files.
│   ├── README.md                        - Info about setting up, writing, and running the tests.
│   ├── <test suite package>             - Each test suite is a golang package.
│   │   ├── <ginkgo test suite setup>    - Ginkgo syntax to setup for any tests in the package.
|   |   |── <ginkgo tests>               - Actual Ginkgo tests.
|   |   |── go.mod                       - Used to import the local utils module (and any other packages).
|   |   |── go.sum
│   ├── containerstatus                  - Test container logs have no errors, containers are running, and all processes are running.
│   │   ├── suite_test.go                - Setup access to the Kubernetes cluster.
|   |   |── container_status_test.go     - Run the tests for each container that's part of our agent.
|   |   |── go.mod
|   |   |── go.sum
│   ├── livenessprobe                    - Test that the pods detect and restart when a process is not running.
│   │   ├── suite_test.go                - Setup access to the Kubernetes cluster.
|   |   |── process_liveness_test.go     - Run the tests for each container that's part of our agent.
|   |   |── go.mod
|   |   |── go.sum
│   ├── prometheusui                     - Test that the Prometheus UI paths are accessible and the API returns data.
│   │   ├── suite_test.go                - Setup access to the Kubernetes cluster.
|   |   |── prometheus_ui_test.go
|   |   |── go.mod
|   |   |── go.sum
│   ├── querymetrics                     - Query the AMW and validate the data returned is expected.
│   │   ├── suite_test.go                - Setup access to the Kubernetes cluster.
|   |   |── query_metrics_test.go
|   |   |── go.mod
|   |   |── go.sum
│   ├── utils                            - Generalized utils functions for the test suites to use.
|   |   |── amw_query_api_utils.go       - Utils to query metrics from the AMW.
|   |   |── kubernetes_api_utils.go      - Utils that call the kubernetes API.
|   |   |── prometheus_ui_api_utils.go   - Utils that call the Prometheus UI.
|   |   |── setup_utils.go               - Setup functions for cluster access.
|   |   |── constants.go                 - Defined constants for test labels and transient errors to ignore.
|   |   |── go.mod
|   |   |── go.sum
│   ├── test-cluster-yamls               - YAMLs to deploy on your test cluster and CI/CD clusters.
|   |   |── configmaps                   - Configmaps for scrape jobs tested.
|   |   |── customresources              - PodMonitor and ServiceMonitors for scrape jobs tested.
│   ├── testkube                         - YAMLS to deploy on CI/CD clusters for TestKube.
|   |   |── api-server-permissions.yaml  - Permissions for the TestKube runner pods to call the API server.
|   |   |── testkube-test-crs.yaml       - CRs for TestKube test suites and tests for AKS CI/CD clusters.
|   |   |── testkube-test-crs-arc.yaml   - CRs for TestKube test suites and tests for Arc CI/CD clusters.
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
- Get the full resource ID of your AMW and run the following command to get a service principal to allow query access to your AMW:

  ```
  az ad sp create-for-rbac --name <myAMWQuerySP> --role "Monitoring Data Reader" --scopes <AMW resource ID>
  ```

- The JSON output should be similar to below. Save the `appId` as the Client ID and the `password` as the Client Secret.

  ```
  {
    "appId": "myAMWQuerySP",
    "displayName": "myAMWQuerySP",
    "password": "myServicePrincipalPassword",
    "tenant": "myTentantId"
  }
  ```

- Get the query endpoint for your AMW by following [these instructions](https://learn.microsoft.com/en-us/azure/azure-monitor/essentials/prometheus-api-promql#query-endpoint).
- Setup your devbox environment by ensuring the following:
  - Kubectl access is pointed to your cluster.
  - You have cloned this repo and your current directory is pointed to the root.
  - You are connected to the corpnet VPN.

## Running the Tests
- Run the commands below by replacing the placeholders with the SP Client ID, SP Secret, and the AMW query endpoint:
  ```
  (bash) export GOPROXY=https://proxy.golang.org / (powershell) $env:GOPROXY = "https://proxy.golang.org"
  sudo -E go install -v github.com/onsi/ginkgo/v2/ginkgo@latest

  cd otelcollector/test

  AMW_QUERY_ENDPOINT="<query endpoint>" QUERY_ACCESS_CLIENT_ID="<client ID>" QUERY_ACCESS_CLIENT_SECRET="<client secret>" \
  ginkgo -p -r --keep-going --label-filter='!/./' -ldflags="-s -X github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring.GroupName=azmonitoring.coreos.com" 
  ```
- `AMW_QUERY_ENDPOINT`, `QUERY_ACCESS_CLIENT_ID`, and `QUERY_ACCESS_CLIENT_SECRET` give access to query from the AMW connected to the cluster.
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
- Has an integrated dashboard to view results, set up tests, test suites, test schedules, etc. with a UX as an alternative to the (also available) CLI.
- Includes test history, pass rate, and execution times.
- Friendly user interface and easy Golang integration with out-of-the-box Ginkgo runner.
- A [Teams channel notification](https://docs.testkube.io/articles/webhooks#microsoft-teams) can integrated with testkube for notifying if a test failed. These tests can be run after every merge to main or scheduled to be run on an interval.
- Test suites can be created out of tests with a dependency flowchart that can be set up for if some tests should run at the same time or after others, or only run if one succeeds.
- There are many other test framework integrations including curl and postman for testing Kubernetes services and their APIs. There is also a k6 and jmeter integration for performance testing Kubernetes services.
- The TestKube dashboard must be accessed from within the cluster for now unless we set up an outside endpoint.

## Getting Started
- Install the CLI on linux/WSL:
  ```bash
    wget -qO - https://repo.testkube.io/key.pub | sudo apt-key add -
    echo "deb https://repo.testkube.io/linux linux main" | sudo tee -a /etc/apt/sources.list
    sudo apt-get update
    sudo apt-get install -y testkube
  ```
  Other OS installation instructions are [here](https://docs.testkube.io/articles/install-cli/).
- Install the [helm chart](https://docs.testkube.io/articles/helm-chart/) on your cluster:
  ```bash
  helm repo add kubeshop https://kubeshop.github.io/helm-charts
  helm install --create-namespace testkube kubeshop/testkube
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
- The file `testkube-test-crs.yaml` will also be applied through the build pipeline for every merge to main right before the tests are run. This is so that any updates can be checked in, consistent between CI/CD clusters, and applied to all clusters at once.
- Create the kubernetes secret with the AMW access through the TestKube UI:
  - Go to the test `Settings` -> `Variables & Secrets` -> `Add a new variable` -> `Secret`.
  - Insert the values for `AMW_QUERY_ENDPOINT`, `QUERY_ACCESS_CLIENT_ID`, `QUERY_ACCESS_TOKEN_SECRET`.
  - TestKube will create a secret named `testkube/<test-name>-testvars` with these fields as keys and values.
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
- Add running the tests on the cluster to the build pipeline yaml. Replace the `az aks get-credentials` command with the corresponding cluster name and resource group.
  ```
  - deployment: Testkube
    displayName: "Test: run testkube tests"
    environment: Prometheus-Collector
    dependsOn: Deploy_AKS_Chart
    pool:
      name: Azure-Pipelines-CI-Test-EO
    condition: and(eq(variables.IS_PR, false), eq(variables.IS_MAIN_BRANCH, true))
    variables:
      skipComponentGovernanceDetection: true
    strategy:
      runOnce:
        deploy:
          steps:
          - bash: |
              wget -qO - https://repo.testkube.io/key.pub | sudo apt-key add -
              echo "deb https://repo.testkube.io/linux linux main" | sudo tee -a /etc/apt/sources.list
              sudo apt-get update
              sudo apt-get install -y testkube
            workingDirectory: $(Build.SourcesDirectory)
            displayName: "Install testkube CLI"
          - task: AzureCLI@1
            displayName: Get kubeconfig
            inputs:
              azureSubscription: 'ContainerInsights_Build_Subscription(9b96ebbd-c57a-42d1-bbe9-b69296e4c7fb)'
              scriptLocation: 'inlineScript'
              inlineScript: 'az aks get-credentials -g cluster-resource-group -n cluster-name'
          - bash: |
              sleep 120
            displayName: "Wait for cluster to be ready"
          - bash: |
              # Run the full test suite
              kubectl testkube run testsuite e2e-tests-merge --verbose

              # Get the current id of the test suite now running
              execution_id=$(kubectl testkube get testsuiteexecutions --test-suite e2e-tests-merge --limit 1 | grep e2e-tests | awk '{print $1}')

              # Watch until the all the tests in the test suite finish
              kubectl testkube watch testsuiteexecution $execution_id

              # Get the results as a formatted json file
              kubectl testkube get testsuiteexecution $execution_id --output json > testkube-results.json

              # For any test that has failed, print out the Ginkgo logs
              if [[ $(jq -r '.status' testkube-results.json) == "failed" ]]; then

                # Get each test name and id that failed
                jq -r '.executeStepResults[].execute[] | select(.execution.executionResult.status=="failed") | "\(.execution.testName) \(.execution.id)"' testkube-results.json | while read line; do
                  testName=$(echo $line | cut -d ' ' -f 1)
                  id=$(echo $line | cut -d ' ' -f 2)
                  echo "Test $testName failed. Test ID: $id"

                  # Get the Ginkgo logs of the test
                  kubectl testkube get execution $id > out 2>error.log

                  # Remove superfluous logs of everything before the last occurence of 'go downloading'.
                  # The actual errors can be viewed from the ADO run, instead of needing to view the testkube dashboard.
                  cat error.log | tac | awk '/go: downloading/ {exit} 1' | tac
                done

                # Explicitly fail the ADO task since at least one test failed
                exit 1
              fi
            workingDirectory: $(Build.SourcesDirectory)
            displayName: "Run tests"
  ```

# Processes
## When to Run Each Test
- During development of a feature or fix, run the e2e tests following the instructions to bootstrap your cluster to run Ginkgo tests.
- In your PR, use the PR checklist to include results of the e2e tests on your cluster.
- After merging the PR into main, the new main build will be deployed on the CI/CD clusters. The e2e tests will be run on the cluster through TestKube. The pipeline is locked to deploy a new chart and run tests sequentially for only one merge at a time, so that there is no conflict between PRs merged around the same time. The tests in the Testkube test suite `e2e-tests-merge` will be run.
- The TestKube tests in the test suite `e2e-tests-nightly` will be run every night. This includes longer-running tests such as the liveness probe tests.

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
