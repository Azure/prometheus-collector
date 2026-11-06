import copy
import json
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

import published_discovery
import yaml


ROOT = Path(__file__).parent


def finding(cve, package, installed, path="os:debian 12", severity="HIGH"):
    return {
        "lane": "linux/amd64",
        "digest": "sha256:" + ("a" if installed == "1.0" else "b") * 64,
        "scanner": "trivy",
        "id": cve,
        "package": package,
        "installed": installed,
        "fixed": "2.0",
        "path": path,
        "evidence": None,
        "severity": severity,
        "cves": None,
        "enrichment": "supplemental",
        "fixAvailable": True,
    }


class PublishedDiscoveryTests(unittest.TestCase):
    def setUp(self):
        self.manifest = json.loads((ROOT / "published-images.json").read_text())
        self.meta = {
            "version": "0.76.1",
            "dbUpdatedAt": "2026-11-06T10:00:00+00:00",
            "dbSha256": "c" * 64,
            "platform": "linux/amd64",
            "configuration": {"scanners": ["vuln"], "listAllPkgs": True},
        }
        self.data = {"Results": [{"Target": "os:debian 12", "Class": "os-pkgs", "Type": "debian"}]}
        self.production_findings = [
            finding("CVE-2026-10001", "pkg-a", "1.0"),
            finding("CVE-2026-10002", "pkg-b", "1.0"),
        ]
        self.main_findings = [
            finding("CVE-2026-10001", "pkg-a", "1.1"),
            finding("CVE-2026-10003", "pkg-c", "3.0", severity="MEDIUM"),
        ]

    def test_scans_immutable_images_with_one_db_and_reports_discovery_only(self):
        production_scan = {"findings": self.production_findings}
        main_scan = {"findings": self.main_findings}
        with tempfile.TemporaryDirectory() as directory:
            output = Path(directory) / "evidence"
            with patch.object(published_discovery.scan_trivy, "scan") as scan, \
                    patch.object(published_discovery.compare_trivy, "load", side_effect=[
                        (self.meta, self.data, production_scan),
                        (self.meta, self.data, main_scan),
                    ]):
                result = published_discovery.run(ROOT / "published-images.json", output)
            production = self.manifest["images"]["production"]
            main = self.manifest["images"]["main"]
            self.assertEqual(scan.call_args_list[0].args, (
                production["ref"], "linux/amd64", output / "production",
            ))
            self.assertEqual(scan.call_args_list[1].args, (
                main["ref"], "linux/amd64", output / "main", output / "production",
            ))
            self.assertEqual(result["verdict"], "DISCOVERY_ONLY")
            self.assertEqual(result["productionFindings"][0]["status"], "PRESENT_IN_BOTH")
            self.assertEqual(result["productionFindings"][1]["status"], "NOT_PRESENT_IN_MAIN")
            self.assertEqual(result["findingsNotPresentInProduction"], [self.main_findings[1]])
            self.assertEqual(result["installedVersionOrSeverityChanges"], [{
                "production": self.production_findings[0],
                "main": self.main_findings[0],
            }])
            self.assertIn("not proof", result["interpretation"])
            self.assertEqual(json.loads((output / "discovery.json").read_text()),
                             json.loads(json.dumps(result)))
            self.assertEqual(json.loads((output / "published-images.json").read_text()), self.manifest)

    def test_incomparable_database_or_target_coverage_blocks_report(self):
        with tempfile.TemporaryDirectory() as directory:
            output = Path(directory) / "evidence"
            changed_meta = copy.deepcopy(self.meta)
            changed_meta["dbSha256"] = "d" * 64
            with patch.object(published_discovery.scan_trivy, "scan"), \
                    patch.object(published_discovery.compare_trivy, "load", side_effect=[
                        (self.meta, self.data, {"findings": []}),
                        (changed_meta, self.data, {"findings": []}),
                    ]):
                with self.assertRaisesRegex(ValueError, "dbSha256"):
                    published_discovery.run(ROOT / "published-images.json", output)
            self.assertFalse((output / "discovery.json").exists())

        with tempfile.TemporaryDirectory() as directory:
            output = Path(directory) / "evidence"
            different_data = {"Results": [{"Target": "different", "Class": "os-pkgs", "Type": "debian"}]}
            with patch.object(published_discovery.scan_trivy, "scan"), \
                    patch.object(published_discovery.compare_trivy, "load", side_effect=[
                        (self.meta, self.data, {"findings": []}),
                        (self.meta, different_data, {"findings": []}),
                    ]):
                with self.assertRaisesRegex(ValueError, "coverage"):
                    published_discovery.run(ROOT / "published-images.json", output)

    def test_manifest_rejects_unverified_main_publication_state(self):
        manifest = copy.deepcopy(self.manifest)
        manifest["images"]["main"]["source"]["buildStatus"] = "completed"
        with self.assertRaisesRegex(ValueError, "partial-build"):
            published_discovery.validate_manifest(manifest)

    def test_manifest_requires_immutable_production_and_main_refs(self):
        manifest = copy.deepcopy(self.manifest)
        manifest["images"]["main"]["ref"] = manifest["images"]["main"]["ref"].split("@")[0] + ":latest"
        with self.assertRaisesRegex(ValueError, "main image provenance"):
            published_discovery.validate_manifest(manifest)

    def test_scan_template_isolated_from_candidate_and_cloud_actions(self):
        template_path = ROOT / "published-scan-steps.yml"
        source = template_path.read_text()
        template = yaml.safe_load(source)
        steps = template["steps"]
        self.assertEqual([step.get("task", "bash") for step in steps],
                         ["bash", "bash", "PublishPipelineArtifact@1"])
        self.assertEqual(steps[-1]["condition"], "always()")
        self.assertEqual(steps[-1]["inputs"]["artifactName"],
                         "published-image-discovery-$(Build.BuildId)")
        for forbidden in (
            "AzureCLI@2", "azureSubscription", "run-scenario.py", "kubectl", "helm",
            "cleanup", "campaignProducer", "candidateBuildId", "fetch_candidate.py",
            "scan_candidate.py", "System.AccessToken", "440",
        ):
            self.assertNotIn(forbidden, source)
        with self.assertRaises(AssertionError):
            self.assertNotIn("AzureCLI@2", source + "\n- task: AzureCLI@2\n")


if __name__ == "__main__":
    unittest.main()
