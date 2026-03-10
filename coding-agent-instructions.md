# Coding Agent Instructions

This document explains how to use the AI coding agent artifacts generated for the prometheus-collector repository. These artifacts make AI assistants (GitHub Copilot, Google Jules, Gemini CLI, Cursor, etc.) understand your codebase deeply and contribute effectively.

## Quick Start

1. Open this repository in VS Code with Copilot enabled.
2. `copilot-instructions.md` loads automatically on every session — no action needed.
3. When you open a `.go` file, `go.instructions.md` auto-activates. Same for `.ts` and `Dockerfile`.
4. Invoke skills by typing trigger phrases in chat (e.g., "add test", "fix bug", "security review").
5. Invoke agents by @-mentioning them (e.g., `@CodeReviewer`, `@DocumentWriter`).

## Generated Artifacts Overview

| Artifact | Path | Loaded | Purpose |
|----------|------|--------|---------|
| `copilot-instructions.md` | `.github/copilot-instructions.md` | Auto every session | Root router — guidelines, skills catalogue, build instructions |
| `AGENTS.md` | Root | Auto (supported tools) | Setup commands, code style, testing, dev environment |
| `.instructions.md` files | `.github/instructions/` | Auto on file match | Language-specific coding rules (Go, TypeScript, Dockerfile) |
| `Prompt.md` | Root | On demand | Reusable task-spec template |
| Skill files (`SKILL.md`) | `.github/skills/` | On keyword trigger | Step-by-step guides for recurring tasks |
| `CodeReviewer.agent.md` | `.github/agents/` | On @-mention | Structured code review |
| `SecurityReviewer.agent.md` | `.github/agents/` | On @-mention | Deep STRIDE security analysis |
| `ThreatModelAnalyst.agent.md` | `.github/agents/` | On @-mention | Threat model with Mermaid diagrams |
| `DocumentWriter.agent.md` | `.github/agents/` | On @-mention | Documentation authoring |
| `prd.agent.md` | `.github/agents/` | On @-mention | PRD generation |
| `.vscode/mcp.json` | `.vscode/mcp.json` | Auto by VS Code | MCP server connections |

## How the Context Loading Chain Works

```
Layer 1: copilot-instructions.md (always loaded)
  ├── General rules, skill catalogue, build instructions
  ├── Routes to →
  │
Layer 2: .instructions.md files (auto-loaded on file match)
  ├── go.instructions.md → *.go files
  ├── typescript.instructions.md → *.ts files
  ├── dockerfile.instructions.md → Dockerfile*
  │
Layer 3: Skills (loaded on trigger phrase)
  └── Step-by-step procedures for specific tasks
```

You don't need to manually load anything — the system activates based on what you're editing and what you ask.

## Using Custom Agents

### @CodeReviewer
- **Invoke**: `@CodeReviewer` in Copilot Chat
- **What it does**: Reviews PRs for correctness, Go conventions, security (STRIDE), telemetry gaps, and multi-module consistency.
- **Example prompts**: `@CodeReviewer review this PR`, `@CodeReviewer check for security issues`

### @DocumentWriter
- **Invoke**: `@DocumentWriter` in Copilot Chat
- **What it does**: Creates documentation following repo conventions (ATX headings, fenced code blocks, table format).
- **Example prompts**: `@DocumentWriter write a README for this module`, `@DocumentWriter update the test documentation`

### @SecurityReviewer
- **Invoke**: `@SecurityReviewer` in Copilot Chat
- **What it does**: Deep STRIDE analysis, dependency audit, K8s security context review, credential scanning.
- **Example prompts**: `@SecurityReviewer review the Helm chart security`, `@SecurityReviewer audit container security`

### @ThreatModelAnalyst
- **Invoke**: `@ThreatModelAnalyst` in Copilot Chat
- **What it does**: Generates persistent threat model artifacts (Mermaid diagrams, STRIDE tables, threat catalogues) under `threat-model/YYYY-MM-DD/`.
- **Example prompts**: `@ThreatModelAnalyst perform a full threat model`, `@ThreatModelAnalyst analyze the metrics collection pipeline`

### @prd
- **Invoke**: `@prd` in Copilot Chat
- **What it does**: Generates PRDs tailored to this project's multi-component architecture.
- **Example prompts**: `@prd create a PRD for adding custom metric filtering`, `@prd write requirements for Windows ARM64 support`

## Using Skills

| Skill | Trigger Phrases | What It Does |
|-------|----------------|--------------|
| `security-review` | "security review", "STRIDE analysis", "credential check" | STRIDE-based security review with credential scanning |
| `telemetry-authoring` | "add telemetry", "add metrics", "instrument code" | Add telemetry following existing log/AI patterns |
| `fix-critical-vulnerabilities` | "fix CVE", "trivy fix", "patch vulnerability" | Fix CRITICAL/HIGH CVEs using Trivy |
| `dependency-update` | "update dependency", "bump package" | Safe Go module/npm dependency updates across 24 modules |
| `test-authoring` | "add test", "write test" | Create Ginkgo E2E tests with proper labels and utilities |
| `bug-fix` | "fix bug", "resolve issue", "hotfix" | Structured bug fix with regression test |
| `feature-development` | "add feature", "implement" | New feature scaffolding across components |
| `ci-cd-pipeline` | "update pipeline", "CI change" | GitHub Actions / Azure Pipelines modifications |
| `infrastructure` | "update helm", "change deployment" | Helm charts, Dockerfiles, IaC templates |

**Example**: Just describe the task naturally in chat:
```
"Add a test for the new config processing feature"
"Fix the critical CVE in the fluent-bit module"
"Update the Helm chart to add a new environment variable"
```

## Prompt Engineering Best Practices

### Structuring Effective Prompts
1. **Break complex tasks into smaller prompts** — One Go module at a time. This repo has 24 modules.
2. **Be specific** — Reference actual paths: `otelcollector/shared/configmap/mp/configmapparserforsettings.go`.
3. **Provide examples** — Show expected metric names, config formats, or test patterns from existing code.
4. **State constraints** — "Using Go 1.23, follow the existing `shared.GetEnv()` pattern for configuration."
5. **Ask for explanations** — Before accepting complex changes: "Explain how this interacts with the multi-module build."

### Prompting Anti-Patterns to Avoid
- **Vague requests** — "Fix this" without specifying component or deployment mode.
- **Overloaded prompts** — Don't ask for Helm chart + Go code + test changes in one prompt.
- **Assuming context** — If the AI gives generic Go advice, point it to `otelcollector/shared/` patterns.

## Choosing the Right Copilot Tool

| Task | Best Tool | Why |
|------|-----------|-----|
| Completing Go code as you type | **Inline suggestions** | Fastest for error handling, struct fields, imports |
| Questions about code, @agents | **Copilot Chat** | Conversational, supports @CodeReviewer and skills |
| Autonomous multi-file tasks | **Copilot CLI** | Terminal-native, `/plan` mode for multi-module changes |
| Reviewing PRs | **@CodeReviewer** | Knows this repo's conventions and review patterns |
| Async work on separate branches | **`/delegate`** | Documentation, refactoring, tangential tasks |

## Context Management

1. **Open relevant files** — Before prompting, open files from the specific Go module you're working on.
2. **Close unrelated files** — This repo spans 24 modules; close files from other modules to focus context.
3. **Start fresh for new tasks** — Switch to a new session when moving between components.
4. **Use @-references** — Reference specific files: `#file:otelcollector/shared/configmap/mp/`.

## Recommended Workflow: Explore → Plan → Code → Commit

1. **Explore** — "Read the OTel collector startup code and explain the initialization sequence"
2. **Plan** — "Plan how to add support for a new cloud environment endpoint"
3. **Code** — "Implement step 1: add the environment configuration in shared/"
4. **Test** — "Run `go test ./...` in the affected module"
5. **Commit** — Use Conventional Commits: `feat: add <cloud> environment support`

| Use this workflow for | Skip for |
|----------------------|----------|
| New features, multi-file refactoring | Quick bug fixes, single-file edits |
| Architecture changes, new components | Documentation-only updates |

## Validating AI-Generated Code

1. **Understand** — Read the code; ask for explanation if unclear.
2. **Build** — `go build ./...` in the affected module.
3. **Test** — `go test ./...` for unit tests; Ginkgo E2E for integration.
4. **Lint** — `go vet ./...` for static analysis.
5. **Security** — Check for hardcoded secrets, verify `.trivyignore`.
6. **Pattern match** — Compare against similar code in `otelcollector/shared/`.

## Test-Driven Development with AI

1. **Write tests first** — "Write a Ginkgo test for the new config validation feature"
2. **Review tests** — Ensure labels, utilities, and patterns match existing suites.
3. **Implement** — "Implement the code to make all tests pass"
4. **Refactor** — Clean up while keeping tests green.

## Codebase Onboarding with AI

- "How does the OTel collector start up and initialize its components?"
- "What's the pattern for adding a new Go shared library module?"
- "Explain the Helm chart structure and how addon/AKS/Arc variants differ"
- "Where are the Ginkgo E2E tests and how do test labels work?"
- "What environment variables does the collector need at runtime?"

## MCP Server Integration

The `.vscode/mcp.json` configures:
- **GitHub MCP**: PR management, issues, branch operations from chat.
- **Microsoft Docs MCP**: Search Azure Monitor, Kubernetes, and OpenTelemetry documentation.

MCP servers use `${input:variable}` prompts — you'll be asked for credentials on first use.

## Security When Using AI Assistants

- **Never commit secrets** — Verify AI code doesn't contain Application Insights keys, Azure credentials, or tokens.
- **Review all changes** — AI can produce code that looks correct but has subtle security flaws.
- **Don't share credentials** — Never paste secrets into chat. Use environment variables.
- **Run security tools** — After changes, run `trivy fs --severity CRITICAL,HIGH .` locally.

## Measuring AI-Assisted Productivity

- **Time from issue to PR** — Track how quickly tasks move with AI assistance.
- **Review iteration cycles** — Compare review rounds on AI-assisted vs. manual PRs.
- **Test coverage** — Whether AI-assisted development maintains Ginkgo E2E coverage.
- **Bug rate** — Track post-merge defects in AI-assisted code.

## Customizing These Artifacts

- **Add rules** to `.instructions.md` when new Go or TypeScript conventions are established.
- **Add skills** when you identify a new recurring workflow.
- **Update `copilot-instructions.md`** when project structure or build commands change.
- **Update `AGENTS.md`** when setup, test, or dev environment requirements change.
- **Re-run generation** periodically to refresh skills from new commit patterns.

## Troubleshooting

| Issue | Solution |
|-------|----------|
| AI doesn't follow Go conventions | Verify `go.instructions.md` `applyTo` matches `**/*.go` |
| Skill not activating | Use trigger phrases from the skills table above |
| Agent not available | Ensure `.agent.md` file is in `.github/agents/` |
| MCP not connecting | Check `.vscode/mcp.json` and provide GitHub token when prompted |
| AI gives generic advice | Open the relevant source file to give Copilot better context |
| Build commands fail | Check `AGENTS.md` Setup Commands against your Go/Node versions |
| Context feels stale | Start a new chat session to reset |
