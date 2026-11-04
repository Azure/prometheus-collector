"""Compare matched Linux Trivy scans; never claim Windows or Qualys/QID closure."""
import argparse
import hashlib
import json
from pathlib import Path
import re
from datetime import datetime, timedelta, timezone

import normalize


def load(directory, now):
    metadata = normalize.read_json(directory / "trivy" / "metadata.json")
    raw = (directory / "trivy" / "result.json").read_bytes()
    data = json.loads(raw)
    if (metadata["exitCode"] != 0
            or hashlib.sha256(raw).hexdigest() != metadata["resultSha256"]
            or metadata["requestedRef"] not in data["Metadata"]["RepoDigests"]):
        raise ValueError("Unbound or unsuccessful Trivy scan")
    if not re.fullmatch(r"[^\s@]+@sha256:[0-9a-f]{64}", metadata["requestedRef"]):
        raise ValueError("Expected child digest")
    config = data["Metadata"]["ImageConfig"]
    if metadata["platform"] not in ("linux/amd64", "linux/arm64") or metadata["platform"] != config["os"] + "/" + config["architecture"]:
        raise ValueError("Linux scan platform mismatch")
    for field in ("dbUpdatedAt", "finishedUtc"):
        if not now - timedelta(hours=24) <= normalize.utc(metadata[field]) <= now + timedelta(minutes=5):
            raise ValueError("Stale database or scan")
    if not metadata["version"] or not re.fullmatch(r"[0-9a-f]{64}", metadata["dbSha256"]):
        raise ValueError("Missing scanner/database identity")
    expected_config = {
        "scanners": ["vuln"], "listAllPkgs": True,
        "ignoreSha256": hashlib.sha256(b"").hexdigest(),
        "configSha256": hashlib.sha256(b"{}\n").hexdigest(),
    }
    if metadata["configuration"] != expected_config or not data["Results"]:
        raise ValueError("Incomplete or noncanonical scanner configuration")
    lane = {"name": metadata["platform"], "scanner": "trivy", "status": "COMPLETE",
            "version": metadata["version"], "dbUpdatedAt": metadata["dbUpdatedAt"]}
    return metadata, data, {"lanes": [lane], "scanVerdict": "PASS", "verdict": "PASS",
                           "findings": normalize.trivy_findings(data, metadata["platform"], metadata["requestedRef"].split("@")[1])}


def compare(baseline, candidate, receipt, requested, now=None):
    normalize.validate_targets(receipt)
    if not requested or any(not re.fullmatch(r"CVE-[0-9]{4}-[0-9]+", cve) for cve in requested):
        raise ValueError("Explicit selected CVEs required")
    now = now or datetime.now(timezone.utc)
    before_meta, before, previous = load(baseline, now)
    after_meta, after, current = load(candidate, now)
    if receipt["buildResult"] != "succeeded" or not any(
            image["ref"] == after_meta["requestedRef"]
            and image["os"] + "/" + image["arch"] == after_meta["platform"] for image in receipt["images"]):
        raise ValueError("Candidate scan is not bound to the successful build receipt")
    for field in ("version", "dbUpdatedAt", "dbSha256", "platform", "configuration"):
        if before_meta[field] != after_meta[field]:
            raise ValueError(f"Incomparable Trivy evidence: {field}")
    coverage = lambda data: {(r["Target"], r.get("Class"), r.get("Type")) for r in data["Results"]}
    if coverage(before) != coverage(after):
        raise ValueError("Analyzed target/type coverage changed")
    normalize.compare_findings(current, previous)
    inventory = {(r["Target"], p["Name"]) for r in after["Results"] for p in r.get("Packages", [])}
    selected = []
    for cve in sorted(set(requested)):
        prior = [f for f in previous["findings"] if f["id"] == cve]
        present = any(f["id"] == cve for f in current["findings"])
        covered = bool(prior) and all((f["path"], f["package"]) in inventory for f in prior)
        selected.append({"id": cve, "status": "OPEN" if present else "CLOSED" if covered else "NOT_REPORTED"})
    verdict = ("BLOCKED" if any(r["status"] == "NOT_REPORTED" for r in selected)
               else "FAIL" if current["newFindings"] or any(r["status"] == "OPEN" for r in selected) else "PASS")
    return {
        "schemaVersion": 1, "scope": "selected Linux Trivy CVEs; not Windows/Qualys clearance",
        "source": receipt["source"], "baseline": before_meta, "candidate": after_meta,
        "requested": selected, "verdict": verdict, "newFindings": current["newFindings"],
        "remainingFindings": current["findings"],
    }


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    for name in ("baseline", "candidate", "receipt", "output"):
        parser.add_argument("--" + name, required=True, type=Path)
    parser.add_argument("--cve", required=True, action="append")
    args = parser.parse_args()
    result = compare(args.baseline, args.candidate, normalize.read_json(args.receipt), args.cve)
    with args.output.open("x", encoding="utf-8") as output:
        json.dump(result, output, indent=2)
        output.write("\n")
    return {"PASS": 0, "FAIL": 1, "BLOCKED": 2}[result["verdict"]]


if __name__ == "__main__":
    raise SystemExit(main())
