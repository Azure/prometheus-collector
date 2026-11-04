"""Scan an authenticated candidate and released baseline with one private Trivy DB."""
import argparse
import json
from pathlib import Path
import re

import compare_trivy
import fetch_candidate
import scan_trivy


def run(build_id, commit, baseline, platform, requested, output):
    if not re.fullmatch(
            r"mcr\.microsoft\.com/azuremonitor/containerinsights/(?:ciprod|cidev)/prometheus-collector/images@sha256:[0-9a-f]{64}",
            baseline):
        raise ValueError("Expected an immutable released ama-metrics baseline child")
    if platform not in ("linux/amd64", "linux/arm64"):
        raise ValueError("This campaign scan covers Linux Trivy findings only")
    if not requested or any(not re.fullmatch(r"CVE-[0-9]{4}-[0-9]+", cve) for cve in requested):
        raise ValueError("Explicit selected CVEs required")
    output.mkdir(parents=True, exist_ok=False)
    receipt = fetch_candidate.fetch(build_id, commit, output / "verified-candidate")
    system, arch = platform.split("/")
    images = [image for image in receipt["images"]
              if image["name"] == "collector" and image["os"] == system and image["arch"] == arch]
    if len(images) != 1:
        raise ValueError("Expected exactly one protected candidate collector child for the platform")
    scan_trivy.scan(baseline, platform, output / "baseline")
    scan_trivy.scan(images[0]["ref"], platform, output / "candidate", output / "baseline")
    result = compare_trivy.compare(output / "baseline", output / "candidate", receipt, requested)
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
    args = parser.parse_args()
    result = run(args.build_id, args.source_commit, args.baseline_image, args.platform,
                 args.cve, args.output.resolve())
    print(json.dumps({"source": result["source"], "verdict": result["verdict"],
                      "comparison": str(args.output.resolve() / "comparison.json")}, indent=2))
    return {"PASS": 0, "FAIL": 1, "BLOCKED": 2}[result["verdict"]]


if __name__ == "__main__":
    raise SystemExit(main())
