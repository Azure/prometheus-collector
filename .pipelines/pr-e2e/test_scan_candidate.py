from contextlib import redirect_stdout
import hashlib
import io
import json
from pathlib import Path
import subprocess
import tempfile
import unittest
from unittest.mock import call, patch
from datetime import datetime, timezone

import scan_candidate
from test_fetch_candidate import protected_scan_artifact


class CampaignScanTests(unittest.TestCase):
    def setUp(self):
        self.baseline = ("mcr.microsoft.com/azuremonitor/containerinsights/ciprod/"
                         "prometheus-collector/images@sha256:" + "a" * 64)
        self.image = {
            "name": "collector", "os": "linux", "arch": "amd64",
            "ref": "candidate@sha256:" + "b" * 64,
            "indexRef": "candidate@sha256:" + "d" * 64,
        }
        self.source = {"repo": "Azure/prometheus-collector", "commit": "c" * 40, "buildId": "123"}
        self.receipt = {"source": self.source, "images": [self.image]}

    def test_cli_logs_each_comparison_and_failure_reason_without_secrets(self):
        baseline = {"id": "CVE-2026-63209", "package": "example/pkg/private-token-value", "path": "opt/main",
                    "digest": "sha256:" + "a" * 64, "installed": "1", "fixed": "2",
                    "evidence": "must-not-appear"}
        new = dict(baseline, id="CVE-2026-99999", digest="sha256:" + "b" * 64, package="p" * 4000)
        result = {"source": self.source, "verdict": "FAIL",
                  "scope": "Linux/amd64 collector Trivy findings; existing deferred findings are reported",
                  "findingPolicy": "No severity or fixability filters; new findings block",
                  "baseline": {"requestedRef": self.baseline}, "candidate": {"requestedRef": self.image["ref"]},
                  "baselineFindings": [
            {"finding": baseline, "candidateFinding": dict(baseline, installed="2"), "status": "OPEN"},
            {"finding": dict(baseline, id="CVE-2026-11111"), "candidateFinding": None, "status": "CLOSED"},
            {"finding": dict(baseline, path="opt/promconfigvalidator"),
             "candidateFinding": dict(baseline, path="opt/promconfigvalidator"), "status": "DEFERRED"},
        ], "newFindings": [new], "smokeTests": [{"id": baseline["id"], "status": "OPEN"}]}
        args = ["scan_candidate.py", "--build-id", "123", "--source-commit", self.source["commit"],
                "--baseline-image", self.baseline, "--platform", "linux/amd64",
                "--cve", baseline["id"], "--output", "evidence"]
        output = io.StringIO()
        with patch("sys.argv", args), patch.dict("os.environ", {"SYSTEM_ACCESSTOKEN": "private-token-value"}), \
                patch.object(scan_candidate, "run", return_value=result), \
                redirect_stdout(output):
            self.assertEqual(scan_candidate.main(), 1)
        text = output.getvalue()
        for expected in ("CVE-2026-63209", "CVE-2026-11111", "CVE-2026-99999",
                         "example/pkg", "opt/main", "opt/promconfigvalidator", "OPEN", "CLOSED",
                         "DEFERRED", "NEW", "finding remains in candidate",
                         "deferred to the separately managed Collector upgrade",
                         "existing deferred findings are reported", "new findings block",
                         self.baseline, self.image["ref"]):
            self.assertIn(expected, text)
        rows = [json.loads(line.removeprefix("SCAN_COMPARISON ")) for line in text.splitlines()
                if line.startswith("SCAN_COMPARISON ")]
        parts = [json.loads(line.removeprefix("SCAN_COMPARISON_PART ")) for line in text.splitlines()
                 if line.startswith("SCAN_COMPARISON_PART ")]
        self.assertEqual([part["part"] for part in parts], list(range(1, len(parts) + 1)))
        self.assertTrue(all(part["parts"] == len(parts) for part in parts))
        rows.append(json.loads("".join(part["data"] for part in parts)))
        self.assertEqual([row["status"] for row in rows], ["OPEN", "CLOSED", "DEFERRED", "NEW"])
        self.assertEqual(rows[-1]["candidate"]["package"], "p" * 4000)
        self.assertNotIn("must-not-appear", text)
        self.assertNotIn("private-token-value", text)

    def test_validates_producer_then_scans_only_baseline_with_producer_database(self):
        comparison = {"source": self.source, "verdict": "PASS"}
        with tempfile.TemporaryDirectory() as directory:
            output = Path(directory) / "evidence"
            verified = Path(directory) / "verified"
            scan_candidate.fetch_candidate.extract_scan_artifact(
                protected_scan_artifact(self.source, self.image, snapshot=True),
                verified / scan_candidate.fetch_candidate.SCAN_ARTIFACT)
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

    def test_fetch_path_also_reuses_producer_scan(self):
        comparison = {"source": self.source, "verdict": "BLOCKED"}
        with tempfile.TemporaryDirectory() as directory:
            output = Path(directory) / "evidence"
            verified = output / "verified-candidate"

            def fetch_scan(build_id, commit, destination):
                scan_candidate.fetch_candidate.extract_scan_artifact(
                    protected_scan_artifact(self.source, self.image, snapshot=True),
                    destination / scan_candidate.fetch_candidate.SCAN_ARTIFACT)
                return self.receipt

            with patch.object(scan_candidate.fetch_candidate, "fetch", side_effect=fetch_scan) as fetch, \
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
                    call(self.baseline, "linux/amd64", output / "baseline", output / "candidate"),
                ])

    def test_fresh_snapshot_is_reused_and_expired_snapshot_is_refreshed(self):
        source = self.source
        image = self.image
        cve = "CVE-2026-63209"
        receipt = {"source": source, "images": [image], "product": "ama-metrics",
                   "buildResult": "succeeded", "requestedQids": [], "requestedCves": [cve]}
        build = {"status": "completed", "result": "succeeded", "id": 123,
                 "definition": {"id": 440}, "sourceVersion": source["commit"],
                 "repository": {"id": source["repo"], "type": "GitHub"},
                 "sourceBranch": "refs/heads/bot/ama-metrics-sre-test",
                 "url": scan_candidate.fetch_candidate.ADO_PROJECT + "/_apis/build/builds/123"}
        for failure in (None, "producer", "missing-db", "stale", "stale-refresh", "future",
                        "db", "version", "coverage", "digest"):
            with self.subTest(failure=failure), \
                    tempfile.TemporaryDirectory(dir=Path(__file__).parent) as directory:
                expired = failure in ("stale", "stale-refresh")
                root = Path(directory).resolve()
                verified, output = root / "verified", root / "comparison"
                scan_candidate.fetch_candidate.extract_scan_artifact(
                    protected_scan_artifact(source, image, snapshot=failure != "missing-db"),
                    verified / scan_candidate.fetch_candidate.SCAN_ARTIFACT)
                (verified / "build.json").write_text(json.dumps(build))
                (verified / "candidate-build.json").write_text(json.dumps(receipt))
                producer_report = verified / scan_candidate.fetch_candidate.SCAN_ARTIFACT / "trivy" / "result.json"
                if failure == "producer":
                    producer_report.write_text("tampered")
                if expired or failure == "future":
                    scan_directory = producer_report.parent.parent
                    metadata_path = scan_directory / "trivy" / "metadata.json"
                    metadata = json.loads(metadata_path.read_text())
                    if expired:
                        database_metadata = scan_directory / "cache/db/metadata.json"
                        db = json.loads(database_metadata.read_text())
                        db["UpdatedAt"] = metadata["dbUpdatedAt"] = "2020-01-01T00:00:00+00:00"
                        database_metadata.write_text(json.dumps(db))
                    else:
                        metadata["finishedUtc"] = "2099-01-01T00:00:00+00:00"
                    metadata_path.write_text(json.dumps(metadata))
                    provenance_path = scan_directory / "provenance.json"
                    provenance = json.loads(provenance_path.read_text())
                    provenance["files"]["trivy/metadata.json"] = hashlib.sha256(metadata_path.read_bytes()).hexdigest()
                    if expired:
                        provenance["files"]["cache/db/metadata.json"] = hashlib.sha256(database_metadata.read_bytes()).hexdigest()
                    provenance_path.write_text(json.dumps(provenance))
                producer_bytes = producer_report.read_bytes()
                timestamp = json.loads((producer_report.parent / "metadata.json").read_text())["dbUpdatedAt"]
                database_bytes = b"trivy database"
                if expired:
                    database_bytes = b"fresh trivy database"
                    if failure == "stale":
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
                    self.assertEqual(command[-1], self.baseline if is_baseline else image["ref"])
                    if is_baseline:
                        self.assertIn("--skip-db-update", command)
                        self.assertEqual(database.read_bytes(), database_bytes)
                    else:
                        self.assertTrue(expired)
                        self.assertNotIn("--skip-db-update", command)
                        database.parent.mkdir(parents=True)
                        database.write_bytes(database_bytes)
                        (database.parent / "metadata.json").write_text(json.dumps({"UpdatedAt": timestamp}))
                    if failure == "db":
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
                    if failure not in (None, "stale"):
                        with self.assertRaises(ValueError):
                            scan_candidate.run(123, source["commit"], self.baseline, "linux/amd64",
                                               [cve], output, verified)
                        self.assertFalse((output / "comparison.json").exists())
                        if failure in ("producer", "missing-db", "future"):
                            self.assertFalse(commands)
                        if failure == "stale-refresh":
                            self.assertEqual(len(commands), 2)
                    else:
                        result = scan_candidate.run(123, source["commit"], self.baseline, "linux/amd64",
                                                    [cve], output, verified)
                        self.assertEqual(result["verdict"], "PASS")
                        self.assertEqual(result["source"], source)
                        self.assertEqual(result["candidate"]["requestedRef"], image["ref"])
                        self.assertEqual(result["candidate"]["dbSha256"],
                                         hashlib.sha256(database_bytes).hexdigest())
                        self.assertEqual(result["candidate"]["dbSha256"], result["baseline"]["dbSha256"])
                        self.assertEqual(len(commands), 4 if failure == "stale" else 2)
                        if failure is None:
                            self.assertEqual((output / "candidate" / "trivy" / "result.json").read_bytes(),
                                             producer_bytes)
                            self.assertEqual((output / "candidate" / "trivy" / "scan.log").read_bytes(),
                                             b"scanner diagnostics")
                self.assertEqual(producer_report.read_bytes(), producer_bytes)

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
