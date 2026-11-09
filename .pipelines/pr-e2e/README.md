# Ama-metrics vulnerability campaign

Phase 1 is a real scan/fix/build/one-cluster/evidence-PR campaign, not a replacement
for the full scenario matrix. That matrix covers representative current clusters
with auto-upgrade enabled, not historical Kubernetes versions. All runtime code
lives in this repository.
The campaign automatically runs its approved build, scan, and run-owned cluster
validation. It never uses a shared cluster or expands the existing resource/IAM
scope. A human reviews and merges the resulting PR; the agent never approves or
merges it.

Pipeline 440 uses the Official 1ES template only for `refs/heads/main`; branch
and PR runs disable optional SDL via Unofficial's `enableAllTools: false`, with
explicit AntiMalware/SPMI opt-outs covering nonproduction artifacts too.
Component Governance remains enabled; Trivy candidate evidence and signing
are unchanged. Repository-owned CodeQL, CredScan and standalone SDL/report
tasks run only on main, as do production Ev2 artifacts. Official main keeps its
enforced security checks, including Defender's refresh for Linux definitions
at least 2,880 minutes old. Non-main runs no longer request that refresh.

## Implemented

- `scan_trivy.py`: remote-only Linux discovery, fresh private output/cache, explicit
  child digest/platform, empty ignore/config files and recorded scanner/DB/hash.
  It does not use or change the Docker daemon. Exit zero means the scanner ran,
  not that vulnerabilities are absent.
- `compare_trivy.py`: every baseline Linux CVE finding and no-new-findings comparison
  for matched scanner/configuration/DB hashes, complete target/package inventory
  and a candidate bound to the successful build receipt. It cannot clear a
  Qualys QID or claim full-image coverage. Every baseline finding is evaluated
  without severity/fixability filters; unresolved, unreported, or new findings
  block readiness. OS targets must match the scan's exact image ref and OS
  metadata before comparing their distribution/version; binary paths remain exact.
- `published_discovery.py` and `published-images.json`: compare the pinned
  production and latest-main collector `linux/amd64` child digests with Trivy
  0.76.1, the same database snapshot and identical configuration. The main image
  came from build 131326: its collector amd64 child was published while the
  overall build remained in progress and the Windows 2019/index outputs were
  pending. The report is discovery-only; absence from the main scan is not proof
  of a fix. `published-scan-steps.yml` runs this without a 440 artifact, Azure
  service connection, cluster, deployment, or cleanup step. The scan-only 979
  route still requires exact-source preview and its routing guard before a live run.
- `scan.ps1` and `scan-steps.yml`: ported per-child CoMET/OIDC producer, output
  ownership guards and scoped cleanup. They still require dedicated trusted
  workers, approved scanner digests and verified onboarding. Coverage is explicitly
  unverified until real scanner configuration/language proof is implemented.
- `normalize.py`: local Qualys XML parser and matched baseline/candidate QID
  closure evaluation, with supplemental Linux Trivy and no-new-findings comparison.
  Missing, partial, stale, unbound or incomparable evidence cannot pass.
- `build_receipt.py`: verifies successful pipeline 440 source/build metadata and
  per-component push receipts against published MCR manifest/config bytes.
  Emits immutable child refs for Linux amd64/arm64, Windows2022, allocator and cfg;
  rejects incomplete images, moved tags and mismatched source/platforms.
  It does not authenticate caller-supplied JSON or replace protected push receipts.
  SRE-branch product jobs publish four protected `sre-push-*` artifacts, captured
  from BuildKit metadata or Windows push output, with unique per-build tags.
  Fetch the completed authenticated ADO build response before resolving them.
  Windows pools, including the mixed scenario, use the protected Windows2022
  collector receipt and verified `10.0.20348.*` image config. No Windows2025
  producer, scan or common-index receipt is required by this campaign.
  The emitter also requires `sre-testkube-runner`: a protected receipt binding
  separately fetched test source to this build, the existing toolchain's immutable
  image and its durable artifact URL. No replacement runner is built or pushed.
  The pinned existing image is not yet demonstrated compatible with the candidate
  workflows; the coverage owner must verify suitability through real execution.
- `fetch_candidate.py`: authenticates to the fixed build project, checks the
  successful build against the requested full source SHA, downloads five
  protected receipt artifacts plus the collector Trivy artifact, and validates
  the scan provenance and every file hash against that exact build/image. It
  retains the raw producer files and build response beside `candidate-build.json`.
- `producer_trivy.py`: on SRE candidate branches, pipeline 440 replaces its collector
  vulnerability console scan with full, unfiltered JSON for the exact pushed Linux
  amd64 child. The protected `sre-trivy-collector` artifact contains
  `trivy/{result.json,metadata.json}`,
  the exact `cache/db` snapshot, empty config/ignore files, and `provenance.json`
  binding source repo/commit/build, definition 440, index/child/platform and every
  file's SHA256. Only successful first-attempt SRE branch jobs/stages publish it.
  MCR lag retries the immutable digest, never a tag. Filtered console findings and existing
  secret/sidecar scans remain; this is not Windows, arm64 or Qualys clearance.
  A consumer must authenticate this artifact from the exact successful 440 build,
  verify its hashes/provenance against the push receipts, and scan the released
  baseline with that same Trivy version/config/DB. Published prod-vs-main discovery
  needs no new 440 build; this artifact is reusable unmerged-candidate evidence.
- `scan_candidate.py`: authenticates that same build before selecting its collector
  child, scans only the released baseline using the exact scanner version and DB
  from the producer artifact, and evaluates the producer's candidate scan without
  rescanning the candidate. It retains raw scans plus `comparison.json`, including
  failed/blocked verdicts. A local report does not acquire a protected pipeline
  producer by being copied.

These are executable repository components, **not a deployed Azure SRE Agent
integration or an end-to-end passing campaign**.

```powershell
python .pipelines\pr-e2e\published_discovery.py --manifest .pipelines\pr-e2e\published-images.json --output <new-discovery-directory>
python .pipelines\pr-e2e\scan_trivy.py --image <repository@sha256:child> --platform linux/amd64 --output <new-run-directory>
python .pipelines\pr-e2e\scan_trivy.py --image <candidate-repository@sha256:child> --platform linux/amd64 --database-from <fresh-baseline-run> --output <new-candidate-run>
python .pipelines\pr-e2e\compare_trivy.py --baseline <fresh-baseline-run> --candidate <new-candidate-run> --receipt .pipelines\pr-e2e\candidate-build.json --cve CVE-2026-63209 --output <new-comparison.json>
python .pipelines\pr-e2e\normalize.py --targets <candidate-targets.json> --scans <candidate-lanes> --baseline-targets <baseline-targets.json> --baseline-scans <baseline-lanes> --output <new-report-directory>
python .pipelines\pr-e2e\build_receipt.py --build <ado-build.json> --pushes <linux-push.json> <windows-push.json> <allocator-push.json> <cfg-push.json> --runner-receipt <runner.json> --output .pipelines\pr-e2e\candidate-build.json
python .pipelines\pr-e2e\fetch_candidate.py --build-id <successful-build-id> --source-commit <full-candidate-sha> --output <new-evidence-directory>
python .pipelines\pr-e2e\scan_candidate.py --build-id <successful-build-id> --source-commit <full-candidate-sha> --baseline-image <released-ama-metrics-child@sha256> --platform linux/amd64 --cve CVE-2026-63209 --output <new-scan-evidence-directory>
python -m unittest discover -s .pipelines\pr-e2e -p 'test_*.py' -v
```

CoMET lane layout is `<name>/<os>/<Windows-osVersion-if-present>/<arch>/`, containing
`provenance.json`, `completion.json`, `comet/{result.xml,result.json,scan.log}` and
Linux `trivy/{result.json,metadata.json}`. The standalone Trivy discovery output
is not a complete lane and must not be relabeled as CoMET evidence.
Requested QIDs must be explicitly present before and absent after under matched
scanner/KB/configuration/coverage. A missing QID is not closure. Requested CVEs
still need verified QID mapping; a Trivy finding alone does not clear a Qualys QID.
For a Trivy-only Phase 1 finding, use the separate `compare_trivy.py` command.
Rescan the baseline with the current producer, then reuse its hashed private DB
snapshot for the candidate. Both scans and DB must be fresh within 24 hours.
The early discovery receipt predates the DB/configuration hash fields, so it
cannot itself pass this stronger comparison.

Baseline and candidate target descriptors bind `product: ama-metrics`, source
`repo`, full `commit`, original `buildId`, child `images` and requested IDs.
Candidate `baseline` additionally binds source commit/build, current rescan build
and artifact identities. Runtime source/build/coverage assertions must come from
protected jobs, not user-authored JSON. Local validation is not authentication.

The receipt command consumes the completed ADO build REST response and a JSON
array or individual files of protected push receipts:
`{name, commit, buildId, indexRef, tag}`.
Names are `collector` (separate Linux and Windows push entries),
`targetallocator` and `config-reader` (cfg). The separate runner receipt binds
`source: {repo, commit, buildId}`, `imageRef` and `artifactUrl` to the current
build's `sre-testkube-runner` artifact. It is a reused toolchain; its image is
not claimed to have been built from the separately fetched test-source commit.
`schemaVersion` is 1 and `artifactPath` is `runner.json`. The product receipt
embeds this as `testRunner: {imageRef, testSource, images, artifactUrl, artifactPath,
receiptSha256, mode}`; `receiptSha256` hashes the original artifact bytes.
`indexRef` is a published MCR
`repository@sha256:...` captured from that build's push, not resolved later from
a mutable tag. It may name a single manifest. The command verifies its children
and configuration hashes and confirms that the published tag still matches.
Keep the resulting JSON as a campaign artifact, not a fictional committed fixture.
Its `built-not-e2e-validated` status never means tests or scans passed.
The SRE-only receipt job verifies the existing public
`docker.io/cerebro31/ginkgo-test@sha256:749da46a2c50b88e20f249ab3302707cb8005a2ff32a5abe2c37f066076afc57`
manifest/config bytes and requires Linux amd64 support. Public Docker Hub token
exchange is anonymous and memory-only; no workstation login or Docker daemon is used.
The actual gate must execute real TestKube workflows/Ginkgo suites, not a direct
Ginkgo substitute. Bind the TestKube source and runner digest from this receipt;
retain the submitted execution IDs, per-case results and JUnit as durable artifacts.
Unknown, aborted, skipped, empty or missing required cases are not successful.
Before invocation, the authenticated artifact adapter must confirm that pipeline
440 completed successfully for `source.commit`, download `runner.json` from that
same build's protected artifact, verify its hash and cross-check its source/build/
image against `testRunner`. Only then set `CANDIDATE_TEST_SOURCE_SHA` from
`testRunner.testSource.commit` and `CANDIDATE_RUNNER_IMAGE` from
`testRunner.imageRef`. A matching free-form field or URL is not producer evidence.
`fetch_candidate.py` implements that authenticated download/content check. Supply
`SYSTEM_ACCESSTOKEN` through a protected job environment mapping or use an existing
Azure CLI identity authorized to read this ADO project. Tokens remain in memory,
never command arguments or artifacts. The fixed project's verified artifact
service (`artprodcus3.artifacts.visualstudio.com`) also requires ADO authentication;
other cross-host redirects, including signed blob downloads, lose the token.
The CLI emits JSON with `source`, `testRunner` and `candidateReceipt`; importable
`fetch(build_id, commit, output_path)` returns the verified receipt. Retain the
entire output directory, including `sre-testkube-runner/runner.json`.
This is executable code, not proof that the actual ci-sre identity can invoke it.
MCR publication may lag ACR push completion; a missing public manifest blocks
the receipt until it is actually readable. Never substitute a released image.

## Moving execution to the existing ci-sre agent

Local execution bootstraps the same repository commands that trusted ADO workers
will execute. The finished flow must not depend on a workstation, Copilot session
directory, developer login, local Docker engine or absolute local path.

Reuse the existing `ci-sre`; do not create another agent. Its resource is
`/subscriptions/13d371f9-5a39-46d5-8e1b-60158c49db84/resourceGroups/containerinsights-sre/providers/Microsoft.App/agents/ci-sre`.
The validation/build subscription is separately
`9b96ebbd-c57a-42d1-bbe9-b69296e4c7fb`. Resource discovery does not verify its
connectors, branch access or pipeline permissions. The agent
needs authenticated source checkout/edit/commit/push access as well as a pipeline
read adapter. Source/dependency fixes are agent work, not a fixed pipeline patch.
ADO owns clean checkout, scanner/build workers, registry publication, test deployment
and durable artifacts. No new service is required by this design.

Azure SRE Agent tool discovery exposes MCP connector configuration and connector
testing, but that does **not** establish that a suitable ADO adapter is installed,
authorized or reachable on an actual agent. Select and test that existing supported
adapter before deployment; do not assume these local Copilot tools exist in SRE Agent.

### Branch trigger and agent iteration

Use `sre/vuln/<campaign-id>` for the SRE remediation branch and commit
`.pipelines/pr-e2e/campaign.json` with
`{"schemaVersion":1,"cve":"CVE-YYYY-N"}`.
The build pipeline includes this namespace alongside its existing main, OTel,
Dependabot and `*/ama-metrics-sre-*` triggers. A successful 440 completion
triggers pipeline 979 for either SRE campaign branch form only after that
completion filter is present on 979's default branch. The campaign PR trigger
also runs 979 directly and binds its run to the source head's successful 440
build. The candidate run validates the source, CVE and protected receipts before
provisioning. The push-receipt and candidate-build validators accept both SRE
branch forms. Pipeline 440's collector Trivy scan publishes reusable structured
`sre-trivy-collector` evidence; its other console scans remain report-only. The
producer signs only `payload.json` in the collector and allocator signing folders;
the original Buildx `push-metadata.json` remains readable JSON for immutable
push receipts and the collector scanner. No mutable-tag lookup replaces that binding.
This POC's `.pipelines/pr-e2e/campaign.json` selects `CVE-2026-63209` as a smoke test;
the scanner also evaluates every baseline finding without severity or fixability
filters. A
manually queued 979 candidate run can instead supply `candidateBuildId`,
`candidateSourceSha` and `candidateCve`. Candidate runs retain their run-owned
resource group and publish its inventory; the pipeline removes its local kubeconfig
but never invokes cloud cleanup. `candidateScenario: all` (the default) runs
`default`, `linux`, `settings`, `classic-azure-cni`, `mixed`, `autoscaler`, `uai`,
`cilium` and `kubenet` in serial stages, even if an earlier row fails. Each row
has its own UUID, checkout, evidence, retained resource group and bootstrap
artifact. `mixed` and `autoscaler` use `eastus2`; the other seven use `centralus`
in the approved build subscription. A named selector runs just that row.
Before any provisioning, a read-only admission stage compiles the selected rows
and checks each region's total simultaneous max-count-plus-surge against live
regional and VM-family quotas, SKU restrictions and the stable Kubernetes version.
Every row requires that stage's successful, source/build-bound placement output,
and still performs its existing live per-row preflight.
Admission is a point-in-time check, not a capacity reservation. Retention and
auto-upgrade channels remain unchanged; serial test stages do not serialize
later retained-cluster upgrades.
After a successful
retained run, the authorized owner revokes the recorded temporary administrator
assignment with the runner's `revoke-bootstrap` action; resource cleanup remains
an explicit action, not an automatic pipeline step.
The trusted pipeline checkout may include pipeline/scanner-only repairs while
the product/test candidate stays immutable; publication records both revisions
as `pipelineSourceSha` and `sourceSha`. Product or test-source changes require
a matching new protected producer and fresh execution.

Pipeline 440's ordinary PR builds do not run the AKS, OTel, or Arc TestKube
jobs. Pipeline 979 therefore has a PR validation trigger for campaign PRs. It
excludes every PR from the legacy shared-cluster job and admits only SRE branches
to the candidate job. There is no path filter: dependency-only PR updates must
also validate. The candidate execution gate checks the allowed branch and resolved
source SHA before fetching or provisioning the candidate. It resolves a successful
440 build for `System.PullRequest.SourceCommitId`, validates
that build's protected receipts, and runs the existing TestKube scenario against
those exact image digests. The 979 PR run is linked to the PR and auto-cancels
when its head changes; only results for the latest head count. The separate 440
push-triggered run remains the producer and is not itself e2e evidence.
The fetch CLI's `trivyVersion` comes from the verified producer scan metadata;
the pipeline installs that version before reusing the producer's database.

Published discovery also emits its full JSON through the private ADO scan log,
framed as `AMA_METRICS_DISCOVERY_BEGIN/DATA/END`. The gzip/base64 envelope binds
the discovery 979 build ID and source SHA separately from the image producer
provenance, with SHA-256 over the decoded envelope. Existing log-read permission
suffices; findings must not be copied to this public repository.
`discovery_log.py read --input build-log.txt --manifest published-images.json
--build-id <979-id> --source-sha <979-sha> --output discovery.json` rejects
incomplete/duplicate frames, bad checksums, and build/source/image mismatches.
Obtain expected build/source from authenticated ADO metadata and the image
manifest from that exact source, not from the unverified envelope. Verify an
actual agent log round-trip before relying on this fallback.

Each row publishes `coverage.json` as a plan, never as proof of execution. It
explicitly lists blocked matrix rows and the five uncovered settings variants:
`helm-customer-values` (no isolated customer Helm adapter), and
`empty-secret-namespaces-v1`, `empty-secret-namespaces-v2`,
`omitted-secret-namespaces-v1`, `omitted-secret-namespaces-v2` (no enforcement on
the current approved Kubernetes 1.36.5 baseline). No topology or version override
is used to manufacture coverage. A new runnable matrix row or unaccounted variant
fails coverage planning before provisioning. Actual source-bound TestKube results
for every applicable row, not this selector or plan, are required for completion.
The nine-row harness runs config-processing variants, five health suites and
fresh ingestion; it does not run every repository Ginkgo package.
`coverage.json` also blocks `regionTests`: its specific deployed recording-rule
and AMW usage/limits checks need an owned rule and authorization adapter
(`parmRuleName`, `parmAmwResourceId`). Overlapping health checks do not replace
those tests. Nine successful rows alone must not be reported as all repo tests passing.

The ci-sre instruction contract is:

1. Freeze the baseline and scan it; select a real finding and verify the fixed
   package/vendor version. Create or resume the campaign branch at a recorded base.
2. Edit the actual code/dependency pins, respecting generated-upstream boundaries.
   Run local checks, then commit/push only within the human-approved write scope.
3. Observe the push-triggered build for that exact branch/head. Persist its run
   identity; do not queue a duplicate when a push run already exists.
4. Read build/scan/test failures and raw evidence. Diagnose and fix the source or
   harness defect within ownership, then push another commit and repeat. There is
   no one-retry limit and no automatic pipeline-supplied fix. Infrastructure,
   missing-permission or owner conflicts require the corresponding real action,
   not speculative package changes.
5. On every changed head/base, invalidate previous build/image/test/closure verdicts.
   Require current-head component receipts, exact tested child digests, every
   baseline finding closed, no new findings, the PR-linked 979 e2e run for that
   exact head, the required scenario tier and verified restoration.
6. Publish the evidence PR and request human review only with honest scope.
   Never approve/merge, silently waive failing tests, treat skipped/not-run as
   passed or re-label an earlier successful image as the new build.

Keep iterating while there is an evidenced actionable fix. Monitor the PR's own
979 run and required checks for the latest head; after each pushed fix, wait for
the new producer and PR e2e run before evaluating readiness. A real external blocker
pauses with durable diagnostics and the exact missing action; repeated identical
failures without a new hypothesis are not useful iteration.

For now, print the intended Teams approval/results message in the automation
output only. Do not configure or send a Teams message.

### Trigger/input/output

An approved ci-sre campaign request starts the branch loop; scheduling is separate.
The protected pipeline
accepts a campaign ID, allowlisted repository, exact source/base commits, immutable
baseline image references, selected vulnerability IDs and an approved scenario
profile. Stage-specific approval references bind the exact intended action and
resources. A string supplied as `approved: true` is not authorization.

The pipeline adapter reads run status/artifacts, and can queue the allowlisted
pipeline only for an explicitly approved run not already triggered by a push.
Source edits/pushes use the separately scoped repository integration. Use ADO's API:

```text
POST https://dev.azure.com/{org}/{project}/_apis/pipelines/{pipelineId}/runs?api-version=7.1
GET  https://dev.azure.com/{org}/{project}/_apis/pipelines/{pipelineId}/runs/{runId}?api-version=7.1
GET  https://dev.azure.com/{org}/{project}/_apis/build/builds/{buildId}/artifacts?api-version=7.1
```

Queue inputs bind `resources.repositories.self.refName` and `version`; protected
pipeline parameters carry the campaign inputs. The selected deployment identity
must prove the effective checkout SHA. Do not retry an uncertain queue request
blindly: recover the existing run by its campaign/run identity first.

Output is a durable campaign receipt plus immutable artifact references: source/
base/build SHA, image tags and child digests for every built component/platform,
raw before/after scans, selected/fixed IDs, expected and executed spec inventory,
JUnit/results, restore proof, PR URL and actual approval/merge state. An artifact
hash validates content, not the identity or trustworthiness of its producer.

### Identity and approval boundary

| Principal | Required scope |
|---|---|
| ci-sre orchestration identity | Read repository and campaign artifacts; scoped source edits/branch pushes and PR writes under approval. Queue only an approved campaign pipeline when the push did not already trigger it. |
| ADO build identity | Checkout reviewed source; use existing scoped signing/registry service connections and job OIDC for CoMET. |
| ADO validation identity | Approved build-sub resources, dedicated workers, exclusive cluster ownership, scoped test deployment and test AMW query permissions. |
| Human reviewer | Approve intended live actions; review the final PR and merge under repository policy. |

Do not pass PATs, bearer tokens, kubeconfigs or scanner credentials through model
messages, manifests, PR bodies or artifacts. Use supported managed/workload identity
where available, otherwise the approved service connection's protected credential
store. Verify actual adapter authentication support rather than promising MSI.

### Remaining deployment prerequisites

Read-only verification of the existing ci-sre found it Running in review mode.
Code Access now contains six repositories: `Docker-Provider`,
`InfrastructureInsights`, `Microsoft_Azure_ContainerInsightsExt`, `aks-rp`,
`ci-build` and `Azure/prometheus-collector`. The user confirmed that the former
`fluent-bit`, `fluentd` and `telegraf` entries were intentionally removed. The
repository connection test passes using the existing `azure-monitor-assistant`
GitHub App, and the host auth domain reports `GitHubApp`. The original OAuth
attempt was unrelated to this app; do not retry its SSO flow.

| Code Access field | Value |
|---|---|
| Client ID | Existing `azure-monitor-assistant` GitHub App client ID |
| Private key URI (Key Vault) | Existing app key's Key Vault **key** URI |
| Key Vault identity | ci-sre's existing user-assigned managed identity |

Use the Key Vault **key** URI (`/keys/`), not the legacy secret URI (`/secrets/`);
the private key remains in Key Vault. The selected ci-sre identity needs the
Key Vault cryptographic permission required to sign with this key. The ci-sre
identity's Key Vault Crypto User assignment was verified before the Code Access
test. Do not repeat the unrelated `SREAgent-OAuth` SSO flow.

The Client ID and key URI were verified in the existing
`managed-prometheus-agent-tsg` setup; ci-sre must use its own managed identity.
Keep those identifiers in the approved configuration surface rather than
committing them here. The separate ci-sre `github` data connector remains
`GitHubOAuth` by user decision; Code Access validation does not verify that
connector's PR/action identity. The successful Code Access test returned HTTP
200 with `isSuccessful: true`.

Next, approve branch writes and pipeline publication, and verify effective 1ES
expansion.
Configure/test the actual pipeline read/queue adapter and least-privilege identities; approve
dedicated Linux/Windows scan/build workers and CoMET coverage; attest candidate
build receipts; connect the scenario owner's live cluster execution; persist and
retrieve campaign artifacts from ADO after agent restarts. Demonstrate one complete
remote campaign before calling the SRE integration deployed.

None of those deployment/permission changes is authorized merely by this runbook.
Full-matrix success remains required before advertising the PR as merge-ready.
The release manifest and Monday consumer spec follow in Phase 3; no release
orchestration is installed by Phase 1.
