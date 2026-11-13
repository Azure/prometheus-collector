"""Offline AMPLS proof checks; --mutations demonstrates that each guard can fail."""

import copy
import importlib.util
import io
import json
from pathlib import Path
import sys
import tempfile
import types
import unittest
from unittest.mock import Mock, patch


SOURCE = Path(__file__).resolve().parents[1] / "ampls_proofs.py"
SPEC = importlib.util.spec_from_file_location("ampls_proofs", SOURCE)
proofs = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(proofs)


class Proofs(unittest.TestCase):
    def test_initialization_preserves_monitoring_resources_and_repairs_addon_overwrites(self):
        self.outputs["clusterId"] = "/cluster"
        for enabled in (False, True):
            state = {"azureMonitorProfile": {"metrics": {"enabled": enabled}}}
            self.resources[self.outputs["dceId"]]["networkAcls"]["publicNetworkAccess"] = "Enabled"
            for name in ("ruleAssociationId", "configurationAssociationId"):
                self.resources.pop(self.outputs[name], None)

            def az(*args, **kwargs):
                if args[:2] == ("resource", "show"):
                    self.assertEqual(args, ("resource", "show", "--ids", "/cluster",
                                            "--api-version", "2024-10-01"))
                    return {"properties": state}
                if args[:2] == ("resource", "update"):
                    target = args[args.index("--ids") + 1]
                    if target == "/cluster":
                        self.assertEqual(args[-1], "properties.azureMonitorProfile.metrics.enabled=false")
                        state["azureMonitorProfile"]["metrics"]["enabled"] = False
                    else:
                        self.assertEqual(target, self.outputs["dceId"])
                        self.assertEqual(args[-1], "properties.networkAcls.publicNetworkAccess=Disabled")
                        self.resources[target]["networkAcls"]["publicNetworkAccess"] = "Disabled"
                    return {}
                url = args[args.index("--url") + 1]
                target = url.split("https://management.azure.com", 1)[1].split("?", 1)[0]
                if args[args.index("--method") + 1] == "put":
                    self.resources[target] = json.loads(args[args.index("--body") + 1])["properties"]
                return {"properties": copy.deepcopy(self.resources[target])}

            self.host.az.side_effect = az
            self.assertEqual(proofs.initialize(self.host, self.outputs), enabled)
            self.assertFalse(state["azureMonitorProfile"]["metrics"]["enabled"])
            self.assertEqual(proofs.verify_control_plane(self.host, self.outputs)["status"], "verified")

    def test_proof_requires_real_owned_group_lock_and_emits_evidence(self):
        with tempfile.TemporaryDirectory() as directory:
            host = Mock()
            host.args.evidence = Path(directory)
            host.receipt = {"resourceGroup": "owned-ampls"}
            for level in ("CanNotDelete", "ReadOnly", None):
                host.az.return_value = {"level": level}
                with patch.object(proofs, "verify_control_plane", return_value={"status": "verified"}), \
                        patch.object(proofs, "verify_private_dns", return_value={}), \
                        patch.object(proofs, "verify_public_denial", return_value={}), \
                        patch("sys.stdout", new_callable=io.StringIO) as log:
                    if level == "CanNotDelete":
                        proofs.verify(host, {})
                        self.assertIn("AMPLS_PROOF", log.getvalue())
                        self.assertIn("CanNotDelete", (Path(directory) / "ampls.json").read_text())
                        self.assertIn("owned-ampls", host.az.call_args.args)
                    else:
                        with self.assertRaises(RuntimeError):
                            proofs.verify(host, {})

    def setUp(self):
        self.outputs = {name: "/" + name for name in (
            "dceId", "dcrId", "ruleAssociationId", "configurationAssociationId", "workspaceId",
            "workspaceDefaultDceId", "amplsId", "monitorPrivateEndpointId", "queryPrivateEndpointId", "vnetId")}
        self.outputs["queryEndpoint"] = "https://query.example"
        o = self.outputs
        self.resources = {
            o["dceId"]: {"networkAcls": {"publicNetworkAccess": "Disabled"},
                         "configurationAccess": {"endpoint": "https://config.example"},
                         "metricsIngestion": {"endpoint": "https://ingest.example"}},
            o["dcrId"]: {"dataCollectionEndpointId": o["dceId"], "immutableId": "dcr-immutable",
                         "destinations": {"monitoringAccounts": [
                             {"name": "amw", "accountResourceId": o["workspaceId"]}]},
                         "dataFlows": [{"streams": ["Microsoft-PrometheusMetrics"], "destinations": ["amw"]}]},
            o["ruleAssociationId"]: {"dataCollectionRuleId": o["dcrId"]},
            o["configurationAssociationId"]: {"dataCollectionEndpointId": o["dceId"]},
            o["workspaceId"]: {"publicNetworkAccess": "Disabled", "defaultIngestionSettings": {
                "dataCollectionEndpointResourceId": o["workspaceDefaultDceId"]}},
            o["amplsId"]: {"accessModeSettings": {"ingestionAccessMode": "PrivateOnly",
                                                  "queryAccessMode": "PrivateOnly", "exclusions": []}},
            o["amplsId"] + "/scopedResources/cluster-dce": {"linkedResourceId": o["dceId"]},
            o["amplsId"] + "/scopedResources/workspace-dce": {"linkedResourceId": o["workspaceDefaultDceId"]},
        }
        for kind, resource, target, group, address in (
                ("monitor", o["monitorPrivateEndpointId"], o["amplsId"], "azuremonitor", "10.225.0.4"),
                ("query", o["queryPrivateEndpointId"], o["workspaceId"], "prometheusMetrics", "10.225.0.5")):
            self.resources[resource] = {
                "provisioningState": "Succeeded", "subnet": {"id": o["vnetId"] + "/subnets/endpoints"},
                "privateLinkServiceConnections": [{"properties": {
                    "privateLinkServiceId": target, "groupIds": [group],
                    "privateLinkServiceConnectionState": {"status": "Approved"}}}],
                "networkInterfaces": [{"id": "/" + kind + "-nic"}],
            }
            self.resources["/" + kind + "-nic"] = {"ipConfigurations": [
                {"properties": {"privateIPAddress": address}}]}
        self.host = Mock()
        self.host.scenario = {"platforms": ["linux", "windows"]}
        self.host.az.side_effect = lambda *a, **k: {
            "properties": copy.deepcopy(self.resources[a[a.index("--url") + 1].split(
                "https://management.azure.com", 1)[1].split("?", 1)[0]])}

    def test_control_plane_rejects_misassociation_public_access_and_unapproved_endpoints(self):
        baseline = copy.deepcopy(self.resources)
        mutations = [
            ("ruleAssociationId", ("dataCollectionRuleId",), "/other"),
            ("configurationAssociationId", ("dataCollectionEndpointId",), "/other"),
            ("dcrId", ("dataCollectionEndpointId",), "/other"),
            ("workspaceId", ("defaultIngestionSettings", "dataCollectionEndpointResourceId"), "/other"),
            ("workspaceId", ("publicNetworkAccess",), "Enabled"),
            ("dceId", ("networkAcls", "publicNetworkAccess"), "Enabled"),
            ("dcrId", ("dataFlows", 0, "destinations"), ["other"]),
            ("dcrId", ("dataFlows", 0, "streams"), ["other"]),
            ("amplsId", ("accessModeSettings", "ingestionAccessMode"), "Open"),
            ("amplsId", ("accessModeSettings", "queryAccessMode"), "Open"),
            ("amplsId", ("accessModeSettings", "exclusions"), [{}]),
        ]
        mutations += [
            (resource, path, value)
            for resource in ("monitorPrivateEndpointId", "queryPrivateEndpointId")
            for path, value in (
                (("provisioningState",), "Failed"),
                (("privateLinkServiceConnections",), []),
                (("privateLinkServiceConnections", 0, "properties", "privateLinkServiceId"), "/other"),
                (("privateLinkServiceConnections", 0, "properties", "groupIds"), ["other"]),
                (("privateLinkServiceConnections", 0, "properties", "privateLinkServiceConnectionState", "status"), "Pending"),
                (("subnet", "id"), "/other"),
                (("networkInterfaces",), []),
            )
        ]
        self.assertEqual(proofs.verify_control_plane(self.host, self.outputs)["status"], "verified")
        for resource, path, value in mutations:
            self.resources = copy.deepcopy(baseline)
            row = self.resources[self.outputs[resource]]
            for part in path[:-1]:
                row = row[part]
            row[path[-1]] = value
            with self.subTest(resource=resource, path=path), self.assertRaises(RuntimeError):
                proofs.verify_control_plane(self.host, self.outputs)
        for resource, path in (
                ("/amplsId/scopedResources/cluster-dce", ("linkedResourceId",)),
                ("/amplsId/scopedResources/workspace-dce", ("linkedResourceId",)),
                ("/monitor-nic", ("ipConfigurations", 0, "properties", "privateIPAddress")),
                ("/query-nic", ("ipConfigurations", 0, "properties", "privateIPAddress"))):
            self.resources = copy.deepcopy(baseline)
            row = self.resources[resource]
            for part in path[:-1]:
                row = row[part]
            row[path[-1]] = "20.1.1.1"
            with self.subTest(resource=resource), self.assertRaises(RuntimeError):
                proofs.verify_control_plane(self.host, self.outputs)

    def test_dns_requires_both_platforms_and_exact_endpoint_addresses(self):
        proof = proofs.verify_control_plane(self.host, self.outputs)
        pods = [{"metadata": {"name": name, "labels": labels}, "status": {"phase": "Running"}}
                for name, labels in (("linux", {"rsName": "ama-metrics"}),
                                     ("windows", {"dsName": "ama-metrics-win-node"}))]
        for case in ("valid", "public-linux", "wrong-pe", "empty", "public-windows", "missing-windows"):
            def kube(*args, **kwargs):
                if args[0] == "get":
                    return {"items": pods[:1] if case == "missing-windows" else pods}
                query = any("query.example" in a for a in args)
                ip = "10.225.0.5" if query else "10.225.0.4"
                if args[1] == "windows":
                    return "20.1.1.1\n" if case == "public-windows" else ip + "\n"
                if case == "empty":
                    return "Server: 10.0.0.10\nAddress: 10.0.0.10:53\n"
                if case == "public-linux":
                    ip += "\nAddress: 20.1.1.1"
                if case == "wrong-pe":
                    ip = "10.225.0.5" if not query else "10.225.0.4"
                return "Server: 10.0.0.10\nAddress: 10.0.0.10:53\nName: endpoint\nAddress: " + ip + "\n"
            self.host.kube.side_effect = kube
            with self.subTest(case=case):
                if case == "valid":
                    self.assertEqual(set(proofs.verify_private_dns(self.host, proof)), {"linux", "windows"})
                else:
                    with self.assertRaises(RuntimeError):
                        proofs.verify_private_dns(self.host, proof)

    def test_public_bypass_requires_authenticated_network_denial_on_both_paths(self):
        proof = proofs.verify_control_plane(self.host, self.outputs)
        for case in ("valid", "public-query", "public-ingest", "auth-403", "401", "network-200", "timeout",
                     "empty-token", "private-dns"):
            self.host.commands.run.reset_mock()
            def run(args, **kwargs):
                if args[0] == "az":
                    return "" if case == "empty-token" else "test-secret"
                self.assertEqual(kwargs["data"], "test-secret\n")
                self.assertNotIn("test-secret", " ".join(args))
                self.assertIn('--noproxy "*"', args[2])
                self.assertIn("--resolve", args[2])
                ingestion = "api/v1/write" in args[-1]
                if case == "timeout":
                    raise RuntimeError("timeout")
                if case == "public-query" and not ingestion or case == "public-ingest" and ingestion:
                    return '{"status":"success"}\n200'
                if case in ("auth-403", "401"):
                    return '{"error":"invalid audience"}\n' + ("401" if case == "401" else "403")
                if case == "network-200":
                    return '{"error":"Public network access is disabled"}\n200'
                return '{"error":"Public network access is disabled"}\n403'
            self.host.commands.run.side_effect = run
            with self.subTest(case=case), patch.object(proofs.socket, "getaddrinfo", return_value=[
                    (None, None, None, None, ("10.0.0.4" if case == "private-dns" else "20.1.1.1", 443))]):
                if case == "valid":
                    self.assertEqual(set(proofs.verify_public_denial(self.host, self.outputs, proof)),
                                     {"query", "ingestion"})
                else:
                    with self.assertRaises(RuntimeError):
                        proofs.verify_public_denial(self.host, self.outputs, proof)


def mutations():
    global proofs
    original = proofs
    source = SOURCE.read_text()
    targets = [
        'enabled is True',
        'protection.get("level") != "CanNotDelete"',
        'not same(rule.get("dataCollectionRuleId"), outputs["dcrId"])',
        'not same(configuration.get("dataCollectionEndpointId"), outputs["dceId"])',
        'not same(dcr.get("dataCollectionEndpointId"), outputs["dceId"])',
        'not same(workspace.get("defaultIngestionSettings", {}).get(\n'
        '                "dataCollectionEndpointResourceId"), outputs["workspaceDefaultDceId"])',
        'workspace.get("publicNetworkAccess") != "Disabled"',
        'dce.get("networkAcls", {}).get("publicNetworkAccess") != "Disabled"',
        'not any("Microsoft-PrometheusMetrics" in row.get("streams", [])\n'
        '               and set(row.get("destinations", [])) & names for row in dcr.get("dataFlows", []))',
        'modes.get("ingestionAccessMode") != "PrivateOnly"',
        'modes.get("queryAccessMode") != "PrivateOnly"',
        'modes.get("exclusions")',
        'not same(linked.get("linkedResourceId"), resource)',
        'endpoint.get("provisioningState") != "Succeeded"',
        'len(connections) != 1',
        'not same(connections[0]["properties"].get("privateLinkServiceId"), target)',
        'connections[0]["properties"].get("groupIds") != [group]',
        'connections[0]["properties"].get(\n'
        '                    "privateLinkServiceConnectionState", {}).get("status") != "Approved"',
        'not same(endpoint.get("subnet", {}).get("id"),\n'
        '                            outputs["vnetId"] + "/subnets/endpoints")',
        'not addresses or any(ipaddress.ip_address(ip) not in ipaddress.ip_network("10.225.0.0/24")\n'
        '                                for ip in addresses)',
        'set(selected) != set(host.scenario["platforms"])',
        'not addresses or not set(addresses) <= set(allowed)',
        'not addresses or any(not ipaddress.ip_address(ip).is_global for ip in addresses)',
        'not token',
        'status.strip() != "403"',
        'not re.search(\n'
        '                    r"public network|private[\\s-]+(?:link|endpoint)|network access", body, re.IGNORECASE)',
    ]
    changes = [(target, "False") for target in targets]
    changes.append((
        'host.az("resource", "show", "--ids", outputs["clusterId"],',
        'host.az("aks", "show", "--ids", outputs["clusterId"],'))
    for target, replacement in changes:
        if source.count(target) != 1:
            raise AssertionError("Mutation target changed: " + target)
        mutant = types.ModuleType("ampls_mutant")
        exec(compile(source.replace(target, replacement), str(SOURCE), "exec"), mutant.__dict__)
        proofs = mutant
        result = unittest.TextTestRunner(stream=io.StringIO()).run(
            unittest.defaultTestLoader.loadTestsFromTestCase(Proofs))
        proofs = original
        if result.wasSuccessful():
            raise AssertionError("Mutation survived: " + target)
        print("KILLED: " + target.splitlines()[0])


if __name__ == "__main__":
    mutate = "--mutations" in sys.argv
    if mutate:
        sys.argv.remove("--mutations")
    if not unittest.main(exit=False).result.wasSuccessful():
        raise SystemExit(1)
    if mutate:
        mutations()
