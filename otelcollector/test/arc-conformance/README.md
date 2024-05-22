# Arc Conformance Testing

Instead of TestKube, Arc uses Sonobuoy as the test runner on the cluster. The underlying Ginkgo tests can be used in the same way as TestKube however.

A custom Sonobuoy plugin container image is created to run the tests. This container has an entrypoint of [e2e_tests.sh](./e2e_tests.sh). It ensures the cluster is connected to Arc, has the Arc pods running, then installs the ama-metrics extension, and waits for the pods to be ready. Then the Ginkgo tests are run inside the cluster and the results are stored in an XML format that the Sonobuoy pod recognizes.

The [Dockerfile](./Dockerfile) for the image uses the Microsoft Golang base image, installs Helm, kubectl, the Azure CLI, and the Ginkgo CLI. It builds the Ginkgo tests as binaries so that the tests don't need to be built at runtime. These test binaries are copied into the container and then [e2e_tests.sh](./e2e_tests.sh) is set as the entrypoint.

The Arc team only uses the file [arc-conformance.yaml](./arc-conformance.yaml) to run our plugin in the conformance test matrix. The latest image tag needs to be updated here whenever a new one is built.

## Building our Sonobuoy plugin image
From the repository root:
```bash
cd otelcollector/test
sudo docker build -f arc-conformance/Dockerfile -t containerinsightsprod.azurecr.io/public/azuremonitor/containerinsights/cidev/prometheus-collector:conf-<tag> .
docker tag 
az acr login -n containerinsightsprod -u containerinsightsprod -p <password>
docker push containerinsightsprod.azurecr.io/public/azuremonitor/containerinsights/cidev/prometheus-collector:conf-<tag>
```

## Testing locally
Use [local-e2e-tests.yaml](./local-e2e-tests.yaml) to setup sonobuoy and run the tests on your cluster. Use the cluster managed identity and give permissions to enable the extension and query the AMW.

In this file, replace the enivronment variables:
```yaml
- name: CLIENT_ID
  value: "<Managed identity client ID>"
- name: TENANT_ID
  value: "<Arc cluster and managed identity tenant ID>"
- name: SUBSCRIPTION_ID
  value: "<Arc cluster subscription ID>"
- name: RESOURCE_GROUP
  value: "<Arc cluster reource group>"
- name: CLUSTER_NAME
  value: "<Arc cluster name>"
```

Run the Sonobuoy pod that will deploy a job to run our plugin:
```bash
kubectl apply -f local-e2e-tests.yaml
kubectl get pods -n sonobuoy
kubectl logs <sonobuoy-agenttests-job-* pod name> -n sonobuoy -f
sonobuoy status --json
```

The logs will have the full output of the Ginkgo tests.

The sonobuoy status command will have the number of tests that passed, failed, or were skipped:
```json
{"plugins":[{"plugin":"agenttests","node":"global","status":"complete","result-status":"passed","result-counts":{"passed":50,"skipped":18}}],"status":"complete","tar-info":{"name":"202405152328_sonobuoy_bf5c02ed-1948-48f1-b12d-5a2d74435e46.tar.gz","created":"2024-05-15T23:49:32.876748551Z","sha256":"559406070bd5738dd077355be5fdb5560497680be938d3d0a63a2a8f4ac66d15","size":282521}}
```


