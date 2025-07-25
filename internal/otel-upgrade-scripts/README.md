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