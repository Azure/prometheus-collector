"""Independent AMPLS control-plane, DNS and public-bypass checks."""

import ipaddress
import json
import re
import socket
from urllib.parse import urlsplit


def initialize(host, outputs, enabled):
    # AKS CLI disable deletes DCRs/DCRAs still owned by this private-link fixture.
    if enabled is True:
        host.az("resource", "update", "--ids", outputs["clusterId"], "--api-version", "2024-10-01",
                "--set", "properties.azureMonitorProfile.metrics.enabled=false")
    host.az("resource", "update", "--ids", outputs["dceId"], "--api-version", "2023-03-11",
            "--set", "properties.networkAcls.publicNetworkAccess=Disabled")
    for association, field, target in (
            ("ruleAssociationId", "dataCollectionRuleId", "dcrId"),
            ("configurationAssociationId", "dataCollectionEndpointId", "dceId")):
        host.az("rest", "--method", "put", "--url",
                f"https://management.azure.com{outputs[association]}?api-version=2023-03-11",
                "--body", json.dumps({"properties": {field: outputs[target]}}))


def verify_control_plane(host, outputs):
    def get(resource, version):
        return host.az("rest", "--method", "get", "--url",
                       f"https://management.azure.com{resource}?api-version={version}",
                       json_output=True)["properties"]

    def same(actual, expected):
        return isinstance(actual, str) and actual.casefold() == expected.casefold()

    dce = get(outputs["dceId"], "2023-03-11")
    dcr = get(outputs["dcrId"], "2023-03-11")
    rule = get(outputs["ruleAssociationId"], "2023-03-11")
    configuration = get(outputs["configurationAssociationId"], "2023-03-11")
    workspace = get(outputs["workspaceId"], "2023-04-03")
    if (not same(rule.get("dataCollectionRuleId"), outputs["dcrId"])
            or not same(configuration.get("dataCollectionEndpointId"), outputs["dceId"])
            or not same(dcr.get("dataCollectionEndpointId"), outputs["dceId"])
            or not same(workspace.get("defaultIngestionSettings", {}).get(
                "dataCollectionEndpointResourceId"), outputs["workspaceDefaultDceId"])
            or workspace.get("publicNetworkAccess") != "Disabled"
            or dce.get("networkAcls", {}).get("publicNetworkAccess") != "Disabled"):
        raise RuntimeError("AMPLS requires the exact DCR/DCE associations and disabled public access")
    destinations = dcr.get("destinations", {}).get("monitoringAccounts", [])
    names = {row["name"] for row in destinations
             if same(row.get("accountResourceId"), outputs["workspaceId"])}
    if not any("Microsoft-PrometheusMetrics" in row.get("streams", [])
               and set(row.get("destinations", [])) & names for row in dcr.get("dataFlows", [])):
        raise RuntimeError("AMPLS DCR must route Prometheus metrics to this workspace")
    scope = get(outputs["amplsId"], "2021-07-01-preview")
    modes = scope.get("accessModeSettings", {})
    if (modes.get("ingestionAccessMode") != "PrivateOnly"
            or modes.get("queryAccessMode") != "PrivateOnly" or modes.get("exclusions")):
        raise RuntimeError("AMPLS ingestion and query must be PrivateOnly without exclusions")
    for name, resource in (("cluster-dce", outputs["dceId"]),
                           ("workspace-dce", outputs["workspaceDefaultDceId"])):
        linked = get(outputs["amplsId"] + "/scopedResources/" + name, "2021-07-01-preview")
        if not same(linked.get("linkedResourceId"), resource):
            raise RuntimeError("AMPLS is missing an independently verified scoped DCE")
    endpoints = {}
    for kind, resource, target, group in (
            ("monitor", outputs["monitorPrivateEndpointId"], outputs["amplsId"], "azuremonitor"),
            ("query", outputs["queryPrivateEndpointId"], outputs["workspaceId"], "prometheusMetrics")):
        endpoint = get(resource, "2024-05-01")
        connections = endpoint.get("privateLinkServiceConnections", [])
        if (endpoint.get("provisioningState") != "Succeeded" or len(connections) != 1
                or not same(connections[0]["properties"].get("privateLinkServiceId"), target)
                or connections[0]["properties"].get("groupIds") != [group]
                or connections[0]["properties"].get(
                    "privateLinkServiceConnectionState", {}).get("status") != "Approved"
                or not same(endpoint.get("subnet", {}).get("id"),
                            outputs["vnetId"] + "/subnets/endpoints")):
            raise RuntimeError(f"AMPLS {kind} endpoint must be approved on the owned VNet")
        addresses = {
            row["properties"]["privateIPAddress"]
            for nic in endpoint.get("networkInterfaces", [])
            for row in get(nic["id"], "2024-05-01").get("ipConfigurations", [])
        }
        if not addresses or any(ipaddress.ip_address(ip) not in ipaddress.ip_network("10.225.0.0/24")
                                for ip in addresses):
            raise RuntimeError(f"AMPLS {kind} endpoint has no owned private NIC addresses")
        endpoints[kind] = sorted(addresses)
    return {
        "status": "verified", "dcrId": outputs["dcrId"], "dceId": outputs["dceId"],
        "ruleAssociationId": outputs["ruleAssociationId"],
        "configurationAssociationId": outputs["configurationAssociationId"],
        "accessModeSettings": modes, "endpointAddresses": endpoints,
        "hosts": {
            "configuration": urlsplit(dce["configurationAccess"]["endpoint"]).hostname,
            "ingestion": urlsplit(dce["metricsIngestion"]["endpoint"]).hostname,
            "query": urlsplit(outputs["queryEndpoint"]).hostname,
        },
        "immutableDcrId": dcr["immutableId"],
    }


def verify_private_dns(host, proof):
    pods = host.kube("get", "pods", "-n", "kube-system", "-o", "json", json_output=True)["items"]
    selected = {}
    for pod in pods:
        labels = pod["metadata"].get("labels", {})
        kind = ("linux" if labels.get("rsName") == "ama-metrics"
                else "windows" if labels.get("dsName") == "ama-metrics-win-node" else None)
        if kind and pod.get("status", {}).get("phase") == "Running":
            selected.setdefault(kind, pod["metadata"]["name"])
    if set(selected) != set(host.scenario["platforms"]):
        raise RuntimeError("AMPLS DNS proof requires a running collector on each selected platform")
    observed = {}
    for platform, pod in selected.items():
        observed[platform] = {}
        for kind, hostname in proof["hosts"].items():
            if not hostname or not re.fullmatch(r"[a-zA-Z0-9.-]+", hostname):
                raise RuntimeError("AMPLS endpoint hostname is missing or invalid")
            if platform == "windows":
                command = ["powershell", "-NoProfile", "-NonInteractive", "-Command",
                           f"[System.Net.Dns]::GetHostAddresses('{hostname}') | "
                           "ForEach-Object { $_.IPAddressToString }"]
                addresses = host.kube("exec", pod, "-n", "kube-system", "-c",
                                      "prometheus-collector", "--", *command).split()
            else:
                answer = host.kube("exec", pod, "-n", "kube-system", "-c",
                                   "prometheus-collector", "--", "/usr/sbin/busybox", "nslookup", hostname)
                # Ignore the DNS server's address, which precedes the first answer name.
                addresses = re.findall(r"^Address(?: \d+)?:\s*(\S+)", answer.split("Name:", 1)[-1],
                                       re.MULTILINE) if "Name:" in answer else []
            allowed = proof["endpointAddresses"]["query" if kind == "query" else "monitor"]
            if not addresses or not set(addresses) <= set(allowed):
                raise RuntimeError(f"AMPLS {platform} {kind} DNS does not resolve exclusively to its approved PE")
            observed[platform][kind] = {"host": hostname, "addresses": sorted(set(addresses))}
    return observed


def verify_public_denial(host, outputs, proof):
    evidence = {}
    for kind, audience, url in (
            ("query", "https://prometheus.monitor.azure.com",
             outputs["queryEndpoint"].rstrip("/") + "/api/v1/query?query=up"),
            ("ingestion", "https://monitor.azure.com",
             "https://" + proof["hosts"]["ingestion"] + "/dataCollectionRules/"
             + proof["immutableDcrId"] + "/streams/Microsoft-PrometheusMetrics/api/v1/write?api-version=2023-04-24")):
        hostname = urlsplit(url).hostname
        addresses = sorted({row[4][0] for row in socket.getaddrinfo(hostname, 443, type=socket.SOCK_STREAM)})
        if not addresses or any(not ipaddress.ip_address(ip).is_global for ip in addresses):
            raise RuntimeError("AMPLS public-bypass proof requires a public-DNS orchestrator")
        token = host.commands.run(["az", "account", "get-access-token", "--resource", audience,
                                   "--query", "accessToken", "-o", "tsv"], private=True).strip()
        if not token:
            raise RuntimeError("AMPLS public-bypass probe requires an authenticated token")
        script = (
            'IFS= read -r token; '
            'curl --noproxy "*" --silent --show-error --max-time 60 '
            '-H "Authorization: Bearer $token" --resolve "$1" -w "\\n%{http_code}" "$2"'
        )
        if kind == "ingestion":
            # Empty Snappy WriteRequest; no synthetic metric is written by the negative probe.
            script = script.replace("curl --", "printf '\\000' | curl --", 1) + (
                ' -H "Content-Type: application/x-protobuf" -H "Content-Encoding: snappy" '
                '-H "X-Prometheus-Remote-Write-Version: 0.1.0" --data-binary @-'
            )
        attempts = []
        for address in addresses:
            resolve = f"{hostname}:443:[{address}]" if ":" in address else f"{hostname}:443:{address}"
            response = host.commands.run(["bash", "-c", script, "public-bypass", resolve, url],
                                         data=token + "\n", private=True)
            body, status = response.rsplit("\n", 1)
            if (status.strip() != "403" or not re.search(
                    r"public network|private[\s-]+(?:link|endpoint)|network access", body, re.IGNORECASE)):
                raise RuntimeError(f"AMPLS public {kind} bypass did not return an explicit network denial: "
                                   f"HTTP {status.strip()} {body[:1000]}")
            attempts.append({"address": address, "httpStatus": 403, "body": body})
        evidence[kind] = {"host": hostname, "origin": "public-DNS pipeline orchestrator", "attempts": attempts}
    return evidence


def verify(host, outputs):
    protection = host.az("lock", "show", "--name", "scenario-retain",
                         "--resource-group", host.receipt["resourceGroup"], json_output=True)
    if protection.get("level") != "CanNotDelete":
        raise RuntimeError("AMPLS retained resource group has no confirmed deletion lock")
    print("AMPLS_DELETION_PROTECTION " + json.dumps(protection), flush=True)
    proof = verify_control_plane(host, outputs)
    proof["deletionProtection"] = protection
    proof["privateDns"] = verify_private_dns(host, proof)
    proof["publicBypass"] = verify_public_denial(host, outputs, proof)
    (host.args.evidence / "ampls.json").write_text(json.dumps(proof, indent=2) + "\n", encoding="utf-8")
    print("AMPLS_PROOF " + json.dumps(proof), flush=True)
    return proof
