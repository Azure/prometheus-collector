import { test } from "node:test";
import assert from "node:assert/strict";
import { matchIcms, isReleaseBump, markers, planEntries, releaseState, lifecycles, unverified, withoutStale } from "../dist/fix-tracker.js";

const pr = (over) => ({ url: "https://github.com/Azure/prometheus-collector/pull/1", number: 1, title: "fix", body: "", branch: "b", state: "open", ...over });
// Incidents named after CVEs default to one verified finding per title CVE, fixed by the default PR.
const icm = (id, title = "", over = {}) => {
  const findings = ("findings" in over ? over.findings : title.match(/CVE-\d{4}-\d+/g)?.map((c) => ({ id: c, fixedBy: [pr().url] })))?.map((f) => ({ evidence: "go version -m", ...f }));
  return { id, title, status: "ACTIVE", entries: [], listedCves: findings?.length, ...over, findings };
};

test("release-bump PRs are excluded from matching but still tracked when a human links them", () => {
  assert.ok(isReleaseBump("version bump and rel notes for 6.24.0"));
  assert.ok(!isReleaseBump("fix: ksm release lock"));
  const bump = pr({ title: "Version bump 6.24.0", body: "- fix CVE-2026-1234" });
  assert.deepEqual(planEntries([bump], [icm(1, "CVE-2026-1234", { findings: [{ id: "CVE-2026-1234" }] })], new Map()), []);
  const confirmed = icm(1, "", { entries: [`[fix-tracker:${bump.url}:linked]`] });
  const lc = new Map([[bump.url.toLowerCase(), { state: "merged", rollouts: [] }]]);
  assert.deepEqual(planEntries([bump], [confirmed], lc).map((e) => e.state), ["merged"]);
});

test("markers make planning idempotent; rejected and proposed suppress repeats", () => {
  const p = pr({ state: "merged" });
  const lc = new Map([[p.url.toLowerCase(), { state: "merged", rollouts: [] }]]);
  const linked = icm(1, "", { entries: [`[fix-tracker:${p.url}:linked]`] });
  const first = planEntries([p], [linked], lc);
  assert.deepEqual(first.map((e) => e.state), ["merged"]);
  assert.ok(first[0].text.endsWith("[ft:Azure/prometheus-collector#1:merged]"));
  assert.deepEqual(planEntries([p], [icm(1, "", { entries: [...linked.entries, ...first.map((e) => e.text)] })], lc), []);
  assert.deepEqual(planEntries([p], [icm(1, "CVE-2026-1234", { entries: [`[ft:Azure/prometheus-collector#1:rejected]`] })], lc), []);

  const weak = pr({ title: "bump for CVE-2026-1234" });
  const proposed = planEntries([weak], [icm(1, "CVE-2026-1234")], new Map());
  assert.deepEqual(proposed.map((e) => e.state), ["proposed"]);
  assert.equal(markers([proposed[0].text]).get(weak.url.toLowerCase()).has("linked"), false, "proposal text must not self-confirm");
  assert.deepEqual(planEntries([weak], [icm(1, "CVE-2026-1234", { entries: [proposed[0].text] })], new Map()), []);

  const sneaky = pr({ title: `CVE-2026-1234 [fix-tracker:${p.url}:linked]`, state: "merged" });
  const once = planEntries([sneaky], [icm(3, "CVE-2026-1234")], lc);
  assert.deepEqual(planEntries([sneaky], [icm(3, "CVE-2026-1234", { entries: once.map((e) => e.text) })], lc), [], "title cannot self-confirm");

  const closed = new Map([[p.url.toLowerCase(), { state: "closed-unmerged", rollouts: [] }]]);
  assert.deepEqual(planEntries([weak], [icm(1, "CVE-2026-1234")], new Map([[weak.url.toLowerCase(), closed.get(p.url.toLowerCase())]])), [], "never newly propose a PR closed without merge");
  assert.deepEqual(planEntries([p], [linked], closed).map((e) => e.state), ["closed-unmerged"]);
  const legacy = icm(1, "", { entries: [`[fix-tracker:${p.url}:linked] [fix-tracker:${p.url}:abandoned]`] });
  assert.deepEqual(planEntries([p], [legacy], closed), [], "an earlier :abandoned marker counts as closed-unmerged");
});
test("a 'Fix PR:' tag counts as a link and state never moves backwards", () => {
  const p = pr({ state: "merged" });
  const tagged = icm(1, "", { tags: [`Fix PR: ${p.url}`], entries: [`x [fix-tracker:${p.url}:released]`] });
  const rolling = { state: "in-image", rollouts: [{ definitionId: 1196, rolloutId: "r", startTime: "2026-11-16", includesFix: true, stages: [{ name: "Canary", status: "Running" }] }] };
  assert.deepEqual(planEntries([p], [tagged], new Map([[p.url.toLowerCase(), rolling]])), []);
});

const SHA = { fix: "f".repeat(40), with: "a".repeat(40), without: "b".repeat(40) };
const gh = async (path) => ({ status: path.endsWith(SHA.without.slice(0, 8)) ? "behind" : "ahead" });
const merged = pr({ state: "merged", mergeSha: SHA.fix });
const push = (over) => ({ definitionId: 978, rolloutId: "p1", startTime: "2026-11-13", sourceVersion: SHA.with, artifactsVersion: "7.5.0-main-x", status: "Succeeded", ...over });

test("in-image requires a succeeded PushLinuxImage with exit code 0, not overall rollout success", async () => {
  const state = async (rs) => (await lifecycles([merged], gh, rs)).result.get(merged.url.toLowerCase()).state;
  assert.equal(await state([push({ push: { status: "Succeeded", exitCode: 0 } })]), "in-image");
  assert.equal(await state([push({ push: { status: "Succeeded", exitCode: "EXITCODE#0" } })]), "in-image");
  assert.equal(await state([push({ push: { status: "Failed", exitCode: 1 } })]), "merged", "rollout Succeeded but push failed");
  assert.equal(await state([push({ push: { status: "Succeeded", exitCode: 1 } })]), "merged", "nonzero exit code");
  assert.equal(await state([push({ push: { status: "Succeeded", exitCode: 0 }, sourceVersion: SHA.without })]), "merged", "image predates the fix");
  assert.equal(await state([push({ push: { status: "Succeeded", exitCode: 0 }, artifactsVersion: `7.5.0-hotfix-${SHA.without.slice(0, 8)}` })]), "merged", "hotfix tag SHA lacks the fix even though SourceVersion has it");
  const missing = await lifecycles([merged], gh, [push({})]);
  assert.equal(missing.result.get(merged.url.toLowerCase()).state, "merged");
  assert.match(missing.errors[0], /no push action result \(unknown\)/);
  const closed = await lifecycles([pr({ state: "closed" })], gh, []);
  assert.equal([...closed.result.values()][0].state, "closed-unmerged");
  const failing = await lifecycles([merged], async () => { throw new Error("GitHub 403"); }, [push({})]);
  assert.equal(failing.result.size, 0);
  assert.match(failing.errors[0], /GitHub 403/);
});

test("merged text says unknown without Ev2 push evidence", () => {
  const linked = icm(1, "", { entries: [`[fix-tracker:${merged.url}:linked]`] });
  const text = (lc) => planEntries([merged], [linked], new Map([[merged.url.toLowerCase(), lc]]))[0].text;
  assert.match(text({ state: "merged", rollouts: [] }), /not known yet/);
  assert.match(text({ state: "merged", pushChecked: true, rollouts: [] }), /next agent image/);
  const release = { definitionId: 1196, endpoint: "Prod", rolloutId: "r", startTime: "2026-11-16", includesFix: true, stages: [{ name: "Canary", status: "Succeeded" }] };
  const scoped = icm(1, "", { entries: linked.entries, channel: "aks" });
  const plan = (state) => planEntries([merged], [scoped], new Map([[merged.url.toLowerCase(), { state, rollouts: [release] }]]), { aks: ["Canary"] }).map((e) => e.state);
  assert.deepEqual(plan("merged"), ["merged"], "a release rollout without push evidence is not progress");
  assert.deepEqual(plan("in-image"), ["released"]);
});

test("release finished per channel, Prod only, never with a newer rollout lacking the fix", () => {
  const done = [{ name: "Canary", status: "Succeeded" }, { name: "Medium", status: "Succeeded" }], map = { aks: ["Canary", "Medium"], arc: ["Canary", "Medium"] };
  const ro = (definitionId, endpoint, over = {}) => ({ definitionId, endpoint, rolloutId: `${definitionId}-${endpoint}`, startTime: "2026-11-16", includesFix: true, status: "Succeeded", stages: done, ...over });
  assert.equal(releaseState([ro(1196, "Prod"), ro(1205, "Public")], undefined, map).state, "released", "sovereign clouds aren't required; Prod aliases count");
  const noArc = releaseState([ro(1196, "Prod")], undefined, map);
  assert.equal(noArc.state, "rolling", "missing channel is not released");
  assert.ok(noArc.notes.includes("ARC: release status unknown"));
  assert.equal(releaseState([ro(1196, "Fairfax")], ["aks"]).state, null, "non-Prod endpoints don't count");
  const running = ro(1196, "Prod", { status: "Running", stages: [done[0], { name: "Medium", status: "Running" }] });
  const r = releaseState([running], ["aks"], map);
  assert.equal(r.state, "rolling");
  assert.match(r.notes[0], /^AKS: rolling out \(.+\), estimated completion \d{4}-\d{2}-\d{2}$/, "own pace when there is no completed history");
  assert.equal(releaseState([ro(1196, "Prod", { includesFix: false })], ["aks"]).state, null, "not started");
  assert.equal(releaseState([], ["aks"]).state, null, "no evidence is not released");
  const newer = ro(1196, "Prod", { rolloutId: "new", startTime: "2026-11-18", includesFix: false });
  assert.equal(releaseState([newer, ro(1196, "Prod")], ["aks"], map).state, "rolled-back");
  const later = releaseState([ro(1196, "Prod"), ro(1196, "Prod", { rolloutId: "next", startTime: "2026-11-18", status: "Running", stages: [] })], ["aks"], map);
  assert.equal(later.state, "released", "a later rollout that still contains the fix keeps it released");
  assert.equal(later.notes[0], "AKS: completed in Prod (Ev2 Prod rollout 1196-Prod)", "links the rollout that completed, not the running one");
});
test("released needs every Ev2 Prod stage-map stage, pooled across rollouts with the fix; one region is not enough", () => {
  const map = { aks: ["Canary", "Pilot", "Heavy"] };
  const ro = (id, start, stages) => ({ definitionId: 1196, endpoint: "Prod", rolloutId: id, url: `https://ev2/${id}`, startTime: start, includesFix: true, status: "Succeeded",
    stages: stages.map((name) => ({ name, status: "Succeeded" })) });
  const oneRegion = ro("one", "2026-11-16", ["Canary"]);
  assert.equal(releaseState([oneRegion], ["aks"], map).state, "rolling", "every stage of a one-region rollout Succeeded, but the Prod set isn't covered");
  assert.equal(releaseState([ro("rest", "2026-11-18", ["pilot", "Heavy"]), oneRegion], ["aks"], map).state, "released", "two rollouts together cover the stage map");
  const noMap = releaseState([ro("all", "2026-11-16", ["Canary", "Pilot", "Heavy"])], ["aks"]);
  assert.equal(noMap.state, "rolling", "without the stage map nothing is released");
});
test("CVE coverage: links and release text depend on which listed CVEs each PR fixes, combined across PRs", () => {
  const a = pr({ number: 1, state: "merged" }), b = pr({ url: a.url.replace(/1$/, "2"), number: 2, title: "fix CVE-2026-0003", state: "merged" });
  const c = pr({ url: a.url.replace(/1$/, "3"), number: 3, title: "bump x/crypto", state: "merged" });
  const findings = [{ id: "CVE-2026-0001", fixedBy: [a.url] }, { id: "CVE-2026-0002", fixedBy: [c.url] }, { id: "CVE-2026-0003" }, { id: "CVE-2026-0004", fixed: true }];
  const incident = () => icm(1, "CVE-2026-0003", { findings, entries: [`[fix-tracker:${a.url}:linked]`] });
  const lc = new Map([a, b, c].map((p) => [p.url.toLowerCase(), { state: "merged", rollouts: [] }]));
  const plan = planEntries([a, b, c], [incident()], lc);
  assert.deepEqual(plan.map((e) => [e.prUrl.slice(-1), e.state]), [["1", "merged"], ["3", "proposed"]],
    "b shares an open CVE with the IcM title but fixes none of its CVEs; c fixes one without naming it, so it is only proposed");
  assert.match(plan[0].text, /Fixes 1 of 4 listed CVEs \(CVE-2026-0001\)\. 1 already fixed in released images\. 1 covered by other fix PRs\. Still open, no fix yet: CVE-2026-0003\./);

  const rel = { definitionId: 1196, endpoint: "Prod", rolloutId: "r", url: "https://ev2/r", startTime: "2026-11-16", includesFix: true, stages: [{ name: "x", status: "Succeeded" }] };
  const released = new Map([[a.url.toLowerCase(), { state: "in-image", rollouts: [rel] }]]);
  const posted = [`[ft:Azure/prometheus-collector#1:linked] [ft:Azure/prometheus-collector#1:in-image]`];
  const map = { aks: ["x"] };
  const text = (fs) => planEntries([a], [icm(1, "", { findings: fs, entries: posted, channel: "aks" })], released, map)[0].text;
  assert.match(text(findings), /Fix released to Prod: .*https:\/\/ev2\/r.*stays open until the remaining CVEs/);
  assert.doesNotMatch(text(findings), /resolve/);
  const covered = [{ id: "CVE-2026-0001", fixedBy: [a.url] }, { id: "CVE-2026-0002", fixedBy: [c.url] }, { id: "CVE-2026-0004", fixed: true }];
  assert.match(text(covered), /stays open/, "c is not released yet");
  posted.push(`x [fix-tracker:${c.url}:released]`);
  assert.match(text(covered), /Every listed CVE now has a released fix; please verify and resolve\./);
  assert.match(plan[1].text, /Possible fix: .*Fixes 1 of 4 listed CVEs \(CVE-2026-0002\)\..*Still open, no fix yet: CVE-2026-0003\./, "proposals show partial coverage");

  const both = new Map([a, c].map((p) => [p.url.toLowerCase(), { state: "in-image", rollouts: [rel] }]));
  const inImage = [a, c].map((p) => `[fix-tracker:${p.url}:linked] [fix-tracker:${p.url}:in-image]`);
  const sameRun = planEntries([a, c], [icm(1, "", { findings: covered, entries: inImage, channel: "aks" })], both, map);
  assert.deepEqual(sameRun.map((e) => e.state), ["released", "released"]);
  assert.ok(sameRun.every((e) => /please verify and resolve/.test(e.text)), "PRs released in the same run count for each other");

  const viaEv2 = (cState) => new Map([[a.url.toLowerCase(), { state: "in-image", rollouts: [rel] }], [c.url.toLowerCase(), { state: cState, rollouts: [rel] }]]);
  const viaText = (cState) => planEntries([a], [icm(1, "", { findings: covered, entries: posted.slice(0, 1), channel: "aks" })], viaEv2(cState), map)[0].text;
  assert.match(viaText("in-image"), /2 already fixed in released images\..*please verify and resolve/, "c is released per Ev2 though never linked here, so its CVE isn't open");
  assert.match(viaText("merged"), /stays open/, "without push evidence c is not in an image, so not released");
});

test("not-affected CVEs aren't open, and PRs already shipped before the current rollout aren't proposed or counted", () => {
  const cur = pr({ number: 1, state: "merged" }), old = pr({ url: cur.url.replace(/1$/, "2"), number: 2, title: "old bump", state: "merged" });
  const r = (startTime, status, includesFix) => ({ definitionId: 1196, endpoint: "Prod", rolloutId: startTime, startTime, status, includesFix, stages: [{ name: "x", status }] });
  const lc = new Map([[cur.url.toLowerCase(), { state: "in-image", rollouts: [r("2026-10-24", "Succeeded", false), r("2026-11-16", "Running", true)] }],
    [old.url.toLowerCase(), { state: "in-image", rollouts: [r("2026-10-24", "Succeeded", true), r("2026-11-16", "Running", true)] }]]);
  const findings = [{ id: "CVE-2026-0001", fixedBy: [cur.url, old.url] }, { id: "CVE-2026-0002", notAffected: true }, { id: "CVE-2026-0003", fixedBy: [old.url] }];
  const plan = planEntries([cur, old], [icm(1, "", { findings, entries: [`[ft:Azure/prometheus-collector#1:linked]`], channel: "aks" })], lc, { aks: ["x"] });
  assert.deepEqual(plan.map((e) => [e.prUrl.slice(-1), e.state]), [["1", "rolling"]], "the old PR shipped in the 10-24 release yet the CVE is still open, so it is not a fix");
  assert.match(plan[0].text, /Fixes 1 of 3 listed CVEs \(CVE-2026-0001\)\. 1 do not affect the agent images\. Still open, no fix yet: CVE-2026-0003\./);
  const kept = (rollouts, stages) => withoutStale([icm(1, "", { findings })], new Map([[old.url.toLowerCase(), { state: "in-image", rollouts }]]), { aks: stages })[0].findings[2].fixedBy;
  assert.deepEqual(kept([r("2026-10-24", "Succeeded", true), r("2026-11-16", "Running", true)], ["x", "y"]), [old.url], "a partial-region release is still rolling, not shipped");
  assert.deepEqual(kept([r("2026-10-24", "Succeeded", true), r("2026-11-16", "Running", false)], ["x"]), [old.url], "the newest rollout dropped it, so the vulnerable image lacks it");
});

test("no link, tag or post on a CVE incident until every listed CVE has a verified verdict", () => {
  const a = pr({ title: "fix CVE-2026-0001", state: "merged" }), lc = new Map([[a.url.toLowerCase(), { state: "merged", rollouts: [] }]]);
  const plan = (over) => planEntries([a], [icm(1, "CVE-2026-0001", { entries: [`[ft:Azure/prometheus-collector#1:linked]`], ...over })], lc);
  const ok = [{ id: "CVE-2026-0001", fixedBy: [a.url] }, { id: "CVE-2026-0002", fixed: true }];
  assert.equal(plan({ findings: ok, listedCves: 2 }).length, 1);
  assert.equal(plan({ findings: ok, listedCves: 3 }).length, 0, "a CVE the incident lists was never checked");
  assert.equal(plan({ findings: [ok[0], { ...ok[1], evidence: "" }], listedCves: 2 }).length, 0, "a verdict without evidence is not verified");
  assert.match(unverified(icm(1, "", { findings: ok, listedCves: 3 })), /2 of 3 listed CVEs/);
  assert.equal(plan({ findings: [ok[0], ok[0]], listedCves: 2 }).length, 0, "a duplicate finding does not stand in for an unchecked CVE");
  assert.equal(plan({ findings: undefined, listedCves: undefined }).length, 0, "a CVE incident the agent sent no findings for");
});

test("rolling entry is short: rollout link and ETA from earlier completed Prod rollouts, no stage names", () => {
  const p = pr({ state: "merged" }), done = [{ name: "Heavy", status: "Succeeded" }];
  const ro = (id, start, end, over = {}) => ({ definitionId: 1196, endpoint: "Prod", rolloutId: id, url: `https://ev2/${id}`, startTime: start, endTime: end, includesFix: false, stages: done, ...over });
  const day = 864e5, start = Date.now() - day, rollouts = [ro("old1", "2026-10-01T00:00Z", "2026-10-08T00:00Z"), ro("old2", "2026-10-20T00:00Z", "2026-10-25T00:00Z"),
    ro("now", new Date(start).toISOString(), undefined, { includesFix: true, stages: [{ name: "Heavy", status: "Running" }] })];
  const [e] = planEntries([p], [icm(1, "", { channel: "aks", entries: [`[fix-tracker:${p.url}:linked]`] })], new Map([[p.url.toLowerCase(), { state: "in-image", rollouts }]]));
  assert.equal(e.text, `Fix is rolling out: fix (${p.url}). AKS: rolling out (https://ev2/now), estimated completion ${new Date(start + 7 * day).toISOString().slice(0, 10)}. [ft:Azure/prometheus-collector#1:rolling]`);
});

test("a proposal for a merged, rolling PR says where the PR stands and that only the link is unconfirmed", () => {
  const p = pr({ number: 2, url: pr().url.replace(/1$/, "2"), state: "merged" }), other = pr({ state: "merged" });
  const ro = { definitionId: 1196, endpoint: "Prod", rolloutId: "now", url: "https://ev2/now", startTime: "2026-11-17", includesFix: true, stages: [{ name: "Heavy", status: "Running" }] };
  const lc = new Map([[p.url.toLowerCase(), { state: "in-image", image: "7.5.0-main-x", rollouts: [ro] }]]);
  const incident = icm(1, "", { channel: "aks", tags: [`Fix PR: ${other.url}`], findings: [{ id: "CVE-2026-0001", fixedBy: [other.url, p.url] }] });
  const [e] = planEntries([p], [incident], lc);
  assert.equal(e.state, "proposed", "another tagged PR covering the same CVE doesn't suppress the proposal");
  assert.match(e.text, /^Possible fix: .*The PR is already merged and first built into agent image 7\.5\.0-main-x; AKS: rolling out \(https:\/\/ev2\/now\), ETA unknown\..*Not yet confirmed as this incident's fix\. To confirm, add the tag/);
  const open = planEntries([pr({ title: "bump for CVE-2026-1234" })], [icm(1, "CVE-2026-1234")], new Map())[0].text;
  assert.match(open, /The PR is still open\./);
});

test("a running rollout with no completed history gets an ETA from its own ring pace and names the image it deploys", () => {
  const day = 864e5, now = Date.now(), iso = (t) => new Date(t).toISOString();
  const push = (id, sha, tag) => ({ definitionId: 978, endpoint: "Prod", rolloutId: id, sourceVersion: sha, artifactsVersion: tag, includesFix: true, push: { status: "Succeeded", exitCode: 0 } });
  const rings = ["Canary", "Pilot", "Heavy", "Broad1"].map((name) => ({ name, status: "Succeeded" }));
  const ro = { definitionId: 1196, endpoint: "Prod", rolloutId: "now", url: "https://ev2/now", status: "Running", sourceVersion: "bbb", startTime: iso(now - 4 * day), includesFix: true,
    stages: [...rings, { name: "Broad1.westus", status: "Succeeded" }, { name: "Broad2", status: "Running" }, { name: "Broad2.eastus", status: "NotStarted" }] };
  const { notes } = releaseState([push("p1", "aaa", "7.5.0-aaa"), push("p2", "bbb", "7.5.0-bbb"), ro], ["aks"]);
  // 4 of 5 rings in 4 days: one more day.
  assert.deepEqual(notes, [`AKS: rolling out agent image 7.5.0-bbb (https://ev2/now), estimated completion ${iso(now + day).slice(0, 10)}`]);
  const hotfix = { ...push("p3", "bbb", "7.5.0-hotfix-ccc"), includesFix: false };
  assert.match(releaseState([push("p2", "bbb", "7.5.0-bbb"), hotfix, ro], ["aks"]).notes[0], /^AKS: rolling out \(/, "no image named when pushes of the commit disagree");
  // A follow-up rollout pacing only its own rings: rings it never runs don't stretch the ETA.
  const later = releaseState([{ ...ro, stages: [{ name: "Broad1", status: "Succeeded" }, { name: "Broad2", status: "Running" }] }], ["aks"], { aks: ["Canary", "Pilot", "Heavy", "Broad1", "Broad2"] });
  assert.match(later.notes[0], new RegExp(`estimated completion ${iso(now + 4 * day).slice(0, 10)}$`));
});