---
name: trigger-test-pipeline
description: Trigger the config-tests pipeline (definition 979) with the branch parameter set to rashmi/secret-restriction-tests.
allowed-tools:
  - Bash
  - Read
  - Write
  - Grep
  - Glob
---

# Skill: Trigger custom test pipeline

## CRITICAL: Branch parameter

The pipeline at `.pipelines/azure-pipeline-config-tests.yml` has a `branch` parameter (displayName: "Branch to run tests from", default: "main").
You **MUST** set `templateParameters.branch` to `rashmi/secret-restriction-tests` — otherwise the pipeline checks out `main` and runs the wrong code.

## Steps

### 1. Get an Azure DevOps access token

```powershell
$token = az account get-access-token --resource "499b84ac-1321-427f-aa17-267ca6975798" --query accessToken -o tsv
$headers = @{ Authorization = "Bearer $token"; "Content-Type" = "application/json" }
```

### 2. Queue the build with the correct branch parameter

```powershell
$body = @{
    definition = @{ id = 979 }
    sourceBranch = "refs/heads/rashmi/secret-restriction-tests"
    templateParameters = @{
        branch = "rashmi/secret-restriction-tests"
    }
} | ConvertTo-Json -Depth 3

$url = "https://github-private.visualstudio.com/azure/_apis/build/builds?api-version=7.0"
$response = Invoke-RestMethod -Uri $url -Headers $headers -Method Post -Body $body
Write-Host "Build ID: $($response.id)"
Write-Host "Build Number: $($response.buildNumber)"
Write-Host "Status: $($response.status)"
Write-Host "URL: $($response._links.web.href)"
```

### 3. Confirm the build was queued

Verify the output shows a valid Build ID and the status is `notStarted` or `inProgress`.
Print the pipeline URL so the user can monitor it.
