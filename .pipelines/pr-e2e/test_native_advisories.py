import unittest
from unittest.mock import patch
from urllib.error import HTTPError
from contextlib import redirect_stdout
from io import StringIO

import native_advisories as native

IMAGE = "example.invalid/image@sha256:" + "1" * 64
METADATA = {"RepoDigests": [IMAGE], "ImageConfig": {"os": "linux", "architecture": "amd64"}}


class NativeAdvisoryTest(unittest.TestCase):
    def test_confirmed_failure_precedes_unresolved_coverage(self):
        hashes = {name: spec["sha256"] for name, spec in native.BINARIES.items()}
        identities = {name: {"sha256": hashes[name], "version": spec["version"]}
                      for name, spec in native.BINARIES.items()}
        scan = {"Metadata": METADATA, "Results": [{"Class": "os-pkgs", "Vulnerabilities": [{
            "PkgName": "metricsext2", "InstalledVersion": native.BINARIES["metricsext2"]["version"],
            "VulnerabilityID": "CVE-2026-1234", "FixedVersion": "1.83.0-3.azl3",
        }]}]}
        with patch.object(native, "package_identities", return_value=identities), \
                patch.object(native, "versions", return_value={"metricsext2": ["2"], "azure-mdsd": []}), \
                patch.object(native, "advisories", return_value=[{"id": "CVE-2026-9999"}]), \
                patch.object(native, "nvd_advisories", return_value=["CVE-2026-1111"]), \
                patch.object(native, "VERIFIED_FINDINGS", {
                    (hashes["metricsext2"], "zlib", "CVE-2026-9999"): ("verified test evidence", "2"),
                }):
            report = native.evaluate(IMAGE, "linux/amd64", scan)
        self.assertEqual(report["verdict"], "FAIL")
        self.assertEqual(report["packages"]["metricsext2"]["verdict"], "FAIL")
        self.assertEqual(report["packages"]["metricsext2"]["needsNewVersion"], "yes")
        self.assertEqual(report["packages"]["metricsext2"]["candidates"][0]["classification"], "UNRESOLVED")
        confirmed = [row for row in report["packages"]["metricsext2"]["candidates"]
                     if row["classification"] == "CONFIRMED"]
        self.assertEqual(confirmed[0]["fixedPackageVersion"], "2")
        self.assertNotIn("fixedPackageVersion", report["packages"]["metricsext2"])
        self.assertEqual(report["packages"]["azure-mdsd"]["verdict"], "BLOCKED")
        self.assertIn("CVE-2026-1111", [row["advisory"] for row in report["packages"]["azure-mdsd"]["candidates"]])

    def test_unknown_binary_and_failed_lookup_do_not_become_clean(self):
        identities = {"metricsext2": {"sha256": None, "version": "unknown"},
                      "azure-mdsd": {"sha256": None, "version": native.BINARIES["azure-mdsd"]["version"]}}
        with patch.object(native, "package_identities", return_value=identities), \
                patch.object(native, "versions", side_effect=ValueError("feed down")), \
                patch.object(native, "advisories", side_effect=ValueError("OSV down")), \
                patch.object(native, "nvd_advisories", side_effect=ValueError("NVD down")):
            report = native.evaluate(IMAGE, "linux/amd64", {"Metadata": METADATA, "Results": []})
        self.assertEqual(report["verdict"], "BLOCKED")
        self.assertIn("No embedded inventory", report["packages"]["metricsext2"]["gap"])
        self.assertIn("OSV down", report["packages"]["azure-mdsd"]["queryErrors"][0])
        self.assertIn("feed down", report["feedGap"])

    def test_missing_rpm_inventory_is_explicitly_blocked(self):
        scan = {"Metadata": METADATA, "Results": [{"Class": "os-pkgs", "Packages": []}]}
        with patch.object(native, "versions", return_value={}), \
                patch.object(native, "advisories") as advisories, \
                patch.object(native, "nvd_advisories") as nvd:
            report = native.evaluate(IMAGE, "linux/amd64", scan)
        self.assertEqual(report["verdict"], "BLOCKED")
        for package in report["packages"].values():
            self.assertIsNone(package["version"])
            self.assertEqual(package["needsNewVersion"], "unknown")
            self.assertIn("Trivy did not enumerate this RPM", package["gap"])
        advisories.assert_not_called()
        nvd.assert_not_called()

    def test_malformed_lookup_responses_are_explicitly_blocked(self):
        packages = [{"Name": name, "Version": spec["version"].rsplit("-", 1)[0],
                     "Release": spec["version"].rsplit("-", 1)[1], "Arch": "x86_64"}
                    for name, spec in native.BINARIES.items()]
        scan = {"Metadata": METADATA, "Results": [{"Class": "os-pkgs", "Packages": packages}]}
        for osv, nvd in (
            (b'{"vulns":[{"id":"GHSA-example","aliases":null}]}', b'{"totalResults":0,"vulnerabilities":[]}'),
            (b'{"vulns":[]}', b'null'),
            (b'{"vulns":[]}', b'{"totalResults":1,"vulnerabilities":[{}]}'),
        ):
            with self.subTest(osv=osv, nvd=nvd), \
                    patch.object(native, "versions", return_value={}), \
                    patch.object(native, "fetch", side_effect=lambda url, body=None: osv if body else nvd):
                report = native.evaluate(IMAGE, "linux/amd64", scan)
                self.assertEqual(report["verdict"], "BLOCKED")
                for package in report["packages"].values():
                    self.assertTrue(package["queryErrors"])

    def test_rpm_match_alone_is_not_confirmed(self):
        identities = {name: {"sha256": None, "version": spec["version"]}
                      for name, spec in native.BINARIES.items()}
        scan = {"Metadata": METADATA, "Results": [{"Class": "os-pkgs",
                "Vulnerabilities": [{"PkgName": "metricsext2", "InstalledVersion": identities["metricsext2"]["version"],
                                      "VulnerabilityID": "CVE-2026-1234"}]}]}
        with patch.object(native, "package_identities", return_value=identities), \
                patch.object(native, "versions", return_value={}), \
                patch.object(native, "advisories", return_value=[]), \
                patch.object(native, "nvd_advisories", return_value=[]):
            report = native.evaluate(IMAGE, "linux/amd64", scan)
        self.assertEqual(report["verdict"], "BLOCKED")
        self.assertEqual(report["packages"]["metricsext2"]["candidates"][0]["classification"], "UNRESOLVED")
        self.assertEqual(native.evaluate("different-image", "linux/amd64", scan)["verdict"], "BLOCKED")
        self.assertIn("platform", native.evaluate(IMAGE, "linux/arm64", scan)["gap"])

    def test_current_cve_associations_are_reported_without_claiming_applicability(self):
        identities = {name: {"sha256": None, "version": spec["version"]}
                      for name, spec in native.BINARIES.items()}
        scan = {"Metadata": METADATA, "Results": []}
        with patch.object(native, "package_identities", return_value=identities), \
                patch.object(native, "versions", return_value={}), \
                patch.object(native, "advisories", return_value=[{"id": "CURL-CVE-2026-1234"}]), \
                patch.object(native, "nvd_advisories", side_effect=lambda vendor, product, version: (
                    ["CVE-2018-0734", "CVE-2026-5678"] if product == "openssl"
                    else ["CVE-2026-1234", "CVE-2026-7654"] if product == "curl" else [])):
            report = native.evaluate(IMAGE, "linux/amd64", scan)
        rows = report["packages"]["azure-mdsd"]["candidates"]
        self.assertIn("CVE-2026-1234", [row["advisory"] for row in rows])
        self.assertIn("CVE-2026-5678", [row["advisory"] for row in rows])
        self.assertNotIn("CVE-2018-0734", [row["advisory"] for row in rows])
        self.assertTrue(all(row["classification"] == "POTENTIAL_ASSOCIATION" for row in rows))
        self.assertIn("not bound to this binary", rows[0]["reason"])
        curl = [row for row in rows if row["component"] == "curl"]
        self.assertEqual(len([row for row in curl if row["advisory"] == "CVE-2026-1234"]), 1)
        self.assertEqual(curl[0]["alsoFoundBy"], ["NVD CPE"])
        self.assertIn("8.0.0-DEV", next(row["reason"] for row in curl if row["advisory"] == "CVE-2026-7654"))

    def test_known_image_and_rpm_identity_are_distinct(self):
        packages = [{"Name": name, "Version": spec["version"].rsplit("-", 1)[0],
                     "Release": spec["version"].rsplit("-", 1)[1], "Arch": "x86_64"}
                    for name, spec in native.BINARIES.items()]
        scan = {"Results": [{"Class": "os-pkgs", "Packages": packages}]}
        known = native.package_identities(native.KNOWN_IMAGE, "linux/amd64", scan)
        other = native.package_identities(IMAGE, "linux/amd64", scan)
        self.assertEqual(known["metricsext2"]["sha256"], native.BINARIES["metricsext2"]["sha256"])
        self.assertIsNone(other["metricsext2"]["sha256"])
        self.assertEqual(other["metricsext2"]["version"], native.BINARIES["metricsext2"]["version"])
        packages[0]["Version"] = "different"
        with self.assertRaises(ValueError):
            native.package_identities(native.KNOWN_IMAGE, "linux/amd64", scan)

    def test_nvd_retries_throttling(self):
        limited = HTTPError("https://example.invalid", 429, "throttled", {}, None)
        with patch.object(native, "fetch", side_effect=[limited, b'{"totalResults":0,"vulnerabilities":[]}']), \
                patch.object(native.time, "sleep") as sleep:
            self.assertEqual(native.nvd_advisories("zlib", "zlib", "1.3.1"), [])
        sleep.assert_called_once_with(6)

    def test_verified_nvd_only_advisory_fails_with_specific_fixed_version(self):
        scan = {"Metadata": {"RepoDigests": [native.KNOWN_IMAGE],
                             "ImageConfig": {"os": "linux", "architecture": "amd64"}}, "Results": []}
        with patch.object(native, "versions", return_value={"azure-mdsd": ["1.43.0-1470"]}), \
                patch.object(native, "advisories", return_value=[]), \
                patch.object(native, "nvd_advisories", side_effect=lambda vendor, product, version: (
                    ["CVE-2024-9143"] if product == "openssl" else [])):
            report = native.evaluate(native.KNOWN_IMAGE, "linux/amd64", scan)
        package = report["packages"]["azure-mdsd"]
        row = package["candidates"][0]
        self.assertEqual(package["verdict"], "FAIL")
        self.assertEqual(row["classification"], "CONFIRMED")
        self.assertEqual(row["fixedPackageVersion"], "1.43.0-1470")

    def test_cares_association_has_verified_fixed_package_without_claiming_linkage(self):
        scan = {"Metadata": {"RepoDigests": [native.KNOWN_IMAGE],
                             "ImageConfig": {"os": "linux", "architecture": "amd64"}}, "Results": []}
        fixed = "2.2026.921.1606-1.azl3"
        with patch.object(native, "versions", return_value={"metricsext2": [fixed]}), \
                patch.object(native, "advisories", side_effect=lambda commit: (
                    [{"id": "CVE-2026-69184"}, {"id": "CVE-2026-69186"}]
                    if commit == native.BINARIES["metricsext2"]["components"][1][2] else [])), \
                patch.object(native, "nvd_advisories", return_value=[]):
            report = native.evaluate(native.KNOWN_IMAGE, "linux/amd64", scan)
        rows = report["packages"]["metricsext2"]["candidates"]
        self.assertEqual({row["advisory"] for row in rows}, {"CVE-2026-69184", "CVE-2026-69186"})
        self.assertTrue(all(row["classification"] == "POTENTIAL_ASSOCIATION" for row in rows))
        self.assertTrue(all(row["fixedPackageVersion"] == fixed for row in rows))
        self.assertEqual(report["verdict"], "BLOCKED")

    def test_unknown_package_emits_shared_rpm_matches_without_clearance(self):
        packages = [{"Name": name, "Version": "9.0", "Release": "1.azl3", "Arch": "x86_64"}
                    for name in native.BINARIES]
        scan = {"Metadata": METADATA, "Results": [{"Class": "os-pkgs", "Packages": packages,
                "Vulnerabilities": [{"PkgName": "boost", "InstalledVersion": "1.83.0-1.azl3",
                                      "VulnerabilityID": "CVE-2026-1234", "FixedVersion": "1.84.0-1.azl3"}]}]}
        console = StringIO()
        with patch.object(native, "versions", side_effect=ValueError("feed down")), redirect_stdout(console):
            report = native.emit(IMAGE, "linux/amd64", scan)
        row = report["packages"]["metricsext2"]["candidates"][0]
        self.assertEqual(row["classification"], "POTENTIAL_ASSOCIATION")
        self.assertEqual(row["fixedComponentVersion"], "1.84.0-1.azl3")
        self.assertEqual(report["verdict"], "BLOCKED")
        self.assertIn("feed down", console.getvalue())
        self.assertIn('"advisory":"CVE-2026-1234"', console.getvalue())
        self.assertIn('"version":"9.0-1.azl3"', console.getvalue())


if __name__ == "__main__":
    unittest.main()
