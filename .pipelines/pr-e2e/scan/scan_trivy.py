"""Resolve, scan and compare immutable Linux images with Trivy."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import sys
import time
from datetime import datetime, timedelta, timezone
from urllib.error import HTTPError
from urllib.request import Request, urlopen


REPOSITORY = "mcr.microsoft.com/azuremonitor/containerinsights/cidev/prometheus-collector/images"
SEVERITIES = ("HIGH", "CRITICAL", "MEDIUM")


def registry_json(kind, reference):
    headers = {"Accept": ", ".join((
        "application/vnd.oci.image.index.v1+json",
        "application/vnd.oci.image.manifest.v1+json",
        "application/vnd.docker.distribution.manifest.list.v2+json",
        "application/vnd.docker.distribution.manifest.v2+json",
    ))}
    request = Request(f"https://{REPOSITORY.split('/', 1)[0]}/v2/"
                      f"{REPOSITORY.split('/', 1)[1]}/{kind}/{reference}", headers=headers)
    with urlopen(request, timeout=60) as response:
        raw = response.read()
    actual = "sha256:" + hashlib.sha256(raw).hexdigest()
    if reference.startswith("sha256:") and actual != reference:
        raise ValueError("Registry bytes do not match the requested digest")
    return json.loads(raw), actual


def resolve_tag(name, tag):
    if not re.fullmatch(r"(?:[a-zA-Z0-9_][a-zA-Z0-9_.-]{0,127}|sha256:[0-9a-f]{64})", tag):
        raise ValueError("Expected a published image tag or manifest digest")
    for attempt in range(12):
        try:
            root, digest = registry_json("manifests", tag)
            index_ref = REPOSITORY + "@" + digest
            images = []
            for child in root.get("manifests", [{"digest": digest}]):
                descriptor = child.get("platform", {})
                if (descriptor.get("os") == "unknown"
                        and child.get("annotations", {}).get("vnd.docker.reference.type") == "attestation-manifest"):
                    continue
                if not re.fullmatch(r"sha256:[0-9a-f]{64}", child["digest"]):
                    raise ValueError("Expected immutable child manifest digest")
                manifest = root if child["digest"] == digest else registry_json("manifests", child["digest"])[0]
                config, _ = registry_json("blobs", manifest["config"]["digest"])
                for key in ("os", "architecture"):
                    if key in descriptor and descriptor[key] != config[key]:
                        raise ValueError("Index platform does not match child configuration")
                image = {"name": name, "ref": REPOSITORY + "@" + child["digest"],
                         "indexRef": index_ref, "os": config["os"], "arch": config["architecture"]}
                if image["os"] == "windows":
                    image["osVersion"] = config["os.version"]
                    if descriptor.get("os.version") not in (None, image["osVersion"]):
                        raise ValueError("Windows index/config OS version mismatch")
                images.append(image)
            return images
        except HTTPError as error:
            if error.code not in (404, 429, 502, 503, 504) or attempt == 11:
                raise
            print(f"MCR publication unavailable ({error.code}); retrying image lookup", file=sys.stderr, flush=True)
            time.sleep(15)


def utc(value):
    value = value.replace("Z", "+00:00")
    value = re.sub(r"(?<=\.\d{6})\d+", "", value)
    parsed = datetime.fromisoformat(value)
    if parsed.tzinfo is None:
        raise ValueError("Timestamp must include timezone")
    return parsed.astimezone(timezone.utc)


def scan(image, platform, output, database_from=None):
    if not re.fullmatch(r"[^\s@]+@sha256:[0-9a-f]{64}", image):
        raise ValueError("An immutable child image digest is required")
    if platform not in ("linux/amd64", "linux/arm64"):
        raise ValueError("This discovery lane covers Linux only; not Windows OS/QID clearance")
    output.mkdir(parents=True, exist_ok=False)
    trivy = output / "trivy"
    trivy.mkdir()
    ignore = output / "empty.ignore"
    ignore.write_bytes(b"")
    config = output / "empty.yaml"
    config.write_bytes(b"{}\n")
    database_repositories = {"TRIVY_DB_REPOSITORY", "TRIVY_JAVA_DB_REPOSITORY"}
    environment = {
        key: value for key, value in os.environ.items()
        if not key.upper().startswith("TRIVY_") or key.upper() in database_repositories
    }
    command = [
        "trivy", "--config", str(config), "--cache-dir", str(output / "cache"),
        "image", "--image-src", "remote", "--platform", platform,
        "--scanners", "vuln,secret", "--list-all-pkgs", "--ignorefile", str(ignore),
        "--ignore-unfixed", "--severity", ",".join(SEVERITIES),
        "--format", "json", "--exit-code", "0", "--no-progress",
        "--timeout", "20m", "--output", str(trivy / "result.json"), image,
    ]
    if database_from is not None:
        prior = json.loads((database_from / "trivy" / "metadata.json").read_text(encoding="utf-8-sig"))
        database = database_from / "cache" / "db"
        if hashlib.sha256((database / "trivy.db").read_bytes()).hexdigest() != prior["dbSha256"]:
            raise ValueError("Baseline database snapshot hash mismatch")
        shutil.copytree(database, output / "cache" / "db")
        command.insert(-1, "--skip-db-update")
    started = datetime.now(timezone.utc).isoformat()
    for attempt in range(5):
        with (trivy / "scan.log").open("w", encoding="utf-8") as log:
            result = subprocess.run(command, env=environment, stdout=log, stderr=subprocess.STDOUT, check=False)
        retryable = b"TOOMANYREQUESTS" in (trivy / "scan.log").read_bytes()
        if result.returncode == 0 or attempt == 4 or not retryable:
            break
        print(f"Error: Too many requests to the Trivy server. Retrying ({attempt + 1}/5)", file=sys.stderr, flush=True)
        time.sleep(5)
    metadata = {
        "exitCode": result.returncode, "startedUtc": started,
        "finishedUtc": datetime.now(timezone.utc).isoformat(),
        "requestedRef": image, "platform": platform, "command": command,
        "scope": "discovery-only; not CoMET/QID or full-image clearance",
    }
    (trivy / "metadata.json").write_text(json.dumps(metadata, indent=2) + "\n", encoding="utf-8")
    if result.returncode:
        raise ValueError(f"Trivy exited {result.returncode}; diagnostics retained in {trivy}")
    version = subprocess.run(
        ["trivy", "--cache-dir", str(output / "cache"), "--version", "--format", "json"],
        env=environment, capture_output=True, text=True, check=True,
    )
    details = json.loads(version.stdout)
    metadata.update(version=details["Version"], dbUpdatedAt=details["VulnerabilityDB"]["UpdatedAt"])
    metadata["dbSha256"] = hashlib.sha256((output / "cache" / "db" / "trivy.db").read_bytes()).hexdigest()
    metadata["configuration"] = {
        "scanners": ["vuln", "secret"], "listAllPkgs": True,
        "ignoreUnfixed": True, "severity": list(SEVERITIES),
        "ignoreSha256": hashlib.sha256(ignore.read_bytes()).hexdigest(),
        "configSha256": hashlib.sha256(config.read_bytes()).hexdigest(),
    }
    database_time = utc(metadata["dbUpdatedAt"])
    now = datetime.now(timezone.utc)
    if not now - timedelta(hours=24) <= database_time <= now + timedelta(minutes=5):
        raise ValueError("Trivy database freshness is not established")
    data = json.loads((trivy / "result.json").read_text(encoding="utf-8-sig"))
    if image not in data["Metadata"]["RepoDigests"]:
        raise ValueError("Trivy result does not match the requested child digest")
    actual_os, actual_arch = platform.split("/")
    config_data = data["Metadata"]["ImageConfig"]
    if config_data["os"] != actual_os or config_data["architecture"] != actual_arch:
        raise ValueError("Trivy result platform mismatch")
    metadata["resultSha256"] = hashlib.sha256((trivy / "result.json").read_bytes()).hexdigest()
    (trivy / "metadata.json").write_text(json.dumps(metadata, indent=2) + "\n", encoding="utf-8")
    return data


def scan_published_image(image, platform, output):
    prefix = "containerinsightsprod.azurecr.io/public/azuremonitor/containerinsights/cidev/prometheus-collector/images:"
    if not image.startswith(prefix):
        raise ValueError("Expected this pipeline's CI/dev image")
    candidates = [entry for entry in resolve_tag("collector", image.removeprefix(prefix))
                  if entry["os"] + "/" + entry["arch"] == platform]
    if len(candidates) != 1:
        raise ValueError(f"Expected exactly one {platform} collector child")
    output.mkdir(parents=True, exist_ok=False)
    work = output / "scan-work"
    scan(candidates[0]["ref"], platform, work)
    subprocess.run(
        ["trivy", "convert", "--format", "table", str(work / "trivy" / "result.json")],
        check=True,
    )
    shutil.move(work / "trivy", output / "trivy")
    (output / "cache").mkdir()
    shutil.move(work / "cache" / "db", output / "cache" / "db")
    shutil.rmtree(work)


DEFERRED_TARGETS = frozenset({
    "opt/microsoft/otelcollector/otelcollector",
    "opt/microsoft/otelcollector/prometheusui",
    "opt/promconfigvalidator",
})


class StaleScanError(ValueError):
    pass


def load(directory, now, *, binary_targets=False):
    metadata = json.loads((directory / "trivy" / "metadata.json").read_text(encoding="utf-8-sig"))
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
    if not metadata["version"] or not re.fullmatch(r"[0-9a-f]{64}", metadata["dbSha256"]):
        raise ValueError("Missing scanner/database identity")
    expected_config = {
        "scanners": ["vuln", "secret"], "listAllPkgs": True,
        "ignoreUnfixed": True, "severity": ["HIGH", "CRITICAL", "MEDIUM"],
        "ignoreSha256": hashlib.sha256(b"").hexdigest(),
        "configSha256": hashlib.sha256(b"{}\n").hexdigest(),
    }
    vulnerability_results = [item for item in data["Results"]
                             if item.get("Class") in ("os-pkgs", "lang-pkgs")]
    if metadata["configuration"] != expected_config or not vulnerability_results:
        raise ValueError("Incomplete or noncanonical scanner configuration")
    for result in vulnerability_results:
        if result.get("Class") == "os-pkgs":
            os = data["Metadata"]["OS"]
            target = f'{os["Family"]} {os["Name"]}'
            if (result["Target"] != f'{metadata["requestedRef"]} ({target})'
                    or result.get("Type") != os["Family"]):
                raise ValueError("OS target is not bound to the scanned image and distribution")
            # Trivy embeds the changing image ref in OS targets, unlike binary paths.
            result["Target"] = "os:" + target
    findings = [
        {"lane": metadata["platform"], "digest": metadata["requestedRef"].split("@")[1],
         "scanner": "trivy", "id": vuln["VulnerabilityID"], "package": vuln["PkgName"],
         "installed": vuln["InstalledVersion"], "fixed": vuln.get("FixedVersion") or None,
         "path": (item["Target"] if binary_targets and item.get("Type") == "gobinary"
                  else vuln.get("PkgPath") or item["Target"]), "evidence": None,
         **({"binaryTarget": item["Target"]} if binary_targets and item.get("Type") == "gobinary" else {}),
         "severity": vuln["Severity"], "cves": None, "enrichment": "supplemental",
         "fixAvailable": bool(vuln.get("FixedVersion"))}
        for item in vulnerability_results for vuln in item.get("Vulnerabilities") or []
    ]
    timestamps = [utc(metadata[field]) for field in ("dbUpdatedAt", "finishedUtc")]
    if any(timestamp > now + timedelta(minutes=5) for timestamp in timestamps):
        raise ValueError("Future database or scan timestamp")
    if min(timestamps) < now - timedelta(hours=24):
        raise StaleScanError("Stale database or scan")
    return metadata, data, findings


def compare(baseline, candidate, now=None):
    now = now or datetime.now(timezone.utc)
    before_meta, before, previous = load(baseline, now, binary_targets=True)
    after_meta, after, current = load(candidate, now, binary_targets=True)
    for field in ("version", "dbUpdatedAt", "dbSha256", "platform", "configuration"):
        if before_meta[field] != after_meta[field]:
            raise ValueError(f"Incomparable Trivy evidence: {field}")
    coverage = lambda data: {
        (r["Target"], r.get("Class"), r.get("Type"))
        for r in data["Results"] if r.get("Class") in ("os-pkgs", "lang-pkgs")
    }
    if coverage(before) != coverage(after):
        raise ValueError("Analyzed target/type coverage changed")
    inventory = {
        (r["Target"], p["Name"]) for r in after["Results"]
        if r.get("Class") in ("os-pkgs", "lang-pkgs")
        for p in r.get("Packages", [])
    }
    def identity(finding):
        return tuple(finding.get(field) for field in ("lane", "scanner", "id", "package", "path", "binaryTarget"))

    def deferred(finding):
        return finding.get("binaryTarget") in DEFERRED_TARGETS

    candidate_by_identity = {identity(finding): finding for finding in current}
    baseline_by_identity = {identity(finding) for finding in previous}
    baseline_findings = []
    for finding in previous:
        candidate_finding = candidate_by_identity.get(identity(finding))
        covered = (finding["path"], finding["package"]) in inventory
        status = "OPEN" if candidate_finding else "CLOSED" if covered else "NOT_REPORTED"
        if status == "OPEN" and deferred(candidate_finding):
            status = "DEFERRED"
        baseline_findings.append({
            "finding": finding,
            "candidateFinding": candidate_finding,
            "status": status,
        })

    new_findings = [dict(finding, status="DEFERRED" if deferred(finding) else "NEW")
                    for finding in current if identity(finding) not in baseline_by_identity]
    statuses = {row["status"] for row in baseline_findings + new_findings}
    verdict = ("BLOCKED" if "NOT_REPORTED" in statuses
               else "FAIL" if statuses & {"NEW", "OPEN"} else "PASS")
    return {
        "schemaVersion": 1,
        "scope": "Linux/amd64 collector HIGH/CRITICAL/MEDIUM findings with fixes; not full image clearance",
        "baseline": before_meta, "candidate": after_meta,
        "findingPolicy": "Only HIGH/CRITICAL/MEDIUM findings with fixes are compared; exact deferred targets are reported as DEFERRED; other open/new findings are FAIL and missing evidence is BLOCKED",
        "deferredTargets": sorted(DEFERRED_TARGETS),
        "baselineFindingCount": len(baseline_findings),
        "baselineFindings": baseline_findings,
        "supportedCoverage": ["Trivy vulnerability scan of the ama-metrics collector linux/amd64 child"],
        "unsupportedCoverage": [
            "Collector linux/arm64 and Windows images",
            "Target allocator and config-reader images",
            "Trivy evidence alone does not establish Qualys QID closure",
        ],
        "verdict": verdict,
        "newFindings": new_findings,
        "unresolvedFindings": [row for row in baseline_findings if row["status"] != "CLOSED"],
        "remainingFindings": current,
    }


def run_comparison(baseline, platform, output, producer_scan):
    if not re.fullmatch(
            r"mcr\.microsoft\.com/azuremonitor/containerinsights/(?:ciprod|cidev)/prometheus-collector/images@sha256:[0-9a-f]{64}",
            baseline):
        raise ValueError("Expected an immutable released ama-metrics baseline child")
    if platform != "linux/amd64":
        raise ValueError("The build artifact covers the Linux amd64 collector only")
    output.mkdir(parents=True, exist_ok=False)
    producer_metadata = json.loads((producer_scan / "trivy" / "metadata.json").read_text(encoding="utf-8-sig"))
    image = producer_metadata["requestedRef"]
    if producer_metadata["platform"] != platform:
        raise ValueError("Producer scan does not cover the selected candidate platform")
    candidate_scan = output / "candidate"
    try:
        load(producer_scan, datetime.now(timezone.utc))
    except StaleScanError:
        print("Build Trivy evidence expired; rescanning the same image with a fresh DB.", flush=True)
        scan(image, platform, candidate_scan)
        metadata, _, _ = load(candidate_scan, datetime.now(timezone.utc))
        if metadata["version"] != producer_metadata["version"]:
            raise ValueError("Refreshed Trivy version does not match producer scan")
    else:
        shutil.copytree(producer_scan, candidate_scan)
    scan(baseline, platform, output / "baseline", candidate_scan)
    result = compare(output / "baseline", candidate_scan)
    with (output / "comparison.json").open("x", encoding="utf-8") as file:
        json.dump(result, file, indent=2)
        file.write("\n")
    return result


def print_comparison(result, output):
    from filter_evidence import filter_json

    print(json.dumps({"verdict": result["verdict"], "scope": result["scope"], "findingPolicy": result["findingPolicy"],
                      "comparison": str(output / "comparison.json")}, indent=2))
    secrets = sorted({value for key, value in os.environ.items()
                      if len(value) >= 8 and re.search(r"(?i)token|secret|password", key)},
                     key=len, reverse=True)
    fields = ("id", "package", "path", "digest", "installed", "fixed", "severity")
    reasons = {"OPEN": "finding remains in candidate", "CLOSED": "finding absent from covered package",
               "NOT_REPORTED": "baseline finding or package not reported",
               "DEFERRED": "finding deferred to the separately managed Collector upgrade",
               "NEW": "new candidate finding"}

    def public_finding(finding):
        if finding is None:
            return None
        return {key: value if isinstance(value, str) else "[non-string field omitted]"
                for key in fields if (value := finding.get(key)) is not None}

    def log_row(label, row):
        text = json.dumps(filter_json(row, secrets), ensure_ascii=True)
        if len(text) <= 3000:
            print(label + " " + text)
        else:
            parts = [text[index:index + 3000] for index in range(0, len(text), 3000)]
            for index, part in enumerate(parts, 1):
                print(label + "_PART " + json.dumps({"part": index, "parts": len(parts), "data": part}))

    log_row("SCAN_IMAGES", {"baseline": result["baseline"]["requestedRef"],
                            "candidate": result["candidate"]["requestedRef"]})
    for row in result["baselineFindings"]:
        public = {"status": row["status"], "reason": reasons[row["status"]],
                  "baseline": public_finding(row["finding"]), "candidate": public_finding(row["candidateFinding"])}
        log_row("SCAN_COMPARISON", public)
    for finding in result["newFindings"]:
        status = finding["status"]
        public = {"status": status, "reason": reasons[status], "candidate": public_finding(finding)}
        log_row("SCAN_COMPARISON", public)
    return {"PASS": 0, "FAIL": 1, "BLOCKED": 2}[result["verdict"]]


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    images = parser.add_mutually_exclusive_group(required=True)
    images.add_argument("--image", help="Immutable child image digest")
    images.add_argument("--published-image", help="Published pipeline image tag")
    images.add_argument("--baseline-image", help="Compare the released image with the producer scan")
    parser.add_argument("--platform", required=True, choices=["linux/amd64", "linux/arm64"])
    parser.add_argument("--output", required=True, type=Path)
    parser.add_argument("--database-from", type=Path, help="Reuse a fresh baseline run's hashed DB snapshot")
    parser.add_argument("--producer-scan", type=Path, help="Collector Trivy artifact from this run's Build stage")
    args = parser.parse_args(argv)
    if args.baseline_image:
        if args.producer_scan is None or args.database_from is not None:
            parser.error("--baseline-image requires --producer-scan and cannot use --database-from")
        output = args.output.resolve()
        result = run_comparison(args.baseline_image, args.platform, output, args.producer_scan.resolve())
        return print_comparison(result, output)
    if args.producer_scan is not None:
        parser.error("--producer-scan requires --baseline-image")
    if args.published_image:
        if args.database_from:
            parser.error("--database-from cannot be used with --published-image")
        scan_published_image(args.published_image, args.platform, args.output.resolve())
    else:
        scan(args.image, args.platform, args.output.resolve(), args.database_from)


if __name__ == "__main__":
    raise SystemExit(main())
