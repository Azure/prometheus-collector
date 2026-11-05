import json
from pathlib import Path
import shlex
import subprocess
import tempfile
import time
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

    def test_bootstrap_publishes_live_and_watcher_stops_without_changing_exit_status(self):
        template = yaml.safe_load(Path(__file__).with_name("azure-template-candidate-config-tests.yml").read_text())
        run = next(step for step in template["steps"] if step.get("displayName", "").startswith("Run authenticated"))
        script = run["inputs"]["inlineScript"]
        watcher = script[script.index("publish_bootstrap() {"):script.index('"$PYTHON" -B "$CANDIDATE_ROOT/otelcollector/test/scenarios/run-scenario.py" run')]
        for variable, value in (("Build.BuildId", "979123"), ("System.StageAttempt", "1"), ("System.JobAttempt", "2")):
            watcher = watcher.replace(f"$({variable})", value)
        self.write("ownership.json", '{"token":"DO_NOT_UPLOAD"}')
        self.write("kubeconfig", "DO_NOT_UPLOAD")
        proof = self.evidence / "rbac-bootstrap.json"
        for publish, status in ((True, 23), (False, 0), (False, 143)):
            proof.unlink(missing_ok=True)
            command = "kill -TERM $$" if status == 143 else f"sleep 4; exit {status}"
            output_path = self.root / "watcher-output.txt"
            output_file = output_path.open("w")
            script_path = self.root / "watcher.sh"
            script_path.write_text(
                "set -euo pipefail\nCANDIDATE_EVIDENCE=" + shlex.quote(self.evidence.as_posix()) +
                "\nCANDIDATE_SOURCE_SHA=source\nPRODUCER_BUILD_ID=131161\nCANDIDATE_SCENARIO=default\n" + watcher +
                '\nprintf "%s" "$bootstrap_watcher" > "$CANDIDATE_EVIDENCE/watcher.pid"\n' + command,
                encoding="utf-8",
            )
            process = subprocess.Popen(
                ["bash", script_path.as_posix()],
                stdout=output_file, stderr=subprocess.PIPE, text=True,
            )
            try:
                if publish:
                    self.write("rbac-bootstrap.json", '{"status":')
                    time.sleep(0.3)
                    self.assertIsNone(process.poll())
                    self.assertEqual(output_path.read_text(), "")
                    self.write("rbac-bootstrap.json", '{"status":"waiting","scope":"/subscriptions/test/resourceGroups/new","principalId":"principal","token":"DO_NOT_UPLOAD"}')
                    deadline = time.monotonic() + 8
                    while not output_path.read_text() and process.poll() is None and time.monotonic() < deadline:
                        time.sleep(0.1)
                    line = output_path.read_text()
                    if process.poll() is not None:
                        self.fail("Publication must precede harness completion: " + process.communicate(timeout=10)[1])
                    public = self.evidence / "rbac-bootstrap-public.json"
                    self.assertEqual(line.strip(), "##vso[artifact.upload containerfolder=rbac-bootstrap;artifactname=candidate-rbac-bootstrap-979123-1-2]" + public.as_posix())
                    data = json.loads(public.read_text())
                    self.assertNotIn("DO_NOT_UPLOAD", public.read_text())
                    self.assertEqual((data["validationBuildId"], data["candidateSourceSha"], data["producerBuildId"], data["scenario"]), ("979123", "source", "131161", "default"))
                _, errors = process.communicate(timeout=10)
                self.assertEqual(process.returncode, status, errors)
                if not publish:
                    self.assertEqual(output_path.read_text(), "")
                watcher_pid = (self.evidence / "watcher.pid").read_text()
                self.assertNotEqual(subprocess.run(["bash", "-c", f"kill -0 {watcher_pid} 2>/dev/null"]).returncode, 0)
            finally:
                if process.poll() is None:
                    process.kill()
                    process.communicate()
                output_file.close()

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
        execution = "customer-transitions-v1/configprocessing/testkube-executions/ConfigTests/workflow/exact-id/"
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
        early = json.loads((self.root / "missing-output" / "publication.json").read_text())
        self.assertEqual((early["publicationError"], early["executions"]), ("No candidate evidence was produced", []))

    def test_pipeline_separates_legacy_and_candidate_and_always_retains_results(self):
        directory = Path(__file__).parent
        pipeline = yaml.safe_load((directory / "azure-pipeline-config-tests.yml").read_text())
        jobs = pipeline["stages"][0]["jobs"]
        self.assertIn("'False'", jobs[0]["condition"])
        self.assertIn("'True'", jobs[1]["condition"])
        self.assertEqual(jobs[1]["cancelTimeoutInMinutes"], 60)
        self.assertEqual(jobs[1]["${{ if eq(parameters.candidateScenario, 'settings') }}"]["timeoutInMinutes"], 900)
        self.assertEqual(jobs[1]["${{ else }}"]["timeoutInMinutes"], 600)
        scenario = next(item for item in pipeline["parameters"] if item["name"] == "candidateScenario")
        self.assertEqual(scenario["default"], "default")
        self.assertEqual(scenario["values"], ["default", "settings"])
        include = jobs[1]["strategy"]["runOnce"]["deploy"]["steps"][0]
        self.assertEqual(include["parameters"]["scenario"], "${{ parameters.candidateScenario }}")
        template = yaml.safe_load((directory / "azure-template-candidate-config-tests.yml").read_text())
        scenario = next(item for item in template["parameters"] if item["name"] == "scenario")
        self.assertEqual(scenario["default"], "default")
        self.assertEqual(scenario["values"], ["default", "settings"])
        steps = template["steps"]
        prereqs = next(i for i, step in enumerate(steps) if step.get("displayName") == "Install candidate host prerequisites")
        helm = next(i for i, step in enumerate(steps) if step.get("task") == "HelmInstaller@1")
        self.assertEqual(steps[helm]["inputs"]["helmVersionToInstall"], "3.19.0")
        self.assertLess(prereqs, helm)
        for required in ("unzip jq gettext-base libicu70", "bicep --version", "command -v az || curl --fail --silent --show-error --location --retry 3 https://aka.ms/InstallAzureCLIDeb | sudo bash",
                         'sudo install -m 0755 "${AZURE_CONFIG_DIR:-$HOME/.azure}/bin/bicep" /usr/local/bin/bicep',
                         "##vso[task.setvariable variable=AZURE_BICEP_USE_BINARY_FROM_PATH]true",
                         "29818b6729dc9262b692307470d0558d027725e738be5dd47c760261868a2634",
                         "for tool in unzip az kubectl jq envsubst"):
            self.assertIn(required, steps[prereqs]["bash"])
        run =  next(i for i, step in enumerate(steps) if "Run authenticated candidate" in step.get("displayName", ""))
        cleanup = next(i for i, step in enumerate(steps) if "retry ownership" in step.get("displayName", ""))
        scan = next(i for i, step in enumerate(steps) if "SRE-owned authenticated" in step.get("displayName", ""))
        self.assertEqual(cleanup, run + 1)
        self.assertLess(cleanup, scan)
        self.assertEqual(steps[cleanup]["condition"], "always()")
        self.assertIn('$CANDIDATE_ROOT/otelcollector/test/scenarios/run-scenario.py', steps[run]["inputs"]["inlineScript"])
        self.assertIn('--receipt-tools-root "$BUILD_SOURCESDIRECTORY"', steps[run]["inputs"]["inlineScript"])
        self.assertIn('--scenario "$CANDIDATE_SCENARIO"', steps[run]["inputs"]["inlineScript"])
        self.assertEqual(steps[run]["env"]["CANDIDATE_SCENARIO"], "${{ parameters.scenario }}")
        self.assertEqual(steps[run]["${{ if eq(parameters.scenario, 'settings') }}"]["timeoutInMinutes"], 540)
        self.assertEqual(steps[run]["${{ else }}"]["timeoutInMinutes"], 300)
        self.assertIn('settings) max_run_seconds=28800 ;; default) max_run_seconds=14400', steps[run]["inputs"]["inlineScript"])
        self.assertIn('--max-run-seconds "$max_run_seconds"', steps[run]["inputs"]["inlineScript"])
        self.assertIn('case "$CANDIDATE_SCENARIO" in default|settings)', steps[1]["bash"])
        self.assertEqual(steps[1]["env"]["CANDIDATE_SCENARIO"], "${{ parameters.scenario }}")
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
