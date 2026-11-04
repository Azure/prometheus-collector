# Ama-metrics vulnerability campaign

Phase 1 is a real scan/fix/build/one-cluster/evidence-PR campaign, not a replacement
for the full scenario matrix. That matrix covers representative current clusters
with auto-upgrade enabled, not historical Kubernetes versions. All runtime code
lives in this repository.
Human approval gates commits/pushes, live scans, image publication, pipeline runs
and cluster mutations. The agent never approves or merges its own PR.

## Implemented

- `scan_trivy.py`: remote-only Linux discovery, fresh private output/cache, explicit
  child digest/platform, empty ignore/config files and recorded scanner/DB/hash.
  It does not use or change the Docker daemon. Exit zero means the scanner ran,
  not that vulnerabilities are absent.
- `compare_trivy.py`: selected Linux CVE closure and no-new-findings comparison
  for matched scanner/configuration/DB hashes, complete target/package inventory
  and a candidate bound to the successful build receipt. It cannot clear a
  Qualys QID or claim Windows/full-image coverage. All residual findings remain
  in its JSON output.
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
  The emitter also requires `sre-testkube-runner`: a protected receipt binding
  separately fetched test source to this build, the existing toolchain's immutable
  image and its durable artifact URL. No replacement runner is built or pushed.
  The pinned existing image is not yet demonstrated compatible with the candidate
  workflows; the coverage owner must verify suitability through real execution.
- `fetch_candidate.py`: authenticates to the fixed build project, checks the
  successful build against the requested full source SHA, downloads its five
  protected receipt artifacts and calls the receipt verifier. Retains the raw
  producer files and build response beside `candidate-build.json`.

These are executable repository components, **not a deployed Azure SRE Agent
integration or an end-to-end passing campaign**.

```powershell
python .pipelines\pr-e2e\scan_trivy.py --image <repository@sha256:child> --platform linux/amd64 --output <new-run-directory>
python .pipelines\pr-e2e\scan_trivy.py --image <candidate-repository@sha256:child> --platform linux/amd64 --database-from <fresh-baseline-run> --output <new-candidate-run>
python .pipelines\pr-e2e\compare_trivy.py --baseline <fresh-baseline-run> --candidate <new-candidate-run> --receipt .pipelines\pr-e2e\candidate-build.json --cve CVE-2026-63209 --output <new-comparison.json>
python .pipelines\pr-e2e\normalize.py --targets <candidate-targets.json> --scans <candidate-lanes> --baseline-targets <baseline-targets.json> --baseline-scans <baseline-lanes> --output <new-report-directory>
python .pipelines\pr-e2e\build_receipt.py --build <ado-build.json> --pushes <linux-push.json> <windows-push.json> <allocator-push.json> <cfg-push.json> --runner-receipt <runner.json> --output .pipelines\pr-e2e\candidate-build.json
python .pipelines\pr-e2e\fetch_candidate.py --build-id <successful-build-id> --source-commit <full-candidate-sha> --output <new-evidence-directory>
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

Use `<owner>/ama-metrics-sre-<campaign>` (for example
`bot/ama-metrics-sre-compress`). The build pipeline includes
`*/ama-metrics-sre-*` alongside its existing main/OTel/Dependabot triggers.
Pushes trigger the existing build and report-only Linux Trivy jobs. The current
main-only TestKube conditions do **not** make this an exact-image candidate test
trigger yet; wiring the owned scenario runner and strict scans remains required.

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
   Require current-head component receipts, exact tested child digests, selected
   closure, no new findings, full required matrix and verified restoration.
6. Publish the evidence PR and human approval request only with honest scope.
   Never approve/merge, silently waive failing tests, treat skipped/not-run as passed
   or re-label an earlier successful image as the new build.

Keep iterating while there is an evidenced actionable fix. A real external blocker
pauses with durable diagnostics and the exact missing approval/action; repeated
identical failures without a new hypothesis are not useful iteration.

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

Verify the existing ci-sre resource and its repository integration, approve branch
writes and pipeline publication, and verify effective 1ES expansion.
Configure/test the actual pipeline read/queue adapter and least-privilege identities; approve
dedicated Linux/Windows scan/build workers and CoMET coverage; attest candidate
build receipts; connect the scenario owner's live cluster execution; persist and
retrieve campaign artifacts from ADO after agent restarts. Demonstrate one complete
remote campaign before calling the SRE integration deployed.

None of those deployment/permission changes is authorized merely by this runbook.
Full-matrix success remains required before advertising the PR as merge-ready.
The release manifest and Monday consumer spec follow in Phase 3; no release
orchestration is installed by Phase 1.
