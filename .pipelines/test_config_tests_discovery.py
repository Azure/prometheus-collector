import pathlib
import unittest

import yaml

ROOT = pathlib.Path(__file__).parent / "azure-pipeline-config-tests.yml"
GATE = "eq('${{ parameters.discoveryOnly }}', 'False')"
ON = "${{ if eq(parameters.discoveryOnly, true) }}"
OFF = "${{ if eq(parameters.discoveryOnly, false) }}"


def load():
    doc = yaml.safe_load(ROOT.read_text(encoding="utf-8"))
    params = {p["name"]: p for p in doc["parameters"]}
    return doc, params


def expand(doc, discovery):
    stages = []
    for item in doc["stages"]:
        if ON in item:
            if discovery:
                stages += item[ON]
        elif OFF in item:
            if not discovery:
                stages += item[OFF]
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

    def test_discovery_does_not_resolve_or_download_producer(self):
        doc, _ = load()
        v = doc["variables"]
        self.assertIn("${{ if eq(parameters.discoveryOnly, true) }}", v)
        self.assertNotIn("resources.pipeline", yaml.safe_dump(v["${{ if eq(parameters.discoveryOnly, true) }}"]))
        # elseif keeps the producer macro out of discovery; a plain else would resolve it.
        self.assertTrue(any(k.startswith("${{ elseif eq(parameters.candidateMode") for k in v))
        job = expand(doc, True)[-1]["jobs"][0]
        self.assertEqual(job["steps"][0], {"download": "none"})
        self.assertNotIn("campaignProducer", yaml.safe_dump(job))
        self.assertEqual(list(doc["resources"]["pipelines"][0]), [OFF])
        self.assertEqual(len(doc["resources"]["pipelines"]), 1)

    def test_every_cluster_job_is_gated_for_all_triggers(self):
        doc, _ = load()
        self.assertEqual([s["stage"] for s in expand(doc, True)], ["DiscoveryScan"])
        deploy = expand(doc, False)[0]
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
