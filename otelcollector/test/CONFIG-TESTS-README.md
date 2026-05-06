# Adding a New Config Test Scenario

This is the short recipe for adding a new scenario to `.pipelines/azure-pipeline-config-tests.yml`.

For general Ginkgo / TestKube background, see [`otelcollector/test/README.md`](./README.md).

## Files to edit

1. `otelcollector/test/test-cluster-yamls/configmaps/...` — add (or reuse) the configmap your scenario needs.
2. `otelcollector/test/ginkgo-e2e/utils/constants.go` — add a label constant (`ConfigProcessingXxx = "config-processing-xxx"`). Skip this if an existing label already covers your assertions.
3. `otelcollector/test/ginkgo-e2e/configprocessing/config_processing_test.go` — add `Entry(...)` rows tagged with `Label(utils.ConfigProcessingXxx)`.
4. `otelcollector/test/testkube/config-processing-test-crs/testkube-config-test-<scenario>-crs.yaml` — copy a sibling CR template and change the workflow `name` and the `--label-filter` value.
5. `otelcollector/test/testkube/run-testkube-workflow.sh` — add a branch to the `case "$SOURCE_TEMPLATE"` block listing the workflow names from your CR file.
6. `.pipelines/azure-pipeline-config-tests.yml` — add two bash steps (described below).

## Pipeline steps to add

Append these two steps before `"List Results Files"`. Replace `<scenario>` with your scenario name (must be unique).

```yaml
- bash: |
    # Pre-clean any state the previous scenario may have left behind.
    # kubectl delete -f ./test-cluster-yamls/configmaps/<some-leftover>.yaml --ignore-not-found

    kubectl apply -f ./test-cluster-yamls/configmaps/<your-configmap>.yaml

    export BUILD_ARTIFACTSTAGINGDIRECTORY="$(Build.ArtifactStagingDirectory)"
    export BUILD_BUILDID="$(Build.BuildId)"
    export SYSTEM_JOBID="$(System.JobId)"
    export SYSTEM_TASKINSTANCEID="$(System.TaskInstanceId)"

    chmod +x ./testkube/run-testkube-workflow.sh
    ./testkube/run-testkube-workflow.sh \
      "${{ parameters.AMW_QUERY_ENDPOINT }}" \
      "${{ parameters.AZURE_CLIENT_ID }}" \
      "testkube-config-test-<scenario>-crs.yaml" \
      "testkube-config-test-<scenario>-crs.yaml" \
      "false" \
      "480" \
      "ConfigTests" \
      "${{ parameters.branch }}"
  workingDirectory: $(Build.SourcesDirectory)/otelcollector/test
  displayName: "Run tests for <scenario>"
  continueOnError: true

- bash: |
    TESTKUBE_RESULTS_OTEL=$(jq -c '.environment="<scenario>"' "$(Build.ArtifactStagingDirectory)/testkube-results-ConfigTests.json" | tr -d '\n\r')
    echo $TESTKUBE_RESULTS_OTEL > "$(Build.ArtifactStagingDirectory)/testkube-results/testkube-results-<scenario>.json"
    rm -f "$(Build.ArtifactStagingDirectory)/testkube-results-ConfigTests.json"

    # Restore baseline state so the next scenario starts clean.
    # kubectl delete -f ./test-cluster-yamls/configmaps/<your-configmap>.yaml --ignore-not-found
  displayName: "Create TestKube Results Summary"
  condition: always()
```

## Things to remember

- **Scenarios run sequentially and share one cluster.** Don't assume the cluster is in a clean state — pre-clean any leftover resources at the start of step 6a, set up the state your scenario needs, and restore the baseline at the end in step 6b. The summary step must use `condition: always()` so cleanup runs even when tests fail.
- **`<scenario>` must be unique** across all scenarios — it ends up as the `.environment` tag and the result-file name in the Teams summary.
- **Pass `"ConfigTests"` as the 7th arg** to `run-testkube-workflow.sh`. That is what makes the script look up your template under `config-processing-test-crs/` and resolve workflows from the `case` block instead of auto-discovering them.
- **The 5th arg `"false"`** tells the script not to apply the baseline settings configmap — config tests manage their own configmaps.
- **`continueOnError: true`** on the run step is required, otherwise a single failure aborts the rest of the pipeline.

## How to run the pipeline

Pipeline: <https://github-private.visualstudio.com/azure/_build?definitionId=979>. Click **Run pipeline** and set two independent branch inputs:

- **Branch/tag dropdown** — the ADO source branch the agent checks out (`checkout: self`). Controls the pipeline YAML, configmap files, TestWorkflow CR templates, and `run-testkube-workflow.sh` (i.e. files from steps 1, 4, 5, 6 of the recipe above).
- **`branch` parameter** ("Branch to run tests from", default `main`) — exported as `BRANCH_NAME` and substituted into each CR's `spec.content.git.revision`. This is the branch TestKube clones **inside the cluster** to run the Ginkgo tests under `otelcollector/test/ginkgo-e2e/` (files from steps 2, 3).

For a new scenario added via this recipe you typically set **both** to your feature branch.

