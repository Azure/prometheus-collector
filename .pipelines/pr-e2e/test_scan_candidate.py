import json
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

import scan_candidate


class CampaignScanTests(unittest.TestCase):
    def setUp(self):
        self.baseline = ("mcr.microsoft.com/azuremonitor/containerinsights/ciprod/"
                         "prometheus-collector/images@sha256:" + "a" * 64)
        self.image = {
            "name": "collector", "os": "linux", "arch": "amd64",
            "ref": "candidate@sha256:" + "b" * 64,
        }
        self.source = {"commit": "c" * 40, "buildId": "123"}
        self.receipt = {"source": self.source, "images": [self.image]}

    def test_consumes_protected_candidate_scan_without_rescanning_it(self):
        comparison = {"source": self.source, "verdict": "PASS"}
        with tempfile.TemporaryDirectory() as directory:
            output = Path(directory) / "evidence"
            verified = Path(directory) / "verified"
            with patch.object(scan_candidate.fetch_candidate, "load_verified",
                              return_value=self.receipt) as load_verified, \
                    patch.object(scan_candidate.fetch_candidate, "fetch") as fetch, \
                    patch.object(scan_candidate.scan_trivy, "scan") as scan, \
                    patch.object(scan_candidate.compare_trivy, "compare",
                                 return_value=comparison) as compare:
                self.assertEqual(scan_candidate.run(
                    123, self.source["commit"], self.baseline, "linux/amd64",
                    ["CVE-2026-63209"], output, verified), comparison)
                load_verified.assert_called_once_with(verified.resolve(), 123, self.source["commit"])
                fetch.assert_not_called()
                scan.assert_called_once_with(
                    self.baseline, "linux/amd64", output / "baseline",
                    verified.resolve() / scan_candidate.fetch_candidate.SCAN_ARTIFACT,
                )
                compare.assert_called_once_with(
                    output / "baseline",
                    verified.resolve() / scan_candidate.fetch_candidate.SCAN_ARTIFACT,
                    self.receipt, ["CVE-2026-63209"],
                )
                self.assertFalse((output / "candidate").exists())
                self.assertEqual(json.loads((output / "comparison.json").read_text()), comparison)
                with self.assertRaises(FileExistsError):
                    scan_candidate.run(
                        123, self.source["commit"], self.baseline, "linux/amd64",
                        ["CVE-2026-63209"], output, verified,
                    )

    def test_legacy_fetch_path_also_uses_protected_candidate_scan(self):
        comparison = {"source": self.source, "verdict": "BLOCKED"}
        with tempfile.TemporaryDirectory() as directory:
            output = Path(directory) / "evidence"
            verified = output / "verified-candidate"
            with patch.object(scan_candidate.fetch_candidate, "fetch", return_value=self.receipt) as fetch, \
                    patch.object(scan_candidate.fetch_candidate, "load_verified") as load_verified, \
                    patch.object(scan_candidate.scan_trivy, "scan") as scan, \
                    patch.object(scan_candidate.compare_trivy, "compare",
                                 return_value=comparison):
                scan_candidate.run(
                    123, self.source["commit"], self.baseline, "linux/amd64",
                    ["CVE-2026-63209"], output,
                )
                fetch.assert_called_once_with(123, self.source["commit"], verified)
                load_verified.assert_not_called()
                scan.assert_called_once_with(
                    self.baseline, "linux/amd64", output / "baseline",
                    verified / scan_candidate.fetch_candidate.SCAN_ARTIFACT,
                )

    def test_untrusted_or_ambiguous_candidate_evidence_never_scans(self):
        cases = [
            ([], False),
            ([self.image, self.image], False),
            ([self.image], True),
        ]
        for images, invalid_receipt in cases:
            with self.subTest(images=len(images), invalid_receipt=invalid_receipt), \
                    tempfile.TemporaryDirectory() as directory:
                output = Path(directory) / "evidence"
                receipt = dict(self.receipt, images=images)
                with patch.object(scan_candidate.fetch_candidate, "load_verified",
                                  side_effect=ValueError("invalid receipt") if invalid_receipt
                                  else None, return_value=receipt), \
                        patch.object(scan_candidate.scan_trivy, "scan") as scan:
                    if invalid_receipt or len(images) != 1:
                        with self.assertRaises(ValueError):
                            scan_candidate.run(
                                123, self.source["commit"], self.baseline, "linux/amd64",
                                ["CVE-2026-63209"], output, Path(directory) / "verified",
                            )
                    scan.assert_not_called()
                    self.assertFalse((output / "comparison.json").exists())

    def test_invalid_baseline_platform_and_empty_selection_never_fetch_or_scan(self):
        for baseline, platform, requested in (
            ("image:tag", "linux/amd64", ["CVE-2026-63209"]),
            ("other@sha256:" + "a" * 64, "linux/amd64", ["CVE-2026-63209"]),
            (self.baseline, "windows/amd64", ["CVE-2026-63209"]),
            (self.baseline, "linux/amd64", []),
            (self.baseline, "linux/amd64", ["not-a-CVE"]),
        ):
            with tempfile.TemporaryDirectory() as directory, \
                    patch.object(scan_candidate.fetch_candidate, "fetch") as fetch, \
                    patch.object(scan_candidate.scan_trivy, "scan") as scan:
                output = Path(directory) / "evidence"
                with self.assertRaises(ValueError):
                    scan_candidate.run(123, "c" * 40, baseline, platform, requested, output)
                fetch.assert_not_called()
                scan.assert_not_called()
                self.assertFalse(output.exists())


if __name__ == "__main__":
    unittest.main()
