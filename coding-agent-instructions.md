# Coding Agent Instructions

This document explains how to use the AI coding agent artifacts generated for this repository. These artifacts make AI assistants (GitHub Copilot, Google Jules, Gemini CLI, Cursor, etc.) understand your codebase deeply and contribute effectively.

## Quick Start

1. Open this repository in VS Code (or your preferred editor with Copilot/AI assistant support).
2. The AI assistant automatically loads `copilot-instructions.md` on every session — no action needed.
3. When you open a file matching a language pattern, the corresponding `.instructions.md` file auto-activates.
4. Invoke skills by typing their trigger phrases in chat (e.g., "add test", "fix bug", "security review").
5. Invoke agents by @-mentioning them in chat (e.g., `@CodeReviewer`, `@DocumentWriter`).

## Generated Artifacts Overview

| Artifact | Path | Loaded | Purpose |
|----------|------|--------|---------|
| `copilot-instructions.md` | `.github/copilot-instructions.md` | Automatically every session | Root router — general rules, build instructions, gotchas |
| `AGENTS.md` | `AGENTS.md` | Automatically (supported tools) | Setup commands, code style, testing, dev tips, PR instructions |
| Go instructions | `.github/instructions/go.instructions.md` | Auto on `**/*.go` | Go coding conventions for this repo |
| TypeScript instructions | `.github/instructions/typescript.instructions.md` | Auto on `**/*.ts` | TypeScript conventions for az-prom-rules-converter |
| Dockerfile instructions | `.github/instructions/dockerfile.instructions.md` | Auto on `**/Dockerfile*` | Container build conventions |
| Kubernetes instructions | `.github/instructions/kubernetes.instructions.md` | Auto on `**/deploy/**/*.yaml` | Helm/K8s conventions |
| `Prompt.md` | `Prompt.md` | On demand | Reusable task-spec template |
| `CodeReviewer.agent.md` | `.github/agents/CodeReviewer.agent.md` | On @-mention | Structured code review |
| `SecurityReviewer.agent.md` | `.github/agents/SecurityReviewer.agent.md` | On @-mention | Deep security analysis |
| `ThreatModelAnalyst.agent.md` | `.github/agents/ThreatModelAnalyst.agent.md` | On @-mention | STRIDE threat modeling with Mermaid diagrams |
| `DocumentWriter.agent.md` | `.github/agents/DocumentWriter.agent.md` | On @-mention | Documentation authoring |
| `prd.agent.md` | `.github/agents/prd.agent.md` | On @-mention | PRD generation |
| `.vscode/mcp.json` | `.vscode/mcp.json` | Automatically by VS Code | MCP server connections |

## How the Context Loading Chain Works

```
Layer 1: copilot-instructions.md (always loaded)
  ├── General rules, build instructions, known gotchas
  ├── Routes to →
  │
Layer 2: .instructions.md files (auto-loaded when you open matching files)
  ├── Go conventions (on *.go files)
  ├── TypeScript conventions (on *.ts files)
  ├── Dockerfile conventions (on Dockerfile*)
  ├── Kubernetes conventions (on deploy/**/*.yaml)
  │
Layer 3: Skills (loaded only when invoked by trigger phrase)
  └── Step-by-step procedures for specific tasks
```

## Using Custom Agents

### @CodeReviewer
- **Invoke:** Type `@CodeReviewer` in Copilot Chat.
- **What it does:** Performs structured code reviews covering Go conventions, STRIDE security, telemetry gaps, and PR checklist compliance.
- **Example prompts:**
  - `@CodeReviewer review this PR`
  - `@CodeReviewer check this file for security issues`
  - `@CodeReviewer review my changes for telemetry gaps`

### @SecurityReviewer
- **Invoke:** Type `@SecurityReviewer` in Copilot Chat.
- **What it does:** Deep security assessment — threat modeling, attack surface analysis, dependency audit, container/Kubernetes security review.
- **Example prompts:**
  - `@SecurityReviewer perform a threat model for the target allocator`
  - `@SecurityReviewer review the RBAC changes in this PR`
  - `@SecurityReviewer audit our container security configuration`

### @ThreatModelAnalyst
- **Invoke:** Type `@ThreatModelAnalyst` in Copilot Chat.
- **What it does:** Generates persistent threat model artifacts under `threat-model/YYYY-MM-DD/` with Mermaid diagrams and STRIDE analysis.
- **Example prompts:**
  - `@ThreatModelAnalyst perform a full threat model analysis`
  - `@ThreatModelAnalyst threat model the metric collection pipeline`

### @DocumentWriter
- **Invoke:** Type `@DocumentWriter` in Copilot Chat.
- **What it does:** Creates and maintains documentation following this repo's conventions.
- **Example prompts:**
  - `@DocumentWriter update the release notes for this version`
  - `@DocumentWriter write a README for this new module`

### @prd (PRD Generator)
- **Invoke:** Type `@prd` in Copilot Chat.
- **What it does:** Generates structured Product Requirements Documents tailored to prometheus-collector's architecture.
- **Example prompts:**
  - `@prd create a PRD for adding a new metrics receiver`
  - `@prd write requirements for Windows ARM64 support`

## Using Skills

Skills are step-by-step guides that activate when you use their trigger phrases. Just describe what you want to do:

### Always-Available Skills

| Skill | Trigger Phrases | What It Does |
|-------|----------------|--------------|
| `security-review` | "security review", "STRIDE analysis", "credential leak check" | STRIDE-based security review with Go-specific patterns |
| `telemetry-authoring` | "add telemetry", "add metrics", "instrument code" | Guides adding telemetry following existing patterns |
| `fix-critical-vulnerabilities` | "fix critical vulnerability", "CVE fix", "trivy fix" | Identifies and fixes critical/high vulnerabilities using Trivy |

### Commit-History-Driven Skills

| Skill | Trigger Phrases | What It Does |
|-------|----------------|--------------|
| `dependency-update` | "update dependency", "bump package" | Safe dependency updates across 23+ Go modules |
| `test-authoring` | "add test", "write test", "add e2e test" | Creates tests following Ginkgo E2E and Go unit test conventions |
| `bug-fix` | "fix bug", "resolve issue", "hotfix" | Structured bug fix with regression test requirement |
| `feature-development` | "add feature", "implement", "new component" | New feature scaffolding with config, chart, and test updates |
| `ci-cd-pipeline` | "modify CI", "update pipeline", "fix workflow" | GitHub Actions and Azure Pipelines modifications |
| `infrastructure` | "update Helm chart", "modify Dockerfile", "change manifest" | Kubernetes, Helm, Bicep, Terraform changes |

**Example usage:**
```
# In Copilot Chat, just describe the task naturally:
"Add a test for the new DCGM exporter scrape configuration"
"Fix the critical CVE in our container base image"
"Update the kube-state-metrics dependency to the latest version"
```

## MCP Server Integration

The `.vscode/mcp.json` file configures connections to external data sources:
- **GitHub MCP:** Enables PR creation, issue management, and branch operations from chat.
- **Microsoft Docs MCP:** Enables the @CodeReviewer to validate code against official Azure documentation.

Secrets use `${input:variable}` prompts — you'll be asked for credentials on first use.

## Tips for Maximum Productivity

1. **Let auto-loading work for you** — Just open the file you're working on. The `.instructions.md` files activate automatically.
2. **Use natural language for skills** — Say "add a test" or "bump dependencies", not skill file names.
3. **Start reviews with @CodeReviewer** — It knows the PR template checklist, STRIDE security checks, and telemetry expectations.
4. **Use @prd before big features** — A structured PRD ensures complete implementation (config, charts, tests, telemetry).
5. **Check AGENTS.md for setup** — If builds fail, verify the Setup Commands match your environment.

## Customizing These Artifacts

These files evolve with the project:
- **Add rules** to `.instructions.md` files when establishing new conventions.
- **Add skills** when identifying new recurring workflows (create `SKILL.md` in `.github/skills/`).
- **Update `copilot-instructions.md`** when build commands or project structure changes.
- **Update `AGENTS.md`** when setup, testing, or dev environment requirements change.

## Troubleshooting

| Issue | Solution |
|-------|----------|
| AI doesn't follow Go conventions | Verify `.github/instructions/go.instructions.md` exists and `applyTo` matches |
| Skill not activating | Use exact trigger phrases from the skills table above |
| Agent not available | Ensure `.github/agents/<name>.agent.md` exists |
| MCP server not connecting | Check `.vscode/mcp.json` and provide GitHub token when prompted |
| Build commands fail | Update Setup Commands in `AGENTS.md` |
