# Integration Testing
## Ginkgo
Tests are run using the [Ginkgo](https://onsi.github.io/ginkgo/) test framework. This is built upon the regular go test framework.

### Install Locally
- While not in a directory that has a go.mod file, run `sudo -E go install -v github.com/onsi/ginkgo/v2/ginkgo@latest`.
- Run `ginkgo version` to check the installation succeeded. You may need to add `$GOBIN` to your `$PATH`.

### Run Locally
- Set your `kubeconfig` context to point to the cluster you'd like the tests to run on.
- Change the directory to one with a ginkgo test suite file. Run `ginkgo -p`.

### Test Suites
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

### Running Tests in Parallel
- Test suites are run one at a time.
- Tests inside a suite are run parallely by default unless `Ordered` or `Serial` is specified as a parameter to a Describe function.

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

## TestKube
Testkube is the runner framework for running the tests inside the cluster. Ginkgo is included as one of the out-of-the-box executors supported.

Has an integrated dashboard to view results, set up tests, test suites, test schedules, etc. with a UX as an alternative to the (also available) CLI.

