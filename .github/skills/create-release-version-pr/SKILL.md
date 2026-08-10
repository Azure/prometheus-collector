---
name: create-release-version-pr
description: Create a prometheus-collector version bump and release notes PR by reviewing every commit merged to main since the previous version bump PR. Use when "create a release PR", "create version bump PR", "prepare release notes", "cut a prometheus-collector release", or "make a PR like #1607".
allowed-tools:
  - run_in_terminal
  - read_file
  - edit_file
  - create_file
---

# Create Release Version and Release Notes PR

Create a release PR like [#1607](https://github.com/Azure/prometheus-collector/pull/1607). The PR must update only:

- `otelcollector/VERSION`
- `RELEASENOTES.md`

The release notes must account for every commit merged to `main` after the previous version bump PR. Do not rely only on commit titles or a hand-selected list.

## Inputs

- **New version** in `MAJOR.MINOR.PATCH` format. Optional: when omitted, bump the current minor version and reset patch to zero.
- **Release date** in `MM-DD-YYYY` format. Default to the current date in Pacific Time if the user does not provide one.

When the user explicitly supplies a version, use it after validation. Otherwise derive the version from `otelcollector/VERSION` on `origin/main`:

```powershell
$currentSemanticVersion = [version]$currentVersion
$newVersion = "{0}.{1}.0" -f `
    $currentSemanticVersion.Major, `
    ($currentSemanticVersion.Minor + 1)
```

For example, `7.2.1` becomes `7.3.0`. Do not ask for confirmation of this default minor bump.

When the date is omitted, calculate it with:

```powershell
$releaseDate = [TimeZoneInfo]::ConvertTimeBySystemTimeZoneId(
    [DateTimeOffset]::UtcNow,
    "Pacific Standard Time"
).ToString("MM-dd-yyyy")
```

## Execution Rules

- Use `gh` for GitHub PR and repository queries.
- Fetch the latest `origin/main` before calculating the release range.
- Never discard or overwrite an existing dirty worktree.
- Create an isolated git worktree from `origin/main` for the release branch.
- Inspect every commit and associated PR in the release range.
- Include every commit exactly once in the release audit.
- Do not commit, push, or create a PR when the user asks only for a preview or draft.
- When the user asks to create the PR, commit, push, and run `gh pr create` without an additional confirmation prompt.

## Phase 1: Prepare an Isolated Release Worktree

1. Verify the repository:

   ```powershell
   git remote get-url origin
   gh repo view Azure/prometheus-collector --json nameWithOwner
   git fetch origin --prune
   ```

2. If the user supplied a version, validate it:

   ```powershell
   if ($suppliedVersion -and $suppliedVersion -notmatch '^\d+\.\d+\.\d+$') {
       throw "Version must use MAJOR.MINOR.PATCH format."
   }
   ```

3. Read the current version from `origin/main`:

   ```powershell
   $currentVersion = (git show origin/main:otelcollector/VERSION).Trim()
   ```

4. Select the new version:

   ```powershell
   $currentSemanticVersion = [version]$currentVersion
   if ($suppliedVersion) {
       $newVersion = $suppliedVersion
   } else {
       $newVersion = "{0}.{1}.0" -f `
           $currentSemanticVersion.Major, `
           ($currentSemanticVersion.Minor + 1)
   }

   $newSemanticVersion = [version]$newVersion
   if ($newSemanticVersion -le $currentSemanticVersion) {
       throw "New version must be greater than $currentVersion."
   }
   ```

5. Create a branch and isolated worktree. Obtain the GitHub login dynamically:

   ```powershell
   $login = gh api user --jq .login
   $branch = "$login/release-{NEW_VERSION}"
   $worktree = Join-Path (Split-Path (git rev-parse --show-toplevel) -Parent) "prometheus-collector-release-{NEW_VERSION}"

   git show-ref --verify --quiet "refs/heads/$branch"
   if ($LASTEXITCODE -eq 0) {
       throw "Local branch $branch already exists."
   }

   git ls-remote --exit-code --heads origin $branch
   if ($LASTEXITCODE -eq 0) {
       throw "Remote branch $branch already exists."
   }

   git worktree add -b $branch $worktree origin/main
   ```

   Perform all remaining file changes and git operations inside this worktree.

## Phase 2: Find the Previous Version Bump PR

The previous release boundary is the most recent commit on `main` that changed both `otelcollector/VERSION` and `RELEASENOTES.md`, mapped back to its merged PR. This includes full and scoped releases such as CCP-only releases. Do not merely choose the latest Git tag, latest release-note-only update, or search by PR title.

1. List first-parent commits that changed `otelcollector/VERSION`:

   ```powershell
   git log origin/main --first-parent --format='%H%x09%s' -- otelcollector/VERSION
   ```

2. Starting with the newest candidate, compare it with its first parent:

   ```powershell
   git diff --name-only "{CANDIDATE_SHA}^1" "{CANDIDATE_SHA}"
   ```

3. Select the first candidate whose changed paths contain both:

   - `otelcollector/VERSION`
   - `RELEASENOTES.md`

4. Map the selected commit to its merged PR:

   - Extract a PR number from a trailing `(#1234)` when available.
   - Otherwise query:

     ```powershell
     gh api `
       -H "Accept: application/vnd.github+json" `
       repos/Azure/prometheus-collector/commits/{CANDIDATE_SHA}/pulls
     ```

   - Select only a PR merged into `main` whose `merge_commit_sha` is the selected commit.
   - Verify its files:

     ```powershell
     gh pr view {PR_NUMBER} --repo Azure/prometheus-collector `
       --json number,title,mergedAt,mergeCommit,files,url
     ```

5. Record:

   - Previous release PR number and URL
   - Previous release merge commit SHA
   - Previous version from that merge commit
   - Previous release date from the first release heading added by that PR

6. Verify the selected merge commit is an ancestor of `origin/main`:

   ```powershell
   git merge-base --is-ancestor {PREVIOUS_RELEASE_SHA} origin/main
   ```

   Stop if no valid previous version bump PR is found or if its merge commit is not on `main`.

## Phase 3: Build the Complete Commit and PR Inventory

1. List every first-parent commit merged after the previous release:

   ```powershell
   git log --first-parent --reverse `
     --format='%H%x09%s' `
     {PREVIOUS_RELEASE_SHA}..origin/main
   ```

   First-parent history is authoritative for what entered `main`.

2. For each commit:

   - Extract the PR number from a trailing `(#1234)` when present.
   - If the number is absent or uncertain, query associated PRs:

     ```powershell
     gh api `
       -H "Accept: application/vnd.github+json" `
       repos/Azure/prometheus-collector/commits/{COMMIT_SHA}/pulls
     ```

   - Use only a PR that was merged into `main` and contains this commit.
   - Query the PR title, body, labels, merge commit, and complete file list:

     ```powershell
     gh pr view {PR_NUMBER} --repo Azure/prometheus-collector `
       --json number,title,body,labels,files,mergeCommit,mergedAt,url
     ```

   - For direct commits with no associated PR, retain the commit title and use a commit URL.

3. Maintain an audit table with one row per first-parent commit:

   | Commit | PR/commit URL | Title | Changed paths | Category | Included |
   |--------|---------------|-------|---------------|----------|----------|

4. Do not silently exclude:

   - Dependabot updates
   - Reverts
   - Release-note hash follow-up PRs
   - Documentation, pipeline, test, workflow, or skill changes
   - Merge commits with unclear titles

5. Exclude a commit only when it is proven to be a duplicate representation of another first-parent entry. Document the reason in the audit.

## Phase 4: Classify Release Note Entries

Use the same two sections as PR #1607:

### AKS and Arc Container Images

Use this category when a PR affects any shipped artifact or its runtime behavior, including:

- Collector, Target Allocator, configuration reader, Fluent Bit, or kube-state-metrics code
- Runtime dependencies used by shipped binaries or images
- Helm chart templates, default configuration, RBAC, toggles, or manifests
- Dockerfiles, base images, packaging, or image build inputs
- Scraping, ingestion, authentication, networking, telemetry, or runtime bug fixes

### Pipeline/Docs/Templates Updates

Use this category when a PR affects only:

- CI/CD pipelines, GitHub workflows, or build/test infrastructure
- Tests that do not alter a shipped image
- Documentation or release notes
- Repository automation, tools, templates, or skills

If a PR affects both categories, list it once under **AKS and Arc Container Images**. Use the PR title as the entry text unless it is unclear; make only a minimal clarification based on the PR body and diff.

Each entry must use this format:

```markdown
  + PR title (https://github.com/Azure/prometheus-collector/pull/1234)
```

For direct commits:

```markdown
  + Commit title (https://github.com/Azure/prometheus-collector/commit/FULL_SHA)
```

Preserve chronological merge order within each category. Do not list the new release PR itself.

## Phase 5: Update Version and Release Notes

1. Replace the exact content of `otelcollector/VERSION` with the new version. Preserve the file's existing trailing-newline convention.

2. Insert a new release section immediately after the top-level heading in `RELEASENOTES.md`:

   ```markdown
   ## Release {MM-DD-YYYY}
   * Linux image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:{NEW_VERSION}-main-{MM-DD-YYYY}-<TBD>`
   * Windows image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:{NEW_VERSION}-main-{MM-DD-YYYY}-<TBD>-win`
   * TA image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:{NEW_VERSION}-main-{MM-DD-YYYY}-<TBD>-targetallocator`
   * cfg sidecar image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:{NEW_VERSION}-main-{MM-DD-YYYY}-<TBD>-cfg`
   * AKS and Arc Container Images :
     + ...

   * Pipeline/Docs/Templates Updates:
     + ...
   ```

3. Omit an empty category rather than adding placeholder text.

4. Leave `<TBD>` in all four image tags. The merge/build commit hash is added by a later PR.

5. Do not alter existing release sections.

## Phase 6: Completeness Verification

Before committing:

1. Re-run the first-parent log and compare it with the audit table.
2. Confirm every commit is included exactly once.
3. Confirm every PR URL exists and matches its title.
4. Confirm no PR merged before or at the previous release boundary is included.
5. Confirm the new section is the first release section.
6. Confirm all four image tags contain:

   - The exact new version
   - The exact release date
   - `<TBD>`
   - The correct component suffix

7. Confirm the only changed files are:

   ```text
   RELEASENOTES.md
   otelcollector/VERSION
   ```

   Use:

   ```powershell
   git diff --check
   git status --short
   git diff -- RELEASENOTES.md otelcollector/VERSION
   ```

Stop and resolve any missing, duplicate, or unexplained entry before proceeding.

## Phase 7: Commit, Push, and Create the PR

1. Commit:

   ```powershell
   git add RELEASENOTES.md otelcollector/VERSION
   git commit -m "version bump and rel notes for {NEW_VERSION} release" `
     -m "Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>"
   ```

2. Push:

   ```powershell
   git push -u origin {BRANCH_NAME}
   ```

3. Create the PR:

   ```powershell
   gh pr create --repo Azure/prometheus-collector `
     --base main `
     --head {BRANCH_NAME} `
     --title "version bump and rel notes for {NEW_VERSION} release" `
     --body-file {PR_BODY_FILE}
   ```

4. The PR body must include:

   ```markdown
   # PR Description
   Version bump and release notes for the **{NEW_VERSION}** release.

   ## Version
   `otelcollector/VERSION`: `{OLD_VERSION}` -> `{NEW_VERSION}`.

   ## Release notes
   Adds `## Release {MM-DD-YYYY}` and covers every commit merged to `main`
   after #{PREVIOUS_RELEASE_PR} (`{PREVIOUS_RELEASE_SHORT_SHA}`).

   **AKS and Arc Container Images**
   - PR title (#1234)

   **Pipeline/Docs/Templates Updates**
   - PR title (#1235)

   ## Follow-up
   Image tags intentionally use `<TBD>`. After this PR merges and the build
   produces `{NEW_VERSION}-main-{MM-DD-YYYY}-<hash>`, update the release notes
   with the actual hash.
   ```

   Omit empty category summaries from the PR body.

5. Return:

   - PR URL
   - New version and release date
   - Previous release boundary PR and commit
   - Total commits reviewed
   - Count in each release-note category

## Failure Handling

- If GitHub API or `gh` authentication fails, stop and report the exact command and error.
- If commit-to-PR association is ambiguous, inspect the commit and candidate PRs; do not guess.
- If a PR's category is genuinely ambiguous after reviewing its files and body, ask the user one focused question.
- If the worktree path or release branch already exists, do not delete it automatically.
- If commit, push, or PR creation fails, leave the worktree and branch intact and report the recovery command.
