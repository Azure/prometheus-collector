import http.server
import json
import re
import sys
import threading
import tomllib
import unittest
from pathlib import Path
from unittest.mock import patch
from urllib.request import urlopen

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
        labels = (COLLECTOR / "test/ginkgo-e2e/utils/constants.go").read_text()
        labels += "".join(path.read_text() for path in (COLLECTOR / "test/ginkgo-e2e/configprocessing").glob("*.go"))
        labels = set(re.findall(r'"(config-processing[-a-z0-9]+)"', labels))
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
                self.assertIn(label, labels, entry["id"])
            if manifest["ginkgoLabels"]:
                self.assertIn("workflow", entry, entry["id"])
            override = entry.get("overrideWorkflowLabels", False)
            self.assertIsInstance(override, bool)
            if "workflow" in entry:
                workflow = contained_file(COLLECTOR / "test/testkube/config-processing-test-crs", entry["workflow"])
                selected = set()
                for doc in yaml.safe_load_all(workflow.read_text()):
                    for step in doc["spec"]["steps"]:
                        selected.update(re.findall(r"config-processing[-a-z0-9]+", step.get("shell", "")))
                if override:
                    self.assertNotEqual(set(manifest["ginkgoLabels"]), selected, entry["id"])
                else:
                    self.assertEqual(set(manifest["ginkgoLabels"]), selected, entry["id"])
        self.assertEqual(paths, set(ROOT.glob("*/manifest.json")))

    def test_legacy_configprocessing_matrix_has_all_supported_combinations(self):
        manifests = {p.parent.name: json.loads(p.read_text()) for p in ROOT.glob("*/manifest.json")}
        expected = {
            "default": {"ama-metrics-settings-configmap-defaults-targets-turned-on.yaml"},
            "custom-scrape": {"ama-metrics-settings-configmap-defaults-targets-turned-on.yaml",
                              "ama-metrics-prometheus-config-configmap-all-actions.yaml"},
            "all-targets-disabled": {"ama-metrics-settings-configmap-all-targets-disabled.yaml"},
            "custom-scrape-no-default-targets": {"ama-metrics-settings-configmap-all-targets-disabled.yaml",
                                                  "ama-metrics-prometheus-config-configmap-all-actions.yaml"},
            "custom-config-error": {"ama-metrics-settings-configmap-defaults-targets-turned-on.yaml",
                                    "ama-metrics-prometheus-config-configmap-with-error.yaml"},
            "custom-config-error-no-default-targets": {"ama-metrics-settings-configmap-all-targets-disabled.yaml",
                                                        "ama-metrics-prometheus-config-configmap-with-error.yaml"},
        }
        for variant, required_files in expected.items():
            with self.subTest(variant=variant):
                manifest = manifests[variant]
                names = {Path(path).name for path in manifest["configmaps"]}
                self.assertTrue(required_files <= names)

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

    def test_empty_secret_scenarios_select_watch_nothing_inputs(self):
        for version in ("v1", "v2"):
            for mode in ("empty", "omitted"):
                variant = f"{mode}-secret-namespaces-{version}"
                with self.subTest(variant=variant):
                    path = ROOT / variant / "manifest.json"
                    manifest = json.loads(path.read_text())
                    values = configured_settings(path, manifest)
                    prefix = f"configmap.{version}.prometheus-collector-settings."
                    self.assertIs(values[prefix + "operator_enabled"], True)
                    if mode == "empty":
                        self.assertEqual(values[prefix + "secrets_access_namespaces"], "")
                    else:
                        self.assertNotIn(prefix + "secrets_access_namespaces", values)
                    self.assertIn("config-processing-empty-secret-namespaces", manifest["ginkgoLabels"])
                    self.assertIn("secrets-watch-enforcement", manifest["requires"]["clusterFeatures"])
                    self.assertEqual(manifest["customResources"], ["../secret-watch-rbac.yaml"])

    def test_transition_inputs_preserve_settings_and_resource_ownership(self):
        auth = list(yaml.safe_load_all((ROOT / "auth-targets.yaml").read_text()))
        identities = {(doc["kind"], doc["metadata"].get("namespace"), doc["metadata"]["name"]) for doc in auth}
        recovered = yaml.safe_load((ROOT / "auth-recovered.yaml").read_text())
        self.assertIn((recovered["kind"], recovered["metadata"]["namespace"], recovered["metadata"]["name"]), identities)
        initial_secret = next(doc for doc in auth if doc["kind"] == "Secret")
        self.assertNotEqual(initial_secret["stringData"]["username"], recovered["stringData"]["username"])
        self.assertEqual(recovered["stringData"], {"username": "admin", "password": "pwd"})
        for version in ("v1", "v2"):
            directory = ROOT / f"customer-transitions-{version}"
            prefix = f"configmap.{version}."
            metrics = prefix + ("cluster-metrics." if version == "v2" else "")
            alias = prefix + "prometheus-collector-settings.cluster_alias"
            namespaces = metrics + "pod-annotation-based-scraping.podannotationnamespaceregex"
            target = "default-scrape-settings-enabled" if version == "v1" else "default-targets-scrape-enabled"
            expected = [
                ("initial", "Customer Config POC", "^ama-settings-included$"),
                ("alias-updated", "Customer Config Updated", "^ama-settings-included$"),
                ("alias-cleared", "", "^ama-settings-included$"),
                ("namespace-updated", "", "^ama-settings-excluded$"),
                ("namespace-removed", "", None),
            ]
            baseline = None
            for phase, cluster, regex in expected:
                with self.subTest(version=version, phase=phase):
                    doc = yaml.safe_load((directory / f"{phase}.yaml").read_text())
                    self.assertEqual(doc["metadata"], {"name": "ama-metrics-settings-configmap", "namespace": "kube-system"})
                    values = config_settings(doc["data"])
                    self.assertEqual(values.pop(alias), cluster)
                    self.assertEqual(values.pop(namespaces, None), regex)
                    values.pop(prefix + "config-version")
                    self.assertIs(values[metrics + target + ".windowskubeproxy"], True)
                    self.assertIs(values[prefix + "prometheus-collector-settings.operator_enabled"], True)
                    if baseline is None:
                        baseline = values
                    self.assertEqual(values, baseline)
            for phase in ("allowed", "empty", "omitted"):
                with self.subTest(version=version, secret_phase=phase):
                    doc = yaml.safe_load((ROOT / f"secret-transitions-{version}" / f"{phase}.yaml").read_text())
                    values = config_settings(doc["data"])
                    setting = prefix + "prometheus-collector-settings.secrets_access_namespaces"
                    if phase == "omitted":
                        self.assertNotIn(setting, values)
                    else:
                        self.assertEqual(values[setting], "ama-settings-auth" if phase == "allowed" else "")
                    self.assertIs(values[prefix + "prometheus-collector-settings.operator_enabled"], True)
                    self.assertEqual(doc["metadata"], {"name": "ama-metrics-settings-configmap", "namespace": "kube-system"})
            manifest = json.loads((ROOT / f"secret-transitions-{version}" / "manifest.json").read_text())
            self.assertEqual(manifest["customResources"], ["../auth-targets.yaml"])
        windows = config_settings(yaml.safe_load((ROOT / "windows-kube-proxy" / "settings.yaml").read_text())["data"])
        self.assertIs(windows["configmap.v2.cluster-metrics.default-targets-scrape-enabled.windowskubeproxy"], True)
        self.assertEqual(windows["configmap.v2.cluster-metrics.default-targets-scrape-interval-settings.windowskubeproxy"], "15s")
        self.assertEqual(windows["configmap.v2.cluster-metrics.default-targets-metrics-keep-list.windowskubeproxy"], "kubeproxy_sync_proxy_rules_duration_seconds.*")

    def test_annotation_regex_edge_inputs_and_controls(self):
        controls = list(yaml.safe_load_all((ROOT / "annotation-edge-targets.yaml").read_text()))
        similar = next(doc for doc in controls if doc["kind"] == "Pod" and doc["metadata"]["name"] == "annotation-target")
        unannotated = next(doc for doc in controls if doc["kind"] == "Pod" and doc["metadata"]["name"] == "unannotated-target")
        self.assertEqual(similar["metadata"]["namespace"], "ama-settings-included-similar")
        self.assertEqual(similar["metadata"]["annotations"]["prometheus.io/scrape"], "true")
        with self.subTest(control="unannotated"):
            self.assertNotIn("prometheus.io/scrape", unannotated["metadata"].get("annotations", {}))
        self.assertEqual(unannotated["metadata"]["namespace"], "ama-settings-included")
        expected = {"default": None, "empty": "", "alternation": "^(ama-settings-included|ama-settings-excluded)$", "wildcard": ".*", "invalid": "["}
        for version in ("v1", "v2"):
            directory = ROOT / f"annotation-regex-edges-{version}"
            manifest = json.loads((directory / "manifest.json").read_text())
            self.assertEqual(manifest["configmaps"], ["default.yaml"])
            self.assertEqual(manifest["customResources"], ["../annotation-targets.yaml", "../annotation-edge-targets.yaml"])
            prefix = f"configmap.{version}."
            metrics = prefix + ("cluster-metrics." if version == "v2" else "")
            baseline = None
            for phase, regex in expected.items():
                with self.subTest(version=version, phase=phase):
                    doc = yaml.safe_load((directory / f"{phase}.yaml").read_text())
                    self.assertEqual(doc["metadata"], {"name": "ama-metrics-settings-configmap", "namespace": "kube-system"})
                    values = config_settings(doc["data"])
                    self.assertEqual(values.pop(metrics + "pod-annotation-based-scraping.podannotationnamespaceregex", None), regex)
                    values.pop(prefix + "config-version")
                    if baseline is None:
                        baseline = values
                    self.assertEqual(values, baseline)

    def test_annotation_fixtures_serve_required_weather_metric(self):
        pods = [
            doc for name in ("annotation-targets.yaml", "annotation-edge-targets.yaml")
            for doc in yaml.safe_load_all((ROOT / name).read_text()) if doc["kind"] == "Pod"
        ]
        self.assertEqual(len(pods), 4)
        scripts = set()
        for pod in pods:
            container = pod["spec"]["containers"][0]
            self.assertEqual(container["command"], ["python3", "-c"])
            self.assertEqual(container["ports"][0]["containerPort"], 2112)
            self.assertRegex(container["image"], r"^mcr\.microsoft\.com/azurelinux/base/python@sha256:[0-9a-f]{64}$")
            scripts.add(container["args"][0])
        self.assertEqual(len(scripts), 1)
        script = scripts.pop()
        self.assertIn('HTTPServer(("", 2112), Handler)', script)

        def verify(server):
            server.timeout = 5
            worker = threading.Thread(target=server.handle_request)
            worker.start()
            try:
                with urlopen(f"http://127.0.0.1:{server.server_port}/metrics", timeout=5) as response:
                    self.assertEqual(response.headers["Content-Type"], "text/plain; version=0.0.4")
                    self.assertEqual(response.read(), b"weather_temperature 1\n")
            finally:
                worker.join(timeout=6)
                server.server_close()
            self.assertFalse(worker.is_alive())

        with patch.object(http.server.HTTPServer, "serve_forever", verify):
            exec(script.replace('("", 2112)', '("127.0.0.1", 0)'), {})

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
