# Ginkgo
Tests are run using the [Ginkgo](https://onsi.github.io/ginkgo/) test framework. This is built upon the regular go test framework. It's advantages are that it:
- Has an easily readable test structure using the Behavior-Driven-Development model that's used in many languages
- Utilizes the Gomega package for easily understandable test failure errors with the goal that the output will tell you exactly what failed.
- Has good support for parallelization and structuring what tests should be run in series and which can be run at the same time.

Ginkgo can be used for any tests written in golang, whether they are unit tests or integration tests.

## Install Locally on your Dev Machine
- While not in a directory that has a go.mod file, run `sudo -E go install -v github.com/onsi/ginkgo/v2/ginkgo@latest`. This installs the ginkgo command-line tool used to run Ginkgo tests.
- Run `ginkgo version` to check the installation succeeded. You may need to add `$GOBIN` to your `$PATH`.

## Run Tests Locally from your Dev Machine
- Change to a directory with a ginkgo test suite file. Run `ginkgo -p`. This runs all the tests in that package with parallelization enabled.

## Test Suites
- Each Ginkgo test suite has a function that handles the testing object and abstracts that away. It runs all Ginkgo tests in the same package.
- BeforeSuite() and AfterSuite() functions can be used for setup and teardown. We use these for connecting to the cluster to get the kubeconfig and creating a kubernetes go-client.

  ```
  func TestE2E(t *testing.T) {
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
- Ginkgo Test suites are run one at a time.
- Ginkgo tests inside a suite are run parallely by default unless `Ordered` or `Serial` is specified as a parameter to a Describe function.

#### Example
- These two DescribeTable() tests will run at the same time. One tests a replica pod and the other tests a daemonset pod. Because Ordered is specified as a Decorator, each Entry in the table, however,  is run one at a time.
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

### What Kinds of Test Can Be Run?
- Any functionalities of the go-client package can be used for Kubernetes-specific tests. This includes:
  - Checking the status or spec of a Kubernetes resource (deployment, pod, configmap, container, CR, etc.)
  - Pulling the container logs
  - Running exec commands on a container
- Use the Query API to query an Azure Monitor Workspace to verify metrics are ingested

### Sonobuoy Integration
- Same tests can be re-used for Arc by having the Sonobuoy container run the Ginkgo tests, just as we right now have it run the python tests.

# TestKube
Testkube is a runner framework for running the tests inside the cluster. Ginkgo is included as one of the out-of-the-box executors supported. It is deployed as a helm chart on the cluster.

- Has an integrated dashboard to view results, set up tests, test suites, test schedules, etc. with a UX as an alternative to the (also available) CLI.
- Includes test history, pass rate, and execution times.
- Friendly user interface and easy Golang integration with out-of-the-box Ginkgo runner.
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
- Create a test connected to the Github repository and branch. Tests are just a custom resource behind the scenes and can be created with the UX, CLI, or applying a CR. Tests can be run through the UX or CLI.

# Unit Tests
Ginkgo is also used the same way for unit testing. When converting the configmap parsing code from Ruby to Golang, we'll also be able to utilize it for unit tests.
Copilot can be helpful as a guideline with generating these tests. It struggles with importing the correct packages, and we'll want to use more realistic test cases, but it gives a good outline for Ginkgo testing. For example below was generated by Copilot for an outline of tests for the prometheus-collector-settings section of the configmap:

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
				// Create a temporary configmap settings file for testing
				// You can use a library like ioutil.TempFile to create a temporary file
				// with the desired content for testing.
			})

			AfterEach(func() {
				// Remove the temporary configmap settings file
				// You can use os.Remove to delete the temporary file after testing.
			})

			It("should parse the configmap settings file and return the values", func() {
				// Call the ParsePrometheusCollectorSettings function
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

With this, we can have many configmap test files and ensure each combination is parsed and the correct prometheus config and environment variables are produced.

# When to Run Each Test
- Unit tests can be included as part of the PR checks.
- E2E tests can be run from dev machine pointing to your test cluster kubeconfig.
- E2E tests run after deploying to dev and prod CI/CD clusters. A [Teams channel notification](https://docs.testkube.io/articles/webhooks#microsoft-teams) can integrated with testkube for notifying if it failed.

## Utilizing a PR Checklist
- Have the e2e tests been run on a test cluster?
- If feature, have tests been added?
- If fix, can a test be added to detect the issue in the future?

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