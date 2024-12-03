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
 