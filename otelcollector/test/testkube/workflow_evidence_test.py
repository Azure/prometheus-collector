import copy
import json
from pathlib import Path
import re
import shlex
import tempfile
import unittest

import yaml

from workflow_evidence import render_candidate, report_counts, validate_summary


ROOT = Path(__file__).parent
SHA = "a" * 40
IMAGE = "docker.io/cerebro31/ginkgo-test@sha256:" + "b" * 64


class WorkflowEvidenceTest(unittest.TestCase):
    def setUp(self):
        self.directory = tempfile.TemporaryDirectory()
        self.addCleanup(self.directory.cleanup)
        self.artifacts = Path(self.directory.name) / "artifacts"
        self.artifacts.mkdir()
        self.suites = [{
            "SuiteSucceeded": True,
            "SpecReports": [{"State": "passed", "LeafNodeType": "BeforeSuite"},
                            {"State": "passed", "LeafNodeType": "It"},
                            {"State": "skipped", "LeafNodeType": "It"}],
        }]
        self.junit = '<testsuites><testsuite><testcase name="[BeforeSuite]"/><testcase name="[It] runs"/><testcase name="[It] skipped"><skipped/></testcase></testsuite></testsuites>'
        self.write_reports()

    def write_reports(self):
        (self.artifacts / "ginkgo.json").write_text(json.dumps(self.suites), encoding="utf-8")
        (self.artifacts / "junit.xml").write_text(self.junit, encoding="utf-8")

    def test_every_existing_workflow_renders_with_pinned_module_cli(self):
        sources = list(ROOT.glob("testkube-test-crs*.yaml"))
        sources += list((ROOT / "config-processing-test-crs").glob("*.yaml"))
        count = 0
        for path in sources:
            source = path.read_text(encoding="utf-8")
            for original in yaml.safe_load_all(source):
                if not original or original.get("kind") != "TestWorkflow":
                    continue
                with self.subTest(path=path.name, workflow=original["metadata"]["name"]):
                    rendered = render_candidate(source, original["metadata"]["name"], SHA, IMAGE)
                    self.assertEqual(rendered["metadata"], original["metadata"])
                    self.assertEqual(rendered["spec"]["content"]["git"]["revision"], SHA)
                    self.assertEqual(rendered["spec"]["pod"]["nodeSelector"]["kubernetes.io/os"], "linux")
                    self.assertEqual(rendered["spec"]["pod"]["nodeSelector"]["kubernetes.io/arch"], "amd64")
                    self.assertEqual(rendered["spec"]["pod"]["labels"]["azure.workload.identity/use"], "true")
                    self.assertEqual(rendered["spec"]["pod"]["serviceAccountName"], "testkube-api-server-tests-job")
                    self.assertIn("otelcollector/test/scenarios/settings", rendered["spec"]["content"]["git"]["paths"])
                    runs = [step for step in rendered["spec"]["steps"] if "shell" in step]
                    self.assertEqual(len(runs), 1)
                    self.assertEqual(runs[0]["container"]["image"], IMAGE)
                    self.assertNotIn("{{", runs[0]["shell"])
                    self.assertIn(
                        "GOTOOLCHAIN=\"$(go env GOVERSION)\" "
                        "go run github.com/onsi/ginkgo/v2/ginkgo@"
                        "\"$(go list -m github.com/onsi/ginkgo/v2 | cut -d ' ' -f 2)\" run ",
                        runs[0]["shell"],
                    )
                    self.assertIn("--fail-on-empty", runs[0]["shell"])
                    self.assertIn("--json-report=/data/reports/ginkgo.json", runs[0]["shell"])
                    self.assertIn("--junit-report=/data/reports/junit.xml", runs[0]["shell"])
                    self.assertEqual(rendered["spec"]["steps"][-1]["condition"], "always")
                    count += 1
        self.assertGreater(count, 30)

    def test_candidate_requires_exact_workflow_and_immutable_inputs(self):
        source = (ROOT / "testkube-test-crs.yaml").read_text(encoding="utf-8")
        for name, sha, image in [
            ("missing", SHA, IMAGE), ("operator", "main", IMAGE),
            ("operator", SHA, "cerebro31/ginkgo-test:latest"),
        ]:
            with self.subTest(name=name, sha=sha, image=image), self.assertRaises(ValueError):
                render_candidate(source, name, sha, image)
        rendered = render_candidate(source, "prometheusui", SHA, IMAGE, "foo || bar")
        self.assertIn("--label-filter 'foo || bar'", rendered["spec"]["steps"][0]["shell"])
        self.assertNotIn("arc-extension", rendered["spec"]["steps"][0]["shell"])
        rendered = render_candidate(source, "prometheusui", SHA, IMAGE, expectations={
            "AMW_QUERY_ENDPOINT": "https://exact-amw.example",
            "AZURE_CLIENT_ID": "scoped-reader-id",
        })
        env = {item["name"]: item.get("value") for item in rendered["spec"]["steps"][0]["container"]["env"]}
        self.assertEqual(env.get("AMW_QUERY_ENDPOINT"), "https://exact-amw.example")
        self.assertEqual(env.get("AZURE_CLIENT_ID"), "scoped-reader-id")
        with self.assertRaises(ValueError):
            render_candidate(source + "\n---\n" + source, "operator", SHA, IMAGE)

    def test_passed_report_allows_selected_out_specs(self):
        self.assertEqual(report_counts(self.artifacts), {
            "specs": 2, "passed": 1, "skipped": 1, "failed": 0,
        })

    def test_missing_empty_failed_and_disagreeing_reports_fail(self):
        for change in ("empty", "all-skipped", "setup-only", "dry-run", "failed", "suite-failed", "junit-failed", "junit-count", "missing", "duplicate"):
            with self.subTest(change=change):
                self.suites = [{"SuiteSucceeded": True, "SpecReports": [{"State": "passed", "LeafNodeType": "It"}]}]
                self.junit = '<testsuites><testsuite><testcase name="[It] runs"/></testsuite></testsuites>'
                if change == "empty":
                    self.suites[0]["SpecReports"] = []
                elif change == "all-skipped":
                    self.suites[0]["SpecReports"][0]["State"] = "skipped"
                elif change == "setup-only":
                    self.suites[0]["SpecReports"][0]["LeafNodeType"] = "BeforeSuite"
                elif change == "dry-run":
                    self.suites[0]["SuiteConfig"] = {"DryRun": True}
                elif change == "failed":
                    self.suites[0]["SpecReports"].append({"State": "failed"})
                elif change == "suite-failed":
                    self.suites[0]["SuiteSucceeded"] = False
                elif change == "junit-failed":
                    self.junit = "<testsuite><testcase><failure/></testcase></testsuite>"
                elif change == "junit-count":
                    self.junit = '<testsuite><testcase name="[It] first"/><testcase name="[It] second"/></testsuite>'
                self.write_reports()
                nested = self.artifacts / "nested"
                if change == "missing":
                    (self.artifacts / "junit.xml").unlink()
                if change == "duplicate":
                    nested.mkdir()
                    (nested / "junit.xml").write_text(self.junit, encoding="utf-8")
                with self.assertRaises(ValueError):
                    report_counts(self.artifacts)

    def test_gate_requires_exact_scenarios_and_rechecks_retained_evidence(self):
        result = {
            "environment": "default-targets-on-v2", "status": "passed",
            "executions": [{
                "workflow": "defaultsettingsenabledv2", "execution_id": "exact-id",
                "status": "passed", "artifact_dir": str(self.artifacts),
                **report_counts(self.artifacts),
            }],
        }
        path = self.artifacts / "result.json"
        (self.artifacts.parent / "submission.json").write_text('{"id":"exact-id"}', encoding="utf-8")
        (self.artifacts.parent / "execution.json").write_text(json.dumps({
            "id": "exact-id", "workflow": {"name": "defaultsettingsenabledv2"},
            "result": {"status": "passed"},
        }), encoding="utf-8")
        path.write_text(json.dumps(result), encoding="utf-8")
        validate_summary([path], [result["environment"]])
        for change in ("missing", "duplicate", "wrong-scenario", "no-execution", "running", "wrong-count"):
            changed = copy.deepcopy(result)
            paths = [path]
            if change == "missing":
                paths = []
            elif change == "duplicate":
                paths += [path]
            elif change == "wrong-scenario":
                changed["environment"] = "another"
            elif change == "no-execution":
                changed["executions"] = []
            elif change == "running":
                changed["executions"][0]["status"] = "running"
            elif change == "wrong-count":
                changed["executions"][0]["passed"] = 20
            path.write_text(json.dumps(changed), encoding="utf-8")
            with self.subTest(change=change), self.assertRaises(ValueError):
                validate_summary(paths, [result["environment"]])
        path.write_text(json.dumps(result), encoding="utf-8")
        for field, value in [("id", "another-execution"), ("workflow", {"name": "another-workflow"}),
                             ("result", {"status": "running"})]:
            receipt = {"id": "exact-id", "workflow": {"name": "defaultsettingsenabledv2"},
                       "result": {"status": "passed"}}
            receipt[field] = value
            (self.artifacts.parent / "execution.json").write_text(json.dumps(receipt), encoding="utf-8")
            with self.subTest(field=field), self.assertRaises(ValueError):
                validate_summary([path], [result["environment"]])
        with self.assertRaises(ValueError):
            validate_summary([path], [result["environment"]], self.artifacts / "elsewhere")

    def test_pipeline_wires_v2_dcgm_and_a_complete_always_gate(self):
        source = (ROOT.parents[2] / ".pipelines" / "azure-pipeline-config-tests.yml").read_text(encoding="utf-8")
        pipeline = yaml.safe_load(source)
        steps = pipeline["stages"][0]["jobs"][0]["strategy"]["runOnce"]["deploy"]["steps"]
        gate = [step for step in steps if "workflow_evidence.py gate" in step.get("bash", "")]
        self.assertEqual(len(gate), 1)
        self.assertEqual(gate[0]["condition"], "always()")
        args = shlex.split(gate[0]["bash"].replace("\\\n", " "))
        expected = set(args[args.index("gate") + 2:])
        summaries = set(re.findall(r"testkube-results/testkube-results-([a-z0-9-]+)\.json", source))
        summaries.update({"customer-settings-v1", "customer-settings-v2"})
        self.assertEqual(expected, summaries)
        for cr, fixture in [
            ("testkube-config-test-default-targets-on-v2-crs.yaml", "ama-metrics-settings-configmap-defaults-targets-turned-on-v2.yaml"),
            ("testkube-config-test-dcgm-crs.yaml", "ama-metrics-settings-configmap-dcgm.yaml"),
        ]:
            run = [step["bash"] for step in steps if cr in step.get("bash", "")]
            self.assertEqual(len(run), 1)
            self.assertLess(run[0].index(fixture), run[0].index("run-testkube-workflow.sh"))
        for task in ("PublishTestResults@2", "PublishPipelineArtifact@1"):
            publication = [step for step in steps if step.get("task") == task]
            self.assertEqual(len(publication), 1)
            self.assertEqual(publication[0]["condition"], "always()")


if __name__ == "__main__":
    unittest.main()
