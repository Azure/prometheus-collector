---
name: secret-access-tests
description: Iterate over tests to make them pass for the secrets access namespaces feature on branch rashmi/secret-restriction-tests.
allowed-tools:
  - Bash
  - Read
  - Write
  - Grep
  - Glob
---

# Skill: Fix secret access namespace tests

## Purpose

Iterate over branch rashmi/secret-restriction-tests to fix the failing tests and get them to pass

### 1. Checking for errors in the pipeline
- Navigate to https://github-private.visualstudio.com/azure/_build?definitionId=979&_a=summary and get the latest run and check for errors
- Print out the errors in the failing steps

### 2. Check code base and fix errors
- Check this code base to see what might be responsible for the errors.
- Fix the errors


### 3. Commit to branch - rashmi/secret-restriction-tests
- Commit only chnages files to fix the tests to rashmi/secret-restriction-tests branch

### 3. Triggering the pipeline
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

### 5. wait for pipeline to finish running

### 6. Repeat steps 1 to 5 until tests pass and there are no failures

