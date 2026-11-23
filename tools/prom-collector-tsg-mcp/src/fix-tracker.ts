// Plans IcM discussion entries that track fix PRs from link to release. Read-only:
// the SRE agent supplies incidents (with their entries) and Ev2 rollouts (from ev2-mcp) and posts the planned entries itself.

// Ev2 definitions: push builds the image; aks/arc roll the extension out per cloud endpoint.
const DEFS = { push: 978, aks: 1196, arc: 1205 };
export const REPO = "Azure/prometheus-collector";
type Channel = "aks" | "arc";
// ponytail: release = the Ev2 Prod endpoint finishing (team decision); sovereign endpoints aren't tracked.
const isProd = (r: Rollout) => !/^(fairfax|mooncake|ussec|usnat|bleu|delos)$/i.test(r.endpoint?.trim() ?? "");
const PROGRESS = ["linked", "merged", "in-image", "rolling", "released"];

// findings: every CVE the incident lists (listedCves = that count); fixedBy = fix PR URLs it verified; fixed = already in a released image;
// notAffected = the shipped images don't contain the vulnerable package/OS build; evidence = the check behind the verdict.
export interface Finding { id: string; fixedBy?: string[]; fixed?: boolean; notAffected?: boolean; evidence?: string }
export interface Incident { id: string | number; title?: string; status?: string; tags?: string[]; entries?: string[]; channel?: Channel; findings?: Finding[]; listedCves?: number }

// A CVE incident is only linked, tagged or posted on once every listed CVE has a verified verdict.
export function unverified(i: Incident) {
  const all = i.findings ?? [], missing = all.filter((f) => !f.evidence?.trim()).map((f) => f.id);
  const ids = new Set(all.map((f) => f.id.trim().toUpperCase()));
  const cve = all.length > 0 || /CVE-\d{4}-\d+/i.test([i.title ?? "", ...(i.entries ?? [])].join(" "));
  if (cve && !i.listedCves) return `${i.id}: CVE incident without listedCves; verify every CVE before posting`;
  if (ids.size < all.length) return `${i.id}: duplicate CVE findings; send one finding per listed CVE`;
  if (ids.size < (i.listedCves ?? 0)) return `${i.id}: ${ids.size} of ${i.listedCves} listed CVEs have findings; verify every CVE before posting`;
  return missing.length ? `${i.id}: no verification evidence for ${missing.join(", ")}; verify every CVE before posting` : null;
}
export interface PullRequest { url: string; number: number; title: string; body?: string | null; branch?: string; state: "open" | "closed" | "merged"; mergeSha?: string | null }
export interface Match { incidentId: string; confidence: "linked" | "proposed"; evidence: string }
// Shaped from ev2-mcp get_rollout_summary (stages) and get_rollout_details for shell/PushLinuxImage (push). No log text.
export interface Rollout {
  definitionId: number; endpoint?: string; rolloutId: string; status?: string; startTime?: string; endTime?: string; sourceVersion?: string;
  artifactsVersion?: string; url?: string; stages?: { name: string; status: string }[]; push?: { status?: string; exitCode?: number | string };
}
export type TrackedRollout = Rollout & { includesFix: boolean };
export interface PlannedEntry { incidentId: string; prUrl: string; state: string; text: string }

// Short so it stays unobtrusive in customer-facing entries; markers() also reads the older [fix-tracker:<url>:<state>] form.
const short = (prUrl: string) => prUrl.replace(/^https:\/\/github\.com\/([^/]+\/[^/]+)\/pull\/(\d+).*$/i, "$1#$2");
export const marker = (prUrl: string, state: string) => `[ft:${short(prUrl)}:${state}]`;

export function markers(entries: string[] = []) {
  const found = new Map<string, Set<string>>();
  for (const m of entries.join("\n").matchAll(/\[(?:fix-tracker:(https:\/\/github\.com\/[^\]\s]+\/pull\/\d+)|ft:([\w.-]+\/[\w.-]+)#(\d+)):([a-z-]+)\]/gi)) {
    const url = (m[1] ?? `https://github.com/${m[2]}/pull/${m[3]}`).toLowerCase();
    if (!found.has(url)) found.set(url, new Set());
    const state = m[4].toLowerCase();
    found.get(url)!.add(state === "abandoned" ? "closed-unmerged" : state);
  }
  return found;
}

export const isReleaseBump = (title: string) => /\bversion bump\b|\brel(?:ease)?[ -]?notes\b/i.test(title);

export function extractEvidence(pr: PullRequest) {
  const text = `${pr.title}\n${pr.body ?? ""}\n${pr.branch ?? ""}`;
  const strong = new Set<string>(), weak = new Set<string>(), msrc = new Set<string>(), cve = new Set<string>();
  for (const m of text.matchAll(/microsofticm\.com\/imp\/v\d+\/incidents\/(?:details|incident)\/(\d{9,14})/gi)) strong.add(m[1]);
  for (const m of text.matchAll(/\b(?:icm|incident)s?\b[^\d\n]{0,20}(\d{9,14})\b/gi)) strong.add(m[1]);
  for (const m of text.matchAll(/\b(\d{9,14})\b/g)) if (!strong.has(m[1])) weak.add(m[1]);
  for (const m of text.matchAll(/\bmsrc[\s_#:-]*(?:case[\s#:]*)?(\d{5,7})\b/gi)) msrc.add(m[1]);
  for (const m of text.matchAll(/\bCVE-\d{4}-\d{4,}\b/gi)) cve.add(m[0].toUpperCase());
  return { strong, weak, msrc, cve };
}

// Auto-link only on an explicit IcM reference or a unique MSRC case; everything weaker is a proposal for a human.
export function matchIcms(pr: PullRequest, incidents: Incident[]): Match[] {
  const ev = extractEvidence(pr), matches: Match[] = [];
  const msrcHits = incidents.filter((i) => [...ev.msrc].some((c) => msrcCase(i.title) === c));
  for (const i of incidents) {
    const id = String(i.id), cves = [...ev.cve].filter((c) => (i.title ?? "").toUpperCase().includes(c));
    if (ev.strong.has(id)) matches.push({ incidentId: id, confidence: "linked", evidence: `PR references IcM ${id}` });
    else if (msrcHits.includes(i)) matches.push({ incidentId: id, confidence: msrcHits.length === 1 ? "linked" : "proposed",
      evidence: `PR references MSRC case ${msrcCase(i.title)}${msrcHits.length > 1 ? ` (matches ${msrcHits.length} IcMs)` : ""}` });
    else if (ev.weak.has(id)) matches.push({ incidentId: id, confidence: "proposed", evidence: `PR contains the number ${id} without an IcM keyword` });
    else if (cves.length) matches.push({ incidentId: id, confidence: "proposed", evidence: `PR and IcM title share ${cves.join(", ")}` });
  }
  return matches;
}

const msrcCase = (title = "") => /\bMSRC\]?\s*\[?\s*(\d{5,7})\b/i.exec(title)?.[1];

const ref = (r: Rollout) => r.url ?? `Ev2 Prod rollout ${r.rolloutId}`;
const ok = (s?: string) => /^succeeded$/i.test(s ?? "");
const median = (xs: number[]) => xs.sort((a, b) => a - b)[Math.floor(xs.length / 2)];

// Full Prod stage list per channel from the Ev2 stage map, not from any one rollout (a region Select drops stages).
export type ProdStages = Partial<Record<Channel, string[]>>;
// Per channel from Prod Ev2 stage summaries. Released = Prod rollouts containing the fix together have Succeeded every
// stage-map stage and no later rollout dropped it. Missing evidence is reported as unknown, never as done. This is not fleet adoption.
export function releaseState(rollouts: TrackedRollout[], channels: Channel[] = ["aks", "arc"], prodStages: ProdStages = {}) {
  let done = 0, started = false, back = false, required = 0;
  const notes: string[] = [];
  for (const channel of channels) {
    required++;
    const label = channel.toUpperCase();
    const rs = rollouts.filter((r) => r.definitionId === DEFS[channel] && isProd(r))
      .sort((a, b) => Date.parse(b.startTime ?? "") - Date.parse(a.startTime ?? ""));
    const latest = rs[0], idx = rs.findIndex((r) => r.includesFix);
    const complete = (r: Rollout) => !!r.stages?.length && r.stages.every((s) => ok(s.status));
    const need = (prodStages[channel] ?? []).map((s) => s.toLowerCase());
    const succeeded = new Set(rs.filter((r) => r.includesFix).flatMap((r) => r.stages ?? []).filter((s) => ok(s.status)).map((s) => s.name.toLowerCase()));
    const shipped = need.length > 0 && need.every((s) => succeeded.has(s))
      && rs.find((r) => r.includesFix && r.stages?.some((s) => ok(s.status) && need.includes(s.name.toLowerCase())));
    if (!latest) notes.push(`${label}: release status unknown`);
    else if (idx > 0) { back = true; notes.push(`${label}: newer rollout ${ref(latest)} does not include the fix`); }
    else if (shipped) { done++; notes.push(`${label}: completed in Prod (${ref(shipped)})`); }
    else if (idx === 0) {
      started = true;
      // ETA = start + median duration of earlier completed Prod rollouts of this definition: an estimate, not a promise.
      const took = rs.slice(1).filter((r) => complete(r) && r.startTime && r.endTime).map((r) => Date.parse(r.endTime!) - Date.parse(r.startTime!));
      const eta = took.length && latest.startTime ? new Date(Date.parse(latest.startTime) + median(took)).toISOString().slice(0, 10) : null;
      notes.push(`${label}: rolling out (${ref(latest)}), ${eta ? `estimated completion ${eta}` : "ETA unknown"}`);
    } else notes.push(`${label}: not started`);
  }
  const state = back ? "rolled-back" : required && done === required ? "released" : started || done ? "rolling" : null;
  return { state, notes };
}

export interface Lifecycle { state: "open" | "closed-unmerged" | "merged" | "in-image"; image?: string; imageRef?: string; pushChecked?: boolean; rollouts: TrackedRollout[] }

// Customer-readable: what is fixed, the rollout link and ETA. Stage internals stay in Ev2.
const DETAIL: Record<string, (fix: string, l: Lifecycle | undefined, notes: string[]) => string> = {
  linked: (fix) => `Fix in progress: ${fix}.`,
  merged: (fix, l) => `Fix merged: ${fix}. ${l?.pushChecked ? "It will ship in the next agent image." : "Image and rollout status not known yet."}`,
  "in-image": (fix, l, n) => `Fix is built into agent image ${l?.image ?? "(tag unknown)"}: ${fix}. ${n.join("; ") || "Rollout not started"}.`,
  rolling: (fix, _, n) => `Fix is rolling out: ${fix}. ${n.join("; ")}.`,
  released: (fix, _, n) => `Fix released to Prod: ${fix}. ${n.join("; ")}.`,
  "rolled-back": (fix, _, n) => `Fix was rolled back: ${fix}. ${n.join("; ")}. We are reviewing.`,
  "closed-unmerged": (fix) => `Fix PR was closed without merging: ${fix}. We are reviewing next steps.`,
};

const list = (ids: string[]) => ids.length > 5 ? `${ids.slice(0, 5).join(", ")} and ${ids.length - 5} more` : ids.join(", ");
const fixes = (i: Incident, key: string) => (i.findings ?? []).filter((f) => f.fixedBy?.some((u) => u.toLowerCase() === key)).map((f) => f.id);
// Findings with neither a verified fix PR nor an already-released fix: candidates for a new fix PR.
export const unfixed = (i: Incident) => (i.findings ?? []).filter((f) => !f.fixed && !f.notAffected && !f.fixedBy?.length).map((f) => f.id);

// Coverage combines all fix PRs; "released" only invites resolving when every finding has a released fix.
function coverage(i: Incident, key: string, state: string, releasedPrs: Set<string>) {
  const all = i.findings ?? [];
  if (!all.length) return state === "released" ? " Please verify and resolve." : "";
  const shipped = (f: Finding) => f.fixed || f.notAffected || !!f.fixedBy?.some((u) => releasedPrs.has(u.toLowerCase()));
  const mine = fixes(i, key), open = unfixed(i), na = all.filter((f) => f.notAffected && !mine.includes(f.id)).length;
  const done = all.filter((f) => shipped(f) && !f.notAffected && !mine.includes(f.id)).length;
  const others = all.length - mine.length - open.length - done - na;
  let text = ` Fixes ${mine.length} of ${all.length} listed CVEs${mine.length ? ` (${list(mine)})` : ""}.`;
  if (done) text += ` ${done} already fixed in released images.`;
  if (na) text += ` ${na} do not affect the agent images.`;
  if (others) text += ` ${others} covered by other fix PRs.`;
  if (open.length) text += ` Still open, no fix yet: ${list(open)}.`;
  if (state !== "released") return text;
  return text + (all.some((f) => !shipped(f)) ? " The incident stays open until the remaining CVEs are fixed and released." : " Every listed CVE now has a released fix; please verify and resolve.");
}

// A PR fully released (every stage-map stage) before the newest Prod rollout, which still contains it, is in the image the agent
// found vulnerable, so it didn't fix the CVE: drop it from fixedBy so it is neither proposed, counted, nor hides needsFixPr.
export function withoutStale(incidents: Incident[], lcs: Map<string, Lifecycle>, prodStages: ProdStages = {}) {
  const shipped = (l: Lifecycle) => (["aks", "arc"] as Channel[]).some((c) => {
    const need = (prodStages[c] ?? []).map((s) => s.toLowerCase());
    const rs = l.rollouts.filter((r) => r.definitionId === DEFS[c] && isProd(r)).sort((a, b) => Date.parse(b.startTime ?? "") - Date.parse(a.startTime ?? ""));
    if (!need.length || rs.length < 2 || !rs[0].includesFix) return false;
    const done = new Set(rs.slice(1).filter((r) => r.includesFix).flatMap((r) => r.stages ?? []).filter((s) => ok(s.status)).map((s) => s.name.toLowerCase()));
    return need.every((s) => done.has(s));
  });
  const stale = new Set([...lcs].filter(([, l]) => shipped(l)).map(([u]) => u));
  return incidents.map((i) => ({ ...i, findings: i.findings?.map((f) => f.fixed ? f : { ...f, fixedBy: f.fixedBy?.filter((u) => !stale.has(u.toLowerCase())) }) }));
}

export function planEntries(prs: PullRequest[], incidents: Incident[], lifecycles: Map<string, Lifecycle>, prodStages: ProdStages = {}): PlannedEntry[] {
  incidents = withoutStale(incidents, lifecycles, prodStages);
  const out: PlannedEntry[] = [], active = incidents.filter((i) => !/resolved/i.test(i.status ?? ""));
  // Match against all active IcMs at once so an MSRC case shared by several IcMs stays a proposal.
  // Release-bump notes quote other PRs' IcM references; only a human link can attach one.
  const matches = new Map(prs.map((pr) => [pr.url.toLowerCase(), isReleaseBump(pr.title) ? [] : matchIcms(pr, active)]));
  for (const incident of active) {
    if (unverified(incident)) continue;
    const id = String(incident.id), seen = markers(incident.entries);
    const releasedPrs = new Set([...seen].filter(([, s]) => s.has("released")).map(([u]) => u));
    // A fix PR that Ev2 shows released counts even if it was never linked here, so its CVEs aren't listed as open again.
    for (const [u, l] of lifecycles) if (l.state === "in-image" && releaseState(l.rollouts, incident.channel ? [incident.channel] : undefined, prodStages).state === "released") releasedPrs.add(u);
    // Rendered after every PR is planned, so PRs released in the same run count as released for each other.
    const render: (() => PlannedEntry)[] = [];
    const tagged = new Set((incident.tags ?? []).map((t) => /^Fix PR:\s*(\S+)/i.exec(t)?.[1]?.toLowerCase()).filter(Boolean));
    for (const pr of prs) {
      const key = pr.url.toLowerCase(), posted = seen.get(key) ?? new Set<string>();
      if (posted.has("rejected")) continue;
      const covers = fixes(incident, key);
      // A PR that fixes none of the incident's listed CVEs is not its fix, whatever its text says; humans can still link it.
      if (incident.findings?.length && !covers.length && !posted.has("linked") && !tagged.has(key)) continue;
      const match = matches.get(key)!.find((m) => m.incidentId === id)
        ?? (covers.length ? { incidentId: id, confidence: "proposed" as const, evidence: `it updates the packages for ${list(covers)}` } : undefined);
      // Brackets stripped so a PR title cannot smuggle a marker into a posted entry.
      const title = pr.title.replace(/[[\]]/g, "");
      const linked = posted.has("linked") || tagged.has(key) || match?.confidence === "linked";
      const fix = `${title} (${pr.url})`, lc = lifecycles.get(key);
      const add = (state: string, notes: string[] = []) => {
        if (state === "released") releasedPrs.add(key);
        render.push(() => ({ incidentId: id, prUrl: pr.url, state,
          text: `${DETAIL[state](fix, lc, notes)}${/closed|rolled/.test(state) ? "" : coverage(incident, key, state, releasedPrs)} ${marker(pr.url, state)}` }));
      };
      if (!linked) {
        // Only the IcM link is unconfirmed; the PR may already be merged or rolling, so say where it stands.
        const where = lc?.state === "in-image"
          ? ` The PR is already merged and built into agent image ${lc.image ?? "(tag unknown)"}; ${releaseState(lc.rollouts, incident.channel ? [incident.channel] : undefined, prodStages).notes.join("; ") || "rollout not started"}.`
          : pr.state === "merged" ? " The PR is already merged." : pr.state === "open" ? " The PR is still open." : "";
        if (match && !posted.has("proposed") && lc?.state !== "closed-unmerged")
          render.push(() => ({ incidentId: id, prUrl: pr.url, state: "proposed", text: `Possible fix: ${fix}, because ${match.evidence}.${where}${coverage(incident, key, "proposed", releasedPrs)} Not yet confirmed as this incident's fix. ` +
            `To confirm, add the tag "Fix PR: ${pr.url}"; to reject, post an entry containing ft:${short(pr.url)}:rejected in square brackets. ${marker(pr.url, "proposed")}` }));
        continue;
      }
      if (!posted.has("linked") && !tagged.has(key)) {
        if (lc?.state === "closed-unmerged") continue;
        add("linked");
      }
      if (!lc || lc.state === "open") continue;
      if (lc.state === "closed-unmerged") { if (!posted.has(lc.state)) add(lc.state); continue; }
      const rel = lc.state === "in-image" ? releaseState(lc.rollouts, incident.channel ? [incident.channel] : undefined, prodStages) : { state: null, notes: [] };
      const state = rel.state ?? lc.state;
      if (state === "rolled-back") { if (!posted.has(state)) add(state, rel.notes); continue; }
      const top = Math.max(...[...posted].map((s) => PROGRESS.indexOf(s)), 0);
      if (PROGRESS.indexOf(state) > top) add(state, rel.notes);
    }
    out.push(...render.map((r) => r()));
  }
  return out;
}

// ---- I/O (GitHub REST). Injectable for the dry run; tests cover the pure functions above.

type Get = (url: string) => Promise<any>;

export function githubGet(token = process.env.GITHUB_TOKEN): Get {
  return async (path) => {
    const res = await fetch(`https://api.github.com/${path}`, { headers: {
      Accept: "application/vnd.github+json", "User-Agent": "prom-collector-tsg-mcp", ...(token ? { Authorization: `Bearer ${token}` } : {}) } });
    if (!res.ok) throw new Error(`GitHub ${res.status}: ${path}`);
    return res.json();
  };
}

const toPr = (p: any): PullRequest => ({ url: p.html_url, number: p.number, title: p.title, body: p.body, branch: p.head?.ref,
  state: p.merged_at ? "merged" : p.state, mergeSha: p.merged_at ? p.merge_commit_sha : null });

export async function recentPrs(gh: Get, since: Date) {
  const prs: PullRequest[] = [];
  for (let page = 1; page <= 10; page++) {
    const batch = await gh(`repos/${REPO}/pulls?state=all&sort=updated&direction=desc&per_page=100&page=${page}`);
    for (const p of batch) if (Date.parse(p.created_at) >= +since || Date.parse(p.merged_at) >= +since) prs.push(toPr(p));
    if (batch.length < 100 || Date.parse(batch.at(-1).updated_at) < +since) break;
  }
  return prs;
}

// Annotates every rollout with whether its SourceVersion contains the PR's merge commit (GitHub compare).
export async function lifecycles(prs: PullRequest[], gh: Get, rollouts: Rollout[], track?: Set<string>) {
  const push = DEFS.push;
  const result = new Map<string, Lifecycle>(), errors: string[] = [], cache = new Map<string, Promise<boolean>>();
  for (const p of prs) if (p.state !== "merged") result.set(p.url.toLowerCase(), { state: p.state === "open" ? "open" : "closed-unmerged", rollouts: [] });
  const contains = (fix: string, sha: string) => {
    const k = `${fix}...${sha}`;
    if (!cache.has(k)) cache.set(k, gh(`repos/${REPO}/compare/${k}`).then((c) => ["ahead", "identical"].includes(c.status)));
    return cache.get(k)!;
  };
  // Hotfix pushes build a tag SHA that differs from SourceVersion; the image tag is what actually shipped.
  const commit = (r: Rollout) => (r.definitionId === push && r.artifactsVersion?.match(/-([0-9a-f]{7,40})$/i)?.[1]) || r.sourceVersion;
  const known = rollouts.filter(commit);
  if (known.length < rollouts.length) errors.push(`${rollouts.length - known.length} rollout(s) without sourceVersion ignored (unknown)`);
  const shas = [...new Set(known.map((r) => commit(r)!))];
  for (const p of prs.filter((x) => x.state === "merged" && x.mergeSha && (!track || track.has(x.url.toLowerCase())))) {
    try {
      const has = new Map(await Promise.all(shas.map(async (s) => [s, await contains(p.mergeSha!, s)] as const)));
      const tracked = known.map((r) => ({ ...r, includesFix: has.get(commit(r)!)! }));
      const pushes = tracked.filter((r) => r.definitionId === push);
      // In image only on the push action's own result; the overall 978 rollout status covers more than the push.
      const pushed = pushes.find((r) => r.includesFix && ok(r.push?.status) && /^(?:exitcode#)?0$/i.test(String(r.push?.exitCode)));
      for (const r of pushes) if (r.includesFix && !r.push) errors.push(`${p.url}: push rollout ${r.rolloutId} contains the fix but has no push action result (unknown)`);
      result.set(p.url.toLowerCase(), { state: pushed ? "in-image" : "merged", image: pushed?.artifactsVersion, imageRef: pushed && ref(pushed),
        pushChecked: pushes.some((r) => r.push), rollouts: tracked });
    } catch (e: any) { errors.push(`${p.url}: ${e.message}`); }
  }
  return { result, errors };
}
export async function planFixTracker(incidents: Incident[], lookbackHours = 24, rollouts: Rollout[] = [], prodStages: ProdStages = {}, gh = githubGet()) {
  const since = new Date(Date.now() - lookbackHours * 3600e3);
  const scanned = await recentPrs(gh, since);
  const tracked = new Set<string>();
  for (const i of incidents) {
    for (const url of markers(i.entries).keys()) tracked.add(url);
    for (const t of i.tags ?? []) { const u = /^Fix PR:\s*(\S+)/i.exec(t)?.[1]; if (u) tracked.add(u.toLowerCase()); }
    for (const f of i.findings ?? []) for (const u of f.fixedBy ?? []) tracked.add(u.toLowerCase());
  }
  const prs = new Map(scanned.map((p) => [p.url.toLowerCase(), p]));
  const prefix = `https://github.com/${REPO.toLowerCase()}/pull/`, fetchErrors: string[] = [];
  for (const url of tracked) if (url.startsWith(prefix) && !prs.has(url)) {
    try { prs.set(url, toPr(await gh(`repos/${REPO}/pulls/${url.slice(prefix.length)}`))); }
    catch (e: any) { fetchErrors.push(`${url}: ${e.message}`); }
  }
  const all = [...prs.values()], active = incidents.filter((i) => !/resolved/i.test(i.status ?? ""));
  // Only linked PRs need Ev2 ancestry; proposals and unrelated PRs would just burn GitHub compare calls.
  const linked = new Set([...tracked, ...all.filter((p) => !isReleaseBump(p.title) && matchIcms(p, active).some((m) => m.confidence === "linked"))
    .map((p) => p.url.toLowerCase())]);
  const { result, errors } = await lifecycles(all, gh, rollouts, linked);
  return {
    since: since.toISOString(), scanned: scanned.length,
    excludedReleaseBumps: scanned.filter((p) => isReleaseBump(p.title)).map((p) => p.url),
    entries: planEntries(all, incidents, result, prodStages),
    errors: [...active.map(unverified).filter((e): e is string => !!e), ...fetchErrors, ...errors,
      ...(["aks", "arc"] as const).filter((c) => !prodStages[c]?.length).map((c) => `prodStages.${c} missing: nothing on ${c} can count as released without the Ev2 Prod stage map`)],
    needsFixPr: withoutStale(active, result, prodStages).filter((i) => !unverified(i)).map((i) => ({ incidentId: String(i.id), cves: unfixed(i) })).filter((n) => n.cves.length),
  };
}
