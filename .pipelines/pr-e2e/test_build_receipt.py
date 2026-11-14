import copy
import hashlib
import io
import json
import os
from pathlib import Path, PurePosixPath
import shlex
import shutil
import subprocess
import tempfile
import inspect
import unittest
from unittest.mock import patch
import yaml

import build_receipt
import push_receipt
import runner_receipt


class ReceiptTests(unittest.TestCase):
    def test_sre_campaign_trigger_and_receipt_conditions_match_agent_branch(self):
        root = Path(__file__).parents[1]
        build_pipeline = yaml.safe_load((root / "azure-pipeline-build.yml").read_text())
        self.assertIn("sre/vuln/*", build_pipeline["trigger"]["branches"]["include"])
        push_template = yaml.safe_load((Path(__file__).parent / "push-receipt-steps.yml").read_text())
        condition = push_template["steps"][0]["condition"]
        self.assertIn("refs/heads/sre/vuln/", condition)
        self.assertIn("/ama-metrics-sre-", condition)

    def test_windows_push_captures_one_exact_tag_digest(self):
        pipeline = yaml.safe_load((Path(__file__).parents[1] / "azure-pipeline-build.yml").read_text())
        steps = next(job["steps"] for stage in pipeline["extends"]["parameters"]["stages"]
                     for job in stage.get("jobs", [])
                     if job.get("displayName") == "Build: windows 2022 prometheus-collector image")
        script = next(step["powershell"] for step in steps
                      if step.get("displayName") == "Build: push image to dev ACR")
        image = "containerinsightsprod.azurecr.io/public/azuremonitor/containerinsights/cidev/prometheus-collector/images:candidate-win"
        digest = "sha256:" + "b" * 64
        line = "candidate-win-ltsc2022: digest: " + digest + " size: 1234"
        for output, passed in (([line], True), ([image], False), ([line, line], False),
                               ([line.replace("candidate-win", "another-tag")], False)):
            with tempfile.TemporaryDirectory() as directory:
                body = script
                for key, value in {"WINDOWS_FULL_IMAGE_NAME": image, "WINDOWS_2022_BASE_IMAGE_VERSION": "ltsc2022",
                                   "Build.ArtifactStagingDirectory": directory, "Agent.TempDirectory": directory,
                                   "ACR_USERNAME": "dummy",
                                   "ACR_PASSWORD": "dummy"}.items():
                    body = body.replace("$(" + key + ")", value)
                stub = "$ErrorActionPreference='Stop'\n$lines=@(" + ",".join("'" + value + "'" for value in output) + ")\n"
                stub += """function docker {
    $global:LASTEXITCODE=0
    if ($args[0] -eq 'push') {
        if ($args -contains '--quiet') { 'mutable-tag' } else { $script:lines }
    }
}
"""
                result = subprocess.run([shutil.which("pwsh"), "-NoProfile", "-NonInteractive",
                                         "-Command", stub + body], text=True, capture_output=True)
                self.assertEqual(result.returncode == 0, passed, result.stderr)
                path = Path(directory) / "windows2022-push-digest.txt"
                if passed:
                    self.assertEqual(path.read_text(encoding="utf-8-sig").strip(), digest)
                else:
                    self.assertFalse(path.exists())

    def test_config_reader_push_metadata_is_created_and_consumed(self):
        pipeline = yaml.safe_load((Path(__file__).parents[1] / "azure-pipeline-build.yml").read_text())
        steps = next(job["steps"] for stage in pipeline["extends"]["parameters"]["stages"]
                     for job in stage.get("jobs", [])
                     if job.get("displayName") == "Build: config reader image")
        script = next(step["bash"] for step in steps
                      if step.get("displayName") == "Build: build and push configuration reader image to dev ACR")
        metadata = shlex.split(script.split("--metadata-file ", 1)[1])[0]
        directories = [shlex.split(line)[2] for line in script.splitlines() if line.startswith("mkdir -p ")]
        self.assertIn(str(PurePosixPath(metadata).parent), directories)
        arguments = next(step["parameters"]["evidenceArguments"] for step in steps
                         if step.get("parameters", {}).get("component") == "config-reader")
        self.assertEqual(shlex.split(arguments), ["--metadata", metadata])
        self.assertEqual(script.splitlines()[0], "set -e")

    def test_config_reader_signing_payload_and_retry(self):
        pipeline = yaml.safe_load((Path(__file__).parents[1] / "azure-pipeline-build.yml").read_text())
        script = next(step["bash"] for stage in pipeline["extends"]["parameters"]["stages"]
                      for job in stage.get("jobs", []) for step in job.get("steps", [])
                      if step.get("displayName") == "Build: build and push configuration reader image to dev ACR")
        jq = next((shlex.split(line) for line in script.splitlines() if line.startswith("jq -e ")), None)
        self.assertIsNotNone(jq, "sign the push metadata descriptor, not a mutable multi-arch tag lookup")
        descriptor = {"mediaType": "application/vnd.oci.image.index.v1+json",
                      "digest": "sha256:" + "b" * 64, "size": 1234}
        for value in (descriptor, None, {**descriptor, "size": None}, {**descriptor, "digest": "latest"}):
            result = subprocess.run(jq[:3], input=json.dumps({"containerimage.descriptor": value}),
                                    text=True, capture_output=True)
            if value == descriptor:
                self.assertEqual(result.returncode, 0, result.stderr)
                self.assertEqual(json.loads(result.stdout), {"targetArtifact": descriptor})
            else:
                self.assertNotEqual(result.returncode, 0)
        builder = "\n".join(line for line in script.splitlines()
                            if line.startswith("docker buildx ") and not line.startswith("docker buildx build "))
        broken = builder.replace("docker buildx inspect dockerbuilder >/dev/null 2>&1 || ", "")
        self.assertNotEqual(broken, builder)
        for commands, existing, passed in ((builder, "0", True), (builder, "1", True), (broken, "1", False)):
            stub = """set -e
docker() {
  case "$1 $2 $3" in
    "buildx inspect dockerbuilder") [[ "$exists" == 1 ]] ;;
    "buildx inspect --bootstrap") [[ "$exists" == 1 ]] ;;
    "buildx create --name") [[ "$exists" == 0 ]] && exists=1 ;;
    "buildx use dockerbuilder") [[ "$exists" == 1 ]] ;;
    *) printf 'Unexpected Docker command: %s\\n' "$*" >&2; return 99 ;;
  esac
}
"""
            result = subprocess.run([shutil.which("bash"), "-c", stub + "\nexists=" + existing + "\n" + commands],
                                    text=True, capture_output=True)
            self.assertEqual(result.returncode == 0, passed, result.stderr)

    def test_worker_accepts_build_and_imagetools_metadata(self):
        digest = "sha256:" + "b" * 64
        for metadata in ({"containerimage.digest": digest},
                         {"containerimage.descriptor": {"digest": digest}},
                         {"containerimage.digest": digest, "containerimage.descriptor": {"digest": "sha256:" + "c" * 64}}):
            with tempfile.TemporaryDirectory() as directory:
                path = Path(directory)
                (path / "metadata.json").write_text(json.dumps(metadata))
                with patch.dict(os.environ, {"BUILD_SOURCEVERSION": "a" * 40, "BUILD_BUILDID": "123"}), \
                        patch("sys.argv", ["push_receipt.py", "--name", "collector",
                              "--image", "containerinsightsprod.azurecr.io/public/azuremonitor/containerinsights/cidev/prometheus-collector/images:candidate",
                              "--metadata", str(path / "metadata.json"), "--output", str(path / "push.json")]), \
                        patch.object(push_receipt.subprocess, "run", return_value=subprocess.CompletedProcess([], 0, stdout="a" * 40)):
                    if len(metadata) == 2:
                        with self.assertRaises(ValueError):
                            push_receipt.main()
                    else:
                        push_receipt.main()
                        self.assertTrue(json.loads((path / "push.json").read_text())["indexRef"].endswith("@" + digest))

    def test_existing_runner_receipt_binds_the_test_checkout(self):
        image = "docker.io/cerebro31/ginkgo-test@sha256:" + "b" * 64
        for checkout in ("a" * 40, "c" * 40):
            with tempfile.TemporaryDirectory() as directory:
                output = Path(directory) / "runner.json"
                env = {"BUILD_SOURCEVERSION": "a" * 40, "BUILD_BUILDID": "123",
                       "SYSTEM_COLLECTIONURI": "https://dev.azure.com/github-private/",
                       "SYSTEM_TEAMPROJECTID": "project"}
                with patch.dict(os.environ, env), patch("sys.argv", [
                        "runner_receipt.py", "--image", image, "--output", str(output)]), \
                        patch.object(runner_receipt.subprocess, "run", return_value=subprocess.CompletedProcess([], 0, stdout=checkout)), \
                        patch.object(build_receipt, "resolve_images", return_value=[{"os": "linux", "arch": "amd64"}]):
                    if checkout != env["BUILD_SOURCEVERSION"]:
                        with self.assertRaises(ValueError):
                            runner_receipt.main()
                    else:
                        runner_receipt.main()
                        result = json.loads(output.read_text())
                        self.assertEqual(result["source"]["commit"], checkout)
                        self.assertEqual(result["imageRef"], image)

    def test_registry_bytes_must_match_the_digest(self):
        raw = b'{"schemaVersion":2}'
        digest = "sha256:" + hashlib.sha256(raw).hexdigest()
        for requested in (digest, "sha256:" + "0" * 64):
            with patch.object(build_receipt.urllib.request, "urlopen", return_value=io.BytesIO(raw)):
                if requested == digest:
                    self.assertEqual(build_receipt.registry_json("mcr.microsoft.com/test", "manifests", requested)[1], digest)
                else:
                    with self.assertRaises(ValueError):
                        build_receipt.registry_json("mcr.microsoft.com/test", "manifests", requested)

    def test_worker_push_output_binds_checkout_and_digest(self):
        image = "containerinsightsprod.azurecr.io/public/azuremonitor/containerinsights/cidev/prometheus-collector/images:sre-123-candidate"
        digest, commit = "sha256:" + "b" * 64, "a" * 40
        result = push_receipt.receipt("collector", image, digest, commit, "123", commit)
        self.assertTrue(result["indexRef"].endswith("@" + digest))
        for bad_digest, checkout in (("latest", commit), (digest, "c" * 40)):
            with self.assertRaises(ValueError):
                push_receipt.receipt("collector", image, bad_digest, commit, "123", checkout)

    def test_exact_complete_build_is_required(self):
        build = {"status": "completed", "result": "succeeded", "id": 123,
                 "definition": {"id": 440}, "sourceVersion": "a" * 40,
                 "repository": {"id": "Azure/prometheus-collector", "type": "GitHub"},
                 "url": "https://dev.azure.com/github-private/project/_apis/build/builds/123",
                 "sourceBranch": "refs/heads/bot/ama-metrics-sre-test",
                 "_links": {"web": {"href": "https://example.invalid/build/123"}}}
        self.assertEqual(build_receipt.build_source({**build, "sourceBranch": "refs/heads/sre/vuln/campaign-123"}),
                         {"repo": "Azure/prometheus-collector", "commit": "a" * 40, "buildId": "123"})
        with self.assertRaisesRegex(ValueError, "SRE campaign branch"):
            build_receipt.build_source({**build, "sourceBranch": "refs/heads/sre/other/campaign-123"})
        pushes = [{"commit": "a" * 40, "buildId": 123, "name": name,
                   "tag": "candidate", "indexRef": f"mcr.microsoft.com/test/{name}@sha256:" + "b" * 64}
                  for name in ("collector", "targetallocator", "config-reader")]
        runner = {"schemaVersion": 1, "artifactPath": "runner.json",
                  "source": {"repo": "Azure/prometheus-collector", "commit": "a" * 40, "buildId": "123"},
                  "imageRef": "mcr.microsoft.com/test/testkube-runner@sha256:" + "b" * 64,
                  "mode": "reused-toolchain-source-fetched-separately",
                  "artifactUrl": build["url"] + "/artifacts?artifactName=sre-testkube-runner&api-version=7.1"}
        runner_hash = hashlib.sha256(json.dumps(runner).encode()).hexdigest()
        configs = {
            "1": {"os": "linux", "architecture": "amd64"},
            "2": {"os": "linux", "architecture": "arm64"},
            "3": {"os": "windows", "architecture": "amd64", "os.version": "10.0.20348.1"},
        }

        def registry(repository, kind, reference):
            if kind == "blobs":
                return configs[reference[-1]], reference
            if reference in ("sha256:" + "b" * 64, "candidate"):
                return {"manifests": [{"digest": "sha256:" + key * 64}
                                      for key in ("123" if repository.endswith("/collector") else "1")]}, "sha256:" + "b" * 64
            return {"config": {"digest": reference}}, reference

        with patch.object(build_receipt, "registry_json", side_effect=registry):
            runner["images"] = build_receipt.resolve_images("testkube-runner", runner["imageRef"])
            receipt = build_receipt.emit(build, pushes, runner, runner_hash)
            self.assertEqual(len(receipt["images"]), 5)
            self.assertEqual(receipt["status"], "built-not-e2e-validated")
            self.assertEqual(receipt["testRunner"]["testSource"], receipt["source"])
            legacy_runner = {**runner, "artifactUrl": runner["artifactUrl"].replace(
                "https://dev.azure.com/github-private/", "https://github-private.visualstudio.com/")}
            self.assertEqual(build_receipt.emit(build, pushes, legacy_runner, runner_hash)["testRunner"]["artifactUrl"],
                             legacy_runner["artifactUrl"])
            for field, value in (("result", "failed"), ("sourceVersion", "c" * 40)):
                with self.assertRaises(ValueError):
                    build_receipt.emit({**build, field: value}, pushes, runner, runner_hash)
            for field, value in (("commit", "c" * 40), ("buildId", "999")):
                invalid = copy.deepcopy(pushes)
                invalid[0][field] = value
                with self.assertRaises(ValueError):
                    build_receipt.emit(build, invalid, runner, runner_hash)
            with self.assertRaises(ValueError):
                build_receipt.emit(build, pushes[:-1], runner, runner_hash)
            invalid = copy.deepcopy(pushes)
            invalid[0]["indexRef"] = "mcr.microsoft.com/test/collector:latest"
            with self.assertRaises(ValueError):
                build_receipt.emit(build, invalid, runner, runner_hash)

    def test_provenance_guards_fail_when_broken(self):
        mutations = [
            (build_receipt, "emit",
             'if pushed["commit"] != source["commit"] or str(pushed["buildId"]) != source["buildId"]:',
             "if False:", "test_exact_complete_build_is_required"),
            (push_receipt, "receipt", " or checkout != commit", "",
             "test_worker_push_output_binds_checkout_and_digest"),
            (build_receipt, "registry_json", 'if reference.startswith("sha256:") and actual != reference:',
             "if False:", "test_registry_bytes_must_match_the_digest"),
            (runner_receipt, "main", 'if checkout != os.environ["BUILD_SOURCEVERSION"]:',
             "if False:", "test_existing_runner_receipt_binds_the_test_checkout"),
        ]
        for module, name, old, new, test in mutations:
            with self.subTest(guard=name):
                original = getattr(module, name)
                source = inspect.getsource(original)
                self.assertIn(old, source)
                exec(source.replace(old, new), vars(module))
                try:
                    result = unittest.TestResult()
                    ReceiptTests(test).run(result)
                finally:
                    setattr(module, name, original)
                self.assertTrue(result.failures, result.errors)
                self.assertFalse(result.errors)


if __name__ == "__main__":
    unittest.main()
