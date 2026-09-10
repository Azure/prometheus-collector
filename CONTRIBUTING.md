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

## Go unit tests

The `Go_Unit_Tests` job in `.pipelines/azure-pipeline-build.yml` runs during PR
validation targeting `main` and builds after merging to `main`. It blocks the
Build stage independently of image builds, deployment, and TestKube.

Use Linux with the pipeline's `GOLANG_VERSION` and run the following from the
repository root. Each module must be selected explicitly because `go test ./...`
does not cross nested `go.mod` boundaries. The MP configmap module's Ginkgo suite
also runs through `go test`; no separate Ginkgo installation is needed.

```bash
set -euo pipefail
for module in \
  shared \
  shared/configmap/mp \
  shared/configmap/ccp \
  configuration-reader-builder \
  otel-allocator \
  otel-allocator/integrationtest; do
  (cd "otelcollector/$module" && go test -count=1 ./...)
done
```

Then use the pipeline's `FLUENTBIT_GOLANG_VERSION`, with a C compiler available,
for the fluent-bit plugin:

```bash
(cd otelcollector/fluent-bit/src && go test -count=1 ./...)
```

These commands retain Go's default vet checks and include the self-contained
allocator integration tests. They do not require a deployed Kubernetes cluster,
Docker daemon, or external telemetry backend. Cluster-dependent suites under
`otelcollector/test/ginkgo-e2e` remain in the existing TestKube flow. Use LF line
endings for the Linux checkout; copying CRLF fixtures from a Windows checkout can
cause byte-for-byte comparisons to fail.

`otelcollector/prometheusreceiver` is intentionally excluded. It is a fork of the
upstream OpenTelemetry Prometheus receiver, and `internal/otel-upgrade-scripts/upgrade.sh`
deletes the `testdata` fixtures its tests read, so the suite cannot run as vendored.

## Test Images

After creating a PR, the pipeline will build the images with the tag as the name of the build in the following format:
- Linux: `0.0.0-{branch}-{date}-{commit}`
- Windows: `0.0.0-{branch}-{date}-{commit}-win`
- Config Reader: `0.0.0-{branch}-{date}-{commit}-cfg`
- Target Allocator: `0.0.0-{branch}-{date}-{commit}-targetallocator`

These values can be substituted into the [values.yaml](./otelcollector/deploy/addon-chart/azure-monitor-metrics-addon/values-template.yaml) and deployed on your cluster. Follow the instructions to deploy through the backdoor [here](./otelcollector/deploy/addon-chart/Readme.md).