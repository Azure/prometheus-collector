# TSG: Vulnerabilities / CVEs

Use this workflow for scanner-generated incidents involving prometheus-collector images or AKS
VHDs. Trace the exact scanned artifact through the release chain; do not substitute current source
or the newest registry tag for the version the scanner evaluated.

## 1. Capture the Scanner Evidence

Read the authored incident summary and all discussion entries. Record:

- exact image tag and digest, when available
- affected VHD SKU or other consuming artifact
- affected binaries and package type, such as an OS package or Go module
- CVE IDs, severities, detected versions, and minimum fixed versions
- scan time, artifact build time, and scanner query or report link

For an AKS VHD alert, confirm that the scanner evaluated the built VHD and its cached images.
Search prior incidents for the same package, fix threshold, and image family to distinguish a
recurring stale consumer from a new vulnerability set. Report related incidents, but do not add or
modify incident relationships without explicit authorization.

## 2. Verify the Vulnerability and Fix

Run the
[prometheus-collector Trivy workflow](https://github.com/Azure/prometheus-collector/actions/workflows/scan.yml)
against the exact vulnerable image and the candidate fixed image, or use the scanner's
authoritative report.

Classify each finding before selecting the version source:

| Finding type | Verification |
|--------------|--------------|
| Base OS image | Compare the scanned packages with the current supported base image. A rebuild after the patched package is published can resolve the finding without a source change. |
| Mariner/Azure Linux package | Compare the package version with the [Mariner CVE database](https://aka.ms/astrolabe). |
| Go binary module | Compare the fixed version with every affected binary-producing module at the exact release commit; do not rely on one root `go.mod`. |
| Bundled third-party binary | Use that component's authoritative advisory and version metadata. |

Call a finding a false positive only when the scanner identified the wrong package/version or the
scanned artifact already meets the authoritative fix threshold. A fixed version in current source
proves that a rebuild can resolve the finding; it does not disprove the finding in an older image.

## 3. Trace the Fixed Artifact Through the Release Chain

Follow the repository's
[build and release process](../../../../internal/docs/BUILDANDRELEASE.md#release-process), and
collect evidence for each handoff:

| Handoff | Required evidence |
|---------|-------------------|
| Source to build | First fixed source commit or rebuild, official [pipeline 440](https://github-private.visualstudio.com/azure/_build?definitionId=440) build ID, result, source SHA, and produced tag |
| Build to MCR | [Pipeline 978](https://github-private.visualstudio.com/azure/_build?definitionId=978) release run, result, and fixed tag in the [production MCR catalog](https://mcr.microsoft.com/v2/azuremonitor/containerinsights/ciprod/prometheus-collector/images/tags/list) |
| MCR to AKS-RP | Current [Azure Monitor metrics manifests](https://msazure.visualstudio.com/CloudNativeCompute/_git/aks-rp?path=/toolkit/versioning/manifests/addon/azure-monitor-metrics&version=GBmaster), exact pin, update strategy, and active adoption PR |
| MCR to AgentBaker | Automated `chore(deps): update ama-metrics` PR, current [`components.json`](https://github.com/Azure/AgentBaker/blob/master/parts/common/components.json) pin, checks, reviews, and merge/revert history |
| AgentBaker to VHD | Latest [VHD release notes or image BOM](https://github.com/Azure/AgentBaker/tree/master/vhdbuilder/release-notes) showing the pre-pulled tag and digest |

MCR availability proves publication only; it does not prove that AKS-RP, AgentBaker, or a released
VHD consumes the image.

Use [Semantic Versioning](https://semver.org/spec/v2.0.0.html) to interpret release intent and the
[AKS-RP versioning rules](https://msazure.visualstudio.com/CloudNativeCompute/_git/aks-rp?path=/toolkit/versioning/README.md&version=GBmaster)
to determine update eligibility. For example, a patch-only strategy does not automatically adopt a
new minor version.

## 4. Identify Current Blockers and Owners

For each incomplete handoff:

1. Inspect the current PR state, required reviews, checks, and mergeability.
2. Check for a later successful run before treating an older failed or stuck run as current.
3. Read failed job logs and record the first causal error. Treat cleanup failures as secondary when
   an earlier build or deployment failure prevented resource creation.
4. Distinguish image failures from capacity, quota, authentication, and pipeline infrastructure
   failures.
5. Inspect merge and revert history when current consumer pins contradict an earlier update.
6. Identify the action owner from repository ownership files, PR assignments, documented release
   responsibilities, or the owning pipeline team. Do not infer ownership from the person who
   queued or authored a build.
7. Assign separate actions when different teams own image validation and release infrastructure.

## 5. Report the Conclusion

Report facts and unknowns in this order:

1. **Scanner finding** - exact vulnerable artifact, packages, binaries, and fix thresholds
2. **Fixed artifact** - first verified fixed source commit or rebuild and production image
3. **Current consumers** - exact AKS-RP, AgentBaker, and released VHD pins
4. **First incomplete handoff** - where the fixed artifact stopped propagating
5. **Current blockers** - specific PRs, builds, jobs, reviews, or missing actions
6. **Action owners** - owner for each blocker, based on authoritative ownership evidence
7. **Required completion sequence** - ordered steps from the current state to a fixed released VHD
8. **Uncertainty** - evidence still required before making any causal claim

Do not conclude that the incident is resolved merely because a fixed image exists in MCR. Require
evidence that the vulnerable tag or digest is no longer present in the artifact the scanner
evaluates.
