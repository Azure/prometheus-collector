# Build Status

## Dev 
| Step | Status |
| -- | -- |
| Linux | [![Build Status](https://github-private.visualstudio.com/azure/_apis/build/status/Azure.prometheus-collector?branchName=main&jobName=Build%20linux%20image)](https://github-private.visualstudio.com/azure/_build/latest?definitionId=440&branchName=main) |
| Windows | [![Build Status](https://github-private.visualstudio.com/azure/_apis/build/status/Azure.prometheus-collector?branchName=main&jobName=Build%20windows%20multi-arch%20image)](https://github-private.visualstudio.com/azure/_build/latest?definitionId=440&branchName=main)
| Chart | [![Build Status](https://github-private.visualstudio.com/azure/_apis/build/status/Azure.prometheus-collector?branchName=main&jobName=Package%20helm%20chart)](https://github-private.visualstudio.com/azure/_build/latest?definitionId=440&branchName=main)
| Deploy | [![Build Status](https://github-private.visualstudio.com/azure/_apis/build/status/Azure.prometheus-collector?branchName=main&jobName=Deploy%20to%20dev%20clusters)](https://github-private.visualstudio.com/azure/_build/latest?definitionId=440&branchName=main) |

## Prod
| Step | Status |
| -- | -- |
| Publish | [![Build Status](https://github-private.vsrm.visualstudio.com/_apis/public/Release/badge/2d36c31d-2f89-409f-9a3e-32e4e9699840/79/127)](https://github-private.visualstudio.com/azure/_release?definitionId=79&view=mine&_a=releases) |
| Deploy | [![Build Status](https://github-private.vsrm.visualstudio.com/_apis/public/Release/badge/2d36c31d-2f89-409f-9a3e-32e4e9699840/79/128)](https://github-private.visualstudio.com/azure/_release?definitionId=79&view=mine&_a=releases) |

# Project  

This project is Azure Monitor managed service for Prometheus, which is the agent based solution to collect Prometheus metrics to be sent to managed Azure Monitor store.

## Contributing 

This project welcomes contributions and suggestions.  Most contributions require you to agree to a
Contributor License Agreement (CLA) declaring that you have the right to, and actually do, grant us
the rights to use your contribution. For details, visit https://cla.opensource.microsoft.com.

When you submit a pull request, a CLA bot will automatically determine whether you need to provide
a CLA and decorate the PR appropriately (e.g., status check, comment). Simply follow the instructions
provided by the bot. You will only need to do this once across all repos using our CLA.

This project has adopted the [Microsoft Open Source Code of Conduct](https://opensource.microsoft.com/codeofconduct/).
For more information see the [Code of Conduct FAQ](https://opensource.microsoft.com/codeofconduct/faq/) or
contact [opencode@microsoft.com](mailto:opencode@microsoft.com) with any additional questions or comments.

## Telemetry

The software may collect information about you and your use of the software and send it to Microsoft. Microsoft may use this information to provide services and improve our products and services. You may turn off the telemetry as described in the repository. There are also some features in the software that may enable you and Microsoft to collect data from users of your applications. If you use these features, you must comply with applicable law, including providing appropriate notices to users of your applications together with a copy of Microsoft’s privacy statement. Our privacy statement is located at https://go.microsoft.com/fwlink/?LinkID=824704. You can learn more about data collection and use in the help documentation and our privacy statement. Your use of the software operates as your consent to these practices.

To disable telemetry, you can set the environment variable `TELEMETRY_DISABLED` to `true` for the container either by yaml or in the [Dockerfile](/otelcollector/build/linux/Dockerfile).

## Testing with Standalone

* Create standalone environment (see [this page](https://dev.azure.com/msazure/CloudNativeCompute/_wiki/wikis/personalplayground/547381/Learning-RP?anchor=1.-create-a-standalone-aks-using-%5Bdev-aks-deploy-pipeline%5D(https%3A//dev.azure.com/msazure/cloudnativecompute/_build%3Fdefinitionid%3D68881%26_a%3Dsummary)) for more info)
* Download the `kubeconfig-cx-1` and `azureconfig.yaml` from the uploaded artifacts in the pipeline.
* Use `aksdev` to create a test cluster
  * Make sure you have aksdev locally, you can download it from the pipeline artifacts if you don't have it
  * Run: `./bin/aksdev cluster create your-test-cluster --azureconfig path/to/azureconfig.yaml`
  * Get the kubeconfig for your test cluster: `./bin/aksdev cluster kubeconfig your-test-cluster > ~/Downloads/kubeconfig`
* Enable the Monitoring extension for your standalone **cx-1** cluster
  * Go to the pipeline run that you used to create the standalone environment
  * Find the azure portal link to your standalone cluster
  * Click on `standalone-####-cx-1` cluster
  * Go to `Monitoring` -> `Insights` -> `Configure monitoring`
  * Select **Enable Prometheus metrics**.
  * Use *Advanced settings* if you want to reuse an existing workspace.
  * Click **Configure** to finish the setup. This might take a few minutes.
* Connect to the customer control plane
  * Set the kubeconfig variable to the downloaded kubeconfig: `export KUBECONFIG=~/Downloads/e2e-underlay-kubeconfig/kubeconfig-cx-1`
  * Check that the extension was installed, `ama-metrics` pod should be running: `k get po -n kube-system`
  * Find the namespace of your test cluster: `kubectl get ns`
  * Set your cluster namespace as current/default: `k config set-context --current --namespace=<your-namespace>`
* Update deployment file to use addon-token-adapter
  * Open `/otelcollector/deploy/addon-chart/ccp-metrics-plugin/templates/ama-metrics-deployment.yaml`
  * Replace `msi-adapter` with
  ```yaml
  - name: addon-token-adapter
  command:
    - /addon-token-adapter
  args:
    - --secret-namespace=kube-system
    - --secret-name=aad-msi-auth-token
    - --token-server-listening-port=7777
    - --health-server-listening-port=9999
  image: "mcr.microsoft.com/aks/msi/addon-token-adapter:master.230804.1"
  imagePullPolicy: IfNotPresent
  env:
    - name: AZMON_COLLECT_ENV
      value: "false"
  livenessProbe:
    httpGet:
      path: /healthz
      port: 9999
    initialDelaySeconds: 10
    periodSeconds: 60
  resources:
    limits:
      cpu: 500m
      memory: 500Mi
    requests:
      cpu: 20m
      memory: 30Mi
  securityContext:
    capabilities:
      drop:
        - ALL
      add:
        - NET_ADMIN
        - NET_RAW
  ```
* Create helm chart with local values
  * Go to `otelcollector/deploy/addon-chart/ccp-metrics-plugin`
  * Use the templates to create the files **Chart.yaml** and **values.yaml** in the same folder
  * Replace variables in `Chart.yaml`: version and appVersion with the latest image version of prom collector
  * Replace variables in `values.yaml`: use your standalone "cx-1" values and the namespace from your test cluster
  * Build your helm chart: `helm template ./ccp-metrics-plugin -f ./ccp-metrics-plugin/values.yaml > output.yaml`
* Install the helm chart
  * From the terminal connected to your customer control plane run: `k apply -f ./otelcollector/deploy/addon-chart/output.yaml`
  * Check that the **ama-metrics-ccp** pod is running: `k get po`
* Connect to your test cluster
  * Open a new terminal
  * Set the kubeconfig variable to the test cluster kubeconfig: `export KUBECONFIG=~/Downloads/kubeconfig`
  * Apply the configmap with the settings: `k apply -f ~/Downloads/ama-metrics-settings-configmap.yaml`
  * Switch to the terminal connected to the underlay and check that the **ama-metrics-ccp** pod restarted once: `k get po`
  * Check the pod logs to see if it started correctly: `k logs -f ama-metrics-ccp-<pod-id>`

## Release validation

Toggle Monitoring clusters for Control Plane image. Link to similar PR [here](https://msazure.visualstudio.com/DefaultCollection/CloudNativeCompute/_git/aks-rp/pullrequest/10083525?_a=files)

The following clusters will get the update once the toggle rolls out : 

- [monitoring-metrics-prod-aks-wcus](https://ms.portal.azure.com/#@microsoft.onmicrosoft.com/resource/subscriptions/9b96ebbd-c57a-42d1-bbe9-b69296e4c7fb/resourceGroups/monitoring-metrics-prod-aks/providers/Microsoft.ContainerService/managedClusters/monitoring-metrics-prod-aks-wcus/overview)
- [monitoring-metrics-prod-aks-eus2euap](https://ms.portal.azure.com/#@microsoft.onmicrosoft.com/resource/subscriptions/9b96ebbd-c57a-42d1-bbe9-b69296e4c7fb/resourceGroups/monitoring-metrics-prod-aks/providers/Microsoft.ContainerService/managedClusters/monitoring-metrics-prod-aks-eus2euap/overview)

All control plane targets have been enabled on these clusters. Validate ingestion volume and e2e data flow for them as part of the release.

## Trademarks 

This project may contain trademarks or logos for projects, products, or services. Authorized use of Microsoft 
trademarks or logos is subject to and must follow 
[Microsoft's Trademark & Brand Guidelines](https://www.microsoft.com/en-us/legal/intellectualproperty/trademarks/usage/general).
Use of Microsoft trademarks or logos in modified versions of this project must not cause confusion or imply Microsoft sponsorship.
Any use of third-party trademarks or logos are subject to those third-party's policies.
 