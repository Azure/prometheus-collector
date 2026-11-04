import copy
import hashlib
import json
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch
from datetime import datetime, timezone

import compare_trivy
import normalize


class ComparisonTests(unittest.TestCase):
    def test_only_matched_present_to_absent_without_new_findings_passes(self):
        now = datetime.now(timezone.utc)
        cve = "CVE-2026-1234"
        image = lambda char: "example.invalid/collector@sha256:" + char * 64
        receipt = {"product": "ama-metrics", "source": {"repo": "Azure/prometheus-collector",
                   "commit": "a" * 40, "buildId": "123"}, "buildResult": "succeeded",
                   "requestedQids": [], "requestedCves": [cve],
                   "images": [{"name": "collector", "ref": image("b"), "os": "linux", "arch": "amd64"}]}
        finding = {"VulnerabilityID": cve, "PkgName": "example/package", "InstalledVersion": "1",
                   "FixedVersion": "2", "Severity": "HIGH"}
        data = {"Metadata": {"RepoDigests": [image("a")], "ImageConfig": {"os": "linux", "architecture": "amd64"}},
                "Results": [{"Target": "opt/main", "Class": "lang-pkgs", "Type": "gobinary",
                             "Packages": [{"Name": "example/package", "Version": "1"}],
                             "Vulnerabilities": [finding]}]}
        metadata = {"exitCode": 0, "requestedRef": image("a"), "platform": "linux/amd64",
                    "version": "test-version", "dbUpdatedAt": now.isoformat(),
                    "finishedUtc": now.isoformat(), "dbSha256": "c" * 64,
                    "configuration": {"scanners": ["vuln"], "listAllPkgs": True,
                                      "ignoreSha256": hashlib.sha256(b"").hexdigest(),
                                      "configSha256": hashlib.sha256(b"{}\n").hexdigest()}}
        with tempfile.TemporaryDirectory() as directory:
            baseline, candidate = [Path(directory) / name for name in ("before", "after")]

            def write(root, content, meta):
                folder = root / "trivy"
                folder.mkdir(parents=True, exist_ok=True)
                raw = json.dumps(content).encode()
                (folder / "result.json").write_bytes(raw)
                (folder / "metadata.json").write_text(json.dumps({
                    **meta, "resultSha256": hashlib.sha256(raw).hexdigest()}))

            write(baseline, data, metadata)
            after = copy.deepcopy(data)
            after["Metadata"]["RepoDigests"] = [image("b")]
            after["Results"][0]["Vulnerabilities"] = []
            after["Results"][0]["Packages"][0]["Version"] = "2"
            candidate_meta = {**metadata, "requestedRef": image("b")}
            write(candidate, after, candidate_meta)
            self.assertEqual(compare_trivy.compare(baseline, candidate, receipt, [cve], now)["verdict"], "PASS")
            for failure in ("db", "missing-package", "open", "new", "wrong-image"):
                with self.subTest(failure=failure):
                    changed, meta = copy.deepcopy(after), dict(candidate_meta)
                    if failure == "db":
                        meta["dbSha256"] = "d" * 64
                    elif failure == "missing-package":
                        changed["Results"][0]["Packages"] = []
                    elif failure in ("open", "new"):
                        changed["Results"][0]["Vulnerabilities"] = [
                            {**finding, "VulnerabilityID": cve if failure == "open" else "CVE-2026-9999"}]
                    else:
                        meta["requestedRef"] = image("c")
                    write(candidate, changed, meta)
                    if failure in ("db", "wrong-image"):
                        with self.assertRaises(ValueError):
                            compare_trivy.compare(baseline, candidate, receipt, [cve], now)
                    else:
                        self.assertNotEqual(compare_trivy.compare(baseline, candidate, receipt, [cve], now)["verdict"], "PASS")

    def test_regression_guard_fails_when_broken(self):
        def broken(current, previous):
            current["newFindings"] = []
        with patch.object(normalize, "compare_findings", side_effect=broken):
            result = unittest.TestResult()
            ComparisonTests("test_only_matched_present_to_absent_without_new_findings_passes").run(result)
        self.assertTrue(result.failures, result.errors)
        self.assertFalse(result.errors)


if __name__ == "__main__":
    unittest.main()
