"""Exercise shipped Windows PromQL with promtool, without a test cluster.

Run: python3 test_contract.py --promtool /path/to/promtool
Pass --templates FILE... to also validate downstream JSON/TypeScript rule templates.
Only Python's standard library and promtool are required.
"""

import argparse
import json
from pathlib import Path
import re
import subprocess
import tempfile
import unittest


ROOT = Path(__file__).resolve().parents[3]
PROMTOOL = "promtool"
EXTRA_TEMPLATES = []
EXTRA_DASHBOARDS = []
BOOT_NAMES = ["windows_system_boot_time_timestamp_seconds",
              "windows_system_boot_time_timestamp", "windows_system_system_up_time"]
MEMORY_NAMES = ["windows_os_visible_memory_bytes", "windows_memory_physical_total_bytes"]
DISK_NAMES = ["windows_logical_disk_read_bytes_total", "windows_logical_disk_write_bytes_total"]


def objects(value):
    if isinstance(value, dict):
        yield value
        for child in value.values():
            yield from objects(child)
    elif isinstance(value, list):
        for child in value:
            yield from objects(child)


def rule_expressions(path):
    # Rule objects use JSON string literals even in the TypeScript templates.
    return [(json.loads(record), json.loads(expression)) for record, expression in
            re.findall(r'"record"\s*:\s*("(?:\\.|[^"\\])*")\s*,\s*'
                       r'"expression"\s*:\s*("(?:\\.|[^"\\])*")', path.read_text())]


def native_names():
    source = (ROOT / "otelcollector/test/ginkgo-e2e/querymetrics/query_metrics_test.go").read_text()
    entry = source.split('Entry("AKS native Windows exporter contract"', 1)[1].split('}, Label', 1)[0]
    return re.findall(r'"(windows_[a-z_]+)"', entry)


def sample(name, instance="win25", value=1):
    return {"series": f'{name}{{instance="{instance}",job="windows-exporter",cluster="test"}}',
            "values": f"{value}+0x40"}


class WindowsContract(unittest.TestCase):
    def evaluate(self, tests):
        with tempfile.TemporaryDirectory(prefix="windows-contract-") as directory:
            suite = Path(directory) / "tests.json"
            suite.write_text(json.dumps({"rule_files": [], "evaluation_interval": "1m", "tests": tests}))
            result = subprocess.run([PROMTOOL, "test", "rules", str(suite)],
                                    capture_output=True, text=True)
            self.assertEqual(result.returncode, 0, result.stdout + result.stderr)

    def test_shipped_boot_and_memory_expressions(self):
        templates = [ROOT / name for name in [
            "AddonArmTemplate/WindowsRecordingRuleGroupTemplate/WindowsRecordingRules.json",
            "AddonArmTemplate/FullAzureMonitorMetricsProfile.json",
            "AddonPolicyTemplate/AddonPolicyMetricsProfile.rules.json",
            "GeneratedMonitoringArtifacts/Default/DefaultRecordingRules.json",
            "otelcollector/test/ci-cd/ci-cd-cluster.json",
        ]] + EXTRA_TEMPLATES
        expressions = []
        for path in templates:
            rules = rule_expressions(path)
            self.assertTrue(rules, f"No recording rules parsed from {path}")
            for record, expr in rules:
                if record == "node:windows_node:sum":
                    expressions.append((path.name, expr, 1))
                elif any(name in expr for name in MEMORY_NAMES):
                    expected = 0.75 if record == ":windows_node_memory_utilisation:" else (
                        75 if record == "ux:node_memory_usage_windows:sum" else 100)
                    expressions.append((path.name, expr, expected))
        self.assertTrue(expressions)
        tests = []
        profiles = [("legacy uptime", [BOOT_NAMES[2]], [MEMORY_NAMES[0]]),
                    ("preview boot", [BOOT_NAMES[0]], [MEMORY_NAMES[0]]),
                    ("native Windows 2025", [BOOT_NAMES[1]], [MEMORY_NAMES[1]]),
                    ("overlapping aliases", BOOT_NAMES, MEMORY_NAMES)]
        for profile, boots, memories in profiles:
            inputs = [sample(name, value=1234) for name in boots]
            inputs += [sample(name, value=100) for name in memories]
            inputs += [sample("windows_memory_available_bytes", value=25)]
            for source, expr, expected in expressions:
                tests.append({"name": f"{source}: {profile}: {expr}", "interval": "1m",
                              "input_series": inputs, "promql_expr_test": [{
                                  "expr": f"sum({expr})", "eval_time": "5m",
                                  "exp_samples": [{"labels": "{}", "value": expected}]}]})
        self.evaluate(tests)

    def test_disk_metrics_survive_minimal_ingestion(self):
        source = (ROOT / "otelcollector/shared/configmap/mp/windows_exporter_config.go").read_text()
        keep = re.search(r'windowsExporterMinimalMetricsRegex\s*=\s*"([^"]+)"', source)[1].split("|")
        for name in DISK_NAMES:
            self.assertIn(name, keep)
            self.assertIn(name, native_names())

    def test_dashboard_memory_usage(self):
        dashboards = list((ROOT / "otelcollector/deploy/dashboard/windows").rglob("*.json"))
        dashboards += list((ROOT / "mixins/kubernetes/dashboards").glob("*windows*.json"))
        dashboards += EXTRA_DASHBOARDS
        tests = []
        for path in dashboards:
            for item in objects(json.loads(path.read_text())):
                expr = item.get("expr", "")
                if "windows_os_visible_memory_bytes" not in expr or "- windows_memory_available_bytes" not in expr:
                    continue
                expr = expr.replace("$cluster", "test").replace("$instance", "win25")
                for memory in MEMORY_NAMES:
                    tests.append({"name": f"{path.name}: {memory}", "interval": "1m",
                                  "input_series": [sample(memory, value=100),
                                                   sample("windows_memory_available_bytes", value=25)],
                                  "promql_expr_test": [{"expr": expr, "eval_time": "5m",
                                                        "exp_samples": [{"labels": "{}", "value": 75}]}]})
        self.assertTrue(tests, "No memory usage panels found")
        self.evaluate(tests)

    def test_ci_uses_windows_2025_fips_gen2(self):
        template = json.loads((ROOT / "otelcollector/test/ci-cd/ci-cd-cluster.json").read_text())
        cluster = next(o for o in objects(template) if o.get("type") == "Microsoft.ContainerService/managedClusters")
        pools = cluster["properties"]["agentPoolProfiles"]
        self.assertNotIn("Windows2019", [p.get("osSKU") for p in pools])
        pool = next(p for p in pools if p.get("osSKU") == "Windows2025")
        self.assertTrue(pool["enableFIPS"])
        self.assertEqual(pool["osDiskType"], "Managed")
        self.assertEqual(template["parameters"]["win25VmSize"]["defaultValue"], "Standard_D2s_v3")

    def test_alert_detects_each_missing_family_on_each_node(self):
        template = json.loads((ROOT / "otelcollector/test/ci-cd/ci-cd-cluster.json").read_text())
        alert = next(o for o in objects(template) if "Windows exporter metric contract missing" in o.get("alert", ""))
        expression = alert["expression"]
        self.assertNotIn("__name__=~", expression, "Azure Monitor rejects metric-name regex selectors")
        names = native_names()
        self.assertEqual(len(names), 25)
        names += ["windows_pod_container_available", "windows_container_total_runtime",
                  "windows_container_memory_usage", "windows_container_private_working_set_usage",
                  "windows_container_network_received_bytes_total", "windows_container_network_transmitted_bytes_total"]
        tests = []
        for missing in [None] + names:
            inputs = [sample("up", node) for node in ["win22", "win25"]]
            inputs += [sample(name, node) for node in ["win22", "win25"]
                       for name in names if not (node == "win25" and name == missing)]
            # Extra series on the healthy node must not hide a missing family on win25.
            inputs.append({"series": 'windows_cpu_time_total{instance="win22",job="windows-exporter",core="extra"}',
                           "values": "1+0x40"})
            tests.append({"name": f"missing={missing}", "interval": "1m", "input_series": inputs,
                          "promql_expr_test": [{"expr": expression, "eval_time": "5m",
                                                "exp_samples": [] if missing is None else [
                                                    {"labels": '{instance="win25"}', "value": 1}]}]})
        self.evaluate(tests)


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--promtool", default="promtool")
    parser.add_argument("--templates", nargs="*", type=Path, default=[])
    parser.add_argument("--dashboards", nargs="*", type=Path, default=[])
    args = parser.parse_args()
    PROMTOOL = args.promtool
    EXTRA_TEMPLATES = args.templates
    EXTRA_DASHBOARDS = args.dashboards
    unittest.main(argv=[__file__], verbosity=2)
