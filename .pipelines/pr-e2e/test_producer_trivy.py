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


class ProducerTests(unittest.TestCase):
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
                    destination.mkdir()
                    for name, content in (("trivy/result.json", b'{"Results":[]}'),
                                          ("trivy/metadata.json", b'{"exitCode":0}'),
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
                        self.assertEqual(len(provenance["files"]), 5)
                        for name, sha in provenance["files"].items():
                            self.assertEqual(sha, hashlib.sha256((output / name).read_bytes()).hexdigest())


if __name__ == "__main__":
    unittest.main()
