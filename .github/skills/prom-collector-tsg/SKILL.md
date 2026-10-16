---
name: prom-collector-tsg
description: >
  Answer Azure Managed Prometheus (prometheus-collector / AMA Metrics addon)
  support questions using troubleshooting guides, and investigate incidents
  with authorized diagnostic tools. Use for ICMs, customer escalations, missing
  metrics, scrape configuration, pod crashes, OOM, high CPU, token/auth errors,
  DCR/DCE, MDSD/ME, target allocator, control plane metrics, proxy bypass, OTel,
  private link/AMPLS, DNS, liveness failures, sample drops, cardinality, cost,
  and vulnerability/CVE incidents. Not for builds, EV2 artifacts, or load testing.
argument-hint: 'Provide a support question, ICM number, or cluster ARM resource ID.'
---

# Azure Managed Prometheus Troubleshooting

Choose reference mode for support questions and investigation mode for incidents.
Load only the companion guides needed for the task.

## Support questions

Read the matching guide from the TSG index before answering; follow related TSG
links when the question spans categories. No incident number, cluster ID, or
local MCP tools are required. For proxy + AMPLS, read both the proxy and
firewall/private-link TSGs.

Cite the TSG and the specific public product documentation relevant to the
question. Distinguish configuration guidance from a verified customer diagnosis.

## Incident investigations

Read [investigation.md](investigation.md) before starting. It contains incident
context collection, authored-summary and attachment retrieval, tool selection,
diagnostic patterns, evidence reporting, internal research, and escalation paths.

1. Gather the authored incident summary, discussion, attachments, and impact window.
   AI summaries are not substitutes for authored evidence.
2. For cluster-scoped incidents, obtain the actual cluster ARM ID before triage;
   do not use the node resource group. Check addon/component versions first.
   Scanner-only CVEs instead start with the exact scanned artifact and
   [the vulnerability TSG](tsgs/vulnerabilities.md); no cluster ID is required.
3. Query the incident timeframe, starting narrowly and widening as needed.
   Use the available diagnostic tools; read [reference.md](reference.md) for
   ad-hoc KQL, data sources, historical windows, versions, and ME deep-dives.
4. Follow the symptom TSG. For partial metric gaps, check node drains/evictions
   before attributing the issue to scraping or capacity.
5. Report evidence, root cause or remaining uncertainty, recommended actions,
   relevant documentation, and the verified escalation path.

Diagnostic connectors are configured separately from plugin installation. Report
missing access explicitly; do not invent tool results. For local setup, use
[the troubleshooting setup skill](../troubleshooting-setup/SKILL.md).

## Guardrails

- Use authoritative code and product documentation as the source of truth for
  behavior and support claims, not TSG summaries or AI answers alone. Read the
  relevant sources and cite documentation links and code paths/lines at the
  applicable version or commit as evidence. Use telemetry for customer-state
  claims; flag conflicts or gaps rather than silently choosing one source.
- State only what evidence supports. Do not infer ownership, intent, connectivity,
  or successful ingestion from names, cached configuration, or historical metrics.
- Internal references are for authorized investigation only. Do not expose
  customer identifiers, incident contents, or internal-only links in customer answers.
- Do not mutate clusters or Azure resources merely to answer a support question.
  Obtain approval for remediation and repository changes.
- Generalize reusable findings; never add customer-specific data to guides or tools.
  Mark unconfirmed causes as hypotheses.

## Loading companion files

Use the packaged files first. If unavailable, retrieve them using the agent's
GitHub connector from `Azure/prometheus-collector` at the installed plugin's
source commit, under `.github/skills/prom-collector-tsg/<relative-path>`.
Without that connector, use:

```text
https://raw.githubusercontent.com/Azure/prometheus-collector/<source-commit>/.github/skills/prom-collector-tsg/<relative-path>
```

This applies to `investigation.md`, `reference.md`, and every TSG below. Do not
silently substitute another branch or claim to have read an inaccessible file.

## TSG index

| Symptom | Guide |
|---------|-------|
| Pod restarts / OOM / resource pressure | [pod-restarts-oom](tsgs/pod-restarts-oom.md) |
| Missing or partial metrics | [missing-metrics](tsgs/missing-metrics.md) |
| Ingestion spikes / cardinality | [spike-in-metrics](tsgs/spike-in-metrics.md) |
| Firewall / network / private link / AMPLS | [firewall-network-private-link](tsgs/firewall-network-private-link.md) |
| Control plane metrics | [control-plane-metrics](tsgs/control-plane-metrics.md) |
| Windows pod restarts | [windows-pod-restarts](tsgs/windows-pod-restarts.md) |
| Remote write | [remote-write](tsgs/remote-write.md) |
| Vulnerabilities / CVEs | [vulnerabilities](tsgs/vulnerabilities.md) |
| Node exporter labels on ARM64 | [node-exporter-arm64](tsgs/node-exporter-arm64.md) |
| Pods not created / addon not deploying | [pods-not-created](tsgs/pods-not-created.md) |
| Proxy / authenticated proxy / OTel bypass | [proxy-authenticated](tsgs/proxy-authenticated.md) |
| Liveness probe 503 | [liveness-probe-503](tsgs/liveness-probe-503.md) |
| Duplicate KSM labels | [duplicate-labels-ksm](tsgs/duplicate-labels-ksm.md) |
| Missing or wrong-region DCR/DCE | [dcr-dce-region-mismatch](tsgs/dcr-dce-region-mismatch.md) |
| AMW usage / cost optimization | [amw-usage-optimization](tsgs/amw-usage-optimization.md) |
| Known issues / expected behavior | [known-issues-faq](tsgs/known-issues-faq.md) |
