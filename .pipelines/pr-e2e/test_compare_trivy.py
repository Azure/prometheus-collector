import copy
import hashlib
import json
from pathlib import Path
import re
import tempfile
import unittest
from unittest.mock import patch
from datetime import datetime, timezone

import compare_trivy
import normalize


class ComparisonTests(unittest.TestCase):
    def test_load_accepts_nanosecond_database_timestamp(self):
        now = datetime.now(timezone.utc)
        image = "example.invalid/collector@sha256:" + "a" * 64
        timestamp = now.isoformat(timespec="microseconds").replace("+00:00", "123+00:00")
        data = {
            "Metadata": {
                "RepoDigests": [image],
                "ImageConfig": {"os": "linux", "architecture": "amd64"},
            },
            "Results": [{"Target": "opt/app", "Class": "lang-pkgs", "Type": "gobinary"}],
        }
        raw = json.dumps(data).encode()
        metadata = {
            "exitCode": 0,
            "requestedRef": image,
            "platform": "linux/amd64",
            "version": "test-version",
            "dbUpdatedAt": timestamp,
            "finishedUtc": now.isoformat(timespec="microseconds"),
            "dbSha256": "c" * 64,
            "configuration": {
                "scanners": ["vuln"],
                "listAllPkgs": True,
                "ignoreSha256": hashlib.sha256(b"").hexdigest(),
                "configSha256": hashlib.sha256(b"{}\n").hexdigest(),
            },
            "resultSha256": hashlib.sha256(raw).hexdigest(),
        }
        with tempfile.TemporaryDirectory() as directory:
            trivy = Path(directory) / "trivy"
            trivy.mkdir()
            (trivy / "result.json").write_bytes(raw)
            (trivy / "metadata.json").write_text(json.dumps(metadata))
            parsed_values = []
            real_datetime = normalize.datetime

            class StrictDateTime:
                @staticmethod
                def fromisoformat(value):
                    parsed_values.append(value)
                    if re.search(r"\.\d{7,}[+-]\d{2}:\d{2}$", value):
                        raise ValueError(f"Invalid isoformat string: {value!r}")
                    return real_datetime.fromisoformat(value)

            with patch.object(normalize, "datetime", StrictDateTime):
                loaded, _, _ = compare_trivy.load(Path(directory), now)
            self.assertEqual(loaded["dbUpdatedAt"], timestamp)
            self.assertEqual(parsed_values[0], now.isoformat(timespec="microseconds"))

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
        data = {"Metadata": {"RepoDigests": [image("a")], "ImageConfig": {"os": "linux", "architecture": "amd64"},
                             "OS": {"Family": "azurelinux", "Name": "3.0"}},
                "Results": [{"Target": "opt/main", "Class": "lang-pkgs", "Type": "gobinary",
                             "Packages": [{"Name": "example/package", "Version": "1"}],
                             "Vulnerabilities": [finding]},
                            {"Target": image("a") + " (azurelinux 3.0)", "Class": "os-pkgs", "Type": "azurelinux",
                             "Packages": [{"Name": "os-package", "Version": "1"}],
                             "Vulnerabilities": [{**finding, "VulnerabilityID": "CVE-2026-5678",
                                                   "PkgName": "os-package", "Severity": "LOW"}]}]}
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
            after["Results"][1]["Target"] = image("b") + " (azurelinux 3.0)"
            after["Results"][0]["Vulnerabilities"] = []
            after["Results"][1]["Vulnerabilities"] = []
            after["Results"][0]["Packages"][0]["Version"] = "2"
            candidate_meta = {**metadata, "requestedRef": image("b")}
            write(candidate, after, candidate_meta)
            result = compare_trivy.compare(baseline, candidate, receipt, [cve], now)
            self.assertEqual(result["verdict"], "PASS")
            self.assertEqual(result["baselineFindingCount"], 2)
            self.assertEqual([row["status"] for row in result["baselineFindings"]], ["CLOSED", "CLOSED"])
            self.assertEqual(result["smokeTests"], [{"id": cve, "status": "CLOSED"}])
            self.assertTrue(result["unsupportedCoverage"])
            for failure in ("db", "missing-package", "open", "residual", "new", "wrong-image",
                            "os-source", "os-version", "os-type", "missing-target", "language-path"):
                with self.subTest(failure=failure):
                    changed, meta = copy.deepcopy(after), dict(candidate_meta)
                    if failure == "db":
                        meta["dbSha256"] = "d" * 64
                    elif failure == "missing-package":
                        changed["Results"][0]["Packages"] = []
                    elif failure == "residual":
                        changed["Results"][1]["Vulnerabilities"] = [
                            {**finding, "VulnerabilityID": "CVE-2026-5678", "PkgName": "os-package",
                             "FixedVersion": "", "Severity": "LOW"}]
                    elif failure in ("open", "new"):
                        changed["Results"][0]["Vulnerabilities"] = [
                            {**finding, "VulnerabilityID":
                             cve if failure == "open" else "CVE-2026-9999"}]
                    elif failure == "wrong-image":
                        meta["requestedRef"] = image("c")
                    elif failure == "os-source":
                        changed["Results"][1]["Target"] = image("a") + " (azurelinux 3.0)"
                    elif failure == "os-version":
                        changed["Metadata"]["OS"]["Name"] = "4.0"
                        changed["Results"][1]["Target"] = image("b") + " (azurelinux 4.0)"
                    elif failure == "os-type":
                        changed["Results"][1]["Type"] = "other-os"
                    elif failure == "missing-target":
                        changed["Results"].pop()
                    else:
                        changed["Results"][0]["Target"] = "opt/different"
                    write(candidate, changed, meta)
                    if failure in ("db", "wrong-image", "os-source", "os-version", "os-type",
                                   "missing-target", "language-path"):
                        with self.assertRaises(ValueError):
                            compare_trivy.compare(baseline, candidate, receipt, [cve], now)
                    else:
                        self.assertNotEqual(compare_trivy.compare(baseline, candidate, receipt, [cve], now)["verdict"], "PASS")
            write(candidate, after, candidate_meta)
            unreported_smoke = compare_trivy.compare(
                baseline, candidate, receipt, ["CVE-2026-9998"], now)
            self.assertEqual(unreported_smoke["verdict"], "BLOCKED")
            self.assertEqual(unreported_smoke["smokeTests"], [
                {"id": "CVE-2026-9998", "status": "NOT_REPORTED"}])

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
