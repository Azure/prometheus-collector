import json
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

import scan_candidate


class CampaignScanTests(unittest.TestCase):
    def test_only_authenticated_exact_child_uses_the_same_database_and_retains_failures(self):
        baseline = ("mcr.microsoft.com/azuremonitor/containerinsights/ciprod/"
                    "prometheus-collector/images@sha256:" + "a" * 64)
        image = {"name": "collector", "os": "linux", "arch": "amd64", "ref": "candidate@sha256:" + "b" * 64}
        source = {"commit": "c" * 40, "buildId": "123"}
        for images, auth_error, verdict in (([image], False, "PASS"), ([image], False, "FAIL"),
                                            ([image], False, "BLOCKED"), ([], False, "PASS"),
                                            ([image, image], False, "PASS"), ([image], True, "PASS")):
            with self.subTest(images=len(images), auth_error=auth_error, verdict=verdict), \
                    tempfile.TemporaryDirectory() as directory:
                output = Path(directory) / "evidence"
                receipt = {"source": source, "images": images}
                comparison = {"source": source, "verdict": verdict}
                with patch.object(scan_candidate.fetch_candidate, "fetch", return_value=receipt,
                                  side_effect=ValueError("unauthenticated") if auth_error else None) as fetch, \
                        patch.object(scan_candidate.scan_trivy, "scan") as scan, \
                        patch.object(scan_candidate.compare_trivy, "compare", return_value=comparison) as compare:
                    if auth_error or len(images) != 1:
                        with self.assertRaises(ValueError):
                            scan_candidate.run(123, source["commit"], baseline, "linux/amd64", ["CVE-2026-63209"], output)
                        scan.assert_not_called()
                        self.assertFalse((output / "comparison.json").exists())
                    else:
                        self.assertEqual(scan_candidate.run(
                            123, source["commit"], baseline, "linux/amd64", ["CVE-2026-63209"], output), comparison)
                        fetch.assert_called_once_with(123, source["commit"], output / "verified-candidate")
                        self.assertEqual(scan.call_args_list[0].args, (baseline, "linux/amd64", output / "baseline"))
                        self.assertEqual(scan.call_args_list[1].args,
                                         (image["ref"], "linux/amd64", output / "candidate", output / "baseline"))
                        compare.assert_called_once_with(output / "baseline", output / "candidate",
                                                        receipt, ["CVE-2026-63209"])
                        self.assertEqual(json.loads((output / "comparison.json").read_text()), comparison)
                        with self.assertRaises(FileExistsError):
                            scan_candidate.run(123, source["commit"], baseline, "linux/amd64", ["CVE-2026-63209"], output)

    def test_invalid_baseline_platform_and_empty_selection_never_fetch_or_scan(self):
        baseline = ("mcr.microsoft.com/azuremonitor/containerinsights/ciprod/"
                    "prometheus-collector/images@sha256:" + "a" * 64)
        for baseline, platform, requested in (("image:tag", "linux/amd64", ["CVE-2026-63209"]),
                                               ("other@sha256:" + "a" * 64, "linux/amd64", ["CVE-2026-63209"]),
                                               (baseline, "windows/amd64", ["CVE-2026-63209"]),
                                               (baseline, "linux/amd64", []),
                                               (baseline, "linux/amd64", ["not-a-CVE"])):
            with tempfile.TemporaryDirectory() as directory, \
                    patch.object(scan_candidate.fetch_candidate, "fetch") as fetch:
                output = Path(directory) / "evidence"
                with self.assertRaises(ValueError):
                    scan_candidate.run(123, "c" * 40, baseline, platform, requested, output)
                fetch.assert_not_called()
                self.assertFalse(output.exists())


if __name__ == "__main__":
    unittest.main()
