# Adding a New Config Test Scenario

This is the short recipe for adding a new scenario to `.pipelines/azure-pipeline-config-tests.yml`.

For general Ginkgo / TestKube background, see [`otelcollector/test/README.md`](./README.md).

## Files to edit

1. `otelcollector/test/test-cluster-yamls/configmaps/...` — add (or reuse) the configmap your scenario needs.
2. `otelcollector/test/ginkgo-e2e/utils/constants.go` — add a label constant (`ConfigProcessingXxx = "config-processing-xxx"`). Skip this if an existing label already covers your assertions.
3. `otelcollector/test/ginkgo-e2e/configprocessing/config_processing_test.go` — add `Entry(...)` rows tagged with `Label(utils.ConfigProcessingXxx)`.
4. `otelcollector/test/testkube/config-processing-test-crs/testkube-config-test-<scenario>-crs.yaml` — add a small `TestWorkflow` wrapper with its name, labels, Ginkgo label filter, and any case-specific environment. All ConfigProcessing workflows use `testkube-config-test-ginkgo-template-crs.yaml` for the shared source/image, Ginkgo reports, node selector, and optional timeout/verbosity/parallel flags; the runner applies it before the workflow.
5. `otelcollector/test/testkube/run-testkube-workflow.sh` — add a branch to the `case "$SOURCE_TEMPLATE"` block listing the workflow names from your CR file.
6. `.pipelines/azure-pipeline-config-tests.yml` — add a call to the shared run/summary step template (described below).

## Pipeline step to add

Append this template call before `"List Results Files"`. Replace `<scenario>` with a unique result name. Put cluster setup in `setupScript`; it runs in the same step immediately before TestKube.

```yaml
- template: azure-template-config-test-case.yml
  parameters:
    AMW_QUERY_ENDPOINT: ${{ parameters.AMW_QUERY_ENDPOINT }}
    AZURE_CLIENT_ID: ${{ parameters.AZURE_CLIENT_ID }}
    branch: ${{ parameters.branch }}
    workflowFile: testkube-config-test-<scenario>-crs.yaml
    scenario: <scenario>
    displayName: "Run tests for <scenario>"
    setupScript: |
      # Pre-clean any state the previous scenario may have left behind.
      # kubectl delete -f ./test-cluster-yamls/configmaps/<some-leftover>.yaml --ignore-not-found
      kubectl apply -f ./test-cluster-yamls/configmaps/<your-configmap>.yaml
```

For scenario-specific cleanup, use `beforeSummaryScript` when it originally ran before result generation, or `afterSummaryScript` when it ran after the summary. The shared template keeps the summary step at `condition: always()`.

## Things to remember

- **Scenarios run sequentially and share one cluster.** Don't assume the cluster is in a clean state — pre-clean any leftover resources in `setupScript` and restore baseline state in the appropriate summary script, preserving its order.
- **`<scenario>` must be unique** across all scenarios — it ends up as the `.environment` tag and the result-file name in the Teams summary.
- The shared step template preserves the `ConfigTests` runner mode, disables automatic baseline settings configmap application, and continues the pipeline after a scenario failure.

## How to run the pipeline

Pipeline: <https://github-private.visualstudio.com/azure/_build?definitionId=979>. Click **Run pipeline** and set two independent branch inputs:

- **Branch/tag dropdown** — the ADO source branch the agent checks out (`checkout: self`). Controls the pipeline YAML and its step template, configmap files, TestWorkflow CR templates, and `run-testkube-workflow.sh`.
- **`branch` parameter** ("Branch to run tests from", default `main`) — exported as `BRANCH_NAME` and substituted into each CR's `spec.content.git.revision`. This is the branch TestKube clones **inside the cluster** to run the Ginkgo tests under `otelcollector/test/ginkgo-e2e/` (files from steps 2, 3).

For a new scenario added via this recipe you typically set **both** to your feature branch.

