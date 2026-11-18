"""Compare receipt-bound candidate and baseline scans using the same fresh Trivy DB."""
import argparse
import json
import os
from pathlib import Path
import re
import shutil
import sys
from datetime import datetime, timezone

import compare_trivy
import fetch_candidate
import scan_trivy

sys.path.append(str(Path(__file__).resolve().parent.parent))
from candidate_evidence import filter_json


def run(build_id, commit, baseline, platform, requested, output, verified_candidate=None):
    if not re.fullmatch(
            r"mcr\.microsoft\.com/azuremonitor/containerinsights/(?:ciprod|cidev)/prometheus-collector/images@sha256:[0-9a-f]{64}",
            baseline):
        raise ValueError("Expected an immutable released ama-metrics baseline child")
    if platform not in ("linux/amd64", "linux/arm64"):
        raise ValueError("This campaign scan covers Linux Trivy findings only")
    if not requested or any(not re.fullmatch(r"CVE-[0-9]{4}-[0-9]+", cve) for cve in requested):
        raise ValueError("Explicit selected CVEs required")
    output.mkdir(parents=True, exist_ok=False)
    if verified_candidate is None:
        verified_candidate = output / "verified-candidate"
        receipt = fetch_candidate.fetch(build_id, commit, verified_candidate)
    else:
        verified_candidate = verified_candidate.resolve()
        receipt = fetch_candidate.load_verified(verified_candidate, build_id, commit)
    system, arch = platform.split("/")
    images = [image for image in receipt["images"]
              if image["name"] == "collector" and image["os"] == system and image["arch"] == arch]
    if len(images) != 1:
        raise ValueError("Expected exactly one protected candidate collector child for the platform")
    producer_scan = verified_candidate / fetch_candidate.SCAN_ARTIFACT
    producer_metadata = fetch_candidate.validate_scan_artifact(producer_scan, receipt, require_database=True)
    image = images[0]["ref"]
    if producer_metadata["requestedRef"] != image or producer_metadata["platform"] != platform:
        raise ValueError("Producer scan does not cover the selected candidate platform")
    candidate_scan = output / "candidate"
    try:
        compare_trivy.load(producer_scan, datetime.now(timezone.utc))
    except compare_trivy.StaleScanError:
        print("Producer Trivy evidence expired; rescanning the receipt-bound candidate with a fresh DB.", flush=True)
        scan_trivy.scan(image, platform, candidate_scan)
        metadata, _, _ = compare_trivy.load(candidate_scan, datetime.now(timezone.utc))
        if metadata["version"] != producer_metadata["version"]:
            raise ValueError("Refreshed Trivy version does not match producer scan")
    else:
        shutil.copytree(producer_scan, candidate_scan)
    scan_trivy.scan(baseline, platform, output / "baseline", candidate_scan)
    result = compare_trivy.compare(output / "baseline", candidate_scan, receipt, requested)
    with (output / "comparison.json").open("x", encoding="utf-8") as file:
        json.dump(result, file, indent=2)
        file.write("\n")
    return result


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--build-id", required=True, type=int)
    parser.add_argument("--source-commit", required=True)
    parser.add_argument("--baseline-image", required=True)
    parser.add_argument("--platform", required=True, choices=("linux/amd64", "linux/arm64"))
    parser.add_argument("--cve", required=True, action="append")
    parser.add_argument("--output", required=True, type=Path, help="New evidence directory")
    parser.add_argument("--verified-candidate", type=Path,
                        help="Previously fetched producer evidence to avoid a second artifact download")
    args = parser.parse_args()
    result = run(args.build_id, args.source_commit, args.baseline_image, args.platform,
                 args.cve, args.output.resolve(), args.verified_candidate)
    print(json.dumps({"source": result["source"], "verdict": result["verdict"],
                      "scope": result["scope"], "findingPolicy": result["findingPolicy"],
                      "comparison": str(args.output.resolve() / "comparison.json")}, indent=2))
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
    for row in result["smokeTests"]:
        log_row("SCAN_SMOKE", {"id": row["id"], "status": row["status"], "reason": reasons[row["status"]]})
    return {"PASS": 0, "FAIL": 1, "BLOCKED": 2}[result["verdict"]]


if __name__ == "__main__":
    raise SystemExit(main())
