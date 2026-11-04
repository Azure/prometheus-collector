"""Normalize authoritative Qualys XML and supplemental Trivy JSON; fail closed."""
import argparse
import hashlib
import json
import re
import xml.etree.ElementTree as ET
from datetime import datetime, timedelta, timezone
from pathlib import Path


def utc(value):
    parsed = datetime.fromisoformat(value.replace("Z", "+00:00"))
    if parsed.tzinfo is None:
        raise ValueError("Timestamp must include timezone")
    return parsed.astimezone(timezone.utc)


def read_json(path):
    return json.loads(path.read_text(encoding="utf-8-sig"))


def lane_id(image):
    parts = [image["name"], image["os"]]
    if image["os"] == "windows":
        parts.append(image["osVersion"])
    parts.append(image["arch"])
    if any(not re.fullmatch(r"[a-zA-Z0-9_.-]+", part) or part in (".", "..") for part in parts):
        raise ValueError("Unsafe scan artifact path")
    return "/".join(parts)


def finding_key(finding):
    location = finding["path"] or hashlib.sha256((finding["evidence"] or "").encode()).hexdigest()
    return (finding["scanner"], finding["id"], finding["package"], finding["installed"], location)

def trivy_findings(data, name, digest):
    return [
        {"lane": name, "digest": digest, "scanner": "trivy",
         "id": vuln["VulnerabilityID"], "package": vuln["PkgName"],
         "installed": vuln["InstalledVersion"], "fixed": vuln.get("FixedVersion") or None,
         "path": vuln.get("PkgPath") or item["Target"], "evidence": None,
         "severity": vuln["Severity"], "cves": None, "enrichment": "supplemental",
         "fixAvailable": bool(vuln.get("FixedVersion"))}
        for item in data["Results"] for vuln in item.get("Vulnerabilities") or []
    ]


def validate_targets(targets):
    if targets.get("product") != "ama-metrics" or targets["source"]["repo"] != "Azure/prometheus-collector":
        raise ValueError("Expected ama-metrics targets from Azure/prometheus-collector")
    if not re.fullmatch(r"[0-9a-f]{40}", targets["source"]["commit"]):
        raise ValueError("Expected full source commit")
    if not isinstance(targets["source"]["buildId"], str) or not targets["source"]["buildId"].strip():
        raise ValueError("Expected source build ID")
    if not targets["images"]:
        raise ValueError("Expected at least one child image")
    for image in targets["images"]:
        lane_id(image)
        if (image["os"] not in ("linux", "windows") or image["arch"] not in ("amd64", "arm64")
                or not re.fullmatch(r"[^\s@]+@sha256:[0-9a-f]{64}", image["ref"])):
            raise ValueError("Expected supported platform and immutable child digest")
    for field in ("requestedQids", "requestedCves"):
        if not isinstance(targets[field], list) or len(set(targets[field])) != len(targets[field]):
            raise ValueError(f"Expected unique {field}")
    if any(type(qid) is not int or qid <= 0 for qid in targets["requestedQids"]):
        raise ValueError("Expected positive numeric QIDs")
    if any(not isinstance(cve, str) or not re.fullmatch(r"CVE-[0-9]{4}-[0-9]+", cve)
           for cve in targets["requestedCves"]):
        raise ValueError("Expected CVE IDs")


def qualys(xml, enrichment):
    if "<!DOCTYPE" in xml.upper() or "<!ENTITY" in xml.upper():
        raise ValueError("XML declarations are not allowed")
    root = ET.fromstring(xml)
    if root.tag != "HOST":
        raise ValueError("Expected one HOST report")
    if root.findtext("PARTIAL_REPORT_FLAG") != "false":
        raise ValueError("PARTIAL_REPORT_FLAG missing or partial")
    if root.findtext("SCA_SCAN_ENABLED") != "true":
        raise ValueError("SCA coverage missing")
    for field in ("OS", "VULNSIGS_VERSION", "AGENT_MANIFEST_VERSION", "REPORT_GENERATED_DATE"):
        if not root.findtext(field):
            raise ValueError(f"Missing XML metadata: {field}")
    vulns = root.findall("VULN")
    not_found = {n.text for n in root.findall("./VULNS_NOT_FOUND/Q")}
    if not vulns and not not_found:
        raise ValueError("Empty Qualys evaluation")
    details = {str(v["id"]).removeprefix("QID-"): v for v in enrichment.get("vulnerabilities", [])}
    findings, informational, flagged = [], [], set()
    for vuln in vulns:
        qid, flags, evidence = vuln.findtext("QID"), vuln.findtext("FLAGS"), vuln.findtext("RESULT", "")
        flag_set = set(re.split(r"[\s,]+", flags.strip())) if flags else set()
        if not qid or not flag_set or flag_set - {"n", "u", "v"}:
            raise ValueError("Malformed VULN record")
        if "v" not in flag_set:
            informational.append({"id": f"QID-{qid}", "evidence": evidence})
            continue
        flagged.add(qid)
        info = details.get(qid, {})
        common = {
            "scanner": "qualys-comet", "id": f"QID-{qid}",
            "severity": info.get("risk"), "cves": info.get("cves"),
            "enrichment": "present" if info else "missing",
            "package": None, "installed": None, "fixed": None,
            "language": None, "path": None, "evidence": evidence, "fixAvailable": None
        }
        lines = evidence.strip().splitlines()
        if lines and re.fullmatch(r'#table cols="5"', lines[0].strip()):
            if len(lines) < 3:
                raise ValueError(f"Truncated package table: {qid}")
            for line in lines[2:]:
                if not line.strip():
                    continue
                fields = line.strip().split(None, 4)
                if len(fields) != 5:
                    raise ValueError(f"Malformed package table row: {qid}")
                package, installed, fixed, language, path = fields
                findings.append(dict(common, package=package, installed=installed,
                                     fixed=None if fixed == "None" else fixed,
                                     language=language, path=path, evidence=None, fixAvailable=fixed != "None"))
        else:
            findings.append(common)
    if flagged & not_found:
        raise ValueError("QID both flagged and not found")
    return root, findings, informational, flagged, not_found


def normalize(targets, directory, now=None, max_age_hours=24):
    validate_targets(targets)
    now = now or datetime.now(timezone.utc)
    if not 0 < max_age_hours <= 24:
        raise ValueError("Scan freshness must be greater than zero and at most 24 hours")
    result = {"schema": "scan-findings/v1", "product": targets["product"],
              "repo": targets["source"]["repo"],
              "commit": targets["source"]["commit"], "buildId": targets["source"]["buildId"],
              "scannedInBuildId": targets.get("scannedInBuildId", targets["source"]["buildId"]),
              "lanes": [], "findings": [], "informational": [], "requested": [],
              "verdict": "PASS", "policy": "report-only-plus-requested-QID-closure"}
    seen = set()
    for image in targets["images"]:
        name = lane_id(image)
        if name in seen:
            raise ValueError(f"Duplicate target lane: {name}")
        seen.add(name)
        folder = directory / name
        lane = {"name": name, "digest": image["ref"].split("@")[-1], "scanner": "qualys-comet",
                "status": "BLOCKED", "errors": []}
        result["lanes"].append(lane)
        try:
            provenance = read_json(folder / "provenance.json")
            completion_path = folder / "completion.json"
            if not completion_path.exists():
                completion_path = folder / "comet" / "completion.json"
            completion = read_json(completion_path)
            if (provenance["requestedRef"] != image["ref"]
                    or image["ref"] not in provenance["target"]["RepoDigests"]
                    or provenance["commit"] != targets["source"]["commit"]
                    or provenance["buildId"] != targets["source"]["buildId"]
                    or provenance.get("scannedInBuildId", provenance["buildId"]) != result["scannedInBuildId"]):
                raise ValueError("DIGEST_OR_SOURCE_MISMATCH")
            inspected = provenance["target"]
            if inspected["Os"] != image["os"] or inspected["Architecture"] != image["arch"]:
                raise ValueError("PLATFORM_MISMATCH")
            if image["os"] == "windows" and inspected["OsVersion"] != image["osVersion"]:
                raise ValueError("OS_VERSION_MISMATCH")
            if not re.fullmatch(r"[^@\s]+@sha256:[0-9a-f]{64}", provenance["scannerImage"]):
                raise ValueError("Scanner image is not immutable")
            start, end = utc(provenance["startedUtc"]), utc(completion["finishedUtc"])
            if not start <= end <= now + timedelta(minutes=5) or now - start > timedelta(hours=max_age_hours):
                raise ValueError("STALE_OR_INVALID_SCAN_TIME")
            if completion["exitCode"] not in (0, 1):
                raise ValueError(f"SCANNER_ERROR:{completion['exitCode']}")
            log = (folder / "comet" / "scan.log").read_text(encoding="utf-8-sig")
            if re.search(r"partial scan|authentication error|timed? out|containerd snapshotter", log, re.I):
                raise ValueError("SCANNER_LOG_INTEGRITY_ERROR")
            if image["os"] == "linux" and provenance.get("storageDriver") != "overlay2":
                raise ValueError("Linux scan requires overlay2")
            xml_path = folder / "comet" / "result.xml"
            xml = xml_path.read_text(encoding="utf-8-sig")
            enrichment_path = folder / "comet" / "result.json"
            enrichment = read_json(enrichment_path) if enrichment_path.exists() else {}
            root, findings, informational, flagged, not_found = qualys(xml, enrichment)
            if (image["os"] == "windows") != ("windows" in root.findtext("OS").lower()):
                raise ValueError("XML_OS_MISMATCH")
            report_time = utc(root.findtext("REPORT_GENERATED_DATE"))
            if not start - timedelta(minutes=5) <= report_time <= end + timedelta(minutes=5):
                raise ValueError("XML_REPORT_OUTSIDE_SCAN_WINDOW")
            lane.update(status="COMPLETE", partialReportFlag=False, scaEnabled=True,
                        startedUtc=provenance["startedUtc"], finishedUtc=completion["finishedUtc"],
                        scannerImage=provenance["scannerImage"],
                        kb={"signatures": root.findtext("VULNSIGS_VERSION"),
                            "manifest": root.findtext("AGENT_MANIFEST_VERSION")},
                        counts={"xmlFlagged": len(flagged), "jsonVulns": len(enrichment.get("vulnerabilities", [])),
                                "notFound": len(not_found), "exitCode": completion["exitCode"]},
                        evidence={"xml": str(xml_path), "xmlSha256": hashlib.sha256(xml_path.read_bytes()).hexdigest()},
                        coverage={"languagesRequested": provenance.get("languagesRequested", []),
                                  "languagesVerified": provenance.get("languagesVerified", []),
                                  "configuration": provenance.get("scanConfiguration"),
                                  "configurationVerified": provenance.get("configurationVerified") is True,
                                  "warning": "Ruby acceptance and excluded directories require onboarding proof"})
            unique = {finding_key(f): dict(f, lane=name, digest=lane["digest"]) for f in findings}
            result["findings"].extend(unique.values())
            result["informational"].extend(dict(f, lane=name) for f in informational)
            for qid in targets["requestedQids"]:
                qid = str(qid)
                state = "PRESENT" if qid in flagged else "ABSENT" if qid in not_found else "NOT_EVALUATED"
                result["requested"].append({"id": f"QID-{qid}", "lane": name, "status": state,
                                            "closureEvidence": False,
                                            "baseline": "NOT_COMPARED"})
                if state == "NOT_EVALUATED":
                    lane["errors"].append(f"Requested QID not evaluated: {qid}")
                elif state == "PRESENT":
                    result["verdict"] = "FAIL"
            if targets.get("requestedCves"):
                lane["errors"].append("Requested CVE closure needs verified Qualys QID mapping")
            if lane["errors"]:
                lane["status"] = "BLOCKED"
        except (OSError, ValueError, KeyError, TypeError, ET.ParseError) as error:
            lane["errors"].append(str(error))
        if lane["status"] != "COMPLETE":
            result["verdict"] = "BLOCKED"

        if image["os"] != "linux":
            continue
        trivy_lane = {"name": name, "digest": lane["digest"], "scanner": "trivy",
                      "supplemental": True, "status": "BLOCKED", "errors": []}
        result["lanes"].append(trivy_lane)
        try:
            data = read_json(folder / "trivy" / "result.json")
            metadata = read_json(folder / "trivy" / "metadata.json")
            if image["ref"] not in data["Metadata"]["RepoDigests"]:
                raise ValueError("TRIVY_DIGEST_MISMATCH")
            if metadata["exitCode"] != 0:
                raise ValueError("TRIVY_TOOL_ERROR")
            if not isinstance(metadata.get("version"), str) or not metadata["version"]:
                raise ValueError("TRIVY_VERSION_MISSING")
            if (utc(metadata["dbUpdatedAt"]) > now + timedelta(minutes=5)
                    or now - utc(metadata["dbUpdatedAt"]) > timedelta(hours=max_age_hours)
                    or not now - timedelta(hours=max_age_hours) <= utc(metadata["finishedUtc"]) <= now + timedelta(minutes=5)):
                raise ValueError("STALE_TRIVY_DB_OR_SCAN")
            result["findings"].extend(trivy_findings(data, name, lane["digest"]))
            trivy_lane.update(status="COMPLETE", version=metadata["version"],
                              dbUpdatedAt=metadata["dbUpdatedAt"])
        except (OSError, ValueError, KeyError, TypeError) as error:
            trivy_lane["errors"].append(str(error))
            result["verdict"] = "BLOCKED"
    if not targets["images"] or any(l["status"] == "BLOCKED" for l in result["lanes"]):
        result["verdict"] = "BLOCKED"
    return result


def compare_baseline(candidate, baseline):
    """Only evaluated PRESENT-to-ABSENT with matched scanner/KB is closure evidence."""
    candidate["baselineCommit"] = baseline["commit"]
    lanes = {lane["name"]: lane for lane in baseline["lanes"] if lane["scanner"] == "qualys-comet"}
    requests = {(row["lane"], row["id"]): row for row in baseline["requested"]}
    scans_complete = candidate["verdict"] != "BLOCKED" and baseline["verdict"] != "BLOCKED"
    candidate_statuses = {lane["name"]: lane["status"] for lane in candidate["lanes"]
                          if lane["scanner"] == "qualys-comet"}
    for row in candidate["requested"]:
        current = next(lane for lane in candidate["lanes"]
                       if lane["name"] == row["lane"] and lane["scanner"] == "qualys-comet")
        previous = lanes.get(row["lane"])
        prior = requests.get((row["lane"], row["id"]))
        coverage = current.get("coverage", {})
        requested = coverage.get("languagesRequested", [])
        verified = coverage.get("languagesVerified", [])
        coverage_complete = (
            coverage.get("configurationVerified") is True
            and isinstance(coverage.get("configuration"), dict) and bool(coverage["configuration"])
            and isinstance(requested, list) and bool(requested)
            and isinstance(verified, list)
            and all(isinstance(value, str) and value for value in requested + verified)
            and set(requested) <= set(verified))
        comparable = (previous is not None and prior is not None
                      and coverage_complete and previous.get("coverage") == coverage
                      and scans_complete
                      and candidate_statuses[row["lane"]] == previous["status"] == "COMPLETE"
                      and current["scannerImage"] == previous["scannerImage"]
                      and current["kb"] == previous["kb"]
                      and utc(current["startedUtc"]).date() == utc(previous["startedUtc"]).date()
                      and candidate["repo"] == baseline["repo"]
                      and candidate["buildId"] == baseline["scannedInBuildId"]
                      and candidate["product"] == baseline["product"])
        row["baseline"] = prior["status"] if prior else "NOT_EVALUATED"
        row["closureEvidence"] = False
        if not comparable:
            row["transition"] = "BLOCKED"
            current["errors"].append("Baseline not comparable: require complete same-day same scanner/KB and verified configuration/language coverage")
            current["status"] = candidate["verdict"] = "BLOCKED"
        elif row["status"] == "ABSENT" and prior["status"] == "PRESENT":
            if current["digest"] == previous["digest"]:
                row["transition"] = "SAME_DIGEST_CONTRADICTION"
                candidate["verdict"] = "BLOCKED"
                current["errors"].append("Same digest has contradictory QID evidence; no image remediation demonstrated")
            else:
                row["transition"] = "PRESENT_TO_ABSENT"
                row["closureEvidence"] = True
        elif row["status"] == prior["status"] == "ABSENT":
            row["transition"] = "ABSENT_IN_BOTH"
            candidate["verdict"] = "BLOCKED"
            current["errors"].append("Requested QID absent in both images; no remediation demonstrated")
        else:
            row["transition"] = f"{prior['status']}_TO_{row['status']}"
            if row["status"] == "PRESENT":
                if candidate["verdict"] != "BLOCKED":
                    candidate["verdict"] = "FAIL"
            else:
                candidate["verdict"] = "BLOCKED"
    return candidate


def compare_findings(candidate, baseline):
    def identity(finding):
        return tuple(finding.get(field) for field in ("lane", "scanner", "id", "package", "path"))

    before = {identity(finding) for finding in baseline["findings"]}
    candidate["newFindings"] = [finding for finding in candidate["findings"]
                                if identity(finding) not in before]
    previous = {(lane["name"], lane["scanner"]): lane for lane in baseline["lanes"]}
    current = {(lane["name"], lane["scanner"]): lane for lane in candidate["lanes"]}
    comparable = set(previous) == set(current) and baseline["scanVerdict"] != "BLOCKED"
    for key, lane in current.items():
        old = previous.get(key, {})
        comparable &= lane["status"] == old.get("status") == "COMPLETE"
        if lane["scanner"] == "qualys-comet":
            coverage = lane.get("coverage", {})
            requested = coverage.get("languagesRequested", [])
            verified = coverage.get("languagesVerified", [])
            comparable &= (lane.get("scannerImage") == old.get("scannerImage")
                           and lane.get("kb") == old.get("kb")
                           and coverage == old.get("coverage")
                           and coverage.get("configurationVerified") is True
                           and isinstance(requested, list) and bool(requested)
                           and isinstance(verified, list) and set(requested) <= set(verified)
                           and utc(lane["startedUtc"]).date() == utc(old["startedUtc"]).date()
                           if old.get("startedUtc") else False)
        else:
            comparable &= (lane.get("version") == old.get("version")
                           and lane.get("dbUpdatedAt") == old.get("dbUpdatedAt"))
    candidate["regressionVerdict"] = (
        "BLOCKED" if not comparable else "FAIL" if candidate["newFindings"] else "PASS")
    if not comparable:
        candidate["verdict"] = "BLOCKED"
    elif candidate["newFindings"] and candidate["verdict"] != "BLOCKED":
        candidate["verdict"] = "FAIL"
    return candidate


def evaluate(targets, directory, baseline_targets=None, baseline_directory=None,
             now=None, max_age_hours=24):
    """Finalize raw scans with descriptor-bound baseline closure when requested."""
    if (baseline_targets is None) != (baseline_directory is None):
        raise ValueError("Provide both baseline targets and scans")
    candidate = normalize(targets, directory, now, max_age_hours)
    scan_verdict = candidate["verdict"]
    if baseline_targets is not None:
        descriptor = targets.get("baseline", {})
        if (not isinstance(descriptor, dict)
                or descriptor.get("sourceCommit") != baseline_targets["source"]["commit"]
                or descriptor.get("buildId") != baseline_targets["source"]["buildId"]
                or descriptor.get("scannedInBuildId") != targets["source"]["buildId"]
                or any(not isinstance(descriptor.get(field), str) or not descriptor[field].strip()
                       for field in ("targetsArtifact", "scanArtifact"))):
            raise ValueError("Baseline descriptor must bind source, source build, artifacts and current rescan build")
        # A trusted artifact adapter must attest this binding before either entrypoint.
        bound_targets = dict(baseline_targets, scannedInBuildId=descriptor["scannedInBuildId"])
        baseline = normalize(bound_targets, baseline_directory, now, max_age_hours)
        baseline["scanVerdict"] = baseline["verdict"]
        candidate = compare_baseline(candidate, baseline)
        candidate = compare_findings(candidate, baseline)
    candidate["scanVerdict"] = scan_verdict
    if any(row["status"] == "PRESENT" for row in candidate["requested"]):
        candidate["verdict"] = "FAIL"
    elif targets.get("requestedQids") and (
            not candidate["requested"]
            or any(not row["closureEvidence"] for row in candidate["requested"])):
        candidate["verdict"] = "BLOCKED"
    return candidate


def report(result):
    def cell(value):
        return str(value if value is not None else "UNRATED").replace("|", "\\|").replace("\n", "<br>")
    lines = [f"# {result['product']} scan: {result['verdict']}",
             f"Source `{result['commit']}`; build `{result['buildId']}`.",
             f"Raw scan evaluation: {result.get('scanVerdict', result['verdict'])}.",
             "XML is authoritative. Trivy is supplemental, never QID closure evidence.",
             "ABSENT alone is not remediation; closure requires comparable baseline PRESENT-to-ABSENT evidence.",
             f"New-finding comparison: {result.get('regressionVerdict', 'NOT_RUN')}.",
             "", "| Lane | Scanner | Status | Detail |", "|---|---|---|---|"]
    for lane in result["lanes"]:
        lines.append("| " + " | ".join(cell(v) for v in
                     [lane["name"], lane["scanner"], lane["status"], "; ".join(lane["errors"])]) + " |")
    lines += ["", "| Lane | Scanner | ID | Severity | Package | Installed | Fixed | Path / evidence |",
              "|---|---|---|---|---|---|---|---|"]
    for f in result["findings"]:
        lines.append("| " + " | ".join(cell(v) for v in
                     [f["lane"], f["scanner"], f["id"], f["severity"], f["package"],
                      f["installed"], f["fixed"], f["path"] or f["evidence"]]) + " |")
    lines += ["", "## Requested QIDs", "", "```json", json.dumps(result["requested"], indent=2), "```", ""]
    return "\n".join(lines)


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--targets", type=Path, required=True)
    parser.add_argument("--scans", type=Path, required=True)
    parser.add_argument("--output", type=Path, required=True)
    parser.add_argument("--max-age-hours", type=int, default=24)
    parser.add_argument("--baseline-targets", type=Path)
    parser.add_argument("--baseline-scans", type=Path)
    args = parser.parse_args(argv)
    if not 0 < args.max_age_hours <= 24:
        parser.error("--max-age-hours must be between 1 and 24")
    if bool(args.baseline_targets) != bool(args.baseline_scans):
        parser.error("Provide both --baseline-targets and --baseline-scans")
    try:
        normalized = evaluate(read_json(args.targets), args.scans,
                              read_json(args.baseline_targets) if args.baseline_targets else None,
                              args.baseline_scans, max_age_hours=args.max_age_hours)
    except (OSError, ValueError, KeyError, TypeError) as error:
        parser.error(str(error))
    args.output.mkdir(parents=True, exist_ok=False)
    (args.output / "findings.json").write_text(json.dumps(normalized, indent=2) + "\n", encoding="utf-8")
    (args.output / "report.md").write_text(report(normalized), encoding="utf-8")
    return {"PASS": 0, "FAIL": 1, "BLOCKED": 2}[normalized["verdict"]]


if __name__ == "__main__":
    raise SystemExit(main())
