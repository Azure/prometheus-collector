import json
from pathlib import Path
import tempfile
import unittest

import yaml

from candidate_evidence import stage


class CandidateEvidenceTest(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory()
        self.addCleanup(self.temporary.cleanup)
        self.root = Path(self.temporary.name)
        self.evidence = self.root / "private"
        self.evidence.mkdir()
        self.output = self.root / "published"
        self.sha = "a" * 40

    def write(self, name, data):
        path = self.evidence / name
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(data, encoding="utf-8")

    def test_filters_credentials_but_retains_bound_execution_and_scan_evidence(self):
        self.write("kubeconfig", "NEVER PUBLISH")
        self.write(".azure/accessTokens.json", '{"accessToken":"NEVER PUBLISH"}')
        self.write("scans/baseline/cache/db/trivy.db", "NEVER PUBLISH")
        self.write("keys/client.key", "NEVER PUBLISH")
        self.write("secret.yaml", "kind: Secret\nstringData:\n  arbitrary: NEVER PUBLISH\n")
        self.write("result.json", json.dumps({"sourceSha": self.sha, "status": "FAIL"}))
        self.write("pipeline-verification/candidate-build.json", json.dumps({
            "source": {"commit": self.sha, "buildId": "4401", "repo": "Azure/prometheus-collector"},
        }))
        execution = "customer-settings-v1/configprocessing/testkube-executions/ConfigTests/workflow/exact-id/"
        self.write(execution + "execution.json", json.dumps({
            "id": "exact-id", "workflow": {"name": "workflow"}, "result": {"status": "failed"},
        }))
        self.write(execution + "artifacts/junit.xml", '<testsuite><testcase name="[It] test"><failure/></testcase></testsuite>')
        alias = "customer-settings-v1/alias-ingestion/testkube-executions/ConfigTests/querymetrics/alias-id/"
        self.write(alias + "execution.json", json.dumps({
            "id": "alias-id", "workflow": {"name": "querymetrics"}, "result": {"status": "passed"},
        }))
        self.write(alias + "artifacts/junit.xml", '<testsuite><testcase name="[It] alias"/></testsuite>')
        self.write("scans/comparison.json", '{"status":"FAIL","password":"arbitrary","source":"candidate"}')
        self.write("commands.log", "Bearer abc.secret.jwt\nBasic c2VjcmV0\npassword=unlisted-value\n"
                   "password: 'quoted secret'\nhttps://user:pass@example.invalid\n"
                   "signed?sig=abcdef&ok=1\nprotected-env-secret\n")
        manifest = stage(self.evidence, self.output, self.sha, 4401, {
            "SYSTEM_ACCESSTOKEN": "protected-env-secret", "BUILD_BUILDID": "9791",
            "BUILD_SOURCEVERSION": "b" * 40,
        })
        self.assertEqual(len(manifest["excluded"]), 5)
        self.assertEqual(manifest["producerBuildId"], 4401)
        self.assertEqual(manifest["verifiedSource"]["commit"], self.sha)
        self.assertEqual(manifest["validationBuildId"], "9791")
        self.assertEqual({item["id"] for item in manifest["executions"]}, {"exact-id", "alias-id"})
        self.assertTrue((self.output / execution / "artifacts/junit.xml").is_file())
        self.assertTrue((self.output / alias / "artifacts/junit.xml").is_file())
        text = "\n".join(path.read_text() for path in self.output.rglob("*") if path.is_file())
        for secret in ("NEVER PUBLISH", "abc.secret.jwt", "unlisted-value", "abcdef",
                       "protected-env-secret", '"arbitrary"', "c2VjcmV0", "quoted secret", "user:pass"):
            self.assertNotIn(secret, text)
        self.assertEqual(json.loads((self.output / "result.json").read_text())["status"], "FAIL")
        changed = next(item for item in manifest["files"] if item["path"] == "commands.log")
        self.assertNotEqual(changed["originalSha256"], changed["publishedSha256"])
        receipt = next(item for item in manifest["files"] if item["path"] == "pipeline-verification/candidate-build.json")
        self.assertEqual(receipt["originalSha256"], receipt["publishedSha256"])

    def test_rejects_stale_unbound_or_missing_evidence(self):
        self.write("result.json", '{"sourceSha":"' + "b" * 40 + '"}')
        with self.assertRaisesRegex(ValueError, "source differs"):
            stage(self.evidence, self.output, self.sha, 1, {})
        self.assertIn("publicationError", json.loads((self.output / "publication.json").read_text()))
        with self.assertRaisesRegex(ValueError, "must be new"):
            stage(self.evidence, self.output, self.sha, 1, {})
        with self.assertRaisesRegex(ValueError, "must be separate"):
            stage(self.evidence, self.evidence / "nested", self.sha, 1, {})
        with self.assertRaisesRegex(ValueError, "No candidate evidence"):
            stage(self.root / "absent", self.root / "missing-output", self.sha, 1, {})

    def test_pipeline_separates_legacy_and_candidate_and_always_retains_results(self):
        directory = Path(__file__).parent
        pipeline = yaml.safe_load((directory / "azure-pipeline-config-tests.yml").read_text())
        jobs = pipeline["stages"][0]["jobs"]
        self.assertIn("'False'", jobs[0]["condition"])
        self.assertIn("'True'", jobs[1]["condition"])
        self.assertEqual(jobs[1]["cancelTimeoutInMinutes"], 60)
        template = yaml.safe_load((directory / "azure-template-candidate-config-tests.yml").read_text())
        steps = template["steps"]
        run = next(i for i, step in enumerate(steps) if "Run authenticated candidate" in step.get("displayName", ""))
        cleanup = next(i for i, step in enumerate(steps) if "retry ownership" in step.get("displayName", ""))
        scan = next(i for i, step in enumerate(steps) if "SRE-owned authenticated" in step.get("displayName", ""))
        self.assertEqual(cleanup, run + 1)
        self.assertLess(cleanup, scan)
        self.assertEqual(steps[cleanup]["condition"], "always()")
        self.assertIn('$CANDIDATE_ROOT/otelcollector/test/scenarios/run-scenario.py', steps[run]["inputs"]["inlineScript"])
        self.assertIn('--receipt-tools-root "$BUILD_SOURCESDIRECTORY"', steps[run]["inputs"]["inlineScript"])
        self.assertLess(
            steps[run]["inputs"]["inlineScript"].index(".pipelines/pr-e2e/fetch_candidate.py"),
            steps[run]["inputs"]["inlineScript"].index("/otelcollector/test/scenarios/run-scenario.py"),
        )
        self.assertIn('rm -f "$CANDIDATE_EVIDENCE/kubeconfig"', steps[cleanup]["inputs"]["inlineScript"])
        for task in ("PublishTestResults@2", "PublishPipelineArtifact@1"):
            self.assertEqual(next(step for step in steps if step.get("task") == task)["condition"], "always()")
        junit = next(step for step in steps if step.get("task") == "PublishTestResults@2")["inputs"]
        self.assertEqual(junit["testResultsFiles"].splitlines(), [
            "*/configprocessing/testkube-executions/**/artifacts/**/junit.xml",
            "*/alias-ingestion/testkube-executions/**/artifacts/**/junit.xml",
            "health/**/artifacts/**/junit.xml",
        ])
        self.assertTrue(junit["failTaskOnFailedTests"])
        self.assertTrue(junit["failTaskOnMissingResultsFile"])


if __name__ == "__main__":
    unittest.main()
