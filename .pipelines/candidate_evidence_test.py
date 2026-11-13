import json
import os
from pathlib import Path
import shlex
import subprocess
import sys
import tempfile
import time
import unittest
from unittest.mock import patch

import yaml

from candidate_evidence import stage
from candidate_coverage import SCENARIOS, plan


class CandidateEvidenceTest(unittest.TestCase):
    def test_candidate_installers_wait_for_dpkg_lock(self):
        root = Path(__file__).parent
        pipeline = yaml.safe_load((root / "azure-pipeline-config-tests.yml").read_text())
        capacity = next(stage for stage in pipeline["stages"] if stage.get("stage") == "CandidateCapacity")
        prepare = next(step["bash"] for step in capacity["jobs"][0]["steps"]
                       if step.get("displayName") == "Prepare exact-source read-only capacity tools")
        template = yaml.safe_load((root / "azure-template-candidate-config-tests.yml").read_text())
        install = next(step["bash"] for step in template["steps"]
                       if step.get("displayName") == "Install candidate host prerequisites")
        for script in (prepare, install):
            with self.subTest(script=script):
                config = script.index('DPkg::Lock::Timeout "300";')
                self.assertIn("/etc/apt/apt.conf.d/99ama-metrics-lock-timeout", script)
                self.assertLess(config, script.index("https://aka.ms/InstallAzureCLIDeb"))
                if "sudo apt-get" in script:
                    self.assertLess(config, script.index("sudo apt-get"))

    def test_capacity_failure_retains_diagnostic_without_admission_claim(self):
        pipeline = yaml.safe_load(Path(__file__).with_name("azure-pipeline-config-tests.yml").read_text())
        capacity = next(stage for stage in pipeline["stages"] if stage.get("stage") == "CandidateCapacity")
        steps = capacity["jobs"][0]["steps"]
        diagnostic = next(step for step in steps
                          if step.get("displayName") == "Retain capacity job diagnostic even when setup fails")
        self.assertEqual(diagnostic["condition"], "always()")
        publish = next(step for step in steps if step.get("task") == "PublishPipelineArtifact@1")
        self.assertLess(steps.index(diagnostic), steps.index(publish))
        with tempfile.TemporaryDirectory() as directory:
            evidence = Path(directory) / "not-created-by-failed-setup"
            result = subprocess.run(["bash", "-c", diagnostic["bash"]], capture_output=True, text=True,
                                    env={**os.environ, "CAPACITY_EVIDENCE": evidence.as_posix(),
                                         "AGENT_JOBSTATUS": "Failed"})
            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertIn("Failed", (evidence / "job-status.txt").read_text())
            self.assertIn("not capacity admission proof", (evidence / "job-status.txt").read_text())
            self.assertFalse((evidence / "campaign-preflight.json").exists())
            self.assertNotIn("task.setvariable", result.stdout)

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

    def test_pr_candidate_gate_binds_allowed_head(self):
        template = yaml.safe_load(Path(__file__).with_name("azure-template-candidate-config-tests.yml").read_text())
        step = next(step for step in template["steps"] if "Validate opt-in inputs" in step.get("displayName", ""))
        gate = step["bash"].split('[[ "$SYSTEM_JOBATTEMPT"', 1)[0]
        for reason, branch, head, candidate, passes in (
            ("Manual", "", "", self.sha, True),
            ("ResourceTrigger", "", "", self.sha, True),
            ("PullRequest", "refs/heads/sre/vuln/campaign", self.sha, self.sha, True),
            ("PullRequest", "refs/heads/grwehner/ama-metrics-sre-vuln-poc", self.sha, self.sha, True),
            ("PullRequest", "refs/heads/untrusted", self.sha, self.sha, False),
            ("PullRequest", "refs/heads/sre/vuln/campaign", self.sha, "b" * 40, False),
        ):
            with self.subTest(reason=reason, branch=branch, candidate=candidate):
                result = subprocess.run(
                    ["bash", "-c", gate], capture_output=True, text=True,
                    env={**os.environ, "BUILD_REASON": reason, "PR_SOURCE_BRANCH": branch,
                         "PR_SOURCE_COMMIT": head, "CANDIDATE_SOURCE_SHA": candidate},
                )
                self.assertEqual(result.returncode == 0, passes, result.stderr)

    def test_candidate_shell_selector_accepts_all_applicable_rows_only(self):
        template = yaml.safe_load(Path(__file__).with_name("azure-template-candidate-config-tests.yml").read_text())
        validation = next(step["bash"] for step in template["steps"]
                          if "Validate opt-in inputs" in step.get("displayName", ""))
        gate = 'git() { printf "%s\\n" "$TEST_SOURCE_SHA"; }\n' + validation.split('[[ ! -e "$CANDIDATE_ROOT"', 1)[0]
        run = next(step["inputs"]["inlineScript"] for step in template["steps"]
                   if step.get("displayName", "").startswith("Run authenticated"))
        duration = run[run.index('case "$CANDIDATE_SCENARIO"'):run.index("CANDIDATE_RUN_ID=")]
        for scenario in (*SCENARIOS, "customer-helm", "operator-secret-watch", "all", "unknown"):
            for script in (gate, "set -euo pipefail\n" + duration):
                with self.subTest(scenario=scenario, script=script):
                    result = subprocess.run(["bash", "-c", script], capture_output=True, text=True,
                                            env={**os.environ, "BUILD_REASON": "Manual",
                                                 "SYSTEM_JOBATTEMPT": "1", "SYSTEM_STAGEATTEMPT": "1",
                                                 "PRODUCER_BUILD_ID": "4401", "CANDIDATE_SOURCE_SHA": self.sha,
                                                 "CAPACITY_SOURCE_SHA": self.sha, "CAPACITY_BUILD_ID": "9791",
                                                 "TEST_SOURCE_SHA": self.sha, "CAPACITY_TEST_SOURCE_SHA": self.sha,
                                                 "BUILD_SOURCESDIRECTORY": "unused",
                                                 "BUILD_BUILDID": "9791",
                                                 "CANDIDATE_SCENARIO": scenario})
                    self.assertEqual(result.returncode == 0, scenario in SCENARIOS, result.stderr)

    def test_bootstrap_storage_is_isolated_by_scenario_and_attempt(self):
        template = yaml.safe_load(Path(__file__).with_name("azure-template-candidate-config-tests.yml").read_text())
        run = next(step for step in template["steps"] if step.get("displayName", "").startswith("Run authenticated"))
        upload = next(line.strip() for line in run["inputs"]["inlineScript"].splitlines()
                      if "##vso[artifact.upload " in line)
        for scenario in SCENARIOS:
            for stage_attempt, job_attempt in (("1", "1"), ("1", "2"), ("2", "1")):
                with self.subTest(scenario=scenario, stage=stage_attempt, job=job_attempt):
                    command = upload
                    for variable, value in (("Build.BuildId", "979123"),
                                            ("System.StageAttempt", stage_attempt),
                                            ("System.JobAttempt", job_attempt)):
                        command = command.replace(f"$({variable})", value)
                    result = subprocess.run(
                        ["bash", "-c", "set -eu\n" + command], capture_output=True, text=True,
                        env={**os.environ, "CANDIDATE_SCENARIO": scenario,
                             "public": "rbac-bootstrap-public.json"},
                    )
                    self.assertEqual(result.returncode, 0, result.stderr)
                    folder = result.stdout.split("containerfolder=", 1)[1].split(";", 1)[0]
                    self.assertEqual(folder, f"candidate-rbac-bootstrap-979123-{scenario}-{stage_attempt}-{job_attempt}")

    def test_bootstrap_publishes_live_and_watcher_stops_without_changing_exit_status(self):
        template = yaml.safe_load(Path(__file__).with_name("azure-template-candidate-config-tests.yml").read_text())
        run = next(step for step in template["steps"] if step.get("displayName", "").startswith("Run authenticated"))
        script = run["inputs"]["inlineScript"]
        watcher = script[script.index("publish_bootstrap() {"):script.index('"$PYTHON" -B "$BUILD_SOURCESDIRECTORY/otelcollector/test/scenarios/run-scenario.py" run')]
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
                "\nCANDIDATE_SOURCE_SHA=source\nPRODUCER_BUILD_ID=131161\nCANDIDATE_SCENARIO=default\nCANDIDATE_RUN_ID=12345678-1234-1234-1234-123456789abc\n" + watcher +
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
                    self.assertEqual(line.strip(), "##vso[artifact.upload containerfolder=candidate-rbac-bootstrap-979123-default-1-2;artifactname=candidate-rbac-bootstrap-979123-default-1-2]" + public.as_posix())
                    data = json.loads(public.read_text())
                    self.assertNotIn("DO_NOT_UPLOAD", public.read_text())
                    self.assertEqual((data["validationBuildId"], data["candidateSourceSha"], data["producerBuildId"], data["scenario"]), ("979123", "source", "131161", "default"))
                    self.assertEqual(data["runId"], "12345678-1234-1234-1234-123456789abc")
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
        self.assertEqual(len(jobs), 1)
        capacity = pipeline["stages"][1]
        self.assertEqual(capacity["stage"], "CandidateCapacity")
        candidate_stage = next(iter(pipeline["stages"][2].values()))[0]
        candidate_stage = next(iter(candidate_stage.values()))[0]
        jobs.append(candidate_stage["jobs"][0])
        self.assertIn("'False'", jobs[0]["condition"])
        self.assertIn("ResourceTrigger", jobs[0]["condition"])
        self.assertIn("PullRequest", jobs[0]["condition"])
        self.assertIn("'True'", capacity["condition"])
        self.assertIn("ResourceTrigger", capacity["condition"])
        self.assertIn("PullRequest", capacity["condition"])
        self.assertTrue(candidate_stage["condition"].startswith("and(not(canceled()),"))
        self.assertIn("eq(dependencies.CandidateCapacity.result, 'Succeeded')", candidate_stage["condition"])
        self.assertEqual(candidate_stage["dependsOn"][0], "CandidateCapacity")
        self.assertNotIn("dependsOn", jobs[1])
        self.assertEqual(pipeline["trigger"], "none")
        self.assertEqual(pipeline["pr"]["branches"]["include"], ["main"])
        self.assertNotIn("paths", pipeline["pr"])
        self.assertIn("startsWith(variables['System.PullRequest.SourceBranch'], 'refs/heads/sre/vuln/')", capacity["condition"])
        loop = next(iter(pipeline["stages"][2]))
        self.assertIn("split('" + ",".join(SCENARIOS) + "', ',')", loop)
        self.assertIn("replace(scenario, '-', '_')", candidate_stage["stage"])
        selection = next(iter(next(iter(pipeline["stages"][2].values()))[0]))
        self.assertIn("eq(parameters.candidateScenario, 'all')", selection)
        self.assertIn("eq(parameters.candidateScenario, scenario)", selection)
        self.assertEqual(candidate_stage["condition"],
                         "and(not(canceled()), eq(dependencies.CandidateCapacity.result, 'Succeeded'))")
        self.assertIn("${{ scenario }}", jobs[1]["variables"]["candidateAttempt"])
        self.assertEqual(jobs[1]["environment"], "Prometheus-Collector-Tests")
        producer = pipeline["resources"]["pipelines"][0]
        self.assertEqual(producer["source"], "Azure.prometheus-collector")
        self.assertEqual(producer["trigger"]["branches"]["include"],
                         ["sre/vuln/*", "*/ama-metrics-sre-*"])
        manual_variables = pipeline["variables"]["${{ if eq(parameters.candidateMode, true) }}"]
        automatic_variables = pipeline["variables"]["${{ else }}"]
        self.assertEqual(manual_variables["campaignProducerBuildId"],
                         "${{ parameters.candidateBuildId }}")
        self.assertEqual(manual_variables["campaignSourceSha"],
                         "${{ parameters.candidateSourceSha }}")
        self.assertEqual(automatic_variables["campaignProducerBuildId"],
                         "$(resources.pipeline.campaignProducer.runID)")
        self.assertEqual(automatic_variables["campaignSourceSha"],
                         "$(resources.pipeline.campaignProducer.sourceCommit)")
        self.assertEqual(jobs[1]["cancelTimeoutInMinutes"], 60)
        self.assertEqual(jobs[1]["timeoutInMinutes"], 900)
        scenario = next(item for item in pipeline["parameters"] if item["name"] == "candidateScenario")
        self.assertEqual(scenario["default"], "all")
        self.assertEqual(set(scenario["values"]), {"all", *SCENARIOS})
        cve = next(item for item in pipeline["parameters"] if item["name"] == "candidateCve")
        self.assertEqual(cve["default"], "")
        campaign = json.loads((directory / "pr-e2e" / "campaign.json").read_text())
        self.assertEqual(campaign, {"schemaVersion": 1, "cve": "CVE-2026-63209"})
        include = jobs[1]["strategy"]["runOnce"]["deploy"]["steps"][0]
        self.assertEqual(include["parameters"]["scenario"], "${{ scenario }}")
        self.assertEqual(include["parameters"]["cve"], "$(candidateCve)")
        self.assertEqual(include["parameters"]["producerBuildId"], "$(campaignProducerBuildId)")
        self.assertEqual(include["parameters"]["sourceSha"], "$(campaignSourceSha)")
        template = yaml.safe_load((directory / "azure-template-candidate-config-tests.yml").read_text())
        scenario = next(item for item in template["parameters"] if item["name"] == "scenario")
        self.assertEqual(scenario["default"], "default")
        self.assertEqual(set(scenario["values"]), set(SCENARIOS))
        steps = template["steps"]
        pr_resolution = next(step for step in steps if "Resolve successful 440" in step.get("displayName", ""))
        self.assertIn("System.PullRequest.SourceCommitId", str(pr_resolution["env"]))
        self.assertIn("--resolve-producer", pr_resolution["bash"])
        self.assertIn("task.setvariable variable=campaignSourceSha", pr_resolution["bash"])
        self.assertLess(steps.index(pr_resolution), next(
            i for i, step in enumerate(steps) if "Validate opt-in inputs" in step.get("displayName", "")))
        input_validation = next(step for step in steps if "Validate opt-in inputs" in step.get("displayName", ""))
        self.assertIn("CANDIDATE_CVE", input_validation["bash"])
        self.assertIn("CANDIDATE_CVE", input_validation["env"])
        self.assertEqual(input_validation["env"]["BUILD_REASON"], "$(Build.Reason)")
        self.assertIn('"$CANDIDATE_SOURCE_SHA" == "$PR_SOURCE_COMMIT"', input_validation["bash"])
        self.assertIn('"$CAPACITY_SOURCE_SHA" == "$CANDIDATE_SOURCE_SHA"', input_validation["bash"])
        self.assertIn('"$CAPACITY_BUILD_ID" == "$BUILD_BUILDID"', input_validation["bash"])
        self.assertNotIn("Candidate execution requires a trusted pipeline branch", input_validation["bash"])
        self.assertIn("campaign.json", input_validation["bash"])
        self.assertLess(input_validation["bash"].index("git fetch"), input_validation["bash"].index("git show"))
        self.assertLess(input_validation["bash"].index("git show"), input_validation["bash"].index("task.setvariable"))
        self.assertLess(input_validation["bash"].index("task.setvariable"), input_validation["bash"].index("git worktree add"))
        scan = next(step for step in steps if "baseline/candidate scan comparison" in step.get("displayName", ""))
        self.assertIn('--cve "$CANDIDATE_CVE"', scan["inputs"]["inlineScript"])
        self.assertEqual(scan["env"]["CANDIDATE_CVE"], "$(candidateCve)")
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
        self.assertNotIn(
            "Always retry ownership-guarded candidate cleanup before scanning",
            [step.get("displayName", "") for step in steps],
        )
        self.assertFalse(any(
            "run-scenario.py" in str(step) and " cleanup" in str(step)
            for step in steps
        ))
        cleanup = next(i for i, step in enumerate(steps) if "Always remove private candidate kubeconfig" in step.get("displayName", ""))
        scan = next(i for i, step in enumerate(steps) if "SRE-owned authenticated" in step.get("displayName", ""))
        self.assertEqual(cleanup, run + 1)
        self.assertLess(cleanup, scan)
        self.assertEqual(steps[cleanup]["condition"], "always()")
        self.assertIn('rm -f -- "$CANDIDATE_EVIDENCE/kubeconfig"', steps[cleanup]["bash"])
        self.assertIn('$BUILD_SOURCESDIRECTORY/otelcollector/test/scenarios/run-scenario.py', steps[run]["inputs"]["inlineScript"])
        run_script = steps[run]["inputs"]["inlineScript"]
        self.assertIn('--source-root "$CANDIDATE_ROOT" --test-source-root "$BUILD_SOURCESDIRECTORY" --test-source-sha "$TEST_SOURCE_SHA"', run_script)
        self.assertIn('"$TEST_SOURCE_SHA" == "$CAPACITY_TEST_SOURCE_SHA"', run_script)
        exit_handler = run_script[run_script.index("stop_bootstrap_watcher() {"):run_script.index("trap 'exit 130'")]
        self.assertIn("trap stop_bootstrap_watcher EXIT", run_script)
        self.assertIn('rm -f -- "$CANDIDATE_EVIDENCE/kubeconfig"', exit_handler)
        self.assertIn('--receipt-tools-root "$BUILD_SOURCESDIRECTORY"', run_script)
        self.assertIn('--scenario "$CANDIDATE_SCENARIO"', run_script)
        self.assertIn('--run-id "$CANDIDATE_RUN_ID"', run_script)
        self.assertIn("uuid.uuid4()", run_script)
        self.assertIn('--admitted-existing "$CANDIDATE_EXISTING"', run_script)
        self.assertIn('--location "$CANDIDATE_LOCATION"', run_script)
        self.assertIn('"$CANDIDATE_LOCATION" == centralus || "$CANDIDATE_LOCATION" == eastus2', run_script)
        self.assertEqual(steps[run]["env"]["CANDIDATE_LOCATION"], "$(candidateLocation)")
        self.assertIn("9b96ebbd-c57a-42d1-bbe9-b69296e4c7fb", run_script)
        self.assertEqual(steps[run]["env"]["CANDIDATE_SCENARIO"], "${{ parameters.scenario }}")
        self.assertEqual(steps[run]["timeoutInMinutes"], 540)
        self.assertIn('default|classic-azure-cni|acns|otlp|private|proxy|proxy-custom-ca) max_run_seconds=28800', run_script)
        self.assertIn('--max-run-seconds "$max_run_seconds"', steps[run]["inputs"]["inlineScript"])
        self.assertIn('case "$CANDIDATE_SCENARIO" in default|classic-azure-cni|acns|otlp|private|proxy|proxy-custom-ca)', input_validation["bash"])
        self.assertIn(".pipelines/candidate_coverage.py", input_validation["bash"])
        self.assertIn('"$CANDIDATE_EVIDENCE/coverage.json"', input_validation["bash"])
        self.assertEqual(input_validation["env"]["CANDIDATE_SCENARIO"], "${{ parameters.scenario }}")
        scan_script = steps[scan]["inputs"]["inlineScript"]
        self.assertLess(
            scan_script.index(".pipelines/pr-e2e/fetch_candidate.py"),
            scan_script.index(".pipelines/pr-e2e/scan_candidate.py"),
        )
        self.assertIn("sha256:195b238b026fcf7cfef5fddf66d4ea8b305ad093d99171b3079089a665030324", scan_script)
        self.assertNotIn("sha256:6d284910117ad790e81f1bd6c7de25f2a798b8efbc11642f3c8bff91c044db99", scan_script)
        self.assertIn('--output "$CANDIDATE_EVIDENCE/pipeline-verification"', scan_script)
        self.assertIn('--verified-candidate "$CANDIDATE_EVIDENCE/pipeline-verification"', scan_script)
        self.assertNotIn("scan_trivy.py", scan_script)
        prepare = next(step for step in steps if "Prepare scoped pinned CLI" in step.get("displayName", ""))
        self.assertEqual(prepare["env"]["CANDIDATE_TRIVY_VERSION"], "$(candidateTrivyVersion)")
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

    def test_comparison_requires_installed_host_and_scan_tools(self):
        steps = yaml.safe_load(
            (Path(__file__).parent / "azure-template-candidate-config-tests.yml").read_text()
        )["steps"]
        host = next(i for i, step in enumerate(steps)
                    if step.get("displayName") == "Install candidate host prerequisites")
        fetch = next(i for i, step in enumerate(steps)
                     if step.get("displayName") == "Fetch exact producer evidence and Trivy version")
        prepare = next(i for i, step in enumerate(steps)
                       if "Prepare scoped pinned CLI" in step.get("displayName", ""))
        comparison = next(step for step in steps
                          if "baseline/candidate scan comparison" in step.get("displayName", ""))
        self.assertLess(host, fetch)
        self.assertLess(fetch, prepare)
        ready = "##vso[task.setvariable variable=candidateScanPrerequisitesReady]true"
        self.assertTrue(steps[prepare]["bash"].rstrip().endswith(f'echo "{ready}"'))
        self.assertEqual(
            comparison["condition"],
            "and(always(), not(canceled()), eq(variables['candidateScanPrerequisitesReady'], 'true'))",
        )

    def test_candidate_coverage_accounts_for_every_row_and_variant(self):
        root = Path(__file__).parents[1] / "otelcollector" / "test" / "scenarios"
        matrix = json.loads((root / "matrix.json").read_text())
        rows = {row["id"]: row for row in matrix["scenarios"]}
        self.assertFalse({"linux", "mixed", "uai"} & rows.keys())
        self.assertFalse({"autoscaler", "cilium", "kubenet"} & rows.keys())
        self.assertEqual(SCENARIOS, ("default", "classic-azure-cni", "acns", "otlp",
                                    "private", "proxy", "proxy-custom-ca"))
        pipeline_path = Path(__file__).with_name("azure-pipeline-config-tests.yml")
        pipeline = yaml.safe_load(pipeline_path.read_text())
        pipeline_scenarios = next(p["values"] for p in pipeline["parameters"]
                                  if p.get("name") == "candidateScenario")
        self.assertEqual(pipeline_scenarios, ["all", *SCENARIOS])
        self.assertIn("split('" + ",".join(SCENARIOS) + "', ',')", pipeline_path.read_text())
        template = yaml.safe_load(Path(__file__).with_name(
            "azure-template-candidate-config-tests.yml").read_text())
        self.assertEqual(next(p["values"] for p in template["parameters"]
                              if p.get("name") == "scenario"), list(SCENARIOS))
        self.assertNotIn("settings", rows)
        self.assertTrue({"fips", "arm64", "network-profile"} <= set(rows["default"]["ginkgoLabels"]))
        self.assertEqual(rows["default"]["settingsVariants"], ["*"])
        self.assertEqual(rows["classic-azure-cni"]["settingsVariants"], ["*"])
        self.assertNotIn("blockedReason", rows["acns"])
        self.assertIn("managed-acns", rows["acns"]["ginkgoLabels"])
        self.assertTrue({"network-profile", "mip-enabled"} <= set(rows["acns"]["ginkgoLabels"]))
        self.assertEqual(rows["acns"]["settingsVariants"], ["*"])
        self.assertEqual(rows["otlp"]["settingsVariants"], ["*"])
        self.assertEqual(rows["otlp"]["platforms"], ["linux"])
        self.assertIn("acns-cilium", rows)
        self.assertIn("blockedReason", rows["acns-cilium"])
        for scenario in SCENARIOS:
            result = plan(root, scenario)
            self.assertFalse(result["executed"])
            self.assertEqual(result["status"], "PLANNED_WITH_BLOCKERS")
            self.assertEqual(result["selectedScenario"], scenario)
            self.assertEqual(set(result["applicableScenarios"]), set(SCENARIOS))
            self.assertEqual([item["id"] for item in result["blockedTestSuites"]],
                             ["arc-conformance", "ccp-health", "regionTests"])
            self.assertTrue(all(item["status"] == "BLOCKED" for item in result["blockedTestSuites"]))
            blocked = {item["id"] for item in result["blockedScenarios"]}
            self.assertEqual(len(blocked | set(SCENARIOS)), len(rows))
            self.assertIn("customer-helm", blocked)
            self.assertIn("operator-secret-watch", blocked)
            self.assertEqual({item["id"] for item in result["blockedVariants"]}, {
                "helm-customer-values", "empty-secret-namespaces-v1", "empty-secret-namespaces-v2",
                "omitted-secret-namespaces-v1", "omitted-secret-namespaces-v2",
            })
        with patch("candidate_coverage.SCENARIOS", SCENARIOS[1:]):
            with self.assertRaisesRegex(ValueError, "every runnable matrix row"):
                plan(root, "default")
        with self.assertRaisesRegex(ValueError, "Unsupported candidate"):
            plan(root, "customer-helm")

    def test_profile_behavior_specs_are_selected_with_a_long_enough_querymetrics_timeout(self):
        root = Path(__file__).parents[1]
        source = (root / "otelcollector" / "test" / "ginkgo-e2e" / "querymetrics" /
                 "query_metrics_test.go").read_text()
        self.assertIn('Label("autoscaler")', source)
        self.assertIn('Label("network-profile")', source)
        self.assertIn('Label("managed-acns")', source)
        workflows = yaml.safe_load_all((root / "otelcollector" / "test" / "testkube" /
                                        "testkube-test-crs.yaml").read_text())
        querymetrics = next(doc for doc in workflows
                            if doc["metadata"]["name"] == "querymetrics")
        script = querymetrics["spec"]["steps"][0]["shell"]
        self.assertIn("--timeout=90m", script)

    def test_candidate_coverage_gate_fails_and_preserves_blocker_report(self):
        root = Path(__file__).parents[1] / "otelcollector" / "test" / "scenarios"
        script = Path(__file__).with_name("candidate_coverage.py")
        with tempfile.TemporaryDirectory() as directory:
            output = Path(directory) / "coverage.json"
            result = subprocess.run(
                [sys.executable, str(script), "--root", str(root), "--scenario", "default",
                 "--output", str(output), "--require-unblocked"],
                capture_output=True, text=True,
            )
            self.assertNotEqual(result.returncode, 0)
            self.assertIn("Candidate coverage gate blocked:", result.stderr)
            self.assertTrue(output.exists(), result.stderr)
            report = json.loads(output.read_text())
            self.assertEqual(report["status"], "PLANNED_WITH_BLOCKERS")
            self.assertEqual(report, plan(root, "default"))

    def test_candidate_coverage_gate_runs_after_tests_and_before_evidence_publish(self):
        steps = yaml.safe_load(
            (Path(__file__).parent / "azure-template-candidate-config-tests.yml").read_text()
        )["steps"]
        results = next(i for i, step in enumerate(steps) if step.get("task") == "PublishTestResults@2")
        gate = next(i for i, step in enumerate(steps)
                    if step.get("displayName") == "Gate candidate approval on complete scenario coverage")
        artifact = next(i for i, step in enumerate(steps) if step.get("task") == "PublishPipelineArtifact@1")
        self.assertLess(results, gate)
        self.assertLess(gate, artifact)
        self.assertEqual(steps[gate]["condition"], "always()")
        self.assertIn("--require-unblocked", steps[gate]["bash"])
        self.assertIn(".pipelines/candidate_coverage.py", steps[gate]["bash"])
        self.assertEqual(steps[gate]["env"]["CANDIDATE_SCENARIO"], "${{ parameters.scenario }}")

    def test_candidate_scenarios_depend_only_on_aggregate_capacity(self):
        pipeline = yaml.safe_load(Path(__file__).with_name("azure-pipeline-config-tests.yml").read_text())
        expansion = next(stage for stage in pipeline["stages"]
                         if any(key.startswith("${{ each scenario") for key in stage))
        loop = expansion[next(key for key in expansion if key.startswith("${{ each scenario"))]
        conditional = loop[0]
        stage = conditional[next(key for key in conditional if key.startswith("${{ if or("))][0]
        self.assertEqual(stage["dependsOn"], ["CandidateCapacity"])

    def test_capacity_output_contract_rejects_missing_or_mismatched_admission(self):
        pipeline = yaml.safe_load(Path(__file__).with_name("azure-pipeline-config-tests.yml").read_text())
        capacity = pipeline["stages"][1]
        task = next(step for step in capacity["jobs"][0]["steps"] if step.get("name") == "admit")
        script = task["inputs"]["inlineScript"]
        self.assertIn("run-scenario.py\" campaign-preflight", script)
        self.assertIn('--build-id "$PRODUCER_BUILD_ID"', script)
        self.assertLess(script.index('run-scenario.py" campaign-preflight'),
                        script.index("isOutput=true"))
        parser = script.split("<<'PY'\n", 1)[1].rsplit("\nPY", 1)[0]
        proof = {
            "status": "PASS", "sourceSha": self.sha, "testSourceSha": "c" * 40,
            "buildId": 4401, "capacityReserved": False,
            "placements": {
                name: ("eastus2" if name in {"autoscaler", "acns"} else "centralus")
                for name in SCENARIOS if name not in ("private", "proxy", "proxy-custom-ca")
            },
            "existing": {name: False for name in SCENARIOS if name not in ("private", "proxy", "proxy-custom-ca")},
            "regions": {"centralus": {}, "eastus2": {}},
        }
        cases = [("valid", {}, True), ("failed", {"status": "FAIL"}, False),
                 ("source", {"sourceSha": "b" * 40}, False), ("build", {"buildId": 4402}, False),
                 ("test-source", {"testSourceSha": "d" * 40}, False),
                 ("missing", {"placements": {}}, False), ("reservation", {"capacityReserved": True}, False),
                 ("missing-existing", {"existing": {}}, False),
                 ("unapproved", {"placements": {name: "eastus" for name in SCENARIOS}}, False),
                 ("missing-region", {"regions": {}}, False)]
        for name, updates, passes in cases:
            path = self.root / (name + ".json")
            path.write_text(json.dumps({**proof, **updates}))
            with self.subTest(case=name):
                result = subprocess.run([sys.executable, "-c", parser, str(path)],
                                        capture_output=True, text=True,
                                        env={**os.environ, "CANDIDATE_SCENARIO": "all",
                                             "CANDIDATE_SOURCE_SHA": self.sha,
                                             "TEST_SOURCE_SHA": "c" * 40,
                                             "PRODUCER_BUILD_ID": "4401", "BUILD_BUILDID": "9791"})
                self.assertEqual(result.returncode == 0, passes, result.stderr)
                if passes:
                    self.assertIn("variable=location_classic_azure_cni;isOutput=true]centralus", result.stdout)
                    self.assertIn("variable=sourceSha;isOutput=true]" + self.sha, result.stdout)
                    self.assertIn("variable=validationBuildId;isOutput=true]9791", result.stdout)
                else:
                    self.assertNotIn("isOutput=true", result.stdout)

    def test_failed_aggregate_check_cannot_emit_admission(self):
        pipeline = yaml.safe_load(Path(__file__).with_name("azure-pipeline-config-tests.yml").read_text())
        task = next(step for step in pipeline["stages"][1]["jobs"][0]["steps"]
                    if step.get("name") == "admit")
        script = task["inputs"]["inlineScript"]
        marker = self.root / "unsafe-admission"
        prefix = (
            'az() { echo 9b96ebbd-c57a-42d1-bbe9-b69296e4c7fb; }\n'
            'git() { printf "%s\\n" "$CANDIDATE_SOURCE_SHA"; }\n'
            'capacity_python() { [[ "${1:-}" != -B ]] || return 23; }\nPYTHON=capacity_python\n'
        )
        command = prefix + script + '\nprintf unsafe > "$UNSAFE_MARKER"\n'
        result = subprocess.run(["bash", "-c", command], capture_output=True, text=True,
                                env={**os.environ, "CAPACITY_ROOT": "unused", "CAPACITY_EVIDENCE": "unused",
                                     "CANDIDATE_SOURCE_SHA": self.sha, "CANDIDATE_SCENARIO": "all",
                                     "TEST_SOURCE_SHA": self.sha, "BUILD_SOURCESDIRECTORY": "unused",
                                     "PRODUCER_BUILD_ID": "4401", "UNSAFE_MARKER": marker.as_posix()})
        self.assertEqual(result.returncode, 23, result.stderr)
        self.assertFalse(marker.exists())


if __name__ == "__main__":
    unittest.main()
