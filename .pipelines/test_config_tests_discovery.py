import pathlib
import unittest

import yaml

ROOT = pathlib.Path(__file__).parent / "azure-pipeline-config-tests.yml"
GATE = "eq('${{ parameters.discoveryOnly }}', 'False')"
ON = "${{ if eq(parameters.discoveryOnly, true) }}"


def load():
    doc = yaml.safe_load(ROOT.read_text(encoding="utf-8"))
    params = {p["name"]: p for p in doc["parameters"]}
    return doc, params


def expand(doc, discovery):
    """Minimal expander: keeps stages gated by `if discoveryOnly == true` only when discovery is on."""
    stages = []
    for item in doc["stages"]:
        if ON in item:
            if discovery:
                stages += item[ON]
        else:
            stages.append(item)
    return stages


class DiscoveryOnlyIsolation(unittest.TestCase):
    def test_parameter_defaults_off(self):
        _, params = load()
        self.assertIs(params["discoveryOnly"]["default"], False)

    def test_default_expansion_has_no_discovery_stage(self):
        doc, _ = load()
        self.assertEqual([s["stage"] for s in expand(doc, False)], ["Deploy"])

    def test_discovery_expansion_is_template_only(self):
        doc, _ = load()
        scan = expand(doc, True)[-1]
        # Without an explicit empty dependsOn it implicitly follows the skipped Deploy stage and never runs.
        self.assertEqual(scan.get("dependsOn"), [])
        text = yaml.safe_dump(scan)
        for banned in ("environment", "deployment", "azureSubscription", "AzureCLI", "kubectl", "testkube", "azure-template-candidate"):
            self.assertNotIn(banned, text)
        self.assertIn("pr-e2e/published-scan-steps.yml", text)

    def test_every_cluster_job_is_gated_for_all_triggers(self):
        doc, _ = load()
        deploy = expand(doc, True)[0]
        self.assertEqual(deploy["stage"], "Deploy")
        self.assertIn(GATE, deploy["condition"])
        jobs = [j for j in deploy["jobs"] if "deployment" in j]
        self.assertEqual([j["deployment"] for j in jobs], ["Testkube", "CandidateTestkube"])
        for job in jobs:
            self.assertIn(GATE, job["condition"])
            # No always()/failed() escape: a skipped job cannot run cleanup steps.
            self.assertTrue(job["condition"].startswith("and(succeeded()"))
        self.assertTrue(all("deployment" in j for j in deploy["jobs"]), "ungated non-deployment job in Deploy")


if __name__ == "__main__":
    unittest.main()
