"""Local evidence checks; synthetic fixtures are not a live scanner result."""
import copy
import json
import tempfile
import unittest
from datetime import datetime, timezone
from pathlib import Path
from unittest.mock import patch

import normalize


NOW = datetime(2026, 11, 4, 12, tzinfo=timezone.utc)
STAMP = "2026-11-04T11:00:00Z"


def targets(digest, commit, build):
    return {
        "product": "ama-metrics",
        "source": {"repo": "Azure/prometheus-collector", "commit": commit * 40, "buildId": build},
        "images": [{"name": "collector", "os": "linux", "arch": "amd64",
                    "ref": "example.invalid/collector@sha256:" + digest * 64}],
        "requestedQids": [1], "requestedCves": [],
    }


def fixture(root, target, present):
    image = target["images"][0]
    directory = root / normalize.lane_id(image)
    values = {
        "provenance.json": {
            "requestedRef": image["ref"],
            "target": {"RepoDigests": [image["ref"]], "Os": "linux", "Architecture": "amd64"},
            "commit": target["source"]["commit"], "buildId": target["source"]["buildId"],
            "scannedInBuildId": "candidate",
            "scannerImage": "example.invalid/scanner@sha256:" + "e" * 64,
            "startedUtc": STAMP, "storageDriver": "overlay2",
            "scanConfiguration": {"arguments": ["--scan-languages", "GO"]},
            "configurationVerified": True, "languagesRequested": ["GO"], "languagesVerified": ["GO"],
        },
        "completion.json": {"exitCode": 1 if present else 0, "finishedUtc": STAMP},
        "comet/result.json": {"vulnerabilities": []},
        "trivy/result.json": {"Metadata": {"RepoDigests": [image["ref"]]}, "Results": []},
        "trivy/metadata.json": {"exitCode": 0, "version": "test-version",
                                "finishedUtc": STAMP, "dbUpdatedAt": STAMP},
    }
    for name, value in values.items():
        path = directory / name
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(json.dumps(value), encoding="utf-8")
    (directory / "comet/scan.log").write_text("completed", encoding="utf-8")
    finding = (
        '<VULN><QID>1</QID><FLAGS>u,v</FLAGS><RESULT>package vulnerable</RESULT></VULN>'
        if present else "<VULNS_NOT_FOUND><Q>1</Q></VULNS_NOT_FOUND>")
    (directory / "comet/result.xml").write_text(
        "<HOST><OS>Linux</OS><PARTIAL_REPORT_FLAG>false</PARTIAL_REPORT_FLAG>"
        "<SCA_SCAN_ENABLED>true</SCA_SCAN_ENABLED><VULNSIGS_VERSION>test-signatures</VULNSIGS_VERSION>"
        "<AGENT_MANIFEST_VERSION>test-manifest</AGENT_MANIFEST_VERSION>"
        f"<REPORT_GENERATED_DATE>{STAMP}</REPORT_GENERATED_DATE>{finding}</HOST>", encoding="utf-8")
    return directory


class EvidenceTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.before = targets("b", "b", "baseline")
        self.after = targets("a", "a", "candidate")
        self.after["baseline"] = {
            "sourceCommit": "b" * 40, "buildId": "baseline", "scannedInBuildId": "candidate",
            "targetsArtifact": "baseline-targets", "scanArtifact": "baseline-scans",
        }
        self.baseline = fixture(self.root / "before", self.before, True)
        self.candidate = fixture(self.root / "after", self.after, False)

    def evaluate(self):
        return normalize.evaluate(self.after, self.root / "after", self.before, self.root / "before", NOW)

    def change_json(self, root, filename, field, value):
        path = root / filename
        data = json.loads(path.read_text())
        data[field] = value
        path.write_text(json.dumps(data))

    def test_matched_closed_finding_and_no_regressions_pass(self):
        result = self.evaluate()
        self.assertEqual(result["verdict"], "PASS")
        self.assertEqual(result["requested"][0]["transition"], "PRESENT_TO_ABSENT")
        self.assertEqual(result["newFindings"], [])

    def test_missing_qid_is_not_closure(self):
        path = self.candidate / "comet/result.xml"
        path.write_text(path.read_text().replace("<Q>1</Q>", "<Q>2</Q>"))
        result = self.evaluate()
        self.assertEqual(result["verdict"], "BLOCKED")
        self.assertFalse(result["requested"][0]["closureEvidence"])

    def test_partial_report_wrong_source_or_unverified_coverage_blocks(self):
        original = (self.candidate / "provenance.json").read_text()
        for field, value in (("commit", "c" * 40), ("configurationVerified", False),
                             ("languagesVerified", []), ("scannerImage", "example.invalid/other@sha256:" + "f" * 64)):
            with self.subTest(field=field):
                self.change_json(self.candidate, "provenance.json", field, value)
                self.assertEqual(self.evaluate()["verdict"], "BLOCKED")
                (self.candidate / "provenance.json").write_text(original)
        path = self.candidate / "comet/result.xml"
        path.write_text(path.read_text().replace("false", "true"))
        self.assertEqual(self.evaluate()["verdict"], "BLOCKED")

    def test_stale_scan_and_mismatched_trivy_database_block(self):
        self.change_json(self.candidate, "trivy/metadata.json", "dbUpdatedAt", "2026-11-04T11:30:00Z")
        self.assertEqual(self.evaluate()["regressionVerdict"], "BLOCKED")
        self.change_json(self.candidate, "provenance.json", "startedUtc", "2026-11-01T11:00:00Z")
        self.assertEqual(self.evaluate()["verdict"], "BLOCKED")

    def test_new_trivy_finding_fails_even_when_qid_closed(self):
        data = json.loads((self.candidate / "trivy/result.json").read_text())
        data["Results"] = [{"Target": "opt/main", "Vulnerabilities": [{
            "VulnerabilityID": "CVE-2026-1234", "PkgName": "example.invalid/module",
            "InstalledVersion": "1", "Severity": "HIGH",
        }]}]
        self.change_json(self.candidate, "trivy/result.json", "Results", data["Results"])
        self.assertEqual(self.evaluate()["verdict"], "FAIL")
        self.assertEqual(len(self.evaluate()["newFindings"]), 1)

    def test_version_change_does_not_make_an_existing_finding_new(self):
        candidate, baseline = self.evaluate(), self.evaluate()
        baseline["scanVerdict"] = "PASS"
        finding = {"lane": "collector/linux/amd64", "scanner": "trivy",
                   "id": "CVE-2026-1234", "package": "example/module", "path": "opt/main"}
        baseline["findings"] = [{**finding, "installed": "1"}]
        candidate["findings"] = [{**finding, "installed": "2"}]
        self.assertEqual(normalize.compare_findings(candidate, baseline)["newFindings"], [])

    def test_tags_and_cross_repo_targets_rejected(self):
        for field, value in (("product", "ama-logs"), ("images", [])):
            invalid = {**self.after, field: value}
            with self.assertRaises(ValueError):
                normalize.normalize(invalid, self.root, NOW)
        invalid = copy.deepcopy(self.after)
        invalid["images"][0]["ref"] = "example.invalid/collector:latest"
        with self.assertRaises(ValueError):
            normalize.normalize(invalid, self.root, NOW)

    def test_guard_detects_deliberately_broken_regression_comparison(self):
        self.test_new_trivy_finding_fails_even_when_qid_closed()
        with patch.object(normalize, "compare_findings", side_effect=lambda candidate, baseline: candidate):
            with self.assertRaises(AssertionError):
                self.test_new_trivy_finding_fails_even_when_qid_closed()


if __name__ == "__main__":
    unittest.main()
