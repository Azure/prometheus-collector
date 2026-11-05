"""Offline contract/guard checks; no Azure or Kubernetes calls."""

import copy
import importlib.util
import io
from pathlib import Path
import shutil
import sys
import tempfile
import types
import unittest
from unittest.mock import Mock, patch


SOURCE = Path(__file__).resolve().parents[1] / "run-scenario.py"
SPEC = importlib.util.spec_from_file_location("cluster_scenarios", SOURCE)
runner = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(runner)


def variant(features=(), platforms=("linux",)):
    return {"id": "test", "requires": {"clusterFeatures": list(features)},
            "platforms": list(platforms), "configmaps": [], "customResources": [],
            "addonParams": {}, "ginkgoLabels": ["config-processing-common"]}


def scenario():
    return {"id": "mixed", "cluster": "clusters/default.bicepparam",
            "clusterFeatures": ["windows", "operator"], "platforms": ["linux", "windows"],
            "settingsVariants": ["linux", "windows"], "ginkgoLabels": ["windows", "operator"],
            "tier": "pr"}


def workflow_fixture(destination):
    artifact_dir = destination / "testkube-executions/ConfigTests/configprocessingcommon/exact-id/artifacts"
    reports = artifact_dir / "nested"
    reports.mkdir(parents=True)
    (reports / "junit.xml").write_text("<testsuite><testcase/><testcase><skipped/></testcase></testsuite>")
    runner.save(reports / "ginkgo.json", [{
        "SuiteSucceeded": True, "SpecReports": [{"State": "passed", "LeafNodeType": "It"},
                                             {"State": "skipped", "LeafNodeType": "It"}],
    }])
    value = {"status": "passed", "test_source_sha": "a" * 40,
             "runner_image": "example.invalid/runner@sha256:" + "1" * 64, "executions": [{
                 "workflow": "configprocessingcommon", "execution_id": "exact-id", "status": "passed",
                 "specs": 2, "passed": 1, "skipped": 1, "failed": 0, "artifact_dir": str(artifact_dir),
             }]}
    runner.save(destination / "testkube-results-ConfigTests.json", value)
    return value, reports


def pool_parameters():
    return {"nodePools": [
        {"name": name, "osType": os, "osSKU": sku, "architecture": "amd64",
         "vmSize": "Standard_D4s_v4", "mode": "System" if os == "Linux" else "User",
         "fips": False, "count": 1, "autoscaling": False,
         "minCount": 1, "maxCount": 1, "maxSurge": "1"}
        for name, os, sku in (("system", "Linux", "Ubuntu"), ("win22", "Windows", "Windows2022"))
    ], "proxyEnabled": False}


class Contracts(unittest.TestCase):
    def test_feature_and_platform_requirements(self):
        self.assertTrue(runner.compatible(scenario(), variant(["windows"], ["windows"])))
        self.assertFalse(runner.compatible(scenario(), variant(["http-proxy"])))
        self.assertFalse(runner.compatible(
            {"clusterFeatures": ["windows"], "platforms": ["linux"]}, variant([], ["windows"])))

    def test_customer_helm_is_not_addon_parameters(self):
        manifest = dict(variant(), helmValues=["values.yaml"])
        self.assertFalse(runner.compatible(scenario(), manifest))
        host = scenario()
        host["clusterFeatures"].append("helmChartInstall")
        self.assertTrue(runner.compatible(host, manifest))

    def test_mixed_host_accepts_many_sequential_settings(self):
        row = scenario()
        document = {"schemaVersion": 1, "scenarios": [row], "coverageGaps": []}
        checked = runner.validate_matrix(document, {
            "linux": variant([], ["linux"]), "windows": variant(["windows"], ["windows"]),
        }, check_paths=False)
        self.assertEqual(len(checked["scenarios"]), 1)
        self.assertEqual(checked["scenarios"][0]["settingsVariants"], ["linux", "windows"])

    def test_forbidden_windows_network_combinations(self):
        for feature in ("http-proxy", "cilium", "kubenet"):
            row = scenario()
            row["clusterFeatures"].append(feature)
            with self.subTest(feature=feature), self.assertRaisesRegex(ValueError, "Windows"):
                runner.validate_matrix({"schemaVersion": 1, "scenarios": [row], "coverageGaps": []},
                                       {}, check_paths=False)

    def test_duplicate_scenarios_fail(self):
        row = scenario()
        row["settingsVariants"] = []
        with self.assertRaisesRegex(ValueError, "Duplicate"):
            runner.validate_matrix({"schemaVersion": 1, "scenarios": [row, row], "coverageGaps": []},
                                   {}, check_paths=False)

    def test_current_regional_default_excludes_preview_and_lts(self):
        versions = {"values": [
            {"version": "1.37", "patchVersions": {"1.37.1": {}, "1.37.2": {}}},
            {"version": "1.36", "isDefault": True, "patchVersions": {"1.36.5": {}}},
            {"version": "1.35", "patchVersions": {"1.35.9": {}}},
            {"version": "1.38", "isPreview": True, "patchVersions": {"1.38.0": {}}},
            {"version": "1.34", "capabilities": {"supportPlan": ["AKSLongTermSupport"]},
             "patchVersions": {"1.34.20": {}}},
        ]}
        self.assertEqual(runner.select_version(versions), "1.36.5")
        versions["values"] = [versions["values"][0]]
        with self.assertRaisesRegex(ValueError, "no stable official default"):
            runner.select_version(versions)

    def test_capacity_includes_autoscaler_surge_and_explicit_proxy_count(self):
        parameters = pool_parameters()
        parameters["nodePools"][0].update(autoscaling=True, maxCount=3)
        parameters.update(proxyEnabled=True, proxyVmSize="Standard_D2s_v4", proxyVmCount=1)
        plan = runner.capacity_plan(parameters, scenario(), "1.36.5")
        self.assertEqual((plan[0]["nodes"], plan[0]["surge"]), (3, 1))
        self.assertEqual((plan[-1]["name"], plan[-1]["nodes"], plan[-1]["surge"]), ("proxy", 1, 0))
        for field, invalid in (("maxSurge", "0"), ("maxCount", 0)):
            broken = copy.deepcopy(parameters)
            broken["nodePools"][0][field] = invalid
            with self.subTest(field=field), self.assertRaisesRegex(ValueError, "positive"):
                runner.capacity_plan(broken, scenario(), "1.36.5")
        with self.assertRaisesRegex(ValueError, "Windows2022 unsupported"):
            runner.capacity_plan(parameters, scenario(), "1.37.1")

    def test_capacity_rejects_restricted_architecture_and_quota_before_provisioning(self):
        plan = runner.capacity_plan(pool_parameters(), scenario(), "1.36.5")
        skus = [{"name": "Standard_D4s_v4", "family": "standardDSv4Family", "locations": ["eastus2"],
                 "capabilities": [{"name": "CpuArchitectureType", "value": "x64"},
                                  {"name": "vCPUs", "value": "4"}], "restrictions": []}]
        usage = [{"name": {"value": name}, "currentValue": 0, "limit": 64}
                 for name in ("cores", "standardDSv4Family")]
        result = runner.verify_capacity(plan, skus, usage, "eastus2")
        self.assertEqual(result["quota"][0]["additional"], 16)
        self.assertFalse(result["capacityReserved"])
        usage = [{**row, "currentValue": str(row["currentValue"]), "limit": str(row["limit"])} for row in usage]
        self.assertEqual(runner.verify_capacity(plan, skus, usage, "eastus2")["quota"], result["quota"])
        skus[0]["restrictions"] = [{"type": "Location", "values": ["eastus2"]}]
        with self.assertRaisesRegex(ValueError, "subscription restriction"):
            runner.verify_capacity(plan, skus, usage, "eastus2")
        skus[0]["restrictions"] = []
        plan[0]["architecture"] = "arm64"
        with self.assertRaisesRegex(ValueError, "architecture mismatch"):
            runner.verify_capacity(plan, skus, usage, "eastus2")
        plan[0]["architecture"] = "amd64"
        usage[1]["limit"] = 15
        with self.assertRaisesRegex(ValueError, "quota"):
            runner.verify_capacity(plan, skus, usage, "eastus2")

    def test_unsupported_watch_enforcement_never_compiles_or_provisions(self):
        commands = Mock()
        commands.run.return_value = {"values": [
            {"version": "1.36", "isDefault": True, "patchVersions": {"1.36.5": {}}},
        ]}
        row = dict(scenario(), clusterFeatures=["secrets-watch-enforcement"])
        args = types.SimpleNamespace(subscription=runner.BUILD_SUBSCRIPTION, location="eastus2")
        with patch.object(runner, "compiled_parameters", side_effect=AssertionError("Unsupported profile compiled")):
            with self.assertRaisesRegex(ValueError, "Unsupported scenario"):
                runner.preflight(args, row, commands)
        self.assertEqual(commands.run.call_count, 1)

    def test_pool_inventory_requires_every_actual_pool_node_and_declared_properties(self):
        pools = pool_parameters()["nodePools"]
        actual = [{"name": p["name"], "osType": p["osType"], "osSKU": p["osSKU"], "vmSize": p["vmSize"],
                   "mode": p["mode"], "enableFIPS": p["fips"], "enableAutoScaling": p["autoscaling"],
                   "count": p["count"], "upgradeSettings": {"maxSurge": p["maxSurge"]}} for p in pools]
        nodes = [{"metadata": {"name": p["name"] + "-node",
                              "labels": {"kubernetes.azure.com/agentpool": p["name"]}},
                  "status": {"nodeInfo": {"architecture": p["architecture"],
                                          "operatingSystem": p["osType"].lower()}}} for p in pools]
        self.assertEqual(len(runner.verify_pool_inventory(pools, actual, nodes)), 2)
        actual[0]["enableFIPS"] = True
        with self.assertRaisesRegex(RuntimeError, "properties"):
            runner.verify_pool_inventory(pools, actual, nodes)
        actual[0]["enableFIPS"] = False
        with self.assertRaisesRegex(RuntimeError, "Missing nodes"):
            runner.verify_pool_inventory(pools, actual, nodes[1:])

    def test_catalog_override_marker_is_an_explicit_boolean(self):
        with tempfile.TemporaryDirectory() as temporary:
            settings = Path(temporary) / "repo/otelcollector/test/scenarios/settings"
            row = {"id": "test", "manifest": "manifest.json", "workflow": "case.yaml",
                   "overrideWorkflowLabels": True}
            catalog = {"lifecycle": {"resetConfigMaps": list(runner.RESET), "namespace": "kube-system",
                                    "restoreOriginalResources": True, "labelCombination": "OR"},
                       "variants": [row]}
            runner.save(settings / "manifest.schema.json", {"type": "object"})
            runner.save(settings / "manifest.json", variant())
            runner.save(settings / "catalog.json", catalog)
            self.assertTrue(runner.catalog(settings)["test"]["_overrideWorkflowLabels"])
            row["overrideWorkflowLabels"] = "true"
            runner.save(settings / "catalog.json", catalog)
            with self.assertRaisesRegex(ValueError, "explicit boolean"):
                runner.catalog(settings)

    def test_receipt_requires_same_source_and_all_pinned_components(self):
        source = "a" * 40
        receipt = {"source": {"commit": source}, "images": [
            {"name": name, "os": system, "arch": "amd64",
             **({"osVersion": "10.0.20348.0"} if system == "windows" else {}),
             "ref": "mcr.microsoft.com/test@sha256:" + char * 64}
            for name, system, char in (("collector", "linux", "1"), ("collector", "windows", "2"),
                                       ("targetallocator", "linux", "3"), ("cfg", "linux", "4"))
        ]}
        images, mapping = runner.candidate_receipt(receipt, source, ["linux", "windows"])
        self.assertEqual(len(mapping), 4)
        with self.assertRaisesRegex(ValueError, "Windows build family"):
            runner.candidate_receipt(receipt, source, ["linux", "windows"], pools=[
                {"name": "win25", "osType": "Windows", "osSKU": "Windows2025", "architecture": "amd64"},
            ])
        with self.assertRaisesRegex(ValueError, "source SHA"):
            runner.candidate_receipt(receipt, "b" * 40, ["linux", "windows"])
        receipt["images"][0]["ref"] = "mcr.microsoft.com/test:latest"
        with self.assertRaisesRegex(ValueError, "sha256"):
            runner.candidate_receipt(receipt, source, ["linux", "windows"])

    def test_receipt_rejects_missing_windows_build_and_duplicate_platform(self):
        source = "a" * 40
        image = {"name": "collector", "os": "windows", "arch": "amd64",
                 "ref": "mcr.microsoft.com/test@sha256:" + "1" * 64}
        receipt = {"source": {"commit": source}, "images": [image]}
        with self.assertRaisesRegex(ValueError, "osVersion"):
            runner.candidate_receipt(receipt, source, ["linux", "windows"])
        image["osVersion"] = "10.0.20348.0"
        receipt["images"].append(dict(image))
        with self.assertRaisesRegex(ValueError, "Duplicate"):
            runner.candidate_receipt(receipt, source, ["linux", "windows"])

    def test_empty_variant_labels_never_select_all_tests(self):
        manifest = variant()
        manifest["ginkgoLabels"] = []
        with self.assertRaisesRegex(ValueError, "empty"):
            runner.label_filter(scenario(), manifest)

    def test_literal_suite_labels_are_discovered_without_weakening_unknown_label_guard(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            suites = root / "otelcollector/test/ginkgo-e2e"
            (suites / "utils").mkdir(parents=True)
            (suites / "utils/constants.go").write_text('const (\n LegacyLabel = "legacy"\n)\n')
            (suites / "suite_test.go").write_text('It("case", Label(utils.LegacyLabel, "new-lane"), body)')
            with patch.object(runner, "TEST_SOURCE", root):
                row = dict(scenario(), ginkgoLabels=["new-lane"])
                self.assertEqual(runner.label_filter(row), "!(legacy)")
                row["ginkgoLabels"] = ["nonexistent"]
                with self.assertRaisesRegex(ValueError, "Unknown"):
                    runner.label_filter(row)

    def test_execution_directory_is_locked_until_exit(self):
        with tempfile.TemporaryDirectory() as temporary:
            evidence = Path(temporary)
            with runner.execution_lock(evidence):
                with self.assertRaises(OSError):
                    with runner.execution_lock(evidence):
                        self.fail("Concurrent execution admitted")
            with runner.execution_lock(evidence):
                pass

    def test_expired_budget_never_starts_another_command(self):
        with tempfile.TemporaryDirectory() as temporary:
            commands = runner.Commands(Path(temporary))
            commands.deadline = 0
            with patch.object(runner.subprocess, "run") as process:
                with self.assertRaisesRegex(RuntimeError, "budget exhausted"):
                    commands.run(["python", "--version"])
            process.assert_not_called()

    def test_deployment_failure_retains_redacted_error_and_history_across_cleanup(self):
        with tempfile.TemporaryDirectory() as temporary:
            evidence = Path(temporary)
            commands = runner.Commands(evidence)
            env = {"SCENARIO_WINDOWS_PASSWORD": "private-password", "SYSTEM_ACCESSTOKEN": "private-token",
                   "SCENARIO_OWNER": "ama-metrics-e2e"}
            failed = types.SimpleNamespace(returncode=1, stdout="private-success-output",
                                           stderr="ERROR: InvalidTemplate private-password private-token")
            with patch.object(runner.shutil, "which", return_value="az"), \
                    patch.object(runner.subprocess, "run", return_value=failed):
                with self.assertRaises(RuntimeError):
                    commands.run(["az", "deployment", "group", "create"], env=env, private=True)
            error = (evidence / "deployment-error.txt").read_text(encoding="utf-8")
            self.assertIn("InvalidTemplate", error)
            self.assertNotIn("private-password", error)
            self.assertNotIn("private-token", error)
            self.assertNotIn("private-success-output", error)
            cleanup = runner.Commands(evidence)
            succeeded = types.SimpleNamespace(returncode=0, stdout="false", stderr="")
            with patch.object(runner.shutil, "which", return_value="az"), \
                    patch.object(runner.subprocess, "run", return_value=succeeded):
                cleanup.run(["az", "group", "exists"])
            records = runner.load(evidence / "commands.json")
            self.assertEqual([record["exitCode"] for record in records], [1, 0])
            self.assertEqual(records[0]["command"], ["az", "deployment", "group", "create"])

    def test_pin_only_candidate_workloads_and_require_all_components(self):
        document = {"kind": "Deployment", "metadata": {"name": "ama-metrics"},
                    "spec": {"template": {"spec": {"containers": [
                        {"name": "prometheus-collector", "image": "old"},
                        {"name": "addon-token-adapter", "image": "sidecar"},
                    ]}}}}
        result = runner.pin_workloads([document], {("collector", "linux"): "candidate"})
        self.assertEqual(result[0]["spec"]["template"]["spec"]["containers"][0]["image"], "candidate")
        self.assertEqual(result[0]["spec"]["template"]["spec"]["containers"][1]["image"], "sidecar")
        with self.assertRaisesRegex(ValueError, "all candidate"):
            runner.pin_workloads([], {("collector", "linux"): "candidate"})
        windows = copy.deepcopy(document)
        windows["kind"] = "DaemonSet"
        windows["metadata"]["name"] = "ama-metrics-win-node"
        self.assertEqual(len(runner.pin_workloads(
            [document, windows], {("collector", "linux"): "candidate"})), 1)

    def test_junit_rejects_empty_failed_and_skipped_only(self):
        with tempfile.TemporaryDirectory() as temporary:
            path = Path(temporary) / "junit.xml"
            for content in ("<testsuite/>", "<testsuite><testcase><skipped/></testcase></testsuite>",
                            "<testsuite><testcase><failure/></testcase></testsuite>"):
                path.write_text(content)
                with self.subTest(xml=content), self.assertRaises(RuntimeError):
                    runner.junit_summary(path)
            path.write_text("<testsuite><testcase/><testcase><skipped/></testcase></testsuite>")
            self.assertEqual(runner.junit_summary(path), {"executed": 1, "failed": 0, "skipped": 1})

    def test_fixture_paths_must_exist_and_remain_in_checkout(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            inner = root / "nested"
            inner.mkdir()
            (root / "fixture.yaml").write_text("kind: ConfigMap")
            self.assertEqual(runner.resource_path(inner, "../fixture.yaml", root), root / "fixture.yaml")
            with self.assertRaises(ValueError):
                runner.resource_path(inner, "../../outside.yaml", root)

    def test_cleanup_refuses_changed_owner(self):
        with tempfile.TemporaryDirectory() as temporary:
            args = types.SimpleNamespace(evidence=Path(temporary), subscription=runner.BUILD_SUBSCRIPTION)
            host = runner.Host(args, scenario(), Mock())
            host.receipt = {"subscription": runner.BUILD_SUBSCRIPTION, "resourceGroup": "owned",
                            "runId": "run", "scenario": "default", "token": "mine"}
            host.az = Mock(side_effect=["true", {"tags": {"scenario-token": "someone-else"}}])
            with self.assertRaisesRegex(ValueError, "ownership changed"):
                host.cleanup()
            self.assertFalse(any(call.args[:2] == ("group", "delete") for call in host.az.call_args_list))

    def test_cleanup_is_idempotent_when_already_deleted(self):
        with tempfile.TemporaryDirectory() as temporary:
            args = types.SimpleNamespace(evidence=Path(temporary), subscription=runner.BUILD_SUBSCRIPTION)
            host = runner.Host(args, scenario(), Mock())
            host.receipt = {"subscription": runner.BUILD_SUBSCRIPTION, "resourceGroup": "owned", "managedGroups": []}
            host.az = Mock(return_value="false")
            host.kubeconfig.write_text("test-only credential")
            host.cleanup()
            self.assertTrue(runner.load(Path(temporary) / "cleanup.json")["deleted"])
            self.assertFalse(host.kubeconfig.exists())
            self.assertEqual(host.az.call_count, 1)

    def test_cleanup_never_claims_external_groups_deleted_or_retains_credentials_on_failure(self):
        with tempfile.TemporaryDirectory() as temporary:
            args = types.SimpleNamespace(evidence=Path(temporary), subscription=runner.BUILD_SUBSCRIPTION)
            host = runner.Host(args, scenario(), Mock())
            host.receipt = {"subscription": runner.BUILD_SUBSCRIPTION, "resourceGroup": "owned"}
            host.kubeconfig.write_text("test-only credential")
            parent = f"/subscriptions/{runner.BUILD_SUBSCRIPTION}/resourceGroups/owned/providers/"
            host.az = Mock(side_effect=["false", [
                {"name": "owned-node-rg", "managedBy": parent + "Microsoft.ContainerService/managedClusters/aks"},
                {"name": "owned-amw-rg", "managedBy": parent + "Microsoft.Monitor/accounts/amw"},
            ], "false", "true"])
            with patch.object(runner.time, "monotonic", side_effect=[0, 601]):
                with self.assertRaisesRegex(RuntimeError, "Managed resource group deletion not confirmed"):
                    host.cleanup()
            self.assertEqual(host.receipt["managedGroups"], ["owned-node-rg", "owned-amw-rg"])
            self.assertFalse(host.kubeconfig.exists())
            self.assertFalse((args.evidence / "cleanup.json").exists())

    def test_restore_failure_stops_slot(self):
        host = Mock()
        host.scenario = scenario()
        host.args.settle_seconds = 0
        host.kube.return_value = ""
        manifest = variant()
        with patch.object(runner, "settings_workflow", return_value=("case.yaml", "case")), patch.object(
                runner, "run_suite", return_value={"executed": 1}), patch.object(
                runner, "clean_resource", return_value={"kind": "unexpected"}):
            calls = 0

            def kube(*args, **kwargs):
                nonlocal calls
                if args[0] == "get":
                    calls += 1
                    return "" if calls <= len(runner.RESET) else '{"metadata": {"name":"unexpected"}}'
                return ""

            host.kube.side_effect = kube
            with self.assertRaisesRegex(RuntimeError, "Restoration failed"), tempfile.TemporaryDirectory() as temporary:
                host.args.evidence = Path(temporary)
                runner.settings_slot(host, manifest)

    def test_existing_fixture_namespace_is_refused_before_any_mutation(self):
        with tempfile.TemporaryDirectory() as temporary:
            path = Path(temporary) / "namespace.yaml"
            path.write_text("apiVersion: v1\nkind: Namespace\nmetadata:\n  name: ama-settings-auth\n")
            host = Mock()
            host.scenario = scenario()
            host.args.evidence = Path(temporary)
            host.args.settle_seconds = 0
            host.kube.return_value = '{"kind":"Namespace","metadata":{"name":"ama-settings-auth"}}'
            manifest = dict(variant(), customResources=[path])
            with patch.object(runner, "settings_workflow", return_value=("case.yaml", "case")), patch.object(
                    runner, "run_suite", return_value={"executed": 1}):
                with self.assertRaisesRegex(ValueError, "already exists"):
                    runner.settings_slot(host, manifest)
            self.assertTrue(all(call.args[0] == "get" for call in host.kube.call_args_list))

    def test_alias_ingestion_runs_only_identity_case_before_restoring_slot(self):
        with tempfile.TemporaryDirectory() as temporary:
            host = Mock()
            host.scenario = scenario()
            host.args.evidence = Path(temporary)
            host.args.settle_seconds = 0
            events = []
            host.kube.side_effect = lambda *args, **kwargs: events.append(args[0]) or ""
            manifest = variant()
            manifest["ginkgoLabels"] = ["config-processing-customer-settings-v1"]

            def submit(host, suite, *args, **kwargs):
                events.append(suite)
                return {"executed": 1}

            with patch.object(runner, "settings_workflow", return_value=("case.yaml", "case")), patch.object(
                    runner, "label_filter", return_value="case-label"), patch.object(
                    runner, "run_suite", side_effect=submit) as workflow:
                result = runner.settings_slot(host, manifest)
            self.assertEqual(workflow.call_count, 2)
            self.assertEqual(workflow.call_args.args[1], "querymetrics")
            self.assertEqual(workflow.call_args.args[3], "cluster-identity")
            self.assertEqual(workflow.call_args.kwargs["expected_cluster"], "Customer_Config_POC")
            self.assertLess(events.index("configprocessing"), events.index("querymetrics"))
            self.assertEqual(events[events.index("querymetrics") + 1], "delete")
            self.assertEqual(result["aliasIngestion"]["executed"], 1)

    def test_no_protected_build_cannot_create_resources(self):
        args = types.SimpleNamespace(build_id=None)
        commands = Mock()
        with self.assertRaisesRegex(ValueError, "before provisioning"):
            runner.execute(args, scenario(), {}, commands)
        commands.run.assert_not_called()

    def test_missing_receipt_verifier_never_provisions_or_trusts_local_json(self):
        with tempfile.TemporaryDirectory() as temporary:
            commands = Mock()
            args = types.SimpleNamespace(build_id=123, receipt_tools_root=Path(temporary))
            with self.assertRaisesRegex(ValueError, "protected runner-receipt"), patch.object(runner, "Host") as host:
                runner.execute(args, scenario(), {}, commands)
            host.assert_not_called()
            commands.run.assert_not_called()

    def test_authenticated_fetch_binds_checkouts_and_retains_raw_runner_receipt(self):
        with tempfile.TemporaryDirectory() as temporary:
            directory = Path(temporary)
            args = types.SimpleNamespace(build_id=123, receipt_tools_root=directory, evidence=directory / "evidence")
            verifier = directory / ".pipelines/pr-e2e/fetch_candidate.py"
            verifier.parent.mkdir(parents=True)
            verifier.write_text("# Mocked protected producer entrypoint\n")
            raw = {"source": {"repo": "Azure/prometheus-collector", "commit": "a" * 40, "buildId": "123"},
                   "imageRef": "example.invalid/runner@sha256:" + "1" * 64}
            receipt = {"source": raw["source"], "testRunner": {"testSource": raw["source"],
                       "imageRef": raw["imageRef"], "receiptSha256": ""}}
            receipt["images"] = [
                {"name": name, "os": "linux", "arch": arch,
                 "ref": f"example.invalid/{name}@sha256:" + child * 64,
                 "indexRef": f"example.invalid/{name}@sha256:" + index * 64}
                for name, index in (("collector", "1"), ("targetallocator", "2"), ("config-reader", "3"))
                for arch, child in (("amd64", "4"), ("arm64", "5"))
            ]
            tamper = None

            def invoke(command, **kwargs):
                if command[0] == "git":
                    return "a" * 40 if "rev-parse" in command else ""
                self.assertEqual(command[1:6], [str(verifier), "--build-id", "123", "--source-commit", "a" * 40])
                self.assertTrue(kwargs["private"])
                output = directory / "evidence/verified-candidate"
                raw_path = output / "sre-testkube-runner/runner.json"
                runner.save(raw_path, raw)
                expected = copy.deepcopy(receipt)
                expected["testRunner"]["receiptSha256"] = runner.hashlib.sha256(raw_path.read_bytes()).hexdigest()
                if tamper == "source":
                    expected["source"] = dict(expected["source"], commit="b" * 40)
                elif tamper == "runner-bytes":
                    expected["testRunner"]["receiptSha256"] = "0" * 64
                runner.save(output / "candidate-build.json", expected)
                return ""

            commands = Mock()
            commands.run.side_effect = invoke
            verified, runtime = runner.verified_candidate(args, commands)
            self.assertEqual(runtime, raw)
            self.assertEqual(verified["source"]["commit"], "a" * 40)
            images, mapping = runner.candidate_receipt(verified, "a" * 40, ["linux"], ["arm64"])
            self.assertEqual(len(images), 6)
            self.assertEqual(mapping[("cfg", "linux")], receipt["images"][-1]["indexRef"])
            self.assertEqual(mapping[("collector", "linux")], receipt["images"][0]["indexRef"])
            persisted = runner.load(args.evidence / "verified-candidate/candidate-build.json")
            self.assertEqual(persisted["images"], receipt["images"])
            for tamper in ("source", "runner-bytes"):
                with self.subTest(tamper=tamper), self.assertRaisesRegex(ValueError, "protected runner bytes"):
                    runner.verified_candidate(args, commands)
            self.assertTrue((args.evidence / "verified-candidate/sre-testkube-runner/runner.json").is_file())

    def test_dirty_source_cannot_establish_provenance(self):
        commands = Mock()
        commands.run.side_effect = ["a" * 40, " M tracked-test.go"]
        with self.assertRaisesRegex(ValueError, "uncommitted"):
            runner.source_commit(commands, runner.REPO, ["otelcollector/test/testkube"])

    def test_fresh_ingestion_requires_this_run_and_never_logs_query_token(self):
        with tempfile.TemporaryDirectory() as temporary:
            host = Mock()
            host.args.evidence = Path(temporary)
            host.receipt = {"token": "fresh-run-marker"}
            host.commands.run.return_value = "test-token"
            host.commands.deadline = None
            host.kube.return_value = {"status": "success", "data": {"resultType": "vector", "result": [
                {"value": [105, "105"]},
            ]}}
            with patch.object(runner.time, "time", return_value=110):
                result = runner.verify_ingestion(host, {"queryEndpoint": "https://amw.example"}, 100)
            self.assertEqual(result["sampleTimestamp"], 105)
            self.assertIn('scenario_run="fresh-run-marker"', result["query"])
            self.assertNotIn("test-token", (Path(temporary) / "ingestion.json").read_text())
            call = host.kube.call_args
            self.assertNotIn("test-token", call.args)
            self.assertIn("Bearer $token", call.args[call.args.index("sh") + 2])
            self.assertEqual(call.kwargs["data"], "test-token\n")
            self.assertTrue(call.kwargs["private"])
            for response in (
                {"status": "error", "data": {"result": []}},
                {"status": "success", "data": {"resultType": "vector", "result": [{"value": [99, "99"]}]}},
                {"status": "success", "data": {"resultType": "vector", "result": [{"value": [105, "NaN"]}]}},
            ):
                host.kube.return_value = response
                with patch.object(runner.time, "sleep"), patch.object(
                        runner.time, "time", return_value=110), self.assertRaisesRegex(RuntimeError, "not verified"):
                    runner.verify_ingestion(host, {"queryEndpoint": "https://amw.example"}, 100)
    def test_testkube_bootstrap_uses_cli_oidc_shape_and_read_only_scoped_roles(self):
        with tempfile.TemporaryDirectory() as temporary:
            host = Mock()
            host.args.evidence = Path(temporary)
            host.kubeconfig = Path(temporary) / "kubeconfig"
            host.receipt = {"resourceGroup": "owned"}
            identity = {"name": "ama-e2e-tests", "principalId": "identity-object", "clientId": "identity-client"}
            cluster = {"oidcIssuerProfile": {"issuerUrl": "https://issuer.example/"},
                       "kubernetesVersion": "1.36.5", "autoUpgradeProfile": {"upgradeChannel": "stable"},
                       "securityProfile": {"workloadIdentity": {"enabled": True}}}

            def az(*args, **kwargs):
                if args[:2] == ("identity", "create"):
                    return identity
                if args[:2] == ("aks", "show"):
                    return cluster
                return {}

            def kube(*args, **kwargs):
                if args[:2] == ("get", "deployments"):
                    return {"items": [{"metadata": {"name": "testkube-api-server"}}]}
                return "testworkflows <none> testworkflows.testkube.io/v1 true TestWorkflow"

            host.az.side_effect = az
            host.kube.side_effect = kube
            outputs = {"clusterName": "owned-cluster", "workspaceId": "/owned/workspace",
                       "queryEndpoint": "https://amw.example"}
            runner.bootstrap_testkube(host, outputs, Path("pinned-chart.tgz"), "caller-object", "User")
            role_calls = [c.args for c in host.az.call_args_list if c.args[:3] == ("role", "assignment", "create")]
            self.assertEqual(len(role_calls), 2)
            for args in role_calls:
                self.assertEqual(args[args.index("--scope") + 1], "/owned/workspace")
                self.assertEqual(args[args.index("--role") + 1], "b0d8363b-8ddd-447d-831f-62ca05bff136")
            federation = next(c.args for c in host.az.call_args_list if c.args[:2] == ("identity", "federated-credential"))
            self.assertEqual(federation[federation.index("--issuer") + 1], "https://issuer.example/")
            proof = runner.load(Path(temporary) / "testkube-bootstrap.json")
            self.assertIn("not submitted", proof["workloadIdentityExecution"])
            cluster["autoUpgradeProfile"]["upgradeChannel"] = "none"
            with self.assertRaisesRegex(RuntimeError, "auto-upgrade"):
                runner.bootstrap_testkube(host, outputs, Path("pinned-chart.tgz"), "caller-object", "User")

    def test_kubeconfig_is_explicit(self):
        with tempfile.TemporaryDirectory() as temporary:
            commands = Mock()
            host = runner.Host(types.SimpleNamespace(evidence=Path(temporary)), {}, commands)
            host.kube("get", "nodes")
            command = commands.run.call_args.args[0]
            self.assertEqual(command[1:3], ["--kubeconfig", str(Path(temporary) / "kubeconfig")])

    def test_workflow_requires_exact_id_pinned_provenance_and_nonempty_reports(self):
        with tempfile.TemporaryDirectory() as temporary:
            destination = Path(temporary)
            value, reports = workflow_fixture(destination)
            result = runner.workflow_result(destination, "configprocessingcommon", "a" * 40, value["runner_image"])
            self.assertEqual(result["executionId"], "exact-id")
            self.assertEqual(result["executed"], 1)
            for key, wrong in (("status", "running"), ("test_source_sha", "b" * 40),
                               ("runner_image", "example.invalid/runner:latest")):
                invalid = dict(value, **{key: wrong})
                runner.save(destination / "testkube-results-ConfigTests.json", invalid)
                with self.subTest(field=key), self.assertRaises(RuntimeError):
                    runner.workflow_result(destination, "configprocessingcommon", "a" * 40, value["runner_image"])
            runner.save(destination / "testkube-results-ConfigTests.json", value)
            (reports / "junit.xml").write_text("<testsuite><testcase><skipped/></testcase></testsuite>")
            with self.assertRaisesRegex(RuntimeError, "executed"):
                runner.workflow_result(destination, "configprocessingcommon", "a" * 40, value["runner_image"])

    def test_workflow_rejects_other_executions_and_artifact_directories(self):
        with tempfile.TemporaryDirectory() as temporary:
            destination = Path(temporary)
            value, reports = workflow_fixture(destination)
            for field, wrong in (("execution_id", ""), ("workflow", "other-case"), ("status", "aborted"),
                                 ("specs", 0), ("passed", 0), ("failed", 1),
                                 ("artifact_dir", str(destination.parent))):
                invalid = copy.deepcopy(value)
                invalid["executions"][0][field] = wrong
                runner.save(destination / "testkube-results-ConfigTests.json", invalid)
                with self.subTest(field=field), self.assertRaises(RuntimeError):
                    runner.workflow_result(destination, "configprocessingcommon", "a" * 40, value["runner_image"])
            value["executions"].append(copy.deepcopy(value["executions"][0]))
            runner.save(destination / "testkube-results-ConfigTests.json", value)
            with self.assertRaisesRegex(RuntimeError, "exactly"):
                runner.workflow_result(destination, "configprocessingcommon", "a" * 40, value["runner_image"])

    def test_workflow_requires_one_successful_ginkgo_report(self):
        with tempfile.TemporaryDirectory() as temporary:
            destination = Path(temporary)
            value, reports = workflow_fixture(destination)
            for document in ([], [{"SuiteSucceeded": False, "SpecReports": [{"State": "passed"}]}],
                             [{"SuiteSucceeded": True, "SpecReports": [{"State": "skipped"}]}],
                             [{"SuiteSucceeded": True, "SpecReports": [
                                 {"State": "passed", "LeafNodeType": "BeforeSuite"}]}]):
                runner.save(reports / "ginkgo.json", document)
                with self.assertRaisesRegex(RuntimeError, "Ginkgo"):
                    runner.workflow_result(destination, "configprocessingcommon", "a" * 40, value["runner_image"])

    def test_existing_adapter_submits_only_selected_case_without_second_lifecycle(self):
        with tempfile.TemporaryDirectory() as temporary:
            destination = Path(temporary) / "slot"
            host = Mock()
            host.args.kubectl = str(Path(temporary) / "bin/kubectl.exe")
            host.args.tools_bin = None
            host.kubeconfig = Path(temporary) / "isolated-kubeconfig"
            host.query_endpoint = "https://amw.example"
            host.query_client_id = "query-client"
            host.cluster_name = "owned-cluster"
            host.scenario = scenario()
            host.runner_receipt = {"source": {"commit": "a" * 40},
                                   "imageRef": "example.invalid/runner@sha256:" + "1" * 64}
            host.commands.run.side_effect = lambda *args, **kwargs: workflow_fixture(destination)
            result = runner.run_suite(host, "configprocessing", destination, "case-label",
                                      template="case.yaml", workflow="configprocessingcommon")
            command = host.commands.run.call_args.args[0]
            env = host.commands.run.call_args.kwargs["env"]
            self.assertEqual(command[0:2], ["bash", "./testkube/run-testkube-workflow.sh"])
            self.assertEqual(command[-4:], ["false", "0", "ConfigTests", "a" * 40])
            self.assertEqual(env["SELECTED_WORKFLOW"], "configprocessingcommon")
            self.assertEqual(env["GINKGO_LABEL_FILTER"], "case-label")
            self.assertEqual(env["CANDIDATE_RUNNER_IMAGE"], host.runner_receipt["imageRef"])
            self.assertEqual(env["KUBECONFIG"], str(host.kubeconfig))
            self.assertEqual(env["EXPECTED_CLUSTER"], "owned-cluster")
            self.assertEqual(env["EXPECT_OPERATOR_HTTPS"], "true")
            self.assertEqual(env["EXPECT_WINDOWS_EXPORTER"], "true")
            self.assertEqual(result["executionId"], "exact-id")


class ChartRendering(unittest.TestCase):
    @unittest.skipUnless(shutil.which("ssh-keygen"), "ssh-keygen is required for owned proxy public keys")
    def test_proxy_key_is_ephemeral_and_only_public_key_enters_deployment_env(self):
        args = types.SimpleNamespace(run_id="key-check", location="eastus2", max_run_seconds=100)
        env = runner.deployment_env(args, "1.36.5", proxy=True)
        self.assertTrue(env["SCENARIO_PROXY_SSH_PUBLIC_KEY"].startswith("ssh-rsa "))
        self.assertTrue(env["SCENARIO_PROXY_SSH_PUBLIC_KEY"].endswith("ama-metrics-e2e"))
        self.assertNotIn("PRIVATE KEY", env["SCENARIO_PROXY_SSH_PUBLIC_KEY"])

    @unittest.skipUnless(shutil.which("helm"), "Helm is required for the local chart-render integration check")
    def test_existing_chart_pins_all_candidate_containers_on_both_platforms(self):
        for windows in (False, True):
            with self.subTest(windows=windows), tempfile.TemporaryDirectory() as temporary:
                evidence = Path(temporary)
                commands = runner.Commands(evidence)
                mapping = {(name, "linux"): "example.invalid/test@sha256:" + digit * 64
                           for name, digit in (("collector", "1"), ("targetallocator", "2"), ("cfg", "3"))}
                if windows:
                    mapping[("collector", "windows")] = "example.invalid/test@sha256:" + "4" * 64
                chart = runner.render_chart(commands, evidence, {
                    "location": "eastus2", "kubernetesVersion": "1.36.5",
                    "clusterId": "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/test/providers/Microsoft.ContainerService/managedClusters/test",
                }, "a" * 40, mapping)
                raw = commands.run(["helm", "template", "ama-metrics", str(chart)], private=True)
                documents = list(runner.yaml.safe_load_all(raw))
                workloads = [doc for doc in documents if doc and doc.get("kind") in {"Deployment", "DaemonSet"}]
                found = {c["image"] for doc in workloads for c in doc["spec"]["template"]["spec"]["containers"]
                         if c["name"] in runner.COMPONENTS}
                self.assertEqual(found, set(mapping.values()))
                self.assertEqual(any(doc["metadata"]["name"] == "ama-metrics-win-node" for doc in workloads), windows)
                versions = [e["value"] for doc in workloads
                            for c in doc["spec"]["template"]["spec"]["containers"]
                            for e in c.get("env", []) if e["name"] == "KUBE_VERSION"]
                self.assertTrue(versions)
                self.assertEqual(set(versions), {"1.36.5"})


def mutations():
    global runner
    original = runner
    source = SOURCE.read_text()
    changes = {
        "cleanup-command-history": ("self.stages = load(history) if history.exists() else []", "self.stages = []"),
        "deployment-error-redaction": ('diagnostic = diagnostic.replace(value, "[REDACTED]")', "pass"),
        "feature-requirements": ('set(variant["requires"]["clusterFeatures"]) <= set(scenario["clusterFeatures"])', "True"),
        "source-binding": ('document.get("source", {}).get("commit") != source_sha', "False"),
        "windows-build": ('image.get("os") == "windows" and not re.fullmatch(r"\\d+\\.\\d+\\.\\d+\\.\\d+", image.get("osVersion", ""))', "False"),
        "duplicate-platform": ("if identity in identities:", "if False:"),
        "explicit-labels": ("if not labels:", "if False:"),
        "time-budget": ("if timeout <= 0:", "if False:"),
        "auto-upgrade": ('if (cluster.get("autoUpgradeProfile") or {}).get("upgradeChannel") != "stable":', "if False:"),
        "workflow-status": ('result.get("status") != "passed"', "False"),
        "workflow-provenance": ('result.get("test_source_sha") != source_sha', "False"),
        "workflow-selection": ('row.get("workflow") != workflow', "False"),
        "workflow-count": ("if len(executions) != 1:", "if False:"),
        "verified-output-source": ('candidate["source"]["commit"] != source', "False"),
        "verified-runner-bytes": ('hashlib.sha256(runner_path.read_bytes()).hexdigest() != expected["receiptSha256"]', "False"),
        "producer-component": ('image["name"] = "cfg"', "pass"),
        "producer-index": ('image["manifestRef"] = image["indexRef"]', "pass"),
        "fixture-namespace": ('if kind == "Namespace" and output.strip():', "if False:"),
        "override-authorization": ("if type(override) is not bool:", "if False:"),
        "sku-quota": ("current + cores > limit", "False"),
        "quota-numeric-strings": ('int(str(row["currentValue"]))', 'row["currentValue"]'),
        "sku-architecture": ('entry["architecture"] != architecture', "False"),
        "sku-restrictions": ('restriction["type"] == "Location"', "False"),
        "explicit-surge": ('or not re.fullmatch(r"[1-9][0-9]*", surge)', "or False"),
        "pool-properties": ('any(row.get(key) != value for key, value in fields.items())', "False"),
        "pool-node-presence": ('not minimum <= len(members) <= maximum', "False"),
        "alias-only": ('if set(variant["ginkgoLabels"]) & {', "if set() & {"),
        "windows-pool-build": ('if pool["osType"] == "Windows" and not any(', "if False and not any("),
        "credential-cleanup": ('self.kubeconfig.unlink(missing_ok=True)', "pass"),
        "external-cleanup": ('while self.az("group", "exists", "--name", managed).strip() != "false":', "while False:"),
        "literal-labels": ('known.update(re.findall(r\'"([^"]+)"\', arguments))', "pass"),
        "query-auth-template": ("Bearer $token", "Bearer"),
        "fresh-sample": ("if not math.isfinite(observed) or not started <= observed <= time.time() + 60:", "if False:"),
        "empty-junit": ("if not executed or failed:", "if failed:"),
        "cleanup-ownership": ('if any(group.get("tags", {}).get(k) != v for k, v in expected.items()):', "if False:"),
    }
    for name, (before, after) in changes.items():
        if source.count(before) != 1:
            raise AssertionError(f"Mutation target changed: {name}")
        mutant = types.ModuleType("mutant")
        mutant.__file__ = str(SOURCE)
        exec(compile(source.replace(before, after), str(SOURCE), "exec"), mutant.__dict__)
        runner = mutant
        result = unittest.TextTestRunner(stream=io.StringIO()).run(
            unittest.defaultTestLoader.loadTestsFromTestCase(Contracts))
        runner = original
        if result.wasSuccessful():
            raise AssertionError(f"Mutation survived: {name}")
        print(f"KILLED: {name}")


if __name__ == "__main__":
    check_mutations = "--mutations" in sys.argv
    if check_mutations:
        sys.argv.remove("--mutations")
    result = unittest.main(exit=False).result
    if not result.wasSuccessful():
        raise SystemExit(1)
    if check_mutations:
        mutations()
