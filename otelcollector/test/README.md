# Ginkgo
Tests are run using the [Ginkgo](https://onsi.github.io/ginkgo/) test framework. This is built upon the regular go test framework. It's advantages are that it:
- Has an easily readable test structure using the `Behavior-Driven Development` model that's used in many languages and is applicable outside golang. Follows a `Given..., When..., Then...` model. This is implemented in Ginkgo using the `Describe()`, `Context()`, and `It()`/`Specify()` functions.
- Utilizes the [Gomega assertion package](https://onsi.github.io/gomega/) for easily understandable test failure errors with the goal that the output will tell you exactly what failed.
- Has good support for parallelization and structuring which tests should be run in series and which can be run at the same time to speed up the tests.
- Has extensive documentation and examples from OSS community.

Ginkgo can be used for any tests written in golang, whether they are unit, integration, or e2e tests.

## Install Locally on your Dev Machine
- While not in a directory that has a go.mod file, run `sudo -E go install -v github.com/onsi/ginkgo/v2/ginkgo@latest`. This installs the ginkgo command-line tool used to run Ginkgo tests.
- Run `ginkgo version` to check the installation succeeded. You may need to add `$GOBIN` to your `$PATH`.

## Run Tests Locally from your Dev Machine
- Change to a directory with a ginkgo test suite file. Run `ginkgo -p`. This runs all the tests in that package with parallelization enabled.

## Writing Tests and Test Suites
- Each Ginkgo test suite has a function that handles the testing object and abstracts that away. It runs all Ginkgo tests in the same package.
- BeforeSuite() and AfterSuite() functions can be used for setup and teardown. We use these for connecting to the cluster to get the kubeconfig and creating a kubernetes go-client.

  ```
  func TestE2E(t *testing.T) {
    // Connects failures to the Gomega assertions
    RegisterFailHandler(Fail)

    RunSpecs(t, "E2E Test Suite")
  }

  var _ = BeforeSuite(func() {
    // Get cluster context and create go-client clientset
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
- Testing the otelcollector is not running will run at around the same time on the replica pod and daemonset pod. However, MetricsExtension not running won't be tested on each pod until the otelcollector test finishes.

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
The ```Label("labelName")``` Ginkgo Decorator can be added to any test. This can be used when running the test to filter which tests should be run, depending on the environment or settings enabled.

For example, some tests have the labels ```"arc-extension"``` or ```"operator"``` that should only be run if the environment has the Arc extension or has the operator enabled.

To run tests for the addon without the operator enabled, run

```
ginkgo -p --label=filter '!(operator,arc-extension)'
```

In TestKube, this extra command can be added to the test in `Settings` -> `Variables and Secrets` -> `Arguments`.

## What Kinds of Test Can Be Run?
- Unit tests for golang code.
- Any functionalities of the Kubernetes go-client package can be used for Kubernetes-specific tests. This includes:
  - Checking the status or spec of a Kubernetes resource (deployment, pod, configmap, container, CR, etc.)
  - Pulling the container logs.
  - Running exec commands on a container.
- Use the Query API to query an Azure Monitor Workspace to verify metrics are ingested.

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
Much of the agent functionality cannot be tested with just unit tests and relies on making sure everything is working inside the container. For this, we use the Kubernetes go-client package to make calls to the Kubernetes API server. This is where a lot of the pre-release manual testing can be implemented with the automated tests. The `containerstatus` and `livenessprobe` packages are examples of these tests, which use the functions in the `utils` package to get the container status and perform container operations.

In the case of E2E tests, the coding language does not matter since we are not testing the code directly, but instead the functionality of our containers running.

These tests can be run on a dev cluster that you have kubeconfig/kubectl access to, or can be run directly inside CI/CD kubernetes clusters by using TestKube.


# Bootstrap a Cluster to Run Ginkgo Tests
- Follow the backdoor deployment instructions to deploy your ama-metrics chart onto the cluster.
- Get the full resource ID of your AMW and run the following command to get a service principal to allow query access to your AMW:

  ```
  az ad sp create-for-rbac --name <myAMWQuerySP> \
  --role "Monitoring Data Reader" \
  --scopes <AMW resource ID>
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
- With kubectl access to your cluster and your directory pointed to the cloned repo, run the following and replace the placeholders with the SP Client ID and Secret:

  ```
  sudo -E go install -v github.com/onsi/ginkgo/v2/ginkgo@latest

  cd otelcollector/test

  AMW_QUERY_ENDPOINT="<query endpoint>" QUERY_CLIENT_ID="<client ID>" QUERY_CLIENT_SECRET="<client secret>" \
  ginkgo -p --label-values='!(operator || arc-extension)'
  ```

- To run only one package of tests, add the path to the tests in the command. For example, to only run the livenessprobe tests on your cluster:

  ```
  ./livenessprobe
  ```

- For more uses of the Ginkgo CLI, refer to the [docs](https://onsi.github.io/ginkgo/#ginkgo-cli-overview).

# TestKube
[Testkube](https://docs.testkube.io/) is an OSS runner framework for running the tests inside a Kubernetes cluster. It is deployed as a helm chart on the cluster. Ginkgo is included as one of the out-of-the-box executors supported.

Behind the scenes, tests and executors are custom resources. Running a test starts a job that deploys the test executor pod which runs the Ginkgo tests (or a different framework setup).

Some highlights are that:
- Has an integrated dashboard to view results, set up tests, test suites, test schedules, etc. with a UX as an alternative to the (also available) CLI.
- Includes test history, pass rate, and execution times.
- Friendly user interface and easy Golang integration with out-of-the-box Ginkgo runner.
- A [Teams channel notification](https://docs.testkube.io/articles/webhooks#microsoft-teams) can integrated with testkube for notifying if a test failed. These tests can be run after every merge to main or scheduled to be run on an interval.
- Test suites can be created out of tests with a dependency flowchart that can be set up for if some tests should run at the same time or after others, or only run if one succeeds.
- Many other test framework integrations including curl and postman for testing Kubernetes services and their APIs. Also has a k6 and jmeter integration for performance testing Kubernetes services.
- Dashboard must be accessed from within the cluster for now unless we set up an outside endpoint.

### Getting Started
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
  helm install --create-namespace my-testkube kubeshop/testkube
  ```
- The helm chart will install in the namespace `testkube`.
- Run `testkube dashboard` to port-forward the dashboard.
- Create a `clusterrole` and `clusterrolebinding` for the Ginkgo runner's service account with access to the Kubernetes API server.
- Create a test connected to the Github repository and branch. Tests are a custom resource behind the scenes and can be created with the UX, CLI, or applying a CR. Tests can be run through the UX or CLI.

# When to Run Each Test
- Unit tests can be included as part of the PR checks.
- E2E tests can be run from dev machine pointing to your test cluster kubeconfig that is running the dev image.
- E2E tests run after deploying to dev and prod CI/CD clusters. A [Teams channel notification](https://docs.testkube.io/articles/webhooks#microsoft-teams) can integrated with testkube for notifying if it failed. These tests can be run after every merge to main or scheduled to be run on an interval.

## Utilizing a PR Checklist
- Have the e2e tests been run on a test cluster?
- If PR is a feature, have tests been added?
- If PR is a fix, can a test be added to detect the issue in the future?

# File Directory Structure
```
├── test                                 - e2e test suites to run on clusters. Unit tests are included alongside the golang files.
│   ├── README.md                        - Info about setting up, writing, and running the tests.
│   ├── <test suite package>             - Each test suite is a golang package.
│   │   ├── <ginkgo test suite setup>    - Ginkgo syntax to setup for any tests in the package.
|   |   |── <ginkgo tests>               - Actual Ginkgo tests.
|   |   |── go.mod                       - Used to import the local utils module.
|   |   |── go.sum
│   ├── containerstatus                  - Test container logs have no errors, containers are running, and all processes are running.
│   │   ├── suite_test.go                - Setup access to the Kubernetes cluster.
|   |   |── container_status_test.go     - Run the tests for each container that's part of our agent.
|   |   |── go.mod                       - Used to import the local utils module.
|   |   |── go.sum
│   ├── livenessprobe                    - Test that the pods detect and restart when a process is not running.
│   │   ├── suite_test.go                - Setup access to the Kubernetes cluster.
|   |   |── process_liveness_test.go     - Run the tests for each container that's part of our agent.
|   |   |── go.mod                       - Used to import the local utils module.
|   |   |── go.sum
│   ├── utils                            - Utils for Kubernetes API calls
|   |   |── utils.go                     - Functions for the test suites to use
|   |   |── go.mod
|   |   |── go.sum
```