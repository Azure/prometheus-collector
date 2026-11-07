import pathlib
import sys
import unittest

import yaml

ROOT = pathlib.Path(__file__).parent / "azure-pipeline-config-tests.yml"
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


def check_expanded(doc):
    stages = doc["stages"]
    assert [stage["stage"] for stage in stages] == ["DiscoveryScan"]
    scan = stages[0]
    assert scan["dependsOn"] == []
    assert [job["job"] for job in scan["jobs"]] == ["DiscoveryScan"]
    text = yaml.safe_dump(stages).lower()
    for forbidden in ("deployment:", "environment:", "azurecli@", "kubectl",
                      "campaignproducer", "resources.pipeline",
                      "az aks", "az deployment", "azureSubscription:".lower()):
        assert forbidden not in text, forbidden
    steps = scan["jobs"][0]["steps"]
    assert steps[0] == {
        "task": "30f35852-3f7e-4c0c-9a88-e127b4f97211@1",
        "condition": False, "inputs": {"alias": "none"},
    }, steps[0]
    assert [step["task"] for step in steps[1:]] == [
        "Bash@3", "Bash@3", "PublishPipelineArtifact@1", "PublishPipelineArtifact@1",
    ]
    variables = {row["name"]: row["value"] for row in doc["variables"]}
    assert variables["campaignProducerBuildId"] == "0"
    assert variables["campaignSourceSha"] == ""


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

    def test_every_cluster_job_is_gated_for_all_triggers(self):
        doc, _ = load()
        self.assertEqual([s["stage"] for s in expand(doc, True)], ["DiscoveryScan"])
        deploy = expand(doc, False)[0]
        self.assertEqual(deploy["stage"], "Deploy")
        jobs = [j for j in deploy["jobs"] if "deployment" in j]
        self.assertEqual([j["deployment"] for j in jobs], ["Testkube", "CandidateTestkube"])
        for job in jobs:
            # No always()/failed() escape: a skipped job cannot run cleanup steps.
            self.assertTrue(job["condition"].startswith("and(succeeded()"))
        self.assertTrue(all("deployment" in j for j in deploy["jobs"]), "ungated non-deployment job in Deploy")


if __name__ == "__main__":
    if len(sys.argv) == 3 and sys.argv[1] == "--expanded-yaml":
        check_expanded(yaml.safe_load(pathlib.Path(sys.argv[2]).read_text(encoding="utf-8")))
        print("Expanded discovery isolation passed")
    else:
        unittest.main()
