# Skill: Fix secret access namespace tests

## Purpose

Iterate over branch rashmi/secret-restriction-tests to fix the failing tests and get them to pass

### 1. Checking for errors in the pipeline
- Navigate to https://github-private.visualstudio.com/azure/_build?definitionId=979&_a=summary and get the latest run and check for errors
- Print out the errors in the failing steps

### 2. Triggering the pipeline
- When triggering a new pipeline run, always set the `branch` parameter to `rashmi/secret-restriction-tests` (not `main`)
- The pipeline YAML is at `.pipelines/azure-pipeline-config-tests.yml` and has a `branch` parameter (displayName: "Branch to run tests from", default: "main")
- Use the Azure DevOps REST API to queue a build with `templateParameters` to override the branch:
  ```json
  {
    "definition": { "id": 979 },
    "sourceBranch": "refs/heads/rashmi/secret-restriction-tests",
    "templateParameters": {
      "branch": "rashmi/secret-restriction-tests"
    }
  }
  ```

