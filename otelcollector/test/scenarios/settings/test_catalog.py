import json
import re
import sys
import tomllib
import unittest
from pathlib import Path

import jsonschema
import yaml

ROOT = Path(__file__).resolve().parent
REPO = ROOT.parents[3]
COLLECTOR = REPO / "otelcollector"


def flatten(value, prefix=""):
    result = {}
    for key, child in value.items():
        name = f"{prefix}.{key}" if prefix else key
        if isinstance(child, dict) and child:
            result.update(flatten(child, name))
        else:
            result[name] = child
    return result


def config_settings(data):
    version = data["schema-version"]
    result = {}
    for section, content in data.items():
        prefix = f"configmap.{version}.{section}"
        if section in ("schema-version", "config-version"):
            result[prefix] = content
        elif section == "ksm-config":
            result.update({f"{prefix}.{key}": value for key, value in yaml.safe_load(content).items()})
        elif section in ("cluster-metrics", "controlplane-metrics"):
            for name, toml in yaml.safe_load(content).items():
                result.update(flatten(toml_settings(toml), f"{prefix}.{name}"))
        else:
            result.update(flatten(toml_settings(content), prefix))
    return result


def toml_settings(content):
    # Existing fixtures repeat identical assignments; conflicting duplicates still fail.
    return tomllib.loads("\n".join(dict.fromkeys(content.splitlines())))


def documented_settings():
    settings = {}
    for version in ("v1", "v2"):
        source = COLLECTOR / "configmaps" / f"ama-metrics-settings-configmap-{version}.yaml"
        values = config_settings(yaml.safe_load(source.read_text())["data"])
        for key in values:
            settings[key] = str(source.relative_to(REPO)).replace("\\", "/")
        # KSM is a commented customer example in both published schemas.
        for key in ("resources", "labels_allow_list", "annotations_allow_list"):
            settings[f"configmap.{version}.ksm-config.{key}"] = str(source.relative_to(REPO)).replace("\\", "/")
    source = COLLECTOR / "deploy/chart/prometheus-collector/values-template.yaml"
    values = yaml.safe_load(source.read_text())
    for key in ("internalSettings", "arcAutonomousSettings"):
        values.pop(key)
    for key in flatten(values, "helm"):
        settings[key] = str(source.relative_to(REPO)).replace("\\", "/")
    parser = COLLECTOR / "shared/configmap/mp/tomlparser-prometheus-collector-settings.go"
    for name in re.findall(r'settings\["([^"]+)"\]', parser.read_text()):
        for version in ("v1", "v2"):
            settings.setdefault(f"configmap.{version}.prometheus-collector-settings.{name}", str(parser.relative_to(REPO)).replace("\\", "/"))
    for version in ("v1", "v2"):
        section = "opentelemetry-metrics" if version == "v1" else "cluster-metrics.opentelemetry-metrics"
        settings[f"configmap.{version}.{section}.enabled"] = "otelcollector/shared/configmap/mp/tomlparser-opentelemetry-metrics.go"
    return settings


def contained_file(base, relative):
    if Path(relative).is_absolute():
        raise ValueError(f"Expected a relative resource path: {relative}")
    path = (base / relative).resolve()
    if not path.is_relative_to(REPO) or not path.is_file():
        raise ValueError(f"Missing or out-of-repository resource: {relative}")
    return path


def validate_manifest(path, manifest, schema):
    jsonschema.validate(manifest, schema)
    if manifest["id"] != path.parent.name:
        raise ValueError(f"Manifest id does not match directory: {path}")
    for kind in ("configmaps", "customResources", "helmValues"):
        for relative in manifest.get(kind, []):
            resource = contained_file(path.parent, relative)
            for doc in yaml.safe_load_all(resource.read_text()):
                if not isinstance(doc, dict):
                    raise ValueError(f"Empty or invalid YAML object: {resource}")
                if kind == "configmaps" and (doc.get("kind") != "ConfigMap" or not all(isinstance(v, str) for v in doc["data"].values())):
                    raise ValueError(f"ConfigMap requires string data: {resource}")


def configured_settings(path, manifest):
    values = {}
    if manifest.get("helmValues"):
        defaults = yaml.safe_load((COLLECTOR / "deploy/chart/prometheus-collector/values-template.yaml").read_text())
        values.update(flatten(defaults, "helm"))
        for relative in manifest["helmValues"]:
            values.update(flatten(yaml.safe_load(contained_file(path.parent, relative).read_text()), "helm"))
    if manifest["expectedOutcome"] != "config-error":
        for relative in manifest["configmaps"]:
            for doc in yaml.safe_load_all(contained_file(path.parent, relative).read_text()):
                if doc["metadata"]["name"] == "ama-metrics-settings-configmap":
                    values.update(config_settings(doc["data"]))
    return values


def validate_coverage(coverage, inventory, variants):
    if set(coverage) != set(inventory):
        raise ValueError(f"Inventory drift: missing={sorted(set(inventory) - set(coverage))}, stale={sorted(set(coverage) - set(inventory))}")
    for setting, entry in coverage.items():
        if not entry["variants"]:
            raise ValueError(f"No covering variant for {setting}")
        for variant in entry["variants"]:
            if variant not in variants or setting not in variants[variant]:
                raise ValueError(f"{variant} does not configure {setting}")


class SettingsCatalogTests(unittest.TestCase):
    def test_catalog_schema_resources_and_existing_labels(self):
        schema = json.loads((ROOT / "manifest.schema.json").read_text())
        catalog = json.loads((ROOT / "catalog.json").read_text())
        ids = set()
        labels = (COLLECTOR / "test/ginkgo-e2e/utils/constants.go").read_text() + (COLLECTOR / "test/ginkgo-e2e/configprocessing/customer_settings_test.go").read_text()
        paths = set()
        for entry in catalog["variants"]:
            path = contained_file(ROOT, entry["manifest"])
            manifest = json.loads(path.read_text())
            validate_manifest(path, manifest, schema)
            self.assertEqual(entry["id"], manifest["id"])
            self.assertNotIn(entry["id"], ids)
            ids.add(entry["id"])
            paths.add(path)
            self.assertIn(entry["schedule"], ("pr", "nightly", "weekly"))
            for label in manifest["ginkgoLabels"]:
                self.assertIn(f'"{label}"', labels)
            if "workflow" in entry:
                workflow = contained_file(COLLECTOR / "test/testkube/config-processing-test-crs", entry["workflow"])
                selected = set()
                for doc in yaml.safe_load_all(workflow.read_text()):
                    for step in doc["spec"]["steps"]:
                        selected.update(re.findall(r"config-processing[-a-z0-9]+", step.get("shell", "")))
                self.assertEqual(set(manifest["ginkgoLabels"]), selected, entry["id"])
        self.assertEqual(paths, set(ROOT.glob("*/manifest.json")))

    def test_every_documented_setting_has_a_real_variant(self):
        variants = {p.parent.name: configured_settings(p, json.loads(p.read_text())) for p in ROOT.glob("*/manifest.json")}
        validate_coverage(json.loads((ROOT / "coverage.json").read_text()), documented_settings(), variants)

    def test_management_plane_target_switches_have_both_inputs(self):
        variants = [configured_settings(p, json.loads(p.read_text())) for p in ROOT.glob("*/manifest.json")]
        for key in documented_settings():
            if ".default-scrape-settings-enabled." not in key and ".cluster-metrics.default-targets-scrape-enabled." not in key:
                continue
            if key.rsplit(".", 1)[-1].startswith("controlplane-"):
                continue
            states = {values[key] for values in variants if key in values}
            with self.subTest(setting=key):
                self.assertEqual(states, {False, True})

    def test_missing_resource_is_rejected(self):
        path = ROOT / "default/manifest.json"
        manifest = json.loads(path.read_text())
        manifest["configmaps"] = ["does-not-exist.yaml"]
        with self.assertRaisesRegex(ValueError, "Missing"):
            validate_manifest(path, manifest, json.loads((ROOT / "manifest.schema.json").read_text()))

    def test_empty_labels_cannot_select_every_suite(self):
        path = ROOT / "default/manifest.json"
        manifest = json.loads(path.read_text())
        manifest["ginkgoLabels"] = []
        schema = json.loads((ROOT / "manifest.schema.json").read_text())
        with self.assertRaises(jsonschema.ValidationError):
            validate_manifest(path, manifest, schema)
        manifest["helmValues"] = []
        with self.assertRaises(jsonschema.ValidationError):
            validate_manifest(path, manifest, schema)

    def test_missing_coverage_is_rejected(self):
        with self.assertRaisesRegex(ValueError, "No covering variant"):
            validate_coverage({"setting": {"variants": []}}, {"setting": "source"}, {})
        with self.assertRaisesRegex(ValueError, "Inventory drift"):
            validate_coverage({}, {"new-setting": "source"}, {})


if __name__ == "__main__":
    if "--require-runtime" in sys.argv:
        coverage = json.loads((ROOT / "coverage.json").read_text())
        gaps = [name for name, entry in coverage.items() if not entry["assertions"]]
        if gaps:
            raise SystemExit("Missing runtime assertions:\n" + "\n".join(gaps))
        sys.argv.remove("--require-runtime")
    unittest.main()
