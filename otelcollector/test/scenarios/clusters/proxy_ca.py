"""Deliberately remove collector trust, prove failure, then restore and ingest."""

from datetime import datetime
import json
import re
import time
from urllib.parse import urlencode
from urllib.request import Request, urlopen


CERTIFICATE_ERROR = re.compile(
    r"certificate verify failed|certificate signed by unknown authority|SSL peer certificate|"
    r"unable to get local issuer|CERTIFICATE_VERIFY_FAILED|CERT_E_UNTRUSTEDROOT|0x800b0109",
    re.IGNORECASE)


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
            untrusted = [mount for mount in mounts if not mount["mountPath"].startswith("/anchors/")]
            if mounts == untrusted:
                raise RuntimeError("CA failure adapter found no collector trust anchors to remove")
            patch = {"op": "replace", "path": f"/spec/template/spec/containers/{index}/volumeMounts"}
            originals.append((resource, dict(patch, value=mounts)))
            host.kube("patch", resource, "-n", "kube-system", "--type=json",
                      "-p", json.dumps([dict(patch, value=untrusted)]), private=True)
        for resource, _ in originals:
            host.kube("rollout", "status", resource, "-n", "kube-system", "--timeout=5m")
        # A new marker cannot be satisfied by samples queued before CA removal.
        started = time.time()
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
            errors += len(CERTIFICATE_ERROR.findall(log))
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
