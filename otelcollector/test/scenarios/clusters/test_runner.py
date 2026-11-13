"""Offline contract/guard checks; no Azure or Kubernetes calls."""

import copy
import importlib.util
import io
from pathlib import Path
import re
import shutil
import sys
import tempfile
import types
import unittest
import uuid
from unittest.mock import Mock, patch


SOURCE = Path(__file__).resolve().parents[1] / "run-scenario.py"
sys.path.insert(0, str(SOURCE.parent))
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


def workflow_fixture(destination, test_sha="a" * 40):
    artifact_dir = destination / "testkube-executions/ConfigTests/configprocessingcommon/exact-id/artifacts"
    reports = artifact_dir / "nested"
    reports.mkdir(parents=True)
    (reports / "junit.xml").write_text("<testsuite><testcase/><testcase><skipped/></testcase></testsuite>")
    runner.save(reports / "ginkgo.json", [{
        "SuiteSucceeded": True, "SpecReports": [{"State": "passed", "LeafNodeType": "It"},
                                             {"State": "skipped", "LeafNodeType": "It"}],
    }])
    value = {"status": "passed", "test_source_sha": test_sha,
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
    def test_vm_script_reads_actual_azure_envelope_but_not_stderr_or_failed_records(self):
        adapter = sys.modules["clusters.private_api"]
        result = 'SCENARIO_RESULT={"keyRemoved":true}'
        for code, message, passed in (
                ("ComponentStatus/StdOut/succeeded", result, True),
                ("ProvisioningState/succeeded", "Enable succeeded: \n[stdout]\n" + result + "\n\n[stderr]\n", True),
                ("ProvisioningState/succeeded", "Enable succeeded: \n[stdout]\n\n[stderr]\n" + result, False),
                ("ProvisioningState/failed", "[stdout]\n" + result + "\n[stderr]\n", False),
                ("ProvisioningState/succeeded", "[stdout]\n" + result + "\n" + result + "\n[stderr]\n", False)):
            with self.subTest(code=code, message=message):
                host = Mock()
                host.receipt = {"resourceGroup": "owned"}
                host.az.return_value = {"value": [{"code": code, "message": message}]}
                if passed:
                    self.assertEqual(adapter.vm_script(host, "runner", "script"), {"keyRemoved": True})
                else:
                    with self.assertRaisesRegex(RuntimeError, "one successful scenario result"):
                        adapter.vm_script(host, "runner", "script")

    def test_private_deletion_protection_requires_actual_lock_readback(self):
        for existing, level in ((True, "CanNotDelete"), (False, "CanNotDelete"), (True, "ReadOnly")):
            with self.subTest(existing=existing, level=level), tempfile.TemporaryDirectory() as directory:
                host = runner.Host(types.SimpleNamespace(evidence=Path(directory)), scenario(), Mock())
                host.receipt = {"resourceGroup": "owned"}
                host.assert_owned = Mock()
                lock = {"id": "owned/locks/scenario-retain", "name": "scenario-retain", "level": "CanNotDelete"}
                host.az = Mock(side_effect=[
                    [lock] if existing else [],
                    *([] if existing else [{}]),
                    dict(lock, level=level),
                ])
                if level == "CanNotDelete":
                    host.protect_private_host()
                    self.assertTrue((Path(directory) / "deletion-protection.json").is_file())
                else:
                    with self.assertRaisesRegex(RuntimeError, "deletion protection"):
                        host.protect_private_host()

    def test_ca_failure_requires_fresh_scraping_cert_errors_and_absent_ingestion(self):
        proof = sys.modules["clusters.proxy_ca"].failure_proof
        scrapes = [{"timestamp": 110, "health": "up"}]
        observations = [{"time": value, "result": []} for value in (120, 155, 190)]
        self.assertEqual(proof(scrapes, 1, observations, 100)["status"], "PASS")
        for values in (([], 1, observations), (scrapes, 0, observations),
                       (scrapes, 1, observations[:1]),
                       (scrapes, 1, [{"time": value, "result": []} for value in (120, 130, 140)]),
                       ([{"timestamp": 90, "health": "up"}], 1, observations),
                       (scrapes, 1, [dict(row, result=[{"value": 1}]) for row in observations])):
            with self.subTest(values=values), self.assertRaises(RuntimeError):
                proof(*values, 100)

    def test_private_api_uses_single_ip_pinned_host_and_cleans_up_on_failure(self):
        adapter = sys.modules["clusters.private_api"]
        self.assertEqual(adapter.agent_prefix("8.8.8.8\n"), "8.8.8.8/32")
        for value in ("0.0.0.0/0", "8.8.8.8/24", "10.0.0.1", "::1", "invalid"):
            with self.subTest(value=value), self.assertRaises(ValueError):
                adapter.agent_prefix(value)
        for fail in (False, True):
            with self.subTest(fail=fail), tempfile.TemporaryDirectory() as directory:
                root = Path(directory)
                fqdn = "sc.privatelink.westcentralus.azmk8s.io"
                kubeconfig = root / "kubeconfig"
                kubeconfig.write_text(runner.yaml.safe_dump(
                    {"clusters": [{"cluster": {"server": "https://" + fqdn}}]}))
                host = Mock()
                host.scenario = {"clusterFeatures": ["private-api"]}
                host.receipt = {"resourceGroup": "owned-rg"}
                host.args.evidence = root
                host.kubeconfig = kubeconfig
                host.commands.deadline = 123
                host.az.side_effect = [
                    {"privateFqdn": fqdn, "apiServerAccessProfile": {
                        "enablePrivateCluster": True, "enablePrivateClusterPublicFqdn": False}},
                    {}, {}, {"access": "Deny"},
                ]
                host.kube.return_value = "ok"
                def keygen(args, **kwargs):
                    Path(args[-1] + ".pub").write_text("ssh-ed25519 AAAA test")
                host.commands.run.side_effect = keygen
                process = Mock()
                process.poll.return_value = None
                outputs = {"clusterName": "sc", "orchestratorVmName": "runner",
                           "orchestratorNsgName": "runner-nsg", "orchestratorPublicIp": "9.9.9.9"}
                response = Mock()
                response.__enter__ = Mock(return_value=io.BytesIO(b"8.8.8.8"))
                response.__exit__ = Mock(return_value=False)
                with patch.object(adapter, "urlopen", return_value=response), \
                        patch.object(adapter.shutil, "which", side_effect=lambda name: name), \
                        patch.object(adapter, "vm_script", side_effect=[
                            {"privateIp": "10.224.0.9", "hostKey": "ssh-ed25519 AAAA root"},
                            {"keyRemoved": True},
                        ]) as remote, patch.object(adapter.subprocess, "Popen", return_value=process) as ssh:
                    try:
                        with adapter.private_api(host, outputs):
                            forwarded = runner.yaml.safe_load(kubeconfig.read_text())["clusters"][0]["cluster"]
                            self.assertEqual(forwarded["tls-server-name"], fqdn)
                            self.assertTrue(forwarded["server"].startswith("https://127.0.0.1:"))
                            if fail:
                                raise RuntimeError("suite failed")
                    except RuntimeError as error:
                        if not fail or str(error) != "suite failed":
                            raise
                    arguments = ssh.call_args.args[0]
                    self.assertIn("StrictHostKeyChecking=yes", arguments)
                    self.assertIn("BatchMode=yes", arguments)
                    create = host.az.call_args_list[1].args
                    self.assertEqual(create[create.index("--source-address-prefixes") + 1], "8.8.8.8/32")
                    cleanup = host.az.call_args_list[2].args
                    self.assertEqual(cleanup[cleanup.index("--access") + 1], "Deny")
                    self.assertIn("rm -f /home/scenario-tunnel/.ssh/authorized_keys", remote.call_args.args[2])
                    process.terminate.assert_called_once()
                    self.assertEqual(host.commands.deadline, 123)

    def test_private_api_rejects_public_profile_before_opening_a_tunnel(self):
        adapter = sys.modules["clusters.private_api"]
        for profile in ({"enablePrivateCluster": False, "enablePrivateClusterPublicFqdn": False},
                        {"enablePrivateCluster": True, "enablePrivateClusterPublicFqdn": True}):
            with self.subTest(profile=profile), tempfile.TemporaryDirectory() as directory:
                host = Mock()
                host.scenario = {"clusterFeatures": ["private-api"]}
                host.receipt = {"resourceGroup": "owned"}
                host.kubeconfig = Path(directory) / "kubeconfig"
                fqdn = "sc.privatelink.westcentralus.azmk8s.io"
                host.kubeconfig.write_text(runner.yaml.safe_dump(
                    {"clusters": [{"cluster": {"server": "https://" + fqdn}}]}))
                host.az.return_value = {"privateFqdn": fqdn, "apiServerAccessProfile": profile}
                with patch.object(adapter, "urlopen") as discover, self.assertRaisesRegex(RuntimeError, "private-only"):
                    with adapter.private_api(host, {"clusterName": "sc"}):
                        self.fail("Public API profile entered the tunnel")
                discover.assert_not_called()

    def test_proxy_adapter_injects_collector_environment_and_public_trust_only(self):
        documents = [
            {"kind": kind, "spec": {"template": {"spec": {"containers": [
                {"name": "prometheus-collector", "env": [{"name": "HTTPS_PROXY", "value": "wrong"}]},
                {"name": "config-reader", "env": []},
            ]}}}} for kind in ("Deployment", "DaemonSet")
        ]
        outputs = {"proxyEndpoint": "http://10.226.0.4:3128",
                   "proxyNoProxyCsv": "localhost,10.244.0.0/16,.svc",
                   "proxyTrustedCa": runner.base64.b64encode(
                       b"-----BEGIN CERTIFICATE-----\npublic\n-----END CERTIFICATE-----\n").decode()}
        runner.configure_proxy(documents, outputs)
        for document in documents[:2]:
            spec = document["spec"]["template"]["spec"]
            env = spec["containers"][0]["env"]
            self.assertEqual([entry["value"] for entry in env if entry["name"] == "HTTPS_PROXY"],
                             [outputs["proxyEndpoint"]])
            self.assertIn({"name": "NO_PROXY", "value": outputs["proxyNoProxyCsv"]}, env)
            self.assertEqual(spec["containers"][0]["volumeMounts"][0]["mountPath"], "/anchors/proxy")
            self.assertEqual(spec["containers"][1]["env"], [])
        self.assertIn("public", documents[-1]["stringData"]["proxy-cert.crt"])
        for change in ({"proxyEndpoint": "http://external:3128"}, {"proxyNoProxyCsv": "*"},
                       {"proxyTrustedCa": "not-base64"}):
            with self.subTest(change=change), self.assertRaises(ValueError):
                runner.configure_proxy([], dict(outputs, **change))
        with self.assertRaisesRegex(ValueError, "both replica and node"):
            runner.configure_proxy([], dict(outputs, proxyTrustedCa=""))

    def test_proxy_routes_reject_missing_stale_bypass_or_unrelated_traffic(self):
        route = "101.000 10.244.0.2 CONNECT ingest.monitor.azure.com 200 HIER_DIRECT/1.2.3.4"
        def verify(log):
            return runner.proxy_routes(log, 100, {"10.244.0.2"}, "ingest.monitor.azure.com",
                                       {"10.244.0.3"})
        self.assertEqual(len(verify(route)), 1)
        for log in ("", route.replace("101.000", "99.000"),
                    route.replace("10.244.0.2", "10.244.0.9"),
                    route.replace(" 200 ", " 403 "),
                    route.replace("ingest.monitor.azure.com", "query.monitor.azure.com"),
                    route + "\n101.100 10.244.0.2 GET 10.244.0.3 200 HIER_DIRECT/10.244.0.3"):
            with self.subTest(log=log), self.assertRaises(RuntimeError):
                verify(log)

    def test_runner_must_come_from_expected_test_checkout(self):
        with tempfile.TemporaryDirectory() as directory:
            repo, test_source = runner.REPO, runner.TEST_SOURCE
            with patch.object(runner, "REPO", repo), patch.object(runner, "TEST_SOURCE", test_source), \
                    self.assertRaisesRegex(ValueError, "runner.*test source"):
                runner.main(["campaign-preflight", "--source-root", directory, "--test-source-root", directory,
                             "--test-source-sha", "a" * 40, "--build-id", "123", "--evidence", directory])
            for flag in ("--matrix", "--settings-root"):
                with self.subTest(flag=flag), patch.object(runner, "REPO", repo), patch.object(runner, "TEST_SOURCE", test_source), \
                        self.assertRaisesRegex(ValueError, "matrix and settings"):
                    runner.main(["campaign-preflight", "--source-root", str(repo),
                                 "--test-source-root", str(test_source), "--test-source-sha", "a" * 40,
                                 "--build-id", "123", "--evidence", directory, flag, directory])

    def test_persistent_reuse_requires_owned_contract_and_locked_fresh_execution(self):
        pools = pool_parameters()["nodePools"]
        name, owner = runner.persistent_identity("mixed")
        cluster_id = "/subscriptions/sub/resourceGroups/rg/providers/Microsoft.ContainerService/managedClusters/sc"
        outputs = {
            "clusterId": cluster_id, "clusterName": "sc", "location": "eastus2",
            "nodeResourceGroupName": "node-rg", "amwManagedResourceGroupName": "amw-rg",
            "expectedPools": pools, "addonEnabled": False, "installationMode": "candidate-addon",
            "identityType": "SystemAssigned", "azureCniMode": "overlay",
            "resourceTags": {"owner": "ama-metrics-e2e", "scenario": "mixed",
                             "run": owner},
        }
        pipeline = runner.yaml.safe_load((runner.REPO / ".pipelines/azure-pipeline-config-tests.yml").read_text())
        loop = next(row for row in pipeline["stages"] if any(
            key.startswith("${{ each scenario in split(") for key in row))
        stage = next(iter(next(iter(loop.values()))[0].values()))[0]
        self.assertEqual(stage["lockBehavior"], "sequential")
        for case in ("valid", "new", "appeared", "active-deployment", "run", "expiry", "deployment-state",
                     "deployment-output", "cluster", "cluster-pools", "missing-admitted"):
            with self.subTest(case=case), tempfile.TemporaryDirectory() as directory:
                tags = {
                    "scenario-token": owner, "scenario-run": owner, "scenario": "mixed",
                    "owner": "ama-metrics-e2e",
                }
                if case == "run":
                    tags["scenario-run"] = "other"
                if case == "expiry":
                    tags["expiresAt"] = "2099-01-01T00:00:00Z"
                deployment_outputs = copy.deepcopy(outputs)
                if case == "deployment-output":
                    deployment_outputs["expectedPools"] = []
                deployment = {"properties": {
                    "provisioningState": "Failed" if case == "deployment-state" else "Succeeded",
                    "outputs": {key: {"value": value} for key, value in deployment_outputs.items()},
                }}
                cluster = {
                    "id": cluster_id, "location": "westcentralus" if case == "cluster" else "eastus2",
                    "provisioningState": "Succeeded", "powerState": {"code": "Running"},
                    "kubernetesVersion": "1.36.5", "nodeResourceGroup": "node-rg",
                    "identity": {"type": "SystemAssigned"},
                    "networkProfile": {"networkPlugin": "azure", "networkPluginMode": "overlay"},
                    "agentPoolProfiles": [{
                        "name": pool["name"], "osType": pool["osType"], "osSKU": pool["osSKU"],
                        "vmSize": pool["vmSize"], "mode": pool["mode"], "enableFIPS": pool["fips"],
                        "enableAutoScaling": pool["autoscaling"], "count": pool["count"],
                        "upgradeSettings": {"maxSurge": pool["maxSurge"]},
                    } for pool in pools],
                }
                if case == "cluster-pools":
                    cluster["agentPoolProfiles"] = []
                host = runner.Host(types.SimpleNamespace(
                    evidence=Path(directory), subscription="sub", run_id="run", location="eastus2",
                    admitted_existing=case == "missing-admitted",
                ), scenario(), Mock())
                group = {"location": "eastus2", "tags": tags}
                if case == "new":
                    calls = ["false", "false", {}, deployment]
                elif case == "appeared":
                    calls = ["false", "true", group, [{"id": cluster_id}], [], deployment]
                elif case == "active-deployment":
                    calls = ["false", "true", group, [], [{"properties": {"provisioningState": "Running"}}],
                             deployment]
                else:
                    calls = ["false" if case == "missing-admitted" else "true",
                             group, [{"id": cluster_id}], deployment, cluster]
                host.az = Mock(side_effect=calls)
                env = {"SCENARIO_KUBERNETES_VERSION": "1.36.5"}
                if case in ("valid", "new"):
                    result = host.provision(env, "principal", "User", pools)
                    self.assertEqual(result["clusterName"], "sc")
                    self.assertEqual(host.reused, case == "valid")
                    self.assertEqual(host.receipt["resourceGroup"], name)
                    self.assertEqual(host.receipt["runId"], owner)
                    self.assertEqual((Path(directory) / "persistent-reuse.json").is_file(), case == "valid")
                else:
                    with self.assertRaises((ValueError, RuntimeError)):
                        host.provision(env, "principal", "User", pools)
                creates = [call for call in host.az.call_args_list if call.args[:2] == ("group", "create")]
                self.assertEqual(len(creates), int(case == "new"))

    def test_candidate_harness_region_guard_matches_campaign_locations(self):
        template = (runner.REPO / ".pipelines/azure-template-candidate-config-tests.yml").read_text(
            encoding="utf-8")
        guard = next(line for line in template.splitlines()
                     if "Regional capacity admission has no approved placement" in line)
        allowed = {clause.rsplit("==", 1)[1].strip()
                   for clause in guard.split("]]", 1)[0].split("||") if "==" in clause}
        self.assertEqual(allowed, {"centralus", "eastus2", "westcentralus"})
        self.assertLessEqual(set(runner.CAMPAIGN_LOCATIONS.values()), allowed)

    def test_campaign_admission_sums_retained_maximum_and_surge_by_region(self):
        matrix = runner.load(SOURCE.parent / "matrix.json")
        skus = [
            {"name": name, "family": family, "locations": ["centralus", "eastus2", "westcentralus"],
             "capabilities": [{"name": "CpuArchitectureType", "value": architecture},
                              {"name": "vCPUs", "value": "4"}], "restrictions": []}
            for name, family, architecture in (
                ("Standard_D4s_v4", "standardDSv4Family", "x64"),
                ("Standard_D4ps_v5", "standardDPSv5Family", "arm64"))
        ]
        usage = [{"name": {"value": name}, "currentValue": 0, "limit": limit}
                 for name, limit in (("cores", 300), ("standardDSv4Family", 100),
                                     ("standardDPSv5Family", 100))]

        def compiled(args, row, commands, version):
            parameters = pool_parameters()
            if row["platforms"] == ["linux"]:
                parameters["nodePools"] = parameters["nodePools"][:1]
            if row["id"] == "default":
                parameters["nodePools"].extend([
                    dict(parameters["nodePools"][0], name=str(index), fips=True)
                    for index in range(5)])
                parameters["nodePools"].extend([
                    dict(parameters["nodePools"][0], name="arm" + str(index),
                         architecture="arm64", vmSize="Standard_D4ps_v5")
                    for index in range(3)])
            return parameters

        def command(args, **kwargs):
            if args[:3] == ["az", "group", "exists"]:
                return "false"
            if args[:3] == ["az", "aks", "get-versions"]:
                return {"values": [{"isDefault": True, "patchVersions": {"1.36.5": {}}}]}
            if args[:3] == ["az", "vm", "list-skus"]:
                return copy.deepcopy(skus)
            if args[:3] == ["az", "vm", "list-usage"]:
                return copy.deepcopy(usage)
            raise AssertionError(f"Unexpected or mutating command: {args}")

        with tempfile.TemporaryDirectory() as temporary:
            args = types.SimpleNamespace(subscription=runner.BUILD_SUBSCRIPTION,
                                         scenario="all", build_id=4401, evidence=Path(temporary),
                                         test_source_sha="b" * 40)
            commands = Mock()
            commands.run.side_effect = command
            test_source = Path(temporary) / "test-source"
            with patch.object(runner, "TEST_SOURCE", test_source), \
                    patch.object(runner, "source_commit",
                                 side_effect=lambda commands, root, paths: "b" * 40 if root == test_source else "a" * 40), \
                    patch.object(runner, "compiled_parameters", side_effect=compiled):
                args.test_source_sha = "c" * 40
                with self.assertRaisesRegex(ValueError, "test source"):
                    runner.campaign_preflight(args, matrix, commands)
                self.assertFalse((args.evidence / "campaign-preflight.json").exists())
                args.test_source_sha = "b" * 40
                result = runner.campaign_preflight(args, matrix, commands)
                self.assertEqual(result["sourceSha"], "a" * 40)
                self.assertEqual(result["testSourceSha"], "b" * 40)
                self.assertEqual(result["buildId"], 4401)
                self.assertEqual(len(result["placements"]), 4)
                self.assertFalse(result["capacityReserved"])
                self.assertEqual(result["placements"]["default"], "centralus")
                self.assertEqual(result["placements"]["classic-azure-cni"], "centralus")
                self.assertEqual(result["placements"]["acns"], "eastus2")
                self.assertEqual(result["placements"]["otlp"], "centralus")
                self.assertEqual(
                    (runner.REPO / ".pipelines/azure-pipeline-config-tests.yml")
                    .read_text(encoding="utf-8").count(
                        'region not in ("centralus", "eastus2", "westcentralus")'), 1)
                expected = {"centralus": (80, 104), "eastus2": (16, 16)}
                for location, demand in expected.items():
                    quota = {row["name"]: row["additional"]
                             for row in result["regions"][location]["quota"]}
                    self.assertEqual((quota["standarddsv4family"], quota["cores"]), demand)
                self.assertEqual(
                    {row["name"]: row["additional"]
                     for row in result["regions"]["centralus"]["quota"]}
                    ["standarddpsv5family"], 24)
                args.evidence = Path(temporary) / "existing"
                with patch.object(runner.Host, "persistent_state", autospec=True,
                                  side_effect=lambda host, env, pools: {"clusterName": "existing"}
                                  if host.scenario["id"] == "default" else None):
                    reused = runner.campaign_preflight(args, matrix, commands)
                quota = {row["name"]: row["additional"] for row in reused["regions"]["centralus"]["quota"]}
                self.assertEqual((quota["cores"], quota["standarddsv4family"], quota["standarddpsv5family"]),
                                 (64, 52, 12))
                self.assertTrue(reused["existing"]["default"])
                args.evidence = Path(temporary)
                with self.assertRaisesRegex(ValueError, "new evidence"):
                    runner.campaign_preflight(args, matrix, commands)
                (args.evidence / "campaign-preflight.json").unlink()
                for row in usage:
                    row["limit"] = 100
                with patch.object(runner, "CAMPAIGN_LOCATIONS",
                                  dict.fromkeys(result["placements"], "eastus2")):
                    with self.assertRaisesRegex(ValueError, "quota"):
                        runner.campaign_preflight(args, matrix, commands)
                self.assertFalse((args.evidence / "campaign-preflight.json").exists())
                for row in usage:
                    row["limit"] = 300
                args.scenario = "default"
                selected = runner.campaign_preflight(args, matrix, commands)
                self.assertEqual(selected["placements"], {"default": "centralus"})

    @unittest.skipUnless(shutil.which("helm"), "Helm is required for chart rendering")
    def test_otlp_chart_enables_node_receiver(self):
        with tempfile.TemporaryDirectory() as temporary:
            evidence = Path(temporary)
            commands = runner.Commands(evidence)
            mapping = {(name, "linux"): "example.invalid/test@sha256:" + digit * 64
                       for name, digit in (("collector", "1"), ("targetallocator", "2"), ("cfg", "3"))}
            chart = runner.render_chart(commands, evidence, {
                "location": "eastus2", "kubernetesVersion": "1.36.5",
                "clusterId": "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/test/providers/Microsoft.ContainerService/managedClusters/test",
            }, "a" * 40, mapping, otlp=True)
            raw = commands.run(["helm", "template", "ama-metrics", str(chart)], private=True)
            node = next(doc for doc in runner.yaml.safe_load_all(raw)
                        if doc and doc.get("kind") == "DaemonSet" and doc["metadata"]["name"] == "ama-metrics-node")
            collector = next(c for c in node["spec"]["template"]["spec"]["containers"]
                             if c["name"] == "prometheus-collector")
            self.assertIn({"name": "APPMONITORING_OPENTELEMETRYMETRICS_ENABLED", "value": "true"}, collector["env"])
            self.assertIn(56681, [port["containerPort"] for port in collector["ports"]])

    def test_campaign_admission_rejects_invalid_scope_and_blocked_rows(self):
        with tempfile.TemporaryDirectory() as temporary:
            args = types.SimpleNamespace(subscription="unapproved", scenario="all",
                                         build_id=4401, evidence=Path(temporary))
            commands = Mock()
            for subscription, build_id, scenario_id in (
                    ("unapproved", 4401, "all"), (runner.BUILD_SUBSCRIPTION, 0, "all"),
                    (runner.BUILD_SUBSCRIPTION, 4401, "private")):
                args.subscription, args.build_id, args.scenario = subscription, build_id, scenario_id
                with self.subTest(scenario=scenario_id), self.assertRaises(ValueError):
                    runner.campaign_preflight(args, {"scenarios": []}, commands)
            commands.run.assert_not_called()

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

    def test_all_compatible_settings_expand_for_every_runnable_profile(self):
        variants = runner.catalog(runner.ROOT / "settings")
        document = runner.validate_matrix(runner.load(runner.ROOT / "matrix.json"), variants)
        rows = {row["id"]: row for row in document["scenarios"]}
        for scenario_id in ("default", "classic-azure-cni", "acns", "otlp"):
            row = rows[scenario_id]
            self.assertEqual(row["settingsVariants"], ["*"])
            if scenario_id != "otlp":
                self.assertNotIn("mip-enabled", runner.label_filter(row))
            selected = runner.scenario_settings(row, variants)
            expected = [name for name, item in variants.items()
                        if runner.compatible(row, item) and not item.get("helmValues")
                        and not item.get("addonParams")]
            self.assertEqual(selected, expected)
        self.assertIn("collector-controls", runner.scenario_settings(rows["default"], variants))
        self.assertNotIn("empty-secret-namespaces-v1",
                         runner.scenario_settings(rows["default"], variants))
        with self.assertRaisesRegex(ValueError, "wildcard must be the only"):
            runner.scenario_settings({**rows["default"], "settingsVariants": ["*", "default"]}, variants)

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
        for host in ("Windows2019", "Windows2022", "Windows2025"):
            for build in ("17763", "20348", "26100"):
                with self.subTest(host=host, build=build):
                    receipt["images"][1]["osVersion"] = f"10.0.{build}.123"
                    pools = [{"name": "windows", "osType": "Windows", "osSKU": host,
                              "architecture": "amd64"}]
                    supported = (host, build) in {
                        ("Windows2019", "17763"), ("Windows2022", "20348"),
                        ("Windows2025", "20348"),
                    }
                    if supported:
                        runner.candidate_receipt(receipt, source, ["linux", "windows"], pools=pools)
                    else:
                        with self.assertRaisesRegex(ValueError, "Windows build family"):
                            runner.candidate_receipt(receipt, source, ["linux", "windows"], pools=pools)
        receipt["images"][1]["osVersion"] = "10.0.20348.0"
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

    def test_runtime_audit_checks_supported_windows_host_image_pairs(self):
        nodes = [{"metadata": {"name": system}, "status": {"nodeInfo": {
            "operatingSystem": system, "architecture": "amd64",
            "kernelVersion": "10.0.26100.123" if system == "windows" else "6.8.0"}}}
            for system in ("linux", "windows")]
        images, pods = [], []
        for component, system, container, digit in (
                ("collector", "linux", "prometheus-collector", "1"),
                ("collector", "windows", "prometheus-collector", "2"),
                ("targetallocator", "linux", "targetallocator", "3"),
                ("cfg", "linux", "config-reader", "4")):
            ref = "example.invalid/test@sha256:" + digit * 64
            images.append({"name": component, "os": system, "arch": "amd64", "ref": ref})
            pods.append({"metadata": {"name": component + system},
                         "spec": {"nodeName": system, "containers": [{"name": container, "image": ref}]},
                         "status": {"containerStatuses": [{"name": container, "ready": True,
                                                           "imageID": ref}]}})
        host = Mock()
        host.kube.side_effect = lambda *args, **kwargs: {"items": nodes if args[1] == "nodes" else pods}
        for build, supported in (("20348", True), ("17763", False), ("26100", False)):
            with self.subTest(build=build):
                images[1]["osVersion"] = f"10.0.{build}.456"
                if supported:
                    self.assertEqual(len(runner.audit_images(host, images)), 4)
                else:
                    with self.assertRaisesRegex(RuntimeError, "wrong candidate imageID"):
                        runner.audit_images(host, images)
        images[1]["osVersion"] = "10.0.20348.456"
        index = "example.invalid/test@sha256:" + "5" * 64
        images[0]["manifestRef"] = index
        pods[0]["spec"]["containers"][0]["image"] = index
        status = pods[0]["status"]["containerStatuses"][0]
        for runtime in (images[0]["ref"], index):
            status["imageID"] = runtime
            self.assertEqual(len(runner.audit_images(host, images)), 4)
        status["imageID"] = "example.invalid/test@sha256:" + "6" * 64
        with self.assertRaisesRegex(RuntimeError, "wrong candidate imageID"):
            runner.audit_images(host, images)
        status["imageID"] = index
        status["ready"] = False
        with self.assertRaisesRegex(RuntimeError, "wrong candidate imageID"):
            runner.audit_images(host, images)
        status["ready"] = True
        images[0]["arch"] = "arm64"
        with self.assertRaisesRegex(RuntimeError, "wrong candidate imageID"):
            runner.audit_images(host, images)

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
            with patch.object(runner.shutil, "which", return_value="python"), \
                    patch.object(runner.subprocess, "run") as process:
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

    def test_private_role_assignment_list_exposes_only_allowlisted_error_codes(self):
        env = {"SCENARIO_WINDOWS_PASSWORD": "private-password", "SYSTEM_ACCESSTOKEN": "private-token"}
        for command, code, diagnostic in (
                (["az", "role", "assignment", "list"], "Authorization_RequestDenied", True),
                (["az", "role", "assignment", "list"], "UnknownFailure", False),
                (["az", "account", "get-access-token"], "Authorization_RequestDenied", False)):
            with self.subTest(command=command), tempfile.TemporaryDirectory() as temporary:
                failed = types.SimpleNamespace(
                    returncode=1, stdout="private-success-output",
                    stderr=f"ERROR: ({code}) private-password private-token not-in-environment-secret")
                evidence = Path(temporary)
                commands = runner.Commands(evidence)
                with patch.object(runner.shutil, "which", return_value="az"), \
                        patch.object(runner.subprocess, "run", return_value=failed):
                    with self.assertRaises(RuntimeError) as failure:
                        commands.run(command, env=env, private=True)
                message = str(failure.exception)
                self.assertEqual(code in message, diagnostic)
                for secret in (*env.values(), failed.stdout, "not-in-environment-secret"):
                    self.assertNotIn(secret, message)
                self.assertFalse((evidence / "commands.log").exists())

    def test_rbac_bootstrap_accepts_only_exact_scoped_condition_and_times_out_closed(self):
        with tempfile.TemporaryDirectory() as temporary:
            evidence = Path(temporary)
            host = runner.Host(types.SimpleNamespace(evidence=evidence, subscription="a" * 36),
                               scenario(), Mock())
            host.receipt = {"resourceGroup": "owned"}
            principal = "df26555d-400a-48dd-b013-940eb85ab5bb"
            host.assert_owned = Mock()
            rejected = []

            def assignments(*args, **kwargs):
                proof = runner.load(evidence / "rbac-bootstrap.json")
                valid = {"id": proof["scope"] + "/providers/Microsoft.Authorization/roleAssignments/one",
                         "scope": proof["scope"], "principalId": principal,
                         "roleDefinitionId": "/providers/Microsoft.Authorization/roleDefinitions/" +
                                             proof["roleDefinitionId"],
                         "conditionVersion": "2.0", "condition": proof["condition"]}
                if not rejected:
                    for field, value in (("scope", "/subscriptions/other"), ("principalId", "other"),
                                         ("roleDefinitionId", "owner"), ("conditionVersion", None),
                                         ("condition", None)):
                        rejected.append({**valid, field: value})
                    return rejected
                return [valid]

            host.az = Mock(side_effect=assignments)
            with patch.object(runner.time, "sleep") as sleep, patch("sys.stdout", new=io.StringIO()):
                host.wait_for_rbac(principal)
            sleep.assert_called_once_with(30)
            listing_calls = [call.args for call in host.az.call_args_list
                             if call.args[:3] == ("role", "assignment", "list")]
            self.assertEqual(len(listing_calls), 2)
            for command in listing_calls:
                self.assertIn("--resource-group", command)
                self.assertEqual(command[command.index("--resource-group") + 1], "owned")
                self.assertNotIn("--scope", command)
                self.assertNotIn("--all", command)
            proof = runner.load(evidence / "rbac-bootstrap.json")
            self.assertEqual(proof["status"], "granted")
            self.assertEqual(len(proof["allowedRoleDefinitionIds"]), 2)
            self.assertNotIn("acdd72a7", proof["condition"])
            self.assertIn("'ServicePrincipal'", proof["condition"])
            self.assertNotIn("'User'", proof["condition"])
            self.assertIn("@Request", proof["condition"])
            self.assertIn("@Resource", proof["condition"])
            self.assertEqual(host.receipt["rbacDelegationId"], proof["assignmentId"])
            command = (evidence / "rbac-bootstrap.ps1").read_text()
            self.assertIn("--scope " + proof["scope"], command)
            self.assertIn("AddMinutes(10)", command)
            self.assertIn("do {", command)
            self.assertIn("$output = & az role assignment create", command)
            self.assertIn("if ($exitCode -eq 0) { $output; break }", command)
            self.assertIn("authorizationfailed", command)
            self.assertIn("microsoft.authorization/roleassignments/write", command)
            self.assertIn("} while ($true)", command)
            host.az = Mock(return_value=[])
            with patch.object(runner.time, "monotonic", side_effect=[0, 1200]), \
                    patch("sys.stdout", new=io.StringIO()):
                with self.assertRaisesRegex(RuntimeError, "not confirmed"):
                    host.wait_for_rbac(principal)
                timeout_command = host.az.call_args.args
                self.assertIn("--resource-group", timeout_command)
                self.assertEqual(timeout_command[timeout_command.index("--resource-group") + 1], "owned")
                self.assertNotIn("--scope", timeout_command)
                self.assertNotIn("--all", timeout_command)

    def test_rbac_bootstrap_is_before_deployment_and_cleanup_verifies_delegation_absence(self):
        with tempfile.TemporaryDirectory() as temporary:
            args = types.SimpleNamespace(evidence=Path(temporary), subscription="sub", run_id="run",
                                         location="eastus2")
            host = runner.Host(args, scenario(), Mock())
            host.az = Mock(side_effect=["false", {}])
            host.wait_for_rbac = Mock(side_effect=RuntimeError("waiting"))
            with patch.object(host, "persistent_state", return_value=None), self.assertRaisesRegex(RuntimeError, "waiting"):
                host.provision({}, "principal", "ServicePrincipal", pool_parameters()["nodePools"])
            self.assertEqual(host.az.call_count, 2)
            host.wait_for_rbac.assert_called_once_with("principal")
            host.receipt["resourceGroup"] = "owned"
            host.receipt["managedGroups"] = []
            host.receipt["rbacBootstrapScope"] = "/subscriptions/sub/resourceGroups/owned"
            host.az = Mock(side_effect=["false", [{"scope": "/SUBSCRIPTIONS/SUB/RESOURCEGROUPS/OWNED"}]])
            with self.assertRaisesRegex(RuntimeError, "bootstrap role assignment still exists"):
                host.cleanup()
            host.az = Mock(side_effect=["false", []])
            host.cleanup()
            self.assertTrue(runner.load(args.evidence / "cleanup.json")["rbacDelegationRemoved"])

    def test_provision_retries_only_role_assignment_authorization_propagation(self):
        with tempfile.TemporaryDirectory() as temporary:
            args = types.SimpleNamespace(evidence=Path(temporary), subscription="sub", run_id="run",
                                         location="eastus2")
            host = runner.Host(args, scenario(), Mock())
            host.commands.deadline = None
            outputs = {"nodeResourceGroupName": "nodes", "amwManagedResourceGroupName": "amw"}
            failure = RuntimeError(
                "Command failed (1): az deployment: ERROR: {\"code\": \"InvalidTemplateDeployment\", \"message\": "
                "\"The template deployment failed with error: 'Authorization failed for template "
                "resource 'assignment-id' of type 'Microsoft.Authorization/roleAssignments'. "
                "The client 'app-id' with object id 'principal-id' does not have permission to "
                "perform action 'Microsoft.Authorization/roleAssignments/write' at scope 'scope'.'.\"}")
            host.az = Mock(side_effect=[
                "false", None, failure,
                {"properties": {"outputs": {key: {"value": value} for key, value in outputs.items()}}},
            ])
            with patch.object(host, "persistent_state", return_value=None), patch.object(host, "verify_deployment"), patch.object(host, "wait_for_rbac"), \
                    patch.object(runner.time, "monotonic", side_effect=[0, 0, 30]), \
                    patch.object(runner.time, "sleep") as sleep, \
                    patch("sys.stdout", new=io.StringIO()):
                self.assertEqual(host.provision({}, "principal", "ServicePrincipal",
                                                pool_parameters()["nodePools"]), outputs)
            deployment_calls = [call for call in host.az.call_args_list
                                if call.args[:3] == ("deployment", "group", "create")]
            self.assertEqual(len(deployment_calls), 2)
            sleep.assert_called_once_with(30)

    def test_provision_fails_immediately_for_other_deployment_errors(self):
        for principal_type, failure in (
                ("ServicePrincipal", "AuthorizationFailed: Microsoft.Compute/virtualMachines/write"),
                ("ServicePrincipal", "InvalidTemplateDeployment: Microsoft.Authorization/roleAssignments/write"),
                ("ServicePrincipal", "InvalidTemplateDeployment: Authorization failed for "
                                     "Microsoft.Compute/virtualMachines/write"),
                ("User", "InvalidTemplateDeployment: Authorization failed for template resource "
                         "of type Microsoft.Authorization/roleAssignments; action "
                         "Microsoft.Authorization/roleAssignments/write")):
            with self.subTest(principal_type=principal_type), tempfile.TemporaryDirectory() as temporary:
                args = types.SimpleNamespace(evidence=Path(temporary), subscription="sub", run_id="run",
                                             location="eastus2")
                host = runner.Host(args, scenario(), Mock())
                host.commands.deadline = None
                host.az = Mock(side_effect=["false", None, RuntimeError(failure)])
                with patch.object(host, "persistent_state", return_value=None), patch.object(host, "wait_for_rbac"), patch.object(runner.time, "sleep") as sleep:
                    with self.assertRaises(RuntimeError) as error:
                        host.provision({}, "principal", principal_type, pool_parameters()["nodePools"])
                self.assertEqual(str(error.exception), failure)
                deployment_calls = [call for call in host.az.call_args_list
                                    if call.args[:3] == ("deployment", "group", "create")]
                self.assertEqual(len(deployment_calls), 1)
                sleep.assert_not_called()

    def test_provision_bounds_role_assignment_retries_to_ten_minutes(self):
        with tempfile.TemporaryDirectory() as temporary:
            args = types.SimpleNamespace(evidence=Path(temporary), subscription="sub", run_id="run",
                                         location="eastus2")
            host = runner.Host(args, scenario(), Mock())
            host.commands.deadline = None
            now, delays = [0], []
            failure = RuntimeError(
                "ERROR: {\"code\": \"InvalidTemplateDeployment\", \"message\": "
                "\"The template deployment failed with error: 'Authorization failed for template "
                "resource 'assignment-id' of type 'Microsoft.Authorization/roleAssignments'. "
                "The client 'app-id' with object id 'principal-id' does not have permission to "
                "perform action 'Microsoft.Authorization/roleAssignments/write' at scope 'scope'.'.\"}")

            def az(*command, **kwargs):
                if command[:2] == ("group", "list"):
                    return []
                if command[:2] == ("group", "exists"):
                    return "false"
                if command[:2] == ("group", "create"):
                    return None
                if command[:3] == ("deployment", "group", "create"):
                    raise failure
                raise AssertionError(f"Unexpected command: {command}")

            host.az = Mock(side_effect=az)

            def sleep(delay):
                delays.append(delay)
                now[0] += delay

            with patch.object(host, "persistent_state", return_value=None), patch.object(host, "wait_for_rbac"), \
                    patch.object(runner.time, "monotonic", side_effect=lambda: now[0]), \
                    patch.object(runner.time, "sleep", side_effect=sleep), \
                    patch("sys.stdout", new=io.StringIO()):
                with self.assertRaisesRegex(RuntimeError, "roleAssignments/write"):
                    host.provision({}, "principal", "ServicePrincipal", pool_parameters()["nodePools"])
            deployment_calls = [call for call in host.az.call_args_list
                                if call.args[:3] == ("deployment", "group", "create")]
            self.assertGreater(len(deployment_calls), 1)
            self.assertGreater(sum(delays), 300)
            self.assertLessEqual(sum(delays), 600)
            self.assertEqual(delays, [30, 60, 120, 240, 150])
            self.assertIsNone(host.commands.deadline)

    def test_provision_routes_only_service_principal_deployments_through_retry(self):
        for kind in ("User", "ServicePrincipal"):
            with self.subTest(kind=kind), tempfile.TemporaryDirectory() as directory:
                host = runner.Host(types.SimpleNamespace(
                    evidence=Path(directory), subscription="sub", location="eastus2", run_id="run"),
                    scenario(), Mock())
                outputs = {"nodeResourceGroupName": "nodes", "amwManagedResourceGroupName": "amw"}
                deployment = {"properties": {"outputs": {
                    key: {"value": value} for key, value in outputs.items()}}}
                host.az = Mock(side_effect=["false", {}, deployment])
                with patch.object(host, "persistent_state", return_value=None), \
                        patch.object(host, "wait_for_rbac") as wait, \
                        patch.object(host, "az_with_rbac_retry", return_value=deployment) as retry:
                    self.assertEqual(host.provision({}, "principal", kind), outputs)
                self.assertEqual(retry.call_count, int(kind == "ServicePrincipal"))
                self.assertEqual(wait.call_count, int(kind == "ServicePrincipal"))

    def test_role_assignment_retry_matches_only_exact_unconditional_service_principal_grants(self):
        denial = RuntimeError("(AuthorizationFailed) Microsoft.Authorization/roleAssignments/write")
        role = runner.RBAC_MONITORING_DATA_READER
        expected = {"principalId": "principal", "principalType": "ServicePrincipal",
                    "roleDefinitionId": "/providers/Microsoft.Authorization/roleDefinitions/" + role,
                    "scope": "/owned"}
        command = ("role", "assignment", "create", "--assignee-object-id", "principal",
                   "--assignee-principal-type", "ServicePrincipal", "--role", role, "--scope", "/owned")
        for field, value in ((None, None), ("principalId", "other"), ("principalType", "User"),
                             ("roleDefinitionId", runner.RBAC_ADMIN_ROLE), ("scope", "/other"),
                             ("condition", "restricted")):
            with self.subTest(field=field):
                host = runner.Host(types.SimpleNamespace(evidence=Path("evidence"), subscription="sub"),
                                   scenario(), Mock())
                host.commands.deadline = None
                observed = dict(expected)
                if field:
                    observed[field] = value
                host.az = Mock(side_effect=[denial, [observed], "created"])
                with patch.object(runner.time, "monotonic", return_value=0), \
                        patch.object(runner.time, "sleep") as sleep:
                    result = host.az_with_rbac_retry(*command)
                self.assertEqual(result, "created" if field else expected)
                self.assertEqual(host.az.call_count, 3 if field else 2)
                self.assertEqual(sleep.call_count, int(bool(field)))
                self.assertIsNone(host.commands.deadline)

    def test_rbac_retry_refreshes_only_on_guidance_and_preserves_run_deadline(self):
        for refresh in (False, True):
            with self.subTest(refresh=refresh):
                host = runner.Host(types.SimpleNamespace(evidence=Path("evidence"), subscription="sub"),
                                   scenario(), Mock())
                host.commands.deadline = 90
                denial = RuntimeError(
                    "(AuthorizationFailed) Microsoft.Authorization/roleAssignments/write" +
                    (". If access was recently granted, please refresh your credentials." if refresh else ""))
                host.az = Mock(side_effect=[denial, [], *([None] if refresh else []), "created"])
                observed_deadlines = []
                with patch.object(runner.time, "monotonic", return_value=0), \
                        patch.object(runner.time, "sleep",
                                     side_effect=lambda delay: observed_deadlines.append(host.commands.deadline)):
                    result = host.az_with_rbac_retry(
                        "role", "assignment", "create", "--assignee-object-id", "principal",
                        "--assignee-principal-type", "ServicePrincipal", "--role",
                        runner.RBAC_MONITORING_DATA_READER, "--scope", "/owned")
                self.assertEqual(result, "created")
                self.assertEqual(observed_deadlines, [90])
                self.assertEqual(host.commands.deadline, 90)
                token_calls = [call for call in host.az.call_args_list if call.args[:2] == ("account", "get-access-token")]
                self.assertEqual(len(token_calls), int(refresh))
                if refresh:
                    self.assertEqual(token_calls[0].args, ("account", "get-access-token", "--output", "none"))
                    self.assertTrue(token_calls[0].kwargs["private"])

    def test_rbac_retry_honors_execution_deadline_and_restores_it_on_failure(self):
        denial = RuntimeError("(AuthorizationFailed) Microsoft.Authorization/roleAssignments/write")
        host = runner.Host(types.SimpleNamespace(evidence=Path("evidence"), subscription="sub"),
                           scenario(), Mock())
        host.commands.deadline = 10
        host.az = Mock(side_effect=denial)
        with patch.object(runner.time, "monotonic", side_effect=[0, 0, 10]), \
                patch.object(runner.time, "sleep") as sleep:
            with self.assertRaisesRegex(RuntimeError, "AuthorizationFailed"):
                host.az_with_rbac_retry("deployment", "group", "create")
        host.az.assert_called_once()
        sleep.assert_called_once_with(10)
        self.assertEqual(host.commands.deadline, 10)

    def test_rbac_retry_rejects_broader_writes_and_unrelated_errors(self):
        host = runner.Host(types.SimpleNamespace(evidence=Path("evidence"), subscription="sub"),
                           scenario(), Mock())
        host.commands.deadline = None
        for kind, role in (("User", runner.RBAC_MONITORING_DATA_READER),
                           ("ServicePrincipal", runner.RBAC_ADMIN_ROLE)):
            with self.subTest(kind=kind, role=role), self.assertRaisesRegex(ValueError, "Monitoring Data Reader"):
                host.az_with_rbac_retry("role", "assignment", "create", "--assignee-object-id", "principal",
                                        "--assignee-principal-type", kind, "--role", role, "--scope", "/owned")
        with self.assertRaisesRegex(ValueError, "limited"):
            host.az_with_rbac_retry("group", "delete")
        for message in ("AuthorizationFailed Microsoft.Authorization/roleAssignments/delete",
                        "AuthorizationFailed Microsoft.Compute/virtualMachines/write",
                        "InvalidTemplateDeployment Microsoft.Authorization/roleAssignments/write"):
            host.az = Mock(side_effect=RuntimeError(message))
            with self.subTest(message=message), patch.object(runner.time, "sleep") as sleep, \
                    patch.object(runner.time, "monotonic", side_effect=[0, 0, 600]):
                with self.assertRaises(RuntimeError):
                    host.az_with_rbac_retry("deployment", "group", "create")
                host.az.assert_called_once()
                sleep.assert_not_called()

    def test_rbac_retry_caps_backoff(self):
        denial = RuntimeError("(AuthorizationFailed) Microsoft.Authorization/roleAssignments/write")
        host = runner.Host(types.SimpleNamespace(evidence=Path("evidence"), subscription="sub"),
                           scenario(), Mock())
        host.commands.deadline = None
        host.az = Mock(side_effect=[denial] * 5 + ["created"])
        with patch.object(runner.time, "monotonic", return_value=0), \
                patch.object(runner.time, "sleep") as sleep:
            self.assertEqual(host.az_with_rbac_retry("deployment", "group", "create"), "created")
        self.assertEqual([call.args[0] for call in sleep.call_args_list], [30, 60, 120, 240, 300])

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

    def test_run_retains_owned_resource_inventory_and_removes_kubeconfig(self):
        with tempfile.TemporaryDirectory() as temporary:
            evidence = Path(temporary)
            args = types.SimpleNamespace(evidence=evidence, subscription=runner.BUILD_SUBSCRIPTION)
            host = runner.Host(args, scenario(), Mock())
            group = "owned"
            parent_id = f"/subscriptions/{runner.BUILD_SUBSCRIPTION}/resourceGroups/{group}"
            tags = {"owner": "ama-metrics-e2e", "scenario-run": "run",
                    "scenario": "mixed", "scenario-token": "token"}
            host.receipt = {
                "subscription": runner.BUILD_SUBSCRIPTION, "resourceGroup": group,
                "runId": "run", "scenario": "mixed", "token": "token",
                "managedGroups": ["owned-node"],
                "rbacBootstrapScope": parent_id, "rbacDelegationId": "assignment-id",
            }
            host.kubeconfig.write_text("test-only credential")
            host.az = Mock(side_effect=[
                "true", {"id": parent_id, "tags": tags},
                [{"name": "owned-node", "managedBy": parent_id + "/providers/aks"},
                 {"name": "unrelated", "managedBy": "/subscriptions/other/resourceGroups/parent/providers/aks"}],
                [{"id": parent_id + "/vm", "name": "vm", "type": "Microsoft.Compute/virtualMachines",
                  "location": "eastus2", "sku": {"name": "Standard_D4s_v4"}, "kind": None}],
                "true",
                {"id": parent_id + "-node", "tags": {}},
                [{"id": parent_id + "-node/vm", "name": "node", "type": "Microsoft.Compute/virtualMachines",
                  "location": "eastus2", "sku": {"name": "Standard_D4s_v4"}, "kind": None}],
            ])
            result = host.retain()
            self.assertTrue(result["retained"])
            self.assertEqual(result["inventory"][0]["tags"], tags)
            self.assertEqual(result["inventory"][0]["resources"][0]["id"], parent_id + "/vm")
            self.assertEqual(result["managedResourceGroups"], ["owned-node"])
            self.assertFalse(result["missingManagedResourceGroups"])
            self.assertEqual(result["rbacBootstrap"]["assignmentId"], "assignment-id")
            self.assertEqual(runner.load(evidence / "retained-resources.json"), result)
            self.assertFalse(host.kubeconfig.exists())
            self.assertFalse(any(call.args[:2] == ("group", "delete") for call in host.az.call_args_list))

    def test_run_uses_retention_even_when_tests_fail(self):
        with tempfile.TemporaryDirectory() as temporary:
            evidence = Path(temporary)
            args = types.SimpleNamespace(
                evidence=evidence, build_id=1, max_run_seconds=60, run_id="00000000-0000-0000-0000-000000000001",
                subscription=runner.BUILD_SUBSCRIPTION, location="eastus2", settle_seconds=0, test_source_sha="b" * 40,
                admitted_existing=False,
            )
            row = dict(scenario(), settingsVariants=[], clusterFeatures=[], platforms=[])
            commands = runner.Commands(evidence)
            with patch.object(runner, "verified_candidate", return_value=({"source": {"commit": "a" * 40}}, {})), \
                    patch.object(runner, "label_filter", return_value=""), \
                    patch.object(runner, "preflight", return_value={"expectedPools": [], "kubernetesVersion": "1.36.5", "existing": False}), \
                    patch.object(runner, "candidate_receipt", return_value=([], {})), \
                    patch.object(runner, "prepare_testkube", return_value=("chart", "principal", "User")), \
                    patch.object(runner, "deployment_env", return_value={}), \
                    patch.object(runner.Host, "provision", side_effect=RuntimeError("test failure")), \
                    patch.object(runner.Host, "retain", return_value={"retained": True}) as retain, \
                    patch.object(runner.Host, "cleanup") as cleanup:
                with self.assertRaisesRegex(RuntimeError, "test failure"):
                    runner.execute(args, row, {}, commands)
            retain.assert_called_once()
            cleanup.assert_not_called()
            self.assertTrue(runner.load(evidence / "result.json")["resources"]["retained"])

    def test_revoke_bootstrap_removes_only_the_exact_assignment_and_preserves_workload_roles(self):
        with tempfile.TemporaryDirectory() as temporary:
            evidence = Path(temporary)
            subscription = runner.BUILD_SUBSCRIPTION
            group = "owned"
            scope = f"/subscriptions/{subscription}/resourceGroups/{group}"
            admin = "f58310d9-a9f6-439a-9e8d-f62e7b41a168"
            allowed = ["4d97b98b-1d4f-4787-a291-c67834d212e7",
                       "b0d8363b-8ddd-447d-831f-62ca05bff136"]
            principal = "df26555d-400a-48dd-b013-940eb85ab5bb"
            assignment_id = scope + "/providers/Microsoft.Authorization/roleAssignments/11111111-1111-4111-8111-111111111111"
            condition = runner.rbac_condition(allowed)
            run_id, scenario_id, token = "run-id", "default", "ownership-token"
            proof = {"status": "granted", "scope": scope, "assignmentId": assignment_id,
                     "principalId": principal, "roleDefinitionId": admin,
                     "allowedRoleDefinitionIds": allowed, "condition": condition}
            network_id = scope + "/providers/Microsoft.Authorization/roleAssignments/22222222-2222-4222-8222-222222222222"
            monitor_id = scope + "/providers/Microsoft.Authorization/roleAssignments/33333333-3333-4333-8333-333333333333"
            assignments = [
                {"id": assignment_id, "scope": scope, "principalId": principal,
                 "roleDefinitionId": admin, "conditionVersion": "2.0", "condition": condition},
                {"id": network_id, "roleDefinitionId": allowed[0]},
                {"id": monitor_id, "roleDefinitionId": allowed[1]},
            ]
            tags = {"owner": "ama-metrics-e2e", "scenario-run": run_id,
                    "scenario": scenario_id, "scenario-token": token}
            runner.save(evidence / "result.json", {
                "status": "PASS", "runId": run_id, "scenario": scenario_id,
            })
            runner.save(evidence / "retained-resources.json", {
                "retained": True, "subscription": subscription, "resourceGroup": group,
                "runId": run_id, "scenario": scenario_id,
                "inventory": [{"id": scope, "name": group, "tags": tags, "resources": []}],
                "rbacBootstrap": {"scope": scope, "assignmentId": assignment_id},
            })
            runner.save(evidence / "rbac-bootstrap.json", proof)
            host = runner.Host(types.SimpleNamespace(evidence=evidence, subscription=subscription), {}, Mock())
            host.az = Mock(side_effect=[{"tags": tags}, "user", assignments, "", assignments[1:]])
            host.revoke_bootstrap()
            listing_calls = [call.args for call in host.az.call_args_list
                            if call.args[:3] == ("role", "assignment", "list")]
            self.assertEqual(len(listing_calls), 2)
            for command in listing_calls:
                self.assertIn("--resource-group", command)
                self.assertEqual(command[command.index("--resource-group") + 1], group)
                self.assertNotIn("--scope", command)
                self.assertNotIn("--all", command)
            deleted = [call.args for call in host.az.call_args_list
                       if call.args[:3] == ("role", "assignment", "delete")]
            self.assertEqual(deleted, [("role", "assignment", "delete", "--ids", assignment_id)])
            receipt = runner.load(evidence / "rbac-bootstrap-revocation.json")
            self.assertTrue(receipt["verifiedAbsent"])
            self.assertEqual(set(receipt["preservedRoleAssignmentIds"]), {network_id, monitor_id})

    def test_revoke_bootstrap_refuses_persistent_grants_before_any_azure_call(self):
        for group in ("ama-e2e-persistent-default", "AMA-E2E-PERSISTENT-default"):
            with self.subTest(group=group), tempfile.TemporaryDirectory() as directory:
                evidence = Path(directory)
                runner.save(evidence / "retained-resources.json", {"resourceGroup": group})
                runner.save(evidence / "result.json", {})
                runner.save(evidence / "rbac-bootstrap.json", {
                    "assignmentId": "/subscriptions/sub/resourceGroups/owned/providers/"
                                    "Microsoft.Authorization/roleAssignments/11111111-1111-4111-8111-111111111111",
                })
                host = runner.Host(types.SimpleNamespace(evidence=evidence, subscription="sub"), {}, Mock())
                host.az = Mock()
                with self.assertRaisesRegex(ValueError, "Persistent campaign bootstrap grants must be retained"):
                    host.revoke_bootstrap()
                host.az.assert_not_called()
                self.assertFalse((evidence / "rbac-bootstrap-revocation.json").exists())

    def test_revoke_bootstrap_rejects_a_mismatched_assignment_without_deleting_it(self):
        with tempfile.TemporaryDirectory() as temporary:
            evidence = Path(temporary)
            subscription = runner.BUILD_SUBSCRIPTION
            group = "owned"
            scope = f"/subscriptions/{subscription}/resourceGroups/{group}"
            assignment_id = scope + "/providers/Microsoft.Authorization/roleAssignments/11111111-1111-4111-8111-111111111111"
            allowed = ["4d97b98b-1d4f-4787-a291-c67834d212e7",
                       "b0d8363b-8ddd-447d-831f-62ca05bff136"]
            principal = "df26555d-400a-48dd-b013-940eb85ab5bb"
            condition = runner.rbac_condition(allowed)
            run_id, scenario_id, token = "run-id", "default", "ownership-token"
            proof = {"status": "granted", "scope": scope, "assignmentId": assignment_id,
                     "principalId": principal, "roleDefinitionId": "f58310d9-a9f6-439a-9e8d-f62e7b41a168",
                     "allowedRoleDefinitionIds": allowed, "condition": condition}
            tags = {"owner": "ama-metrics-e2e", "scenario-run": run_id,
                    "scenario": scenario_id, "scenario-token": token}
            runner.save(evidence / "result.json", {
                "status": "PASS", "runId": run_id, "scenario": scenario_id,
            })
            runner.save(evidence / "retained-resources.json", {
                "retained": True, "subscription": subscription, "resourceGroup": group,
                "runId": run_id, "scenario": scenario_id,
                "inventory": [{"id": scope, "name": group, "tags": tags, "resources": []}],
                "rbacBootstrap": {"scope": scope, "assignmentId": assignment_id},
            })
            for field, value in (("roleDefinitionId", "not-the-approved-role"),
                                 ("condition", "unrestricted"),
                                 ("groupTags", {"owner": "other"})):
                case_proof = dict(proof)
                if field == "condition":
                    case_proof["condition"] = value
                runner.save(evidence / "rbac-bootstrap.json", case_proof)
                assignment = {"id": assignment_id, "scope": scope, "principalId": principal,
                              "roleDefinitionId": proof["roleDefinitionId"],
                              "conditionVersion": "2.0", "condition": condition}
                assignment[field] = value
                host = runner.Host(types.SimpleNamespace(evidence=evidence, subscription=subscription), {}, Mock())
                actual_tags = value if field == "groupTags" else tags
                host.az = Mock(side_effect=[{"tags": actual_tags}, "user", [assignment]])
                with self.subTest(field=field), self.assertRaises(ValueError):
                    host.revoke_bootstrap()
                self.assertFalse(any(call.args[:3] == ("role", "assignment", "delete")
                                     for call in host.az.call_args_list))

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

    def test_collector_controls_runs_live_ksm_allowlist_query_before_restoring_slot(self):
        with tempfile.TemporaryDirectory() as temporary:
            host = Mock()
            host.scenario = scenario()
            host.args.evidence = Path(temporary)
            host.args.settle_seconds = 0
            events = []
            host.kube.side_effect = lambda *args, **kwargs: events.append(args[0]) or ""
            manifest = variant()
            manifest["id"] = "collector-controls"

            def submit(host, suite, *args, **kwargs):
                events.append((suite, args[1]))
                return {"executed": 1}

            with patch.object(runner, "settings_workflow", return_value=("case.yaml", "case")), patch.object(
                    runner, "label_filter", return_value="case-label"), patch.object(
                    runner, "run_suite", side_effect=submit) as workflow:
                result = runner.settings_slot(host, manifest)
            self.assertEqual(workflow.call_count, 2)
            self.assertEqual(workflow.call_args_list[1].args[1], "querymetrics")
            self.assertEqual(workflow.call_args_list[1].args[3], "ksm-allowlist")
            query_index = events.index(("querymetrics", "ksm-allowlist"))
            self.assertLess(query_index, events.index("delete", query_index))
            self.assertEqual(result["ksmAllowlist"]["executed"], 1)

    def test_settings_slots_run_live_ingestion_before_restoring_configmaps(self):
        cases = (
            ("windows", ["linux", "windows"], ["config-processing-settings-node-config-map"],
             "config-processing-settings-node-config-map", "nodeConfigMapIngestion"),
            ("minimal-ingestion-disabled", ["linux"], ["config-processing-mip-false"],
             "mip-disabled", "mipIngestion"),
        )
        for variant_id, platforms, labels, query_label, result_key in cases:
            with self.subTest(variant=variant_id), tempfile.TemporaryDirectory() as temporary:
                host = Mock()
                host.scenario = scenario()
                host.scenario["platforms"] = platforms
                host.args.evidence = Path(temporary)
                host.args.settle_seconds = 0
                events = []
                host.kube.side_effect = lambda *args, **kwargs: events.append(args[0]) or ""
                manifest = variant()
                manifest["id"] = variant_id
                manifest["ginkgoLabels"] = labels

                def submit(host, suite, *args, **kwargs):
                    events.append((suite, args[1]))
                    return {"executed": 1}

                with patch.object(runner, "settings_workflow", return_value=("case.yaml", "case")), patch.object(
                        runner, "label_filter", return_value="case-label"), patch.object(
                        runner, "run_suite", side_effect=submit) as workflow:
                    result = runner.settings_slot(host, manifest)

                self.assertEqual(workflow.call_count, 2)
                self.assertEqual(workflow.call_args_list[1].args[1], "querymetrics")
                self.assertEqual(workflow.call_args_list[1].args[3], query_label)
                expected_config_filter = "case-label" if "windows" in platforms else "(case-label) && !windows"
                self.assertEqual(workflow.call_args_list[0].args[3], expected_config_filter)
                query_index = events.index(("querymetrics", query_label))
                self.assertLess(query_index, events.index("delete", query_index))
                self.assertEqual(result[result_key]["executed"], 1)

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
            args = types.SimpleNamespace(build_id=123, receipt_tools_root=directory, evidence=directory / "evidence",
                                         test_source_sha="b" * 40)
            verifier = directory / ".pipelines/pr-e2e/fetch_candidate.py"
            verifier.parent.mkdir(parents=True)
            verifier.write_text("# Mocked protected producer entrypoint\n")
            raw = {"source": {"repo": "Azure/prometheus-collector", "commit": "a" * 40, "buildId": "123"},
                   "imageRef": "example.invalid/runner@sha256:" + "1" * 64}
            receipt = {"source": raw["source"], "testRunner": {"testSource": raw["source"],
                       "imageRef": raw["imageRef"], "receiptSha256": "",
                       "mode": "reused-toolchain-source-fetched-separately"}}
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
                    if "rev-parse" not in command:
                        return ""
                    return "b" * 40 if command[2] == str(directory / "test-source") else "a" * 40
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
            with patch.object(runner, "TEST_SOURCE", directory / "test-source"):
                verified, runtime = runner.verified_candidate(args, commands)
                self.assertEqual(runtime, raw)
                self.assertEqual(verified["source"]["commit"], "a" * 40)
                images, mapping = runner.candidate_receipt(verified, "a" * 40, ["linux"], ["arm64"])
                self.assertEqual(len(images), 6)
                self.assertEqual(mapping[("cfg", "linux")], receipt["images"][-1]["indexRef"])
                self.assertEqual(mapping[("collector", "linux")], receipt["images"][0]["indexRef"])
                persisted = runner.load(args.evidence / "verified-candidate/candidate-build.json")
                self.assertEqual(persisted["images"], receipt["images"])
                args.test_source_sha = "c" * 40
                with self.assertRaisesRegex(ValueError, "test source"):
                    runner.verified_candidate(args, commands)
                args.test_source_sha = "b" * 40
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
            host.args.run_id = "fresh-run-marker"
            host.receipt = {"token": "persistent-owner"}
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
            host.args.run_id = "next-run-marker"
            with patch.object(runner.time, "time", return_value=110):
                next_run = runner.verify_ingestion(host, {"queryEndpoint": "https://amw.example"}, 100)
            self.assertIn('scenario_run="next-run-marker"', next_run["query"])
            self.assertNotEqual(result["query"], next_run["query"])
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
            host.scenario = {"clusterFeatures": []}
            identity = {"name": "ama-e2e-tests", "principalId": "identity-object", "clientId": "identity-client"}
            existing_roles = []
            cluster = {"oidcIssuerProfile": {"issuerUrl": "https://issuer.example/"},
                       "kubernetesVersion": "1.36.5", "autoUpgradeProfile": {"upgradeChannel": "stable"},
                       "securityProfile": {"workloadIdentity": {"enabled": True}}}

            def az(*args, **kwargs):
                if args[:2] == ("identity", "create"):
                    return identity
                if args[:2] == ("aks", "show"):
                    return cluster
                if args[:3] == ("role", "assignment", "list"):
                    self.assertIn(args[args.index("--assignee-object-id") + 1], ("identity-object", "caller-object"))
                    self.assertEqual(args[args.index("--fill-principal-name") + 1], "false")
                    return existing_roles
                return {}

            def kube(*args, **kwargs):
                if args[:2] == ("get", "deployments"):
                    return {"items": [{"metadata": {"name": "testkube-api-server"}}]}
                return "testworkflows <none> testworkflows.testkube.io/v1 true TestWorkflow"

            host.az.side_effect = az
            host.commands.deadline = None
            def retry(*args, **kwargs):
                self.assertEqual(args[args.index("--assignee-principal-type") + 1], "ServicePrincipal")
                return runner.Host.az_with_rbac_retry(host, *args, **kwargs)
            host.az_with_rbac_retry.side_effect = retry
            host.kube.side_effect = kube
            outputs = {"clusterName": "owned-cluster", "workspaceId": "/owned/workspace",
                       "queryEndpoint": "https://amw.example"}
            runner.bootstrap_testkube(host, outputs, Path("pinned-chart.tgz"), "caller-object", "User")
            self.assertEqual(host.az_with_rbac_retry.call_count, 1)
            helm = next(c.args[0] for c in host.commands.run.call_args_list if c.args[0][0] == "helm")
            for setting in ("testkube-api.minio.podSecurityContext.fsGroup=1001",
                            "mongodb.nodeAffinityPreset.type=hard",
                            "mongodb.nodeAffinityPreset.key=kubernetes.azure.com/agentpool",
                            "mongodb.nodeAffinityPreset.values[0]=system",
                            "mongodb.updateStrategy.type=Recreate",
                            "global.testWorkflows.builtinTemplatesInstaller.nodeSelector.kubernetes\\.io/os=linux"):
                self.assertIn(setting, helm)
            role_calls = [c.args for c in host.az.call_args_list if c.args[:3] == ("role", "assignment", "create")]
            self.assertEqual(len(role_calls), 2)
            for args in role_calls:
                self.assertEqual(args[args.index("--scope") + 1], "/owned/workspace")
                self.assertEqual(args[args.index("--role") + 1], "b0d8363b-8ddd-447d-831f-62ca05bff136")
            existing_roles.extend({
                "scope": "/owned/workspace", "principalId": object_id,
                "roleDefinitionId": "/providers/Microsoft.Authorization/roleDefinitions/b0d8363b-8ddd-447d-831f-62ca05bff136",
            } for object_id in ("identity-object", "caller-object"))
            host.az.reset_mock()
            host.az_with_rbac_retry.reset_mock()
            runner.bootstrap_testkube(host, outputs, Path("pinned-chart.tgz"), "caller-object", "User")
            host.az_with_rbac_retry.assert_not_called()
            self.assertFalse(any(c.args[:3] == ("role", "assignment", "create")
                                 for c in host.az.call_args_list))
            federation = next(c.args for c in host.az.call_args_list if c.args[:2] == ("identity", "federated-credential"))
            self.assertEqual(federation[federation.index("--issuer") + 1], "https://issuer.example/")
            proof = runner.load(Path(temporary) / "testkube-bootstrap.json")
            self.assertIn("not submitted", proof["workloadIdentityExecution"])
            host.scenario = {"clusterFeatures": ["managed-acns"]}
            cluster["networkProfile"] = {"advancedNetworking": {
                "enabled": True, "observability": {"enabled": True},
            }}
            runner.bootstrap_testkube(host, outputs, Path("pinned-chart.tgz"), "caller-object", "User")
            acns = runner.load(Path(temporary) / "managed-acns.json")
            self.assertEqual(acns, {"advancedNetworkingEnabled": True, "observabilityEnabled": True})
            cluster["networkProfile"]["advancedNetworking"]["observability"]["enabled"] = False
            with self.assertRaisesRegex(RuntimeError, "managed advanced networking"):
                runner.bootstrap_testkube(host, outputs, Path("pinned-chart.tgz"), "caller-object", "User")
            cluster["networkProfile"]["advancedNetworking"]["observability"]["enabled"] = True
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
            host.args.test_source_sha = "b" * 40
            host.kubeconfig = Path(temporary) / "isolated-kubeconfig"
            host.query_endpoint = "https://amw.example"
            host.query_client_id = "query-client"
            host.cluster_name = "owned-cluster"
            host.scenario = scenario()
            host.runner_receipt = {"source": {"commit": "a" * 40},
                                   "imageRef": "example.invalid/runner@sha256:" + "1" * 64}
            generated = []

            def submit(command, **kwargs):
                generated.append(runner.TEST_SOURCE / "otelcollector/test/testkube" / command[5])
                generated[-1].write_text("generated")
                return workflow_fixture(destination, host.args.test_source_sha)

            host.commands.run.side_effect = submit
            result = runner.run_suite(host, "configprocessing", destination, "case-label",
                                      template="case.yaml", workflow="configprocessingcommon")
            command = host.commands.run.call_args.args[0]
            env = host.commands.run.call_args.kwargs["env"]
            self.assertEqual(command[0:2], ["bash", "./testkube/run-testkube-workflow.sh"])
            script = (runner.TEST_SOURCE / "otelcollector/test/testkube/run-testkube-workflow.sh").read_text()
            pattern = re.search(r'\[\[ "\$TARGET_OUTPUT" =~ (.+?) \]\]', script).group(1)
            self.assertEqual(pattern, r"^[A-Za-z0-9_-]+\.yaml$")
            self.assertIsNotNone(re.fullmatch(pattern, command[5]))
            self.assertIsNone(re.fullmatch(pattern, "." + command[5]))
            self.assertFalse(generated[0].exists())
            self.assertEqual(command[-4:], ["false", "0", "ConfigTests", "b" * 40])
            self.assertEqual(env["CANDIDATE_TEST_SOURCE_SHA"], "b" * 40)
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
        "persistent-bootstrap-grants": (
            'if group.casefold().startswith("ama-e2e-persistent-"):', "if False:"),
        "rbac-retry-write-only": ('"microsoft.authorization/roleassignments/write" not in message', "False"),
        "rbac-retry-exact-role": ('options.get("--role", "").casefold() != RBAC_MONITORING_DATA_READER', "False"),
        "rbac-retry-existing-role": ('assigned_role.casefold() == role_assignment["roleDefinitionId"]', "True"),
        "rbac-retry-existing-principal": (
            '== role_assignment["principalId"].casefold()', "== assignment.get(\"principalId\", \"\").casefold()"),
        "rbac-retry-existing-scope": (
            '== role_assignment["scope"].casefold()', "== assignment.get(\"scope\", \"\").casefold()"),
        "rbac-retry-existing-condition": ('and not assignment.get("condition")', "and True"),
        "rbac-retry-backoff-cap": (
            "interval = min(interval * 2, RBAC_AUTH_RETRY_MAX_INTERVAL)", "interval = interval * 2"),
        "rbac-retry-ten-minute-window": ("RBAC_AUTH_RETRY_TIMEOUT = 600", "RBAC_AUTH_RETRY_TIMEOUT = 300"),
        "rbac-retry-run-deadline": ("deadline = min(deadline, run_deadline)", "pass"),
        "rbac-retry-preserves-user-bootstrap": (
            'assign = host.az_with_rbac_retry if kind == "ServicePrincipal" else host.az',
            "assign = host.az_with_rbac_retry"),
        "rbac-retry-preserves-user-deployment": (
            'deploy = self.az_with_rbac_retry if principal_type == "ServicePrincipal" else self.az',
            "deploy = self.az_with_rbac_retry"),
        "rbac-before-deployment": ("self.wait_for_rbac(principal)", "pass"),
        "rbac-exact-scope": ('assignment.get("scope", "").casefold() == scope.casefold()', "True"),
        "rbac-exact-principal": ('assignment.get("principalId", "").casefold() == principal.casefold()', "True"),
        "rbac-required-condition": ('re.sub(r"\\s+", "", assignment.get("condition") or "") == re.sub(r"\\s+", "", condition)', "True"),
        "rbac-cleanup": ('if any(row["scope"].casefold() == self.receipt["rbacBootstrapScope"].casefold()\n                       for row in assignments):', 'if False:'),
        "bootstrap-revocation-exact-role": ('assignment.get("roleDefinitionId", "").rsplit("/", 1)[-1].casefold() != admin_role', "False"),
        "bootstrap-revocation-condition": ('re.sub(r"\\s+", "", condition) != re.sub(r"\\s+", "", rbac_condition(allowed))', "False"),
        "bootstrap-revocation-owned-group": (
            '        self.assert_owned()\n        if self.az("account", "show", "--query", "user.type")',
            '        if False: self.assert_owned()\n        if self.az("account", "show", "--query", "user.type")'),
        "run-retains-resources": ('outcome["resources"] = host.retain()', 'host.cleanup()'),
        "retained-resource-inventory": ('"resources": resources,', '"resources": []'),
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
        "campaign-region-split": ('"classic-azure-cni": "centralus",', '"classic-azure-cni": "eastus2",'),
        "campaign-aggregate": ('regional_capacity(regional, capacity, commands)',
                               'regional_capacity(regional, capacity[:1], commands)'),
        "quota-numeric-strings": ('int(str(row["currentValue"]))', 'row["currentValue"]'),
        "sku-architecture": ('entry["architecture"] != architecture', "False"),
        "sku-restrictions": ('restriction["type"] == "Location"', "False"),
        "explicit-surge": ('or not re.fullmatch(r"[1-9][0-9]*", surge)', "or False"),
        "pool-properties": ('any(row.get(key) != value for key, value in fields.items())', "False"),
        "pool-node-presence": ('not minimum <= len(members) <= maximum', "False"),
        "alias-only": ('if set(variant["ginkgoLabels"]) & {', "if set() & {"),
        "windows-pool-build": ('if pool["osType"] == "Windows" and not any(', "if False and not any("),
        "windows-2025-downlevel": ('("10.0.26100", "10.0.20348")',
                                   '("10.0.26100", "10.0.26100")'),
        "windows-2025-legacy-image": ('("10.0.26100", "10.0.20348")',
                                     '("10.0.26100", "10.0.17763")'),
        "windows-runtime-build": ('and windows_image_compatible(info["kernelVersion"], i["osVersion"])',
                                  'and True'),
        "windows-runtime-exact-build": ('and windows_image_compatible(info["kernelVersion"], i["osVersion"])',
                                        'and info["kernelVersion"].split(".")[:3] == i["osVersion"].split(".")[:3]'),
        "credential-cleanup": ('self.kubeconfig.unlink(missing_ok=True)', "pass"),
        "external-cleanup": ('while self.az("group", "exists", "--name", managed).strip() != "false":', "while False:"),
        "literal-labels": ('known.update(re.findall(r\'"([^"]+)"\', arguments))', "pass"),
        "query-auth-template": ("Bearer $token", "Bearer"),
        "fresh-sample": ("if not math.isfinite(observed) or not started <= observed <= time.time() + 60:", "if False:"),
        "empty-junit": ("if not executed or failed:", "if failed:"),
        "cleanup-ownership": ('if any(group.get("tags", {}).get(k) != v for k, v in expected.items()):', "if False:"),
        "retry-deployment-state": ('if properties.get("provisioningState") != "Succeeded":', "if False:"),
        "role-assignment-retry-loop": ('"do {\\n"', '"if ($false) {\\n"'),
        "managed-acns-enabled": (
            'if (advanced.get("enabled") is not True\n                or (advanced.get("observability") or {}).get("enabled") is not True):',
            "if False:"),
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
