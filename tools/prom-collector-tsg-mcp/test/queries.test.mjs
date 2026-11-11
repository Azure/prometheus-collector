import assert from "node:assert/strict";
import { test } from "node:test";
import { QUERIES } from "../dist/queries.js";

const allQueries = Object.values(QUERIES).flat();
const queryNamed = (name) => {
  const query = allQueries.find((candidate) => candidate.name === name);
  assert.ok(query, `missing query: ${name}`);
  return query.kql;
};

test("replica memory keeps each process P95 separate", () => {
  const kql = queryNamed("Max P95 Memory GB by Process per Replica");
  assert.match(kql, /by pod, name, bin\(timestamp,/);
  assert.doesNotMatch(kql, /summarize value=sum\(value\)/);
});

for (const name of [
  "Replicaset ME Drop Metrics (per-field maxima)",
  "DaemonSet ME Drop Metrics (per-field maxima)",
]) {
  test(`${name} computes each drop metric maximum independently`, () => {
    const kql = queryNamed(name);
    assert.match(kql, /samplesDropped=max\(samplesDropped\)/);
    assert.match(kql, /aggregatedSamplesDropped=max\(aggregatedSamplesDropped\)/);
    assert.match(kql, /currentQueueSize=max\(currentQueueSize\)/);
    assert.doesNotMatch(kql, /argmax\(/);
  });
}
