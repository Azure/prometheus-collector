# OtelCollector Upgrade Bot

This directory contains scripts used to automate the upgrading of OpenTelemetry Collector components in this repository. These scripts are orchestrated by the GitHub action [otelcollector-upgrade.yml](../../.github/workflows/otelcollector-upgrade.yml).

## GitHub Action: otelcollector-upgrade.yml

The `otelcollector-upgrade.yml` GitHub action is an automated workflow that:

1. **Runs Daily**: Executes automatically every day at 7 AM UTC via a cron schedule
2. **Can be Triggered Manually**: Also supports manual execution via workflow_dispatch

### What the Action Does:

1. **Version Check**: Checks for new versions of the OpenTelemetry Collector and Target Allocator
2. **Pull Request Management**: 
   - Creates a new pull request for upgrades when a new version is detected
   - Closes older upgrade PRs in favor of newer versions
   - Avoids creating duplicate PRs for the same version
   - Uses the identity of the [azure-monitor-assistant](https://github.com/apps/azure-monitor-assistant) GitHub App for authentication and PR creation
3. **Build Validation**: 
   - Attempts to build the otelcollector and related Go binaries after upgrade
   - Reports build success/failure status in PR comments
4. **Security Scanning**: 
   - Runs Trivy security scans on built binaries
   - Updates `.trivyignore` file with new CVEs if needed
   - Reports CVE changes in PR comments
5. **Automation**: 
   - The build pipeline is triggered upon every commit to the PR
   - Includes relevant changelog summaries in PR descriptions

## Upgrade Scripts

### `check-otel-version.sh`
- Fetches the latest release information from the OpenTelemetry Operator GitHub repository
- Compares current versions with the latest available versions
- Updates `OPENTELEMETRY_VERSION` and `TARGETALLOCATOR_VERSION` files if newer versions are found
- Calls other scripts to perform the actual upgrade process

### `upgrade.sh`
- Performs the core upgrade logic by cloning and updating OpenTelemetry Collector components
- Takes parameters for collector version, stable version, target allocator version, and current versions
- Clones the `opentelemetry-collector-contrib` repository at the specified version
- Updates various configuration files and dependencies throughout the codebase

### `updatetrivyignore.sh`
- Scans built binaries using Trivy security scanner
- Compares current CVEs with newly detected ones after upgrade
- Automatically updates the `.trivyignore` file to handle new security vulnerabilities
- Generates a report of CVE changes for inclusion in pull requests
- Handles multiple binaries: otelcollector, promconfigvalidator, targetallocator, configurationreader, and prometheusui

### `changelogsummary.sh`
- Extracts relevant changelog entries for specific components between version ranges
- Supports filtering by component name or custom regex patterns
- Categorizes changes as Breaking, Feature, Bug Fix, or Other
- Generates markdown-formatted summaries for inclusion in upgrade pull requests
- Handles multi-line changelog entries and provides GitHub PR links

## GitHub App Authentication

The upgrade workflow uses the [azure-monitor-assistant](https://github.com/apps/azure-monitor-assistant) GitHub App for authentication and performing Git operations. This provides secure, scoped access for creating pull requests, adding comments, and managing repository content.

Information on how this GitHub App was created and how it is maintained can be found in the Microsoft Engineering documentation: [GitHub Apps Creation and Publishing Guide](https://eng.ms/docs/initiatives/open-source-at-microsoft/github/apps/creation/publishing)

## Adding Custom Changes to the Upgrade

If new custom changes are added that need to be applied on top of the OSS otelcollector or targetallocator, you can create a PR to merge these changes into main on top of the existing otelcollector and targetallocator versions. Then use  Github Copilot Agent mode using Claude Sonnet with a similar prompt to below by giving it the new instructions in `UpgradeotelCol.md` and the commit id of the merged PR/

Example agent prompt with context pointing to `upgrade.sh` and `UpgradeotelCol.md`:

    `upgrade.sh is a script based off the instructions in UpgradeotelCol.md. New instructions have been added in the TargetAllocator Update section. The actual code changes needed are in the commit 341e15c4014e14cff73c5a4b2b9dabaff8fd8aa9. Add these changes under otelcollector/otel-operator as part of the upgrade.sh script. These are custom changes that won't be included in any upgrades of the otel-operator, but we will need to apply these changes on top of the upgrade.`

The updates made to the script can be tested locally by running `internal/otel-upgrade-scripts/check-otel-version.sh`.

The changes to the script are usually a starting point and may require some tweaking such as what the regex is for the existing line to add the new code after it. Anything too specific might not work if the existing code gets updated in the future.

## Script Enhancement Prompt Examples
- **Prompt**:
   ```
   Currently this script in # Step 5, updates the golang version based on opentelemetry-collector-contrib/go.mod. We need to actually take the highest version between otel-allocator/go.mod or opentelemetry-collector-contrib/go.mod
   ```
- **Results**:
   - Removed the original Step 5 that only looked at `opentelemetry-collector-contrib/go.mod`
   - Added a new Step 8 that runs after both the `opentelemetry-collector-contrib` and `otel-allocator` updates are complete
   - Created a version comparison function that properly compares version strings
   - Extracts Go versions from both `go.mod` files:
      - `opentelemetry-collector-contrib/go.mod`
      -` otel-allocator/go.mod`
   - Compares the versions and selects the highest one
   Updates the pipeline file only if the `major.minor` version changes
- **Prompt**
   ```
   Now if the golang version does need to be changed, find what patch version to use based on the latest patch version tag available from the image repository mcr.microsoft.com/oss/go/microsoft/golang
   ```
- **Results**:
   - **Tool Check**: The script now checks for `curl` and `jq` availability
   - **Registry Query**: Uses curl to query the MCR API endpoint: `https://mcr.microsoft.com/v2/oss/go/microsoft/golang/tags/list`
   - **JSON Parsing**: The response is parsed with `jq` to extract the tags array
   - **Version Selection**: The same logic as before finds the highest patch version for the given `major.minor` version
   - **Fallback**: If the registry is unreachable or tools are missing, it falls back to using the version from the `go.mod` files

- **Prompt**:
   ```
   Do not use skopeo, just curl the endpoint and parse the json with jq
   ```
- **Results**:
   - **Tool Replacement**: Replaced skopeo dependency with curl and jq approach
   - **Direct API Access**: Uses curl to directly query the MCR REST API
   - **JSON Processing**: Leverages jq for robust JSON parsing and filtering
   - **Simplified Dependencies**: Removes the need for container registry tools, using only standard HTTP tools

- **Prompt**:
   ```
   I need the prometheus package versions to be updated in go.mod to be the same as those in the opentelemetry-collector and the otel-allocator
   ```
- **Results**:
   - **Added Step 7.1**: New step to synchronize Prometheus dependencies in test utils
   - **Version Extraction**: Extracts Prometheus package versions from both main modules:
     - `github.com/prometheus/client_golang`
     - `github.com/prometheus/common`
     - `github.com/prometheus/client_model`
   - **Version Comparison**: Uses highest version between otel-builder and otel-allocator
   - **Target Module**: Updates `otelcollector/test/ginkgo-e2e/utils/go.mod`

- **Prompt**:
   ```
   After updating in the utils directory, it also nede to be updated in the prometheusui go.mod
   ```
- **Results**:
   - **Added Step 7.2**: Extended Prometheus synchronization to prometheusui module
   - **Additional Package**: Also handles `github.com/prometheus/prometheus` package
   - **Version Reuse**: Leverages variables from Step 7.1 to avoid code duplication
   - **Target Module**: Updates `otelcollector/test/ginkgo-e2e/prometheusui/go.mod`
   - **Special Handling**: Includes regex escaping for complex version strings with special characters

- **Prompt**:
   ```
   You don't need to do the same code twice for 7.2 to get the versions
   ```
- **Results**:
   - **Code Optimization**: Eliminated duplicate version extraction code
   - **Variable Reuse**: Step 7.2 now reuses variables calculated in Step 7.1
   - **Improved Maintainability**: Single source of truth for Prometheus version selection
   - **Performance**: Reduced redundant operations and improved script efficiency

- **Prompt**:
   ```
   The script is getting way more versions than we want in the grep such as: Using Go version from otel-allocator: 1.24.0 v1.1.12 v0.32.3 v1.2.2...
   ```
- **Results**:
   - **Grep Pattern Fix**: Changed from `grep "go "` to `grep "^go "` 
   - **Anchor Matching**: Added `^` to ensure pattern only matches lines starting with "go "
   - **False Positive Prevention**: Avoids matching dependency lines containing "go" in package names
   - **Precise Extraction**: Only captures the actual Go version declaration line

- **Prompt**:
   ```
   The script is now failing with these logs: ... sed: -e expression #1, char 110: unknown option to `s'
   ```
- **Results**:
   - **Pipeline Extraction Fix**: Added `head -1` to extract only the first `GOLANG_VERSION` match
   - **Multi-line Prevention**: Prevents capturing multiple lines that could break sed commands
   - **Improved Regex**: Enhanced sed pattern from `s/GOLANG_VERSION: '//;s/'//g` to `s/.*GOLANG_VERSION: '//;s/'.*//g`

- **Prompt**:
   ```
   The script is upgrading the build version for other lines we don't want like FLUENTBIT_GOLANG_VERSION and TESTKUBE_GOLANG_VERSION
   ```
- **Results**:
   - **Precise Pattern Matching**: Changed sed pattern to `s/^  GOLANG_VERSION: '.*'/  GOLANG_VERSION: '${FINAL_GO_VERSION}'/g`
   - **Indentation Awareness**: Added `^  ` to match exact YAML indentation
   - **False Match Prevention**: Ensures only the main `GOLANG_VERSION` line is updated
   - **YAML Formatting**: Preserves proper indentation in the replacement string
