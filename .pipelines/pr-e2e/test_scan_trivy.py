"""Exercise the actual discovery entrypoint without making registry requests."""
import json
from pathlib import Path
import subprocess
import tempfile
import unittest
from unittest.mock import patch
from datetime import datetime, timezone

import scan_trivy


IMAGE = "example.invalid/collector@sha256:" + "a" * 64


class DiscoveryTests(unittest.TestCase):
    def test_digest_platform_and_failed_tool_are_checked(self):
        for mismatch in ("digest", "platform", "tool", None):
            with self.subTest(mismatch=mismatch), tempfile.TemporaryDirectory() as directory:
                root = Path(directory) / "run"

                def invoke(command, **kwargs):
                    if "--version" in command:
                        return subprocess.CompletedProcess(command, 0, stdout=json.dumps({
                            "Version": "test-version", "VulnerabilityDB": {
                                "UpdatedAt": datetime.now(timezone.utc).isoformat()}}))
                    output = Path(command[command.index("--output") + 1])
                    database = root / "cache" / "db"
                    database.mkdir(parents=True)
                    (database / "trivy.db").write_bytes(b"offline database")
                    output.write_text(json.dumps({"Metadata": {
                        "RepoDigests": [IMAGE if mismatch != "digest" else "other"],
                        "ImageConfig": {"os": "linux", "architecture": "amd64" if mismatch != "platform" else "arm64"},
                    }, "Results": []}))
                    return subprocess.CompletedProcess(command, 1 if mismatch == "tool" else 0)

                with patch.object(scan_trivy.subprocess, "run", side_effect=invoke):
                    if mismatch:
                        with self.assertRaises(ValueError):
                            scan_trivy.scan(IMAGE, "linux/amd64", root)
                    else:
                        scan_trivy.scan(IMAGE, "linux/amd64", root)
                        receipt = json.loads((root / "trivy/metadata.json").read_text())
                        self.assertEqual(receipt["requestedRef"], IMAGE)
                        self.assertEqual(receipt["version"], "test-version")
                        self.assertEqual(len(receipt["resultSha256"]), 64)
                    self.assertTrue((root / "trivy/metadata.json").exists())

    def test_existing_output_tag_and_windows_rejected_without_tool_execution(self):
        with tempfile.TemporaryDirectory() as directory, patch.object(scan_trivy.subprocess, "run") as invoke:
            root = Path(directory)
            for image, platform, error in (
                (IMAGE, "linux/amd64", FileExistsError),
                ("example.invalid/collector:latest", "linux/amd64", ValueError),
                (IMAGE, "windows/amd64", ValueError),
            ):
                with self.assertRaises(error):
                    scan_trivy.scan(image, platform, root)
            invoke.assert_not_called()


if __name__ == "__main__":
    unittest.main()
