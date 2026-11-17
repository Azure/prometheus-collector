"""Deliberately remove collector trust, prove failure, then restore and ingest."""

from datetime import datetime
import json
import re
import time
from urllib.parse import urlencode
from urllib.request import Request, urlopen


DCE_CERTIFICATE_ERROR = re.compile(
    r'PublicationEndpoint:\s*"https://[^"]*\.metrics\.ingest\.monitor\.azure\.com[^"]*"'
    r'.*(?:certificate verify failed|certificate signed by unknown authority|SSL peer certificate|'
    r'unable to get local issuer|CERTIFICATE_VERIFY_FAILED|CERT_E_UNTRUSTEDROOT|0x800b0109|'
    r'Error in SSL handshake)', re.IGNORECASE)


def require_certificate_rejection(result):
    lines = str(result).splitlines()
    match = re.fullmatch(r"CURL_EXIT_CODE=(\d+)", lines[-1]) if lines else None
    if not match:
        raise RuntimeError(
            "Proxy CA distrust probe did not fail certificate verification (non-numeric curl result)")
    code = match.group(1)
    curl_stderr = "\n".join(lines[:-1]).strip()
    if code != "60":
        raise RuntimeError(
            f"Proxy CA distrust probe did not fail certificate verification (curl exit code {code}): "
            f"{curl_stderr or 'no curl stderr'}")
    return {"curlExitCode": int(code), "certificateRejected": True, "curlStderr": curl_stderr}


def failure_proof(scrapes, errors, observations, started):
    if not any(row["timestamp"] >= started and row["health"] == "up" for row in scrapes):
        raise RuntimeError("CA negative phase has no successful fresh collector scrape")
    if errors < 1:
        raise RuntimeError("CA negative phase has no collector certificate-validation error")
    if len(observations) < 3 or observations[-1]["time"] - observations[0]["time"] < 60:
        raise RuntimeError("CA negative phase did not observe a full minute without ingestion")
    if any(row["result"] for row in observations):
        raise RuntimeError("Samples ingested despite deliberate proxy CA distrust")
    return {"status": "PASS", "successfulScrapes": len(scrapes), "certificateErrors": errors,
            "absentIngestionObservations": len(observations),
            "observationSeconds": observations[-1]["time"] - observations[0]["time"]}


def verify_ca_recovery(host, outputs, mark_probe, verify_ingestion, verify_proxy_routes):
    originals = []
    marker = host.args.run_id + "-untrusted-ca"
    collectors = [
        pod for pod in host.kube("get", "pods", "-n", "kube-system", "-o", "json",
                                 json_output=True)["items"]
        if any(container["name"] == "prometheus-collector"
               for container in pod["spec"]["containers"])
    ]
    if not collectors:
        raise RuntimeError("No running collector pods to verify the MetricsExtension override")
    config_proof = []
    for pod in collectors:
        owners = pod["metadata"].get("ownerReferences", [])
        if any(owner.get("kind") == "ReplicaSet" for owner in owners):
            config_file = "me.config"
        elif any(owner.get("kind") == "DaemonSet" for owner in owners):
            config_file = "me_ds.config"
        else:
            raise RuntimeError("Cannot determine the MetricsExtension config file for a collector pod")
        config_path = f"/usr/sbin/{config_file}"
        config = host.kube(
            "exec", pod["metadata"]["name"], "-n", "kube-system", "-c", "prometheus-collector",
            "--", "cat", config_path)
        try:
            overrides = json.loads(config)
        except (TypeError, json.JSONDecodeError) as error:
            raise RuntimeError("MetricsExtension config override is not valid JSON") from error
        if overrides.get("validateCertificates") != 2:
            raise RuntimeError("MetricsExtension proxy certificate validation override is not active")
        config_proof.append({"pod": pod["metadata"]["name"], "configOverridesPath": config_path,
                             "validateCertificates": 2})
    print("MetricsExtension proxy certificate validation: " + json.dumps({
        "scope": "proxy-custom-ca", "collectors": config_proof,
    }), flush=True)

    token = host.commands.run([
        "az", "account", "get-access-token", "--resource", "https://prometheus.monitor.azure.com",
        "--query", "accessToken", "-o", "tsv"], private=True).strip()
    if not token:
        raise RuntimeError("CA negative probe requires an authenticated independent AMW reader")
    proxy_endpoint = outputs.get("proxyEndpoint")
    if not proxy_endpoint:
        raise RuntimeError("CA negative probe requires the owned proxy endpoint")
    query = outputs["queryEndpoint"].rstrip("/") + "/api/v1/query?" + urlencode({
        "query": f'myapp_temperature{{scenario_run="{marker}"}}',
    })
    started = time.time()
    try:
        for resource in ("deployment/ama-metrics", "daemonset/ama-metrics-node"):
            document = host.kube("get", resource, "-n", "kube-system", "-o", "json", json_output=True)
            containers = document["spec"]["template"]["spec"]["containers"]
            index = next(i for i, container in enumerate(containers) if container["name"] == "prometheus-collector")
            mounts = containers[index].get("volumeMounts", [])
            trust_mounts = {"scenario-proxy-ca", "anchors-mariner"}
            if not trust_mounts.issubset({mount.get("name") for mount in mounts}):
                raise RuntimeError("CA failure adapter is missing an owned or node proxy CA trust mount")
            # AKS also exposes its trustedCa through this node anchor mount.
            untrusted = [mount for mount in mounts if mount.get("name") not in trust_mounts]
            patch = {"op": "replace", "path": f"/spec/template/spec/containers/{index}/volumeMounts"}
            originals.append((resource, dict(patch, value=mounts)))
            host.kube("patch", resource, "-n", "kube-system", "--type=json",
                      "-p", json.dumps([dict(patch, value=untrusted)]), private=True)
        for resource, _ in originals:
            host.kube("rollout", "status", resource, "-n", "kube-system", "--timeout=5m")
        # A new marker cannot be satisfied by samples queued before CA removal.
        started = time.time()
        ca_probes = []
        trust_anchor = "/etc/pki/ca-trust/source/anchors/proxy-cert.crt"
        trust_bundle = "/etc/pki/tls/certs/ca-bundle.crt"
        untrusted_pods = host.kube(
            "get", "pods", "-n", "kube-system", "-o", "json", json_output=True)["items"]
        for pod in untrusted_pods:
            collector = next((container for container in pod["spec"]["containers"]
                              if container["name"] == "prometheus-collector"), None)
            if collector is None:
                continue
            if any(mount.get("name") == "scenario-proxy-ca"
                   for mount in collector.get("volumeMounts", [])):
                raise RuntimeError("Collector still mounts the owned proxy CA during distrust")
            reset_trust = (
                f'set -e; rm -f -- {trust_anchor}; update-ca-trust; '
                f'if [ -e {trust_anchor} ]; then echo PROXY_CA_ANCHOR_REMAINS; exit 1; fi; '
                'anchors=$(trust list --filter=ca-anchors); '
                'if printf "%s\\n" "$anchors" | grep -Fq "Retained AKS scenario proxy CA"; then '
                'echo PROXY_CA_STILL_TRUSTED; exit 1; fi; '
                'echo PROXY_CA_TRUST_RESET_PASS')
            host.kube(
                "exec", pod["metadata"]["name"], "-n", "kube-system", "-c", "prometheus-collector",
                "--", "sh", "-c", reset_trust, "scenario-ca-trust-reset")
            result = host.kube(
                "exec", pod["metadata"]["name"], "-n", "kube-system", "-c", "prometheus-collector",
                "--", "sh", "-c",
                'set +e; curl --noproxy "" --proxy "$1" --connect-timeout 10 --max-time 30 '
                f'--cacert {trust_bundle} --silent --show-error --output /dev/null "$2" 2>&1; '
                'code=$?; printf "\\nCURL_EXIT_CODE=%s\\n" "$code"',
                "scenario-ca-probe", proxy_endpoint, outputs["dceMetricsIngestionEndpoint"])
            ca_probes.append({"pod": pod["metadata"]["name"],
                              "trustAnchorCleared": True, "proxyCaAbsentFromTrustList": True,
                              **require_certificate_rejection(result)})
        if not ca_probes:
            raise RuntimeError("No collectors available for the proxy CA distrust probe")
        print("Proxy CA distrust probe: " + json.dumps({
            "status": "PASS", "scope": "proxy-custom-ca", "trustAnchor": trust_anchor,
            "trustBundle": trust_bundle, "collectors": ca_probes,
        }), flush=True)
        mark_probe(host, marker)
        time.sleep(90)
        observations = []
        for attempt in range(3):
            with urlopen(Request(query, headers={"Authorization": "Bearer " + token}), timeout=60) as response:
                result = json.load(response)
            if result.get("status") != "success" or result.get("data", {}).get("resultType") != "vector":
                raise RuntimeError("Independent AMW query did not succeed during CA distrust")
            observations.append({"time": time.time(), "result": result["data"]["result"]})
            if attempt < 2:
                time.sleep(35)
        proxy_negative = verify_proxy_routes(host, outputs, started, phase="untrusted-ca")
        pods = host.kube("get", "pods", "-n", "kube-system", "-o", "json", json_output=True)["items"]
        scrapes, errors = [], 0
        for pod in pods:
            if not any(c["name"] == "prometheus-collector" for c in pod["spec"]["containers"]):
                continue
            name = pod["metadata"]["name"]
            targets = host.kube(
                "exec", name, "-n", "kube-system", "-c", "prometheus-collector", "--",
                "curl", "--fail", "--silent", "--show-error", "--noproxy", "*", "--max-time", "30",
                "http://localhost:9090/api/v1/targets", json_output=True, private=True)
            for target in targets.get("data", {}).get("activeTargets", []):
                if "scenario-network-probe" in target.get("scrapePool", ""):
                    scrapes.append({"health": target["health"],
                                    "timestamp": datetime.fromisoformat(target["lastScrape"].replace("Z", "+00:00")).timestamp()})
            log = host.kube("exec", name, "-n", "kube-system", "-c", "prometheus-collector",
                            "--", "cat", "/MetricsExtensionConsoleDebugLog.log", private=True)
            errors += sum(1 for line in log.splitlines()
                          if DCE_CERTIFICATE_ERROR.search(line))
        proof = failure_proof(scrapes, errors, observations, started)
        proof["proxy"] = proxy_negative
        print("Collector CA distrust proof: " + json.dumps(proof), flush=True)
    finally:
        deadline, host.commands.deadline = host.commands.deadline, None
        try:
            errors = []
            for resource, patch in originals:
                try:
                    host.kube("patch", resource, "-n", "kube-system", "--type=json",
                              "-p", json.dumps([patch]), private=True)
                    host.kube("rollout", "status", resource, "-n", "kube-system", "--timeout=10m")
                    document = host.kube("get", resource, "-n", "kube-system", "-o", "json", json_output=True)
                    index = int(patch["path"].split("/")[-2])
                    actual = document["spec"]["template"]["spec"]["containers"][index]["volumeMounts"]
                    if actual != patch["value"]:
                        raise RuntimeError("Collector proxy CA trust mounts were not restored exactly")
                except (RuntimeError, ValueError, KeyError) as error:
                    errors.append(f"{resource}: {error}")
            if errors:
                raise RuntimeError("Collector CA restoration failed: " + "; ".join(errors))
        finally:
            host.commands.deadline = deadline
    recovered = time.time()
    mark_probe(host, host.args.run_id)
    recovery_ingestion = verify_ingestion(host, outputs, recovered)
    recovery_proxy = verify_proxy_routes(host, outputs, recovered, phase="recovered-ca")
    proof["recovery"] = {"ingestion": recovery_ingestion, "proxy": recovery_proxy}
    (host.args.evidence / "proxy-ca-proof.json").write_text(json.dumps(proof, indent=2) + "\n")
    print("Collector CA recovery proof: " + json.dumps(proof), flush=True)
    return proof
