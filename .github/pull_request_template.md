
[comment]: # (Note that your PR title should follow the conventional commit format: https://conventionalcommits.org/en/v1.0.0/#summary)
# PR Description

[comment]: # (The below checklist is for PRs adding new features. If a box is not checked, add a reason why it's not needed.)
# New Feature Checklist

- [ ] List telemetry added about the feature.
- [ ] Link to the one-pager about the feature.
- [ ] List any tasks necessary for release (3P docs, AKS RP chart changes, etc.) after merging the PR.
- [ ] Attach results of scale and perf testing.

[comment]: # (The below checklist is for code changes. Not all boxes necessarily need to be checked. Build, doc, and template changes do not need to fill out the checklist.)
# Tests Checklist

- [ ] Have end-to-end Ginkgo tests been run on your cluster and passed? To bootstrap your cluster to run the tests, follow [these instructions](/otelcollector/test/README.md#bootstrap-a-dev-cluster-to-run-ginkgo-tests).
  - Labels used when running the tests on your cluster:
    - [ ] `operator`
    - [ ] `windows`
    - [ ] `arm64`
    - [ ] `arc-extension`
    - [ ] `fips`
- [ ] Have new tests been added? For features, have tests been added for this feature? For fixes, is there a test that could have caught this issue and could validate that the fix works?
  - [ ] Is a new scrape job needed?
    - [ ] The scrape job was added to the folder [test-cluster-yamls](/otelcollector/test/test-cluster-yamls/) in the correct configmap or as a CR. 
  - [ ] Was a new test label added?
    - [ ] A string constant for the label was added to [constants.go](/otelcollector/test/utils/constants.go).
    - [ ] The label and description was added to the [test README](/otelcollector/test/README.md).
    - [ ] The label was added to this [PR checklist](/.github/pull_request_template).
    - [ ] The label was added as needed to [testkube-test-crs.yaml](/otelcollector/test/testkube/testkube-test-crs.yaml).
  - [ ] Are additional API server permissions needed for the new tests?
    - [ ] These permissions have been added to [api-server-permissions.yaml](/otelcollector/test/testkube/api-server-permissions.yaml).
  - [ ] Was a new test suite (a new folder under `/tests`) added?
    - [ ] The new test suite is included in [testkube-test-crs.yaml](/otelcollector/test/testkube/testkube-test-crs.yaml).
