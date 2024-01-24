# Contributing

This project welcomes contributions and suggestions. Most contributions require you to
agree to a Contributor License Agreement (CLA) declaring that you have the right to,
and actually do, grant us the rights to use your contribution. For details, visit
https://cla.microsoft.com.

When you submit a pull request, a CLA-bot will automatically determine whether you need
to provide a CLA and decorate the PR appropriately (e.g., label, comment). Simply follow the
instructions provided by the bot. You will only need to do this once across all repositories using our CLA.

This project has adopted the [Microsoft Open Source Code of Conduct](https://opensource.microsoft.com/codeofconduct/).
For more information see the [Code of Conduct FAQ](https://opensource.microsoft.com/codeofconduct/faq/)
or contact [opencode@microsoft.com](mailto:opencode@microsoft.com) with any additional questions or comments.

## Test Images

After creating a PR, the pipeline will build the images with the tag as the name of the build in the following format:
- Linux: `0.0.0-{branch}-{date}-{commit}`
- Windows: `0.0.0-{branch}-{date}-{commit}-win`
- Config Reader: `0.0.0-{branch}-{date}-{commit}-cfg`
- Target Allocator: `0.0.0-{branch}-{date}-{commit}-targetallocator`

These values can be substituted into the [values.yaml](./otelcollector/deploy/addon-chart/azure-monitor-metrics-addon/values-template.yaml) and deployed on your cluster. Follow the instructions to deploy through the backdoor [here](./otelcollector/deploy/addon-chart/Readme.md).