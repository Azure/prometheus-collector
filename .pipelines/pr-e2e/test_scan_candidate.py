from datetime import datetime, timezone
import hashlib
import json
from pathlib import Path
import subprocess
import tempfile
import unittest
from unittest.mock import call, patch

import scan_candidate
from test_fetch_candidate import protected_scan_artifact


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

    def test_validates_producer_then_scans_local_pair_with_one_database(self):
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
                self.assertEqual(scan.call_args_list, [
                    call(self.image["ref"], "linux/amd64", output / "candidate"),
                    call(self.baseline, "linux/amd64", output / "baseline", output / "candidate"),
                ])
                compare.assert_called_once_with(
                    output / "baseline", output / "candidate",
                    self.receipt, ["CVE-2026-63209"],
                )
                self.assertEqual(json.loads((output / "comparison.json").read_text()), comparison)
                with self.assertRaises(FileExistsError):
                    scan_candidate.run(
                        123, self.source["commit"], self.baseline, "linux/amd64",
                        ["CVE-2026-63209"], output, verified,
                    )

    def test_legacy_fetch_path_also_scans_local_pair(self):
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
                self.assertEqual(scan.call_args_list, [
                    call(self.image["ref"], "linux/amd64", output / "candidate"),
                    call(self.baseline, "linux/amd64", output / "baseline", output / "candidate"),
                ])

    def test_report_only_evidence_runs_real_local_scanners_and_strict_comparison_offline(self):
        source = dict(self.source, repo="Azure/prometheus-collector")
        image = dict(self.image, indexRef="candidate@sha256:" + "d" * 64)
        cve = "CVE-2026-63209"
        receipt = {"source": source, "images": [image], "product": "ama-metrics",
                   "buildResult": "succeeded", "requestedQids": [], "requestedCves": [cve]}
        build = {"status": "completed", "result": "succeeded", "id": 123,
                 "definition": {"id": 440}, "sourceVersion": source["commit"],
                 "repository": {"id": source["repo"], "type": "GitHub"},
                 "sourceBranch": "refs/heads/bot/ama-metrics-sre-test",
                 "url": scan_candidate.fetch_candidate.ADO_PROJECT + "/_apis/build/builds/123"}
        for failure in (None, "producer", "db", "version", "coverage", "digest"):
            with self.subTest(failure=failure), \
                    tempfile.TemporaryDirectory(dir=Path(__file__).parent) as directory:
                root = Path(directory).resolve()
                verified, output = root / "verified", root / "comparison"
                scan_candidate.fetch_candidate.extract_scan_artifact(
                    protected_scan_artifact(source, image),
                    verified / scan_candidate.fetch_candidate.SCAN_ARTIFACT)
                (verified / "build.json").write_text(json.dumps(build))
                (verified / "candidate-build.json").write_text(json.dumps(receipt))
                producer_report = verified / scan_candidate.fetch_candidate.SCAN_ARTIFACT / "trivy" / "result.json"
                if failure == "producer":
                    producer_report.write_text("tampered")
                producer_bytes = producer_report.read_bytes()
                timestamp = datetime.now(timezone.utc).isoformat()
                commands = []

                def invoke(command, **kwargs):
                    commands.append(command)
                    cache = Path(command[command.index("--cache-dir") + 1])
                    is_baseline = cache.parent.name == "baseline"
                    if "--version" in command:
                        return subprocess.CompletedProcess(command, 0, stdout=json.dumps({
                            "Version": "0.76.2" if failure == "version" and is_baseline else "0.76.1",
                            "VulnerabilityDB": {"UpdatedAt": timestamp}}))
                    # Model the Linux scan host's config bytes even when tests run on Windows.
                    Path(command[command.index("--config") + 1]).write_bytes(b"{}\n")
                    database = cache / "db" / "trivy.db"
                    if is_baseline:
                        self.assertIn("--skip-db-update", command)
                        self.assertEqual(database.read_bytes(), b"local frozen database")
                    else:
                        self.assertNotIn("--skip-db-update", command)
                        database.parent.mkdir(parents=True)
                        database.write_bytes(b"local frozen database")
                    if failure == "db" and is_baseline:
                        database.write_bytes(b"unexpected database")
                    requested = command[-1]
                    result = {"Metadata": {
                        "RepoDigests": ["wrong" if failure == "digest" else requested],
                        "ImageConfig": {"os": "linux", "architecture": "amd64"}},
                        "Results": [{"Target": "opt/other" if failure == "coverage" and is_baseline else "opt/app",
                                     "Class": "lang-pkgs", "Type": "gobinary",
                                     "Packages": [{"Name": "example/package", "Version": "1"}],
                                     "Vulnerabilities": [{
                                         "VulnerabilityID": cve, "PkgName": "example/package",
                                         "InstalledVersion": "1", "FixedVersion": "2",
                                         "Severity": "LOW"}] if is_baseline else []}]}
                    Path(command[command.index("--output") + 1]).write_text(json.dumps(result))
                    return subprocess.CompletedProcess(command, 0)

                with patch.object(scan_candidate.scan_trivy.subprocess, "run", side_effect=invoke):
                    if failure:
                        with self.assertRaises(ValueError):
                            scan_candidate.run(123, source["commit"], self.baseline, "linux/amd64",
                                               [cve], output, verified)
                        self.assertFalse((output / "comparison.json").exists())
                        if failure == "producer":
                            self.assertFalse(commands)
                    else:
                        result = scan_candidate.run(123, source["commit"], self.baseline, "linux/amd64",
                                                    [cve], output, verified)
                        self.assertEqual(result["verdict"], "PASS")
                        self.assertEqual(result["source"], source)
                        self.assertEqual(result["candidate"]["requestedRef"], image["ref"])
                        self.assertEqual(result["candidate"]["dbSha256"],
                                         hashlib.sha256(b"local frozen database").hexdigest())
                        self.assertEqual(result["candidate"]["dbSha256"], result["baseline"]["dbSha256"])
                        self.assertEqual(len(commands), 4)
                self.assertEqual(producer_report.read_bytes(), producer_bytes)
                self.assertFalse((verified / scan_candidate.fetch_candidate.SCAN_ARTIFACT / "cache").exists())

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
