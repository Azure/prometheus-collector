"""Compare published production and main collector images without claiming remediation."""
import argparse
from collections import Counter
import csv
import json
from pathlib import Path
import re
from datetime import datetime, timezone

import compare_trivy
import scan_trivy


REPOSITORY = "Azure/prometheus-collector"
IMAGE_REF = re.compile(
    r"mcr\.microsoft\.com/azuremonitor/containerinsights/(ciprod|cidev)/"
    r"prometheus-collector/images@sha256:[0-9a-f]{64}"
)


def validate_manifest(manifest):
    if (manifest.get("schemaVersion") != 1
            or manifest.get("product") != "ama-metrics"
            or manifest.get("repository") != REPOSITORY
            or manifest.get("platform") != "linux/amd64"
            or set(manifest.get("images", {})) != {"production", "main"}):
        raise ValueError("Expected the ama-metrics production/main linux/amd64 discovery manifest")
    for role, repository in (("production", "ciprod"), ("main", "cidev")):
        image = manifest["images"][role]
        source = image.get("source", {})
        publication = image.get("publication", {})
        if (not IMAGE_REF.fullmatch(image.get("ref", ""))
                or f"/{repository}/" not in image["ref"]
                or image.get("tag", "").strip() == ""
                or source.get("repo") != REPOSITORY
                or not re.fullmatch(
                    r"refs/heads/(?:main|hotfix-[A-Za-z0-9._-]+)" if role == "production" else r"refs/heads/main",
                    source.get("branch", ""))
                or not re.fullmatch(r"[0-9a-f]{40}", source.get("commit", ""))
                or not re.fullmatch(r"[0-9]+", source.get("buildId", ""))
                or publication.get("collectorLinuxAmd64Push") != "succeeded"):
            raise ValueError(f"Invalid {role} image provenance")
    production = manifest["images"]["production"]
    main = manifest["images"]["main"]
    if (production["source"]["buildStatus"] != "completed"
            or production["source"]["buildResult"] != "succeeded"
            or not production["publication"].get("indexPublished")
            or not re.fullmatch(r"sha256:[0-9a-f]{64}", production.get("indexDigest", ""))):
        raise ValueError("Production image must have a successful published index")
    if (main["source"]["buildStatus"] != "inProgress"
            or main["source"]["buildResult"] is not None
            or main.get("indexDigest") is not None
            or main["publication"].get("indexPublished") is not False
            or main["publication"].get("collectorLinuxArm64Push") != "succeeded"
            or main["publication"].get("collectorWindows2019Push") != "pending"
            or not main["publication"].get("unpublished")):
        raise ValueError("Main image publication status must preserve its partial-build caveats")
    return manifest


def finding_identity(finding):
    return tuple(finding.get(field) for field in ("lane", "scanner", "id", "package", "path"))


def compare_scans(production, main, production_image, main_image, now=None):
    now = now or datetime.now(timezone.utc)
    production_meta, production_data, before = compare_trivy.load(production, now)
    main_meta, main_data, after = compare_trivy.load(main, now)
    for field in ("version", "dbUpdatedAt", "dbSha256", "platform", "configuration"):
        if production_meta[field] != main_meta[field]:
            raise ValueError(f"Incomparable Trivy evidence: {field}")
    coverage = lambda data: {(row["Target"], row.get("Class"), row.get("Type"))
                             for row in data["Results"]}
    if coverage(production_data) != coverage(main_data):
        raise ValueError("Analyzed target/type coverage changed")

    before_findings = before["findings"]
    main_findings = after["findings"]
    main_by_identity = {}
    for finding in main_findings:
        main_by_identity.setdefault(finding_identity(finding), []).append(finding)
    production_counts = Counter(finding_identity(finding) for finding in before_findings)
    main_counts = Counter(finding_identity(finding) for finding in main_findings)
    production_findings = [
        {
            "finding": finding,
            "status": "PRESENT_IN_BOTH" if main_by_identity.get(finding_identity(finding))
            else "NOT_PRESENT_IN_MAIN",
            "matchingMainFindings": main_by_identity.get(finding_identity(finding), []),
        }
        for finding in before_findings
    ]
    version_changes = []
    for identity, count in (production_counts & main_counts).items():
        prior = sorted((finding for finding in before_findings if finding_identity(finding) == identity),
                       key=lambda finding: finding["installed"])
        current = sorted((finding for finding in main_findings if finding_identity(finding) == identity),
                         key=lambda finding: finding["installed"])
        for old, new in zip(prior[:count], current[:count]):
            if (old["installed"], old["fixed"], old["severity"]) != (
                    new["installed"], new["fixed"], new["severity"]):
                version_changes.append({"production": old, "main": new})
    new_in_main = [finding for finding in main_findings
                   if production_counts[finding_identity(finding)] == 0]
    new_in_production = [finding for finding in before_findings
                         if main_counts[finding_identity(finding)] == 0]
    return {
        "schemaVersion": 1,
        "kind": "published-image-discovery",
        "verdict": "DISCOVERY_ONLY",
        "scope": "All Trivy vulnerability findings in the collector linux/amd64 images; not full image clearance.",
        "interpretation": (
            "NOT_PRESENT_IN_MAIN is a scan comparison only, not proof that a vulnerability was fixed."
        ),
        "images": {"production": production_image, "main": main_image},
        "scanner": {
            "version": production_meta["version"],
            "databaseUpdatedAt": production_meta["dbUpdatedAt"],
            "databaseSha256": production_meta["dbSha256"],
            "configuration": production_meta["configuration"],
            "matchedAnalyzedTargets": sorted(coverage(production_data)),
        },
        "summary": {
            "productionFindingCount": len(before_findings),
            "mainFindingCount": len(main_findings),
            "productionFindingsNotPresentInMain": len(new_in_production),
            "findingsNotPresentInProduction": len(new_in_main),
            "installedVersionOrSeverityChanges": len(version_changes),
        },
        "productionFindings": production_findings,
        "findingsNotPresentInProduction": new_in_main,
        "findingsNotPresentInMain": new_in_production,
        "installedVersionOrSeverityChanges": version_changes,
        "mainFindings": main_findings,
        "unsupportedCoverage": [
            "collector linux/arm64 and Windows images",
            "target allocator and config-reader images",
            "Qualys/CoMET and QID coverage",
        ],
    }


def run(manifest_path, output):
    manifest = validate_manifest(json.loads(manifest_path.read_text(encoding="utf-8-sig")))
    output.mkdir(parents=True, exist_ok=False)
    (output / "published-images.json").write_text(json.dumps(manifest, indent=2) + "\n", encoding="utf-8")
    production = output / "production"
    main = output / "main"
    production_image = manifest["images"]["production"]
    main_image = manifest["images"]["main"]
    scan_trivy.scan(production_image["ref"], manifest["platform"], production)
    scan_trivy.scan(main_image["ref"], manifest["platform"], main, production)
    result = compare_scans(production, main, production_image, main_image)
    (output / "discovery.json").write_text(json.dumps(result, indent=2) + "\n", encoding="utf-8")
    findings = output / "findings"
    findings.mkdir()
    (findings / "published-images.json").write_text(json.dumps(manifest, indent=2) + "\n", encoding="utf-8")
    (findings / "discovery.json").write_text(json.dumps(result, indent=2) + "\n", encoding="utf-8")
    summary = {key: result[key] for key in ("kind", "verdict", "scope", "interpretation", "images", "scanner", "summary", "unsupportedCoverage")}
    (findings / "summary.json").write_text(json.dumps(summary, indent=2) + "\n", encoding="utf-8")
    with (findings / "findings.csv").open("w", encoding="utf-8", newline="") as stream:
        fields = ["image", "id", "package", "installed", "fixed", "severity", "path", "lane", "scanner", "digest"]
        writer = csv.DictWriter(stream, fieldnames=fields, extrasaction="ignore")
        writer.writeheader()
        for role, rows in (("production", [row["finding"] for row in result["productionFindings"]]),
                           ("main", result["mainFindings"])):
            writer.writerows({"image": role, **row} for row in rows)
    return result


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--manifest", required=True, type=Path)
    parser.add_argument("--output", required=True, type=Path)
    args = parser.parse_args()
    result = run(args.manifest, args.output.resolve())
    print(json.dumps({"verdict": result["verdict"], "summary": result["summary"]}, indent=2))


if __name__ == "__main__":
    main()
