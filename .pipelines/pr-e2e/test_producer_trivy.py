"""Check producer binding, immutable publication and complete artifact hashes."""
import hashlib
import json
import os
from pathlib import Path
import subprocess
import tempfile
import unittest
from unittest.mock import patch

import producer_trivy
import yaml


class ProducerTests(unittest.TestCase):
    def test_signing_keeps_buildx_push_metadata_readable(self):
        pipeline = yaml.safe_load((Path(__file__).parents[1] / "azure-pipeline-build.yml").read_text())
        jobs = pipeline["extends"]["parameters"]["stages"][0]["jobs"]
        for component in ("Linux_Prometheus_Collector", "Linux_Target_Allocator"):
            job = next(item for item in jobs if item["job"] == component)
            signing = next(step for step in job["steps"] if step.get("task") == "EsrpCodeSigning@6")
            with self.subTest(component=component), tempfile.TemporaryDirectory() as directory:
                root = Path(directory)
                metadata = root / "push-metadata.json"
                raw = json.dumps({
                    "containerimage.digest": "sha256:" + "b" * 64,
                    "containerimage.descriptor": {
                        "mediaType": "application/vnd.oci.image.index.v1+json",
                        "digest": "sha256:" + "b" * 64, "size": 856,
                    },
                }).encode()
                metadata.write_bytes(raw)
                (root / "payload.json").write_text('{"targetArtifact":{}}')
                signed = []
                for path in root.glob(signing["inputs"]["Pattern"]):
                    signed.append(path.name)
                    path.write_bytes(b"\xd2\x84\x43\xa1\x01\x26" + path.read_bytes())
                self.assertEqual(json.loads(metadata.read_text(encoding="utf-8-sig")),
                                 json.loads(raw))
                self.assertEqual(metadata.read_bytes(), raw)
                self.assertEqual(signed, ["payload.json"])
                self.assertTrue((root / "payload.json").read_bytes().startswith(b"\xd2"))
                receipt_step = next(step for step in job["steps"]
                                    if step.get("template", "").endswith("push-receipt-steps.yml@self"))
                self.assertIn("/push-metadata.json", receipt_step["parameters"]["evidenceArguments"])
                if component == "Linux_Prometheus_Collector":
                    scan = next(step for step in job["steps"]
                                if step.get("displayName") == "Build: scan exact collector child and retain complete Trivy evidence")
                    self.assertIn('/linux/push-metadata.json"', scan["bash"])

    def test_signing_guard_rejects_metadata_wildcard(self):
        original = yaml.safe_load

        def restore_wildcard(text):
            pipeline = original(text)
            jobs = pipeline["extends"]["parameters"]["stages"][0]["jobs"]
            for job in jobs:
                if job["job"] in ("Linux_Prometheus_Collector", "Linux_Target_Allocator"):
                    for step in job["steps"]:
                        if step.get("task") == "EsrpCodeSigning@6":
                            step["inputs"]["Pattern"] = "*.json"
            return pipeline

        with patch.object(yaml, "safe_load", side_effect=restore_wildcard):
            result = unittest.TestResult()
            ProducerTests("test_signing_keeps_buildx_push_metadata_readable").run(result)
        self.assertEqual(len(result.errors), 2)
        self.assertTrue(all("UnicodeDecodeError" in error for _, error in result.errors))

    def test_scan_replacement_and_release_artifacts_are_scoped(self):
        pipeline = yaml.safe_load((Path(__file__).parents[1] / "azure-pipeline-build.yml").read_text())
        conditional = "${{ if eq(variables['Build.SourceBranch'], 'refs/heads/main') }}"
        self.assertEqual(pipeline["extends"][conditional]["template"],
                         "v1/1ES.Official.PipelineTemplate.yml@1ESPipelineTemplates")
        self.assertEqual(pipeline["extends"]["${{ else }}"]["template"],
                         "v1/1ES.Unofficial.PipelineTemplate.yml@1ESPipelineTemplates")
        self.assertNotIn("enableProductionSDL", str(pipeline["extends"]["parameters"]["sdl"]))
        sdl = pipeline["extends"]["parameters"]["sdl"]
        non_main = sdl["${{ if ne(variables['Build.SourceBranch'], 'refs/heads/main') }}"]
        self.assertFalse(non_main["enableAllTools"])
        self.assertFalse(non_main["antimalwareScan"]["enabled"])
        self.assertFalse(non_main["spmi"]["enabled"])
        self.assertTrue(sdl["componentgovernance"]["enabled"])
        self.assertIn("refs/heads/main", pipeline["variables"]["IS_MAIN_BRANCH"])
        jobs = pipeline["extends"]["parameters"]["stages"][0]["jobs"]
        for job in jobs:
            if job["job"] in ("SDL_Policheck_Scan", "SDL_Binary_Scan"):
                self.assertEqual(job["condition"], "and(succeeded(), eq(variables.IS_MAIN_BRANCH, true))")
            for step in job.get("steps", []):
                if step.get("task", "").startswith(("CodeQL3000", "CredScan@", "SdtReport@",
                                                      "PublishSecurityAnalysisLogs@", "PostAnalysis@")):
                    self.assertEqual(step["condition"], "and(succeeded(), eq(variables.IS_MAIN_BRANCH, true))")
        tags = next(job for job in jobs if job["job"] == "Image_Tags_and_Ev2_Artifacts")
        self.assertIn("${{ if eq(variables['Build.SourceBranch'], 'refs/heads/main') }}",
                      tags["templateContext"]["outputs"][0])
        for job in jobs:
            for output in job.get("templateContext", {}).get("outputs", []):
                if output.get("artifactName", "").startswith("sre-push-"):
                    self.assertEqual(output["condition"], "and(succeeded(), eq(variables.IS_SRE_BRANCH, true))")
        runner = next(job for job in jobs if job["job"] == "SRE_TestKube_Runner_Receipt")
        self.assertEqual(runner["condition"], "and(succeeded(), eq(variables.IS_SRE_BRANCH, true))")
        collector = next(job for job in jobs if job["job"] == "Linux_Prometheus_Collector")
        artifact = next(item for item in collector["templateContext"]["outputs"]
                        if item.get("artifactName") == "sre-trivy-collector")
        self.assertIn("eq(variables.IS_SRE_BRANCH, true)", artifact["condition"])
        self.assertIn("eq(variables['System.JobAttempt'], '1')", artifact["condition"])
        step = next(item for item in collector["steps"]
                    if item.get("displayName") == "Build: scan exact collector child and retain complete Trivy evidence")
        self.assertEqual(step["condition"], "and(succeeded(), eq(variables.IS_SRE_BRANCH, true))")
        self.assertNotIn('"$evidence/empty.yaml"', step["bash"])
        self.assertNotIn('"$evidence/empty.ignore"', step["bash"])
        self.assertIn('printf \'{}\\n\' > "$console_config"', step["bash"])
        self.assertIn(': > "$console_ignore"', step["bash"])
        self.assertIn('trivy --config "$console_config" convert', step["bash"])
        self.assertIn('--ignorefile "$console_ignore"', step["bash"])
        console = next(item["bash"] for item in collector["steps"]
                       if item.get("displayName") == "Build: run trivy scan")
        self.assertIn('scanners="vuln,secret"', console)
        self.assertIn('[ "$(IS_SRE_BRANCH)" = "True" ]; then scanners="secret"', console)

    def test_exact_push_binding_and_fail_closed_publication(self):
        commit = "a" * 40
        digest = "sha256:" + "b" * 64
        image = "containerinsightsprod.azurecr.io/public/azuremonitor/containerinsights/cidev/prometheus-collector/images:test"
        repository = "mcr.microsoft.com/azuremonitor/containerinsights/cidev/prometheus-collector/images"
        selected = {"name": "collector", "ref": repository + "@sha256:" + "c" * 64,
                    "indexRef": repository + "@" + digest, "os": "linux", "arch": "amd64"}
        environment = {"SYSTEM_DEFINITIONID": "440", "SYSTEM_JOBATTEMPT": "1",
                       "SYSTEM_STAGEATTEMPT": "1", "BUILD_SOURCEVERSION": commit, "BUILD_BUILDID": "123"}
        for failure in (None, "checkout", "digest", "platform", "scan", "retry", "definition"):
            with self.subTest(failure=failure), tempfile.TemporaryDirectory() as directory:
                root = Path(directory)
                metadata = root / "push.json"
                metadata.write_text(json.dumps({"containerimage.digest": digest,
                    "containerimage.descriptor": {"digest": "sha256:" + "d" * 64 if failure == "digest" else digest}}))
                output = root / "scan"

                def scan(ref, platform, destination):
                    self.assertEqual(ref, selected["ref"])
                    self.assertEqual(platform, "linux/amd64")
                    destination.mkdir(parents=True)
                    for name, content in (("trivy/result.json", b'{"Results":[]}'),
                                          ("trivy/metadata.json", json.dumps({
                                              "exitCode": 0, "dbSha256": hashlib.sha256(b"database").hexdigest(),
                                              "version": "0.76.1"}).encode()),
                                          ("trivy/scan.log", b"scanner diagnostics"),
                                          ("cache/db/trivy.db", b"database"), ("empty.ignore", b""),
                                          ("empty.yaml", b"{}\n")):
                        path = destination / name
                        path.parent.mkdir(parents=True, exist_ok=True)
                        path.write_bytes(content)
                    if failure == "scan":
                        raise ValueError("Scanner failure")

                env = dict(environment)
                if failure == "retry":
                    env["SYSTEM_JOBATTEMPT"] = "2"
                if failure == "definition":
                    env["SYSTEM_DEFINITIONID"] = "979"
                with patch.dict(os.environ, env), \
                     patch.object(producer_trivy.subprocess, "run", return_value=subprocess.CompletedProcess(
                         [], 0, stdout="e" * 40 if failure == "checkout" else commit)), \
                     patch.object(producer_trivy, "resolve_images", return_value=[] if failure == "platform" else [selected]) as resolve, \
                     patch.object(producer_trivy, "scan", side_effect=scan):
                    if failure:
                        with self.assertRaises(ValueError):
                            producer_trivy.produce(image, metadata, output)
                        self.assertFalse((output / "provenance.json").exists())
                    else:
                        producer_trivy.produce(image, metadata, output)
                        resolve.assert_called_once_with("collector", selected["indexRef"])
                        provenance = json.loads((output / "provenance.json").read_text())
                        self.assertEqual(provenance["source"], {"repo": "Azure/prometheus-collector",
                            "commit": commit, "buildId": "123"})
                        self.assertEqual(provenance["image"], selected)
                        self.assertEqual(json.loads((output / "trivy" / "metadata.json").read_text())["dbSha256"],
                                         hashlib.sha256(b"database").hexdigest())
                        self.assertEqual(set(provenance["files"]),
                                         {"trivy/result.json", "trivy/metadata.json"})
                        self.assertEqual({path.relative_to(output).as_posix()
                                          for path in output.rglob("*") if path.is_file()},
                                         {"trivy/result.json", "trivy/metadata.json", "provenance.json"})
                        for name, sha in provenance["files"].items():
                            self.assertEqual(sha, hashlib.sha256((output / name).read_bytes()).hexdigest())


if __name__ == "__main__":
    unittest.main()
