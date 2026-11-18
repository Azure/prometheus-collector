# Ama-metrics vulnerability campaign

Phase 1 supplies scan/fix/build/cluster/evidence-PR components, **not a deployed
Azure SRE Agent integration or an end-to-end passing campaign**. It does not
replace the full scenario matrix: representative current, auto-upgraded clusters,
not historical Kubernetes versions. Approved validation never uses shared
clusters or expands resource/IAM scope. Only humans approve and merge.

Pipeline 440 uses the Official 1ES template only for `refs/heads/main`; branch
and PR runs disable optional SDL via Unofficial's `enableAllTools: false`, with
explicit AntiMalware/SPMI opt-outs covering nonproduction artifacts too.
Component Governance remains enabled; Trivy candidate evidence and signing
are unchanged. Repository-owned CodeQL, CredScan and standalone SDL/report
tasks run only on main, as do production Ev2 artifacts. Official main keeps its
enforced security checks, including Defender's refresh for Linux definitions
at least 2,880 minutes old. Non-main runs no longer request that refresh.

## Components

- `scan_trivy.py`: remote-only Linux discovery, fresh private output/cache, explicit
  child digest/platform, empty ignore/config files and recorded scanner/DB/hash.
  No Docker daemon is used. Exit zero means the scanner ran, not a clean image.
- `compare_trivy.py`: baseline Linux CVE and no-new-findings comparison for
  matched scanner/configuration/DB hashes, complete target/package inventory
  and a receipt-bound candidate. OS targets must match the exact scan image ref
  and OS metadata; binary paths remain exact. Collector deferrals are described
  below. It cannot clear Qualys QIDs or claim full-image coverage.
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
  Windows pools, including the mixed scenario, use the protected Windows2022
  collector receipt and `10.0.20348.*` config; no Windows2025 receipt is required.
  The protected `sre-testkube-runner` receipt binds separately fetched test source,
  this build, the existing immutable toolchain image and its artifact URL.
  No replacement runner is built; suitability still requires real execution.
- `fetch_candidate.py`: authenticates to the fixed build project, checks the
  successful build against the requested full source SHA, downloads five
  protected receipt artifacts plus the collector Trivy artifact, and validates
  the scan provenance and every file hash against that exact build/image. It
  retains the raw producer files and build response beside `candidate-build.json`.
- `producer_trivy.py`: on SRE candidate branches, pipeline 440 replaces its collector
  vulnerability console scan with full, unfiltered JSON for the exact pushed Linux
  amd64 child. `sre-trivy-collector` retains the raw scan/log, hashed private DB
  snapshot and provenance binding repo/commit/build, definition 440,
  index/child/platform and published file hashes. Downloads are bounded at
  512 MiB, extraction at 2 GiB. Only successful first-attempt jobs/stages publish.
  MCR lag retries the digest, never a tag. Existing filtered console, secret and
  sidecar scans remain; this is not Windows, arm64 or Qualys clearance.
  Published prod-vs-main discovery needs no new 440 build.
- `scan_candidate.py`: authenticates that same build before selecting its collector
  child, reuses its fresh scan or rescans the exact digest when authenticated
  evidence expires. The baseline uses the same scanner version and frozen
  database. Raw scans and `comparison.json` are retained, including failed or
  blocked verdicts. A local refresh does not replace the protected producer report.

```powershell
python .pipelines\pr-e2e\scan_trivy.py --image <repository@sha256:child> --platform linux/amd64 --output <new-run-directory>
python .pipelines\pr-e2e\scan_trivy.py --image <candidate-repository@sha256:child> --platform linux/amd64 --database-from <fresh-baseline-run> --output <new-candidate-run>
python .pipelines\pr-e2e\compare_trivy.py --baseline <fresh-baseline-run> --candidate <new-candidate-run> --receipt .pipelines\pr-e2e\candidate-build.json --output <new-comparison.json>
python .pipelines\pr-e2e\normalize.py --targets <candidate-targets.json> --scans <candidate-lanes> --baseline-targets <baseline-targets.json> --baseline-scans <baseline-lanes> --output <new-report-directory>
python .pipelines\pr-e2e\build_receipt.py --build <ado-build.json> --pushes <linux-push.json> <windows-push.json> <allocator-push.json> <cfg-push.json> --runner-receipt <runner.json> --output .pipelines\pr-e2e\candidate-build.json
python .pipelines\pr-e2e\fetch_candidate.py --build-id <successful-build-id> --source-commit <full-candidate-sha> --output <new-evidence-directory>
python .pipelines\pr-e2e\scan_candidate.py --build-id <successful-build-id> --source-commit <full-candidate-sha> --baseline-image <released-ama-metrics-child@sha256> --platform linux/amd64 --output <new-scan-evidence-directory>
```

CoMET lanes use `<name>/<os>/<Windows-osVersion-if-present>/<arch>/`, containing
`provenance.json`, `completion.json`, `comet/{result.xml,result.json,scan.log}` and
Linux `trivy/{result.json,metadata.json}`. The standalone Trivy discovery output
is not a complete lane and must not be relabeled as CoMET evidence.
QIDs must be explicitly present before and absent after with matched scanner/
KB/configuration/coverage; CVEs need verified QID mapping. Missing QIDs or Trivy
findings alone cannot prove Qualys closure. For Trivy-only comparison, rescan
the baseline and reuse its hashed private DB for the candidate. Both scans and
DB must be fresh within 24 hours; legacy receipts lacking DB/config hashes fail.

Baseline and candidate target descriptors bind `product: ama-metrics`, source
`repo`, full `commit`, original `buildId`, child `images` and requested IDs.
Candidate `baseline` additionally binds source commit/build, current rescan build
and artifact identities. Runtime source/build/coverage assertions must come from
protected jobs, not user-authored JSON. Local validation is not authentication.

`build_receipt.py` consumes the authenticated completed build response and four
protected `sre-push-*` receipts from BuildKit/Windows push output:
`{name, commit, buildId, indexRef, tag}`. Names are `collector` (Linux and Windows
entries), `targetallocator` and `config-reader` (cfg). Tags are unique per build;
`indexRef` is the published MCR digest captured at push, possibly a single
manifest, never a later tag lookup. Children/config hashes and the tag must match.
The schema-1 `runner.json` binds `source: {repo, commit, buildId}`, `imageRef` and
`artifactUrl` to the same build's `sre-testkube-runner` artifact; embedded
`testRunner.receiptSha256` hashes its original bytes. The reused toolchain image
is not claimed to be built from the separately fetched test source.
Retain receipts as artifacts, not committed fixtures; `built-not-e2e-validated`
does not mean tests or scans passed.
The SRE-only receipt job verifies the existing public
`docker.io/cerebro31/ginkgo-test@sha256:749da46a2c50b88e20f249ab3302707cb8005a2ff32a5abe2c37f066076afc57`
manifest/config bytes and requires Linux amd64 support. Public Docker Hub token
exchange is anonymous and memory-only; no workstation login or Docker daemon is used.
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
MCR publication may lag ACR push completion; a missing public manifest blocks
the receipt until it is actually readable. Never substitute a released image.

## Moving execution to the existing ci-sre agent

The SRE Agent consumes private build logs, not artifact downloads. Candidate
collector scans now emit `AMA_METRICS_CANDIDATE_SCAN_BEGIN/DATA/END` using the
same counted, checksummed gzip/base64 framing as discovery. The bounded 32 MiB
envelope preserves the original provenance, scanner metadata and full result
bytes; it does not truncate findings. `producer_trivy.py` emits it directly.
`push_receipt.py` also emits `AMA_METRICS_PUSH_RECEIPT` JSON with the original
push digest for independent image binding.

Before consuming a candidate scan, authenticate 440 build metadata and require
the expected repository/source SHA, successful terminal build, successful scan
and push tasks, and first job/stage attempts. Read the full scan and push logs
through the agent's authorized connector. Cross-check the push receipt's source/
build against metadata, resolve its immutable index with `resolve_images`, and
select the Linux amd64 child. Never take expected bindings from the scan envelope.
Read trusted decoder code from that build's exact source, then run:

```text
python .pipelines\pr-e2e\candidate_log.py --input scan-log.txt --build-id <440-id> --source-sha <sha> --image-ref <resolved-child@sha256:...> --index-ref <push-index@sha256:...> --output candidate-scan.json
```

The decoder rejects incomplete/duplicate frames, oversized envelopes, wrong
source/build/image/platform/coverage, hashes, configuration or DB identity
without writing output. Record actual connector call IDs and negative checks.
Missing frames block consumption; console tables and older baselines are not
substitutes. This remains Linux amd64 collector Trivy evidence only. ADO worker
artifact checks are unchanged; SRE Agent readiness does not require artifact downloads.

Reuse the existing `ci-sre`; do not create another agent. Its resource is
`/subscriptions/13d371f9-5a39-46d5-8e1b-60158c49db84/resourceGroups/containerinsights-sre/providers/Microsoft.App/agents/ci-sre`.
The validation/build subscription is separately
`9b96ebbd-c57a-42d1-bbe9-b69296e4c7fb`. Resource discovery does not verify its
connectors, branch access or pipeline permissions. The agent
needs authenticated source checkout/edit/commit/push access and a tested pipeline
read adapter; local Copilot tool availability does not prove either. ADO owns
checkout, scan/build workers, publication, deployment and durable artifacts.
The remote flow must not depend on a workstation, developer login, local Docker
engine, Copilot session directory or absolute local path.

### Candidate pipeline

Use `sre/vuln/<campaign-id>` for the SRE remediation branch.
Pipeline 440 triggers on this namespace and `*/ama-metrics-sre-*`. Its successful
completion triggers 979 only once that completion filter is on 979's default
branch. Campaign PRs also trigger 979 directly, with no path filter, including
dependency-only updates. PRs cannot enter the legacy shared-cluster job.
Before fetching or provisioning, 979 admits only an authorized SRE head and
resolves its successful 440 build using `System.PullRequest.SourceCommitId`.
Only the latest head's PR-linked 979 results count; head changes auto-cancel
the old PR run. A 440 producer run is not e2e evidence. Manual 979 runs instead
supply `candidateBuildId` and `candidateSourceSha`.

`CandidateCapacity` validates receipts, frozen DB and source-bound placements/
retained-host state before admission. `CandidateScan` reuses fresh findings;
after 24 hours it rescans the receipt-bound digest and uses that frozen DB for
the baseline. Both paths use the verified producer scanner version and canonical
configuration, installed before comparison. Missing/invalid snapshots or
incomparable refreshes fail closed; legacy report-only artifacts cannot pass.
A failed/blocked scan does not skip functional lanes but still prevents approval.
Full findings remain in the authenticated log; DB bytes stay in the protected
artifact. Signing covers only `payload.json`, leaving original Buildx
`push-metadata.json` readable for digest-bound push receipts/scanning.
Comparison has no severity, fixability or CVE filters. Open/new findings
(including Go standard library) in the exact
`opt/microsoft/otelcollector/otelcollector`, `opt/microsoft/otelcollector/prometheusui`
and `opt/promconfigvalidator` targets remain unresolved/deferred to the separate
Collector upgrade, not fixed. Other targets' findings fail; missing evidence
blocks. All raw findings are retained.

`candidateScenario: all` runs the nine required lanes; see the
[scenario runbook](../../otelcollector/test/scenarios/README.md) for lane selection,
placements, exclusive locks, retained-host ownership and bootstrap revocation.
Read-only admission checks source/build bindings, pool/version compatibility and
fixture ownership; each lane still performs live preflight. There is **no capacity
or quota admission** despite the compatibility name `CandidateCapacity`.
Provisioning/scale-out can fail for insufficient capacity or quota.
Persistent profile resource groups are retained, ownership-checked and never
automatically deleted; local kubeconfigs are removed. Tests are serialized per
profile, but automatic cluster upgrades remain enabled independently.
The trusted pipeline checkout may contain harness/scanner repairs while product
images remain immutable; publication records pipeline/product/test revisions.
Product changes require a matching protected producer; test changes require
fresh execution at the recorded test-source SHA. Earlier branch passes do not
validate the consolidated tree.

### Vulnerability Management findings for Linux and Windows

Use the existing attached `shavulnmgmt` connector as a second findings source,
alongside Trivy, for both Linux and Windows ama-metrics images. Load the
[query-vulnerability-findings skill](../../.github/skills/query-vulnerability-findings/SKILL.md)
during setup and each campaign's baseline/candidate evaluation. It defines the
exact query, actual-agent execution proof, complete private snapshot, native OS
and digest binding, source-preserving reconciliation and fix-or-blocker contract.
Do not substitute workstation results for agent consumption. Trivy deferrals
do not establish Kusto closure; absent or stale Kusto rows are not closure.

`coverage.json` is report-only, not execution proof. Known blockers (including
`helm-customer-values`) do not fail individual runs; functional, scan and cleanup
failures do. New runnable rows or unaccounted variants fail planning before provisioning.

Before merge approval, every required lane must have a passing result and
non-empty executed JUnit for the same test source, product source, producer build,
and candidate digests. Missing, skipped, failed, or mismatched results do not
count. Cross-run approval enforcement is not implemented. The strict
`candidate_coverage.py --require-unblocked` mode still fails on global plan
blockers when explicitly requested; it is not wired to merge approval.
The harness runs config-processing, five health suites and fresh ingestion,
not every Ginkgo package. `regionTests` remains blocked pending its owned
recording rule and AMW authorization adapter (`parmRuleName`, `parmAmwResourceId`).
Overlapping health checks or nine passing rows do not mean all repo tests passed.

### Agent loop

1. Freeze the baseline, scan it with Trivy and collect a fresh complete Kusto
   snapshot for Linux and Windows through the actual agent connector. Bind and
   reconcile both sources as above; select a real finding and verify the fixed
   package/vendor version. Create or resume the campaign branch at a recorded base.
2. Edit the actual code/dependency pins, respecting generated-upstream boundaries.
   Keep coordinated upstream OpenTelemetry Collector version upgrades in their
   separate PR; report remaining collector findings rather than bundling that
   upgrade or claiming closure. Minimal module tidies required by changed local
   product dependencies are integration fixes, not a Collector version upgrade.
   Run local checks, then commit/push only within the human-approved write scope.
3. Observe the push-triggered build for that exact branch/head. Persist its run
   identity; do not queue a duplicate when a push run already exists.
4. Diagnose build/scan/test failures from raw evidence, fix source/harness defects
   within ownership and repeat. Infrastructure, permission and owner conflicts
   require real remediation, not speculative package changes.
5. On every changed head/base, invalidate previous build/image/test/closure verdicts.
   Require current-head component receipts, exact tested child digests, all
   non-deferred findings closed with no new non-deferred findings, the PR-linked
   979 e2e run for that head, the required scenario tier and verified restoration.
   Deferred Collector findings remain unresolved. Require fresh
   source-specific Kusto evidence for the relevant digests as well; missing or
   pending external scans remain explicit blockers, not successful rescans.
6. Publish the evidence PR and request human review only with honest scope.
   Never approve/merge, silently waive failing tests, treat skipped/not-run as
   passed or re-label an earlier successful image as the new build.

After each fix, wait for the latest producer and PR e2e checks. Continue while
there is an evidenced actionable fix; external blockers need durable diagnostics
and the exact missing action, not identical retries.

For now, print the intended Teams approval/results message in the automation
output only. Do not configure or send a Teams message.

### Trigger/input/output

An approved ci-sre campaign request starts the branch loop; scheduling is separate.
The planned adapter must bind the allowlisted repository, exact source/base
commits, immutable baseline image references and approved scenario profile.
Stage-specific approvals must bind the intended action and resources; a string
supplied as `approved: true` is not authorization.

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

Code Access uses the existing `azure-monitor-assistant` GitHub App. Do not retry
the unrelated `SREAgent-OAuth` flow or restore intentionally removed repositories.

| Code Access field | Value |
|---|---|
| Client ID | Existing `azure-monitor-assistant` GitHub App client ID |
| Private key URI (Key Vault) | Existing app key's Key Vault **key** URI |
| Key Vault identity | ci-sre's existing user-assigned managed identity |

Use `/keys/`, not `/secrets/`; the private key stays in Key Vault. ci-sre needs
its own managed identity with signing permission (Key Vault Crypto User).
Keep app identifiers in approved configuration, not this repository. Code Access
validation does not verify the separate `github` data connector's PR/action
identity; that connector remains `GitHubOAuth` by user decision.

Before calling the integration deployed: approve branch writes/publication,
verify 1ES expansion, test the actual pipeline adapter and least-privilege
identities, approve dedicated Linux/Windows workers and CoMET coverage, attest
receipts, connect live scenario execution, and demonstrate durable ADO artifact
retrieval after restart in one complete remote campaign.
This runbook authorizes no deployment/permission changes. Full-matrix success is
required for merge readiness; Phase 1 installs no release orchestration.

### Native advisory log report

`SCAN_ADVISORY` and `NATIVE_ADVISORY_REPORT` reuse Trivy results and hash
executables from digest-verified layers without executing images, rescanning or
adding artifact files. Only byte-bound code evidence confirms findings (FAIL).
Version/RPM matches are potential associations, not ME/mdsd upgrade advice.
Missing files, unsupported links and registry failures yield `binaryIdentityError`.
Incomplete inventory, applicability/backport evidence or lookups remain BLOCKED,
never PASS. These diagnostics cannot grant native clearance or replace the
authenticated full Trivy report and frozen-DB checks.
