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
az acr login -n containerinsightsprod -u containerinsightsprod -p <password>
docker push containerinsightsprod.azurecr.io/public/azuremonitor/containerinsights/cidev/prometheus-collector:conf-<tag>
```

## Testing locally
Use [local-e2e-tests.yaml](./local-e2e-tests.yaml) to setup sonobuoy and run the tests on your cluster. Use the cluster managed identity and give permissions to enable the extension and query the AMW.

In this file, replace the enivronment variables:
```yaml
- name: WORKLOAD_CLIENT_ID
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

## Testing on the Arc conformance matrix
1. In the [release](https://github-private.visualstudio.com/azure/_releaseDefinition?definitionId=79&_a=definition-pipeline), the task `Deploy to Prod Clusters` will deploy the arc extension to the `Staging` release train. This is the release train our conformance tests use.
2. After releasing to `Staging`, create a duplicate task of [this format](https://dev.azure.com/ArcValidationProgram/ArcValidationProgram/_workitems/edit/1161) and update the title to have the latest agent version.
3. Post in the [Teams channel](https://teams.microsoft.com/l/channel/19%3ArlnJ5tIxEMP-Hhe-pRPPp9C6iYQ1CwAelt4zTqyC_NI1%40thread.tacv2/General?groupId=a077ab34-99ea-490c-b204-358d31c24fbe&tenantId=72f988bf-86f1-41af-91ab-2d7cd011db47) asking for the conformance tests to be run. An example post is [here](https://teams.microsoft.com/l/message/19:rlnJ5tIxEMP-Hhe-pRPPp9C6iYQ1CwAelt4zTqyC_NI1@thread.tacv2/1715902653350?tenantId=72f988bf-86f1-41af-91ab-2d7cd011db47&groupId=a077ab34-99ea-490c-b204-358d31c24fbe&parentMessageId=1715902653350&teamName=Azure%20Arc%20Conformance%20Testing&channelName=General&createdTime=1715902653350).
4. Wait until the Arc team responds if the `Extension Plugin` tests have passed. The logs of the Ginkgo tests can be viewed by navigating to the test result page and downloading all logs.
5. After the tests have passed, the extension can be released to the `Stable` release train by starting the `ARC Small Region` release task.

