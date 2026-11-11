import assert from "node:assert/strict";
import { afterEach, test } from "node:test";
import { createServer } from "node:http";
import { mkdtempSync, readFileSync, rmSync, rmdirSync } from "node:fs";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";
import { Client } from "@modelcontextprotocol/sdk/client/index.js";
import { StdioClientTransport } from "@modelcontextprotocol/sdk/client/stdio.js";
import { queryMdmMetric, queryMdmThrottling, queryMeInternalMetrics, queryScrapeTargetHealth, queryMdmKqlm, queryMdmMetadata } from "../dist/mdm.js";

const originalFetch = globalThis.fetch;
afterEach(() => { globalThis.fetch = originalFetch; });

function mockMdm(reply) {
  globalThis.fetch = async (_url, options) => {
    const request = JSON.parse(options.body);
    if (request.method === "initialize") return Response.json({ result: {} });
    const result = reply(request.params);
    return Response.json({
      result: { content: [{ text: JSON.stringify({ Success: true, Error: null, Result: result }) }] },
    });
  };
}

function kqlm(values) {
  return [JSON.stringify({
    Status: "Success",
    Results: values === undefined ? [] : [{ Dimensions: {}, SamplingTypes: { Sum: values } }],
  })];
}

test("MDM errors inside a successful wrapper cannot report data or scrape health", async () => {
  mockMdm(() => ["Error: Error code: InvalidSeries. Metric doesn't have preaggregate defined."]);
  const metric = await queryMdmMetric("test-account", "up", "test-cluster");
  assert.match(metric, /Query failed:.*InvalidSeries/);
  assert.doesNotMatch(metric, /Data exists/);
  const scrape = await queryScrapeTargetHealth("test-account", "test-job", "test-cluster");
  assert.match(scrape, /Query failed:.*InvalidSeries/);
  const allJobs = await queryScrapeTargetHealth("test-account", "", "test-cluster");
  assert.match(allJobs, /some queries failed/);
  assert.match(allJobs, /InvalidSeries/);
  mockMdm(({ arguments: args }) => args.metrics === "up"
    ? ["Sum: [1] Count: [1]"]
    : ["Error: InvalidSeries"]);
  const sampleErrors = await queryScrapeTargetHealth("test-account", "", "test-cluster");
  assert.match(sampleErrors, /some queries failed/);
  assert.match(sampleErrors, /\(postRelabel\): Error: InvalidSeries/);
});

test("unrecognized metric responses do not confirm data", async () => {
  mockMdm(() => ["Unexpected response"]);
  const output = await queryMdmMetric("test-account", "up", "test-cluster");
  assert.match(output, /Unrecognized MDM response/);
  assert.doesNotMatch(output, /Data exists/);
});

for (const [label, result] of [["empty", []], ["NaN", ["Sum: [NaN] Count: [NaN]"]], ["zero count", ["Sum: [0] Count: [0]"]]]) {
  test(`single-job health is inconclusive with ${label} observations`, async () => {
    mockMdm(({ arguments: args }) => args.metrics === "up" ? result : []);
    const output = await queryScrapeTargetHealth("test-account", "test-job", "test-cluster");
    assert.match(output, /Inconclusive/);
    assert.doesNotMatch(output, /fully healthy|✅/);
    if (result.length) {
      assert.match(output, /Healthy buckets \(all up=1\) \| 0 \/ 0/);
      assert.match(output, /Unobserved buckets.*\| 1 \|/);
    }
  });
}

test("single-job health scopes healthy observations and identifies unobserved buckets", async () => {
  mockMdm(({ arguments: args }) => args.metrics === "up" ? ["Sum: [2,NaN,0] Count: [2,NaN,0]"] : []);
  const output = await queryScrapeTargetHealth("test-account", "test-job", "test-cluster");
  assert.match(output, /No scrape failures among observed scrapes/);
  assert.match(output, /Healthy buckets \(all up=1\) \| 1 \/ 1/);
  assert.match(output, /Unobserved buckets.*\| 2 \|/);
  assert.match(output, /health during these buckets is unknown/);
  assert.doesNotMatch(output, /fully healthy/);
});

test("single-job health preserves healthy and failed observed scrapes", async () => {
  for (const [sum, expected] of [[2, /No scrape failures among observed scrapes/], [0, /has scrape failures/]]) {
    mockMdm(({ arguments: args }) => args.metrics === "up" ? [`Sum: [${sum}] Count: [2]`] : []);
    const output = await queryScrapeTargetHealth("test-account", "test-job", "test-cluster");
    assert.match(output, expected);
    assert.match(output, /Total scrape events \| 2 \|/);
    assert.match(output, /Unobserved buckets.*\| 0 \|/);
    assert.doesNotMatch(output, /Inconclusive|health during these buckets is unknown/);
  }
});

for (const [label, isError, Success, Error] of [["MCP", true, true, null], ["Success=false", false, false, null], ["Error", false, true, "Denied"]]) {
  test(`${label} failures are surfaced even when numeric data is present`, async () => {
    globalThis.fetch = async (_url, options) => {
      if (JSON.parse(options.body).method === "initialize") return Response.json({ result: {} });
      return Response.json({ result: { isError, content: [{ text: JSON.stringify({
        Success, Error, Result: ["Sum: [1] Count: [1]"],
      }) }] } });
    };
    const output = await queryMdmMetric("test-account", "up", "test-cluster");
    assert.match(output, /Query failed:/);
    assert.doesNotMatch(output, /Data exists/);
  });
}

test("parsed metric data still confirms availability", async () => {
  mockMdm(() => ["Sum: [1,2] Count: [1,1]"]);
  assert.match(await queryMdmMetric("test-account", "up", "test-cluster"), /Data exists/);
});

test("metric queries distinguish unobserved buckets from observed zero values", async () => {
  for (const [count, expected] of [[0, /No observed data/], [2, /Data exists/]]) {
    mockMdm(() => [`Sum: [0] Count: [${count}]`]);
    const output = await queryMdmMetric("test-account", "up", "test-cluster");
    assert.match(output, expected);
    if (count === 0) assert.doesNotMatch(output, /Data exists/);
  }
  mockMdm(() => ["Sum: [7,0] Count: [1,0]"]);
  const partial = await queryMdmMetric("test-account", "up", "test-cluster");
  assert.match(partial, /1 active buckets out of 2 total \(1 unobserved\)/);
  assert.match(partial, /Latest observed value \| 7 \|/);
  assert.doesNotMatch(partial, /Significant Changes|Sum values are stable/);
});

for (const dimensions of ["{", "null", "[]", '{"job":"test-job"}', '{"job":[1]}']) {
  test(`invalid metric dimensions ${dimensions} cannot broaden the query`, async () => {
    let queries = 0;
    mockMdm(() => { queries++; return ["Sum: [1] Count: [1]"]; });
    const output = await queryMdmMetric("test-account", "up", "test-cluster", "customdefault", dimensions);
    assert.match(output, /Invalid dimensions/);
    assert.equal(queries, 0);
  });
}

test("valid metric dimensions preserve job and instance filters alongside cluster", async () => {
  let actual;
  mockMdm(({ arguments: args }) => {
    actual = JSON.parse(args.dimensionMapJson);
    return ["Sum: [0] Count: [2]"];
  });
  const output = await queryMdmMetric("test-account", "up", "test-cluster", "customdefault", '{"job":["test-job"],"instance":[]}');
  assert.match(output, /Data exists/);
  assert.deepEqual(actual, { job: ["test-job"], instance: [], cluster: ["test-cluster"] });
});

test("KQLM string results from the live helper are normalized", async () => {
  mockMdm(() => kqlm([0])[0]);
  const output = await queryMdmThrottling("test-account");
  assert.match(output, /data points, all zero/);
  assert.match(output, /### ✅ No throttling or drops/);
});

for (const [name, values] of [["empty", undefined], ["NaN", ["NaN"]], ["null", [null]], ["zero", [0, 0]]]) {
  test(`ME and QoS distinguish ${name} panels from verified zero`, async () => {
    mockMdm(() => kqlm(values));
    for (const query of [queryMeInternalMetrics, queryMdmThrottling]) {
      const output = await query("test-account");
      if (name === "zero") {
        assert.match(output, /### ✅ No .*drops/);
      } else {
        assert.match(output, /Inconclusive/);
        assert.doesNotMatch(output, /✅|No drops\/errors/);
      }
    }
  });
}

for (const [label, raw] of [["failed", JSON.stringify({ Status: "Failed", Messages: ["InvalidSeries"] })], ["malformed", "not JSON"]]) {
  test(`${label} KQLM responses are errors, not empty or healthy panels`, async () => {
    mockMdm(() => [raw]);
    for (const query of [queryMeInternalMetrics, queryMdmThrottling]) {
      const output = await query("test-account");
      assert.match(output, /Error:/);
      assert.match(output, /Inconclusive/);
      assert.doesNotMatch(output, /### ✅/);
    }
  });
}

test("partial drop telemetry cannot rule out drops", async () => {
  mockMdm(({ arguments: args }) => kqlm(args.finalKqlmQuery.includes("RawEventsDroppedCount") ? undefined : [0]));
  assert.match(await queryMeInternalMetrics("test-account"), /Inconclusive/);
});

test("duplicate MStore drops are not labeled throttling", async () => {
  mockMdm(({ arguments: args }) => {
    const result = JSON.parse(kqlm(args.finalKqlmQuery.includes("MStoreDroppedSamplesCount") ? [2] : [0])[0]);
    result.Results[0].Dimensions = { Reason: "Duplicated" };
    return [JSON.stringify(result)];
  });
  const output = await queryMdmThrottling("test-account");
  assert.match(output, /DROPS DETECTED/);
  assert.match(output, /`Duplicated` \| 2/);
  assert.doesNotMatch(output, /THROTTLING DETECTED/);
});

const startTime = "2026-01-01T00:00:00Z";
const endTime = "2026-01-01T00:10:00Z";

test("flexible KQL-M preserves family, grouped and raw queries and the exact window", async () => {
  const target = 'metric("kube_pod_status_phase").dimensions("cluster","job","instance").resolution(1m).samplingTypes("Count") | where cluster == "test-cluster" and job == "test-job" and instance == "test-instance"';
  const queries = [
    `${target} | summarize Observed=sum(Count) by cluster,job,instance`,
    `${target.replace("kube_pod_status_phase", "kube_node_labels")} | summarize Observed=sum(Count) by cluster,job,instance`,
    `${target.replace('"instance")', '"instance","condition","status")')} | summarize Observed=sum(Count) by condition,status`,
    target.replace(".resolution(1m)", "").replace(' and instance == "test-instance"', ""),
  ];
  const calls = [];
  mockMdm(({ name, arguments: args }) => {
    calls.push({ name, ...args });
    return JSON.stringify({
      Status: "Success", Query: args.finalKqlmQuery,
      Results: [{ Dimensions: { cluster: "test-cluster", phase: "Running", uid: "test-uid" }, SamplingTypes: { Observed: [0, "NaN", null, 2] } }],
    });
  });
  const progress = [];
  const output = JSON.parse(await queryMdmKqlm("test-account", "customdefault", queries, startTime, endTime,
    async (completed, total) => { progress.push([completed, total]); }));
  assert.deepEqual(progress, [[1, 4], [2, 4], [3, 4], [4, 4]]);
  assert.deepEqual(calls, queries.map((query) => ({
    name: "KqlmQuery", monitoringAccount: "test-account", metricNamespace: "customdefault",
    startTime, endTime, finalKqlmQuery: query,
  })));
  assert.deepEqual(output.requestedWindow, { startTime, endTime });
  for (const [i, response] of output.results.entries()) {
    assert.equal(response.query, queries[i]);
    assert.equal(response.finiteValueCount, 2);
    assert.equal(response.missingValueCount, 2);
    assert.deepEqual(response.result.Results[0].SamplingTypes.Observed, [0, "NaN", null, 2]);
    assert.equal(response.result.Results[0].Dimensions.uid, "test-uid");
    assert.equal("Timestamps" in response.result, false);
    assert.equal("Timestamps" in response.result.Results[0], false);
  }
});

test("flexible KQL-M preserves actual timestamps rather than substituting requested times", async () => {
  const timestamps = ["2026-01-01T00:03:00Z"];
  mockMdm(() => JSON.stringify({
    Status: "Success", Query: 'metric("up")',
    Results: [{ Dimensions: {}, SamplingTypes: { Count: [1] }, Timestamps: timestamps }],
  }));
  const output = JSON.parse(await queryMdmKqlm("test-account", "customdefault", ['metric("up")'], startTime, endTime));
  assert.deepEqual(output.results[0].result.Results[0].Timestamps, timestamps);
});

test("flexible KQL-M distinguishes finite zero, missing values and no series without declaring health", async () => {
  for (const [values, finite, missing] of [[[0], 1, 0], [["NaN", null], 0, 2], [undefined, 0, 0]]) {
    mockMdm(() => kqlm(values)[0]);
    const text = await queryMdmKqlm("test-account", "customdefault", ['metric("up")'], startTime, endTime);
    const response = JSON.parse(text).results[0];
    assert.equal(response.finiteValueCount, finite);
    assert.equal(response.missingValueCount, missing);
    assert.equal(response.result.Results.length, values === undefined ? 0 : 1);
    assert.doesNotMatch(text, /Data exists|healthy/);
  }
});

test("flexible KQL-M rejects absent helper payloads but preserves successful no-series results", async () => {
  for (const payload of [null, []]) {
    mockMdm(() => payload);
    await assert.rejects(
      queryMdmKqlm("test-account", "customdefault", ['metric("up")'], startTime, endTime),
      /no KQL-M payload.*inconclusive/
    );
  }
  mockMdm(() => kqlm()[0]);
  const output = JSON.parse(await queryMdmKqlm("test-account", "customdefault", ['metric("up")'], startTime, endTime));
  assert.deepEqual(output.results[0].result.Results, []);
});

test("flexible KQL-M rejects invalid windows and empty queries before requesting data", async () => {
  let calls = 0;
  mockMdm(() => { calls++; return kqlm([1])[0]; });
  for (const [queries, start, end] of [
    [[], startTime, endTime],
    [[" "], startTime, endTime],
    [['metric("up")'], endTime, startTime],
    [['metric("up")'], startTime, startTime],
    [['metric("up")'], "not-a-dateZ", endTime],
    [['metric("up")'], "2025-12-31T00:00:00", endTime],
    [['metric("up")'], startTime, "2026-01-02T00:00:00"],
    [['metric("up")'], startTime, "not-a-dateZ"],
  ]) {
    await assert.rejects(queryMdmKqlm("test-account", "customdefault", queries, start, end), /explicit UTC|nonempty/);
  }
  assert.equal(calls, 0);
});

test("flexible KQL-M fails on query errors instead of exporting them as absent data", async () => {
  for (const result of [
    "Error: InvalidSeries",
    JSON.stringify({ Status: "Error", Messages: ["InvalidSeries"] }),
    JSON.stringify({ ...JSON.parse(kqlm([1])[0]), Status: "Error", Messages: ["InvalidSeries"] }),
    "not-json",
  ]) {
    mockMdm(() => result);
    await assert.rejects(queryMdmKqlm("test-account", "customdefault", ['metric("up")'], startTime, endTime));
  }
});

test("malformed helper result shapes cannot become empty successful queries", async () => {
  for (const result of [
    { unexpected: "object" },
    [{ Status: "Success", Results: [] }],
    JSON.stringify({ Status: "Success" }),
    JSON.stringify({ Status: "Success", Results: [{ SamplingTypes: { Count: 1 } }] }),
    JSON.stringify({ Status: "Success", Results: [{ SamplingTypes: [[1]] }] }),
  ]) {
    mockMdm(() => result);
    await assert.rejects(queryMdmKqlm("test-account", "customdefault", ['metric("up")'], startTime, endTime), /Invalid .*result/);
  }
});

test("metadata discovery dispatches the helper's exact argument names and preserves structured metadata", async () => {
  const calls = [];
  const entries = [{ Dimensions: ["cluster", "job", "instance"], SamplingTypes: ["Count"] }];
  mockMdm(({ name, arguments: args }) => { calls.push({ name, args }); return entries; });
  for (const operation of ["namespaces", "metrics", "dimensions", "preaggregates"]) {
    const output = JSON.parse(await queryMdmMetadata("test-account", operation, "customdefault", "up"));
    assert.deepEqual(output.entries, entries);
    assert.match(output.caveat, /do not prove/);
  }
  assert.deepEqual(calls, [
    { name: "GetNameSpaces", args: { monitoringAccount: "test-account" } },
    { name: "GetMetrics", args: { monitoringAccount: "test-account", nameSpace: "customdefault" } },
    { name: "GetDimensions", args: { monitoringAccount: "test-account", metricNamespace: "customdefault", metric: "up" } },
    { name: "GetPreAggregateConfigurations", args: { monitoringAccount: "test-account", metricNamespace: "customdefault", metric: "up" } },
  ]);
});

test("empty preaggregate metadata remains inconclusive and missing metric cannot broaden discovery", async () => {
  let calls = 0;
  mockMdm(() => { calls++; return []; });
  const output = await queryMdmMetadata("test-account", "preaggregates", "customdefault", "up");
  assert.match(output, /Inconclusive.*does not prove absence/);
  for (const operation of ["dimensions", "preaggregates"]) {
    await assert.rejects(queryMdmMetadata("test-account", operation), /requires a metric/);
  }
  assert.equal(calls, 1);
  mockMdm(() => ["Error: Denied"]);
  await assert.rejects(queryMdmMetadata("test-account", "metrics"), /Denied/);
});

test("missing namespace samples cannot become zero active series", async () => {
  mockMdm(({ arguments: args }) => {
    const result = JSON.parse(kqlm(args.finalKqlmQuery.includes('.dimensions("MetricNamespace")') ? ["NaN", null] : [0])[0]);
    result.Results[0].Dimensions = { MetricNamespace: "test-namespace" };
    return JSON.stringify(result);
  });
  const output = await queryMdmThrottling("test-account");
  assert.doesNotMatch(output, /`test-namespace` \| 0/);
});

test("registered MDM tools export every raw series and return MCP errors for failed queries", { timeout: 20000 }, async (t) => {
  const series = Array.from({ length: 101 }, (_, i) => ({
    Dimensions: { uid: `test-${i}` }, SamplingTypes: { Count: [0, "NaN"] },
  }));
  const helper = createServer((request, response) => {
    let body = "";
    request.on("data", (chunk) => { body += chunk; });
    request.on("end", () => {
      const { id, params } = JSON.parse(body);
      const result = params.arguments.finalKqlmQuery === "missing-payload" ? null : params.name === "KqlmQuery"
        ? JSON.stringify(params.arguments.finalKqlmQuery === "bad-query"
          ? { Status: "Error", Messages: ["InvalidSeries"] }
          : { Status: "Success", Results: series })
        : ["cluster", "job", "instance"];
      response.setHeader("Content-Type", "application/json");
      response.end(JSON.stringify({ jsonrpc: "2.0", id,
        result: { content: [{ type: "text", text: JSON.stringify({ Success: true, Error: null, Result: result }) }] },
      }));
    });
  });
  const directory = mkdtempSync(join(tmpdir(), "tsg-mdm-test-"));
  const outputFile = join(directory, "raw-series.json");
  const client = new Client({ name: "tsg-test", version: "1.0" });
  t.after(async () => {
    await client.close();
    helper.closeAllConnections();
    await new Promise((resolve) => helper.close(resolve));
    rmSync(outputFile, { force: true });
    rmdirSync(directory);
  });
  await new Promise((resolve) => helper.listen(0, "127.0.0.1", resolve));
  await client.connect(new StdioClientTransport({
    command: process.execPath,
    args: [resolve(import.meta.dirname, "../dist/index.js")],
    env: { ...process.env, MDM_MCP_URL: `http://127.0.0.1:${helper.address().port}/mcp` },
    stderr: "pipe",
  }));
  const tools = await client.listTools();
  for (const name of ["tsg_mdm_kqlm", "tsg_mdm_metadata"]) {
    assert.ok(tools.tools.some((tool) => tool.name === name));
  }
  const args = { monitoringAccount: "test-account", queries: ['metric("kube_pod_status_phase")'], startTime, endTime, outputFile };
  const reply = await client.callTool({ name: "tsg_mdm_kqlm", arguments: args });
  assert.notEqual(reply.isError, true);
  const saved = JSON.parse(readFileSync(outputFile, "utf8"));
  assert.deepEqual(saved.results[0].result.Results, series);
  const metadata = await client.callTool({
    name: "tsg_mdm_metadata",
    arguments: { monitoringAccount: "test-account", operation: "dimensions", metric: "up" },
  });
  assert.deepEqual(JSON.parse(metadata.content[0].text).entries, ["cluster", "job", "instance"]);
  const failure = await client.callTool({
    name: "tsg_mdm_kqlm", arguments: { ...args, queries: ["bad-query"] },
  });
  assert.equal(failure.isError, true);
  assert.match(failure.content[0].text, /InvalidSeries/);
  const missing = await client.callTool({
    name: "tsg_mdm_kqlm", arguments: { ...args, queries: ["missing-payload"] },
  });
  assert.equal(missing.isError, true);
  assert.match(missing.content[0].text, /no KQL-M payload.*inconclusive/);
});
