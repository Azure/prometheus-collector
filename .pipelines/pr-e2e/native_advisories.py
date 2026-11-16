"""Reevaluate known shipped native binaries against current upstream advisories."""

import gzip
import argparse
import hashlib
import json
from pathlib import Path
import re
import time
from datetime import datetime, timezone
from urllib.error import HTTPError, URLError
from urllib.parse import urlencode, urljoin
from urllib.request import Request, urlopen
import xml.etree.ElementTree as ET


OSV = "https://api.osv.dev/v1/query"
FEED = "https://packages.microsoft.com/azurelinux/3.0/prod/ms-non-oss/{arch}/"
KNOWN_IMAGE = ("mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images"
               "@sha256:90040631f37a121482691e8137213c1088ed548444b3a2813575cdb906a57da9")
# Applicability and backports must be verified for the exact executable bytes.
VERIFIED_FINDINGS = {
    ("a4526cadafa66f599b59eda9ef0829397d9161712434b25a8bc6be19ead53baf", "zlib", "CVE-2026-27171"):
        ("Shipped CRC-combine code lacks the negative-length guard fixed in zlib 1.3.2",
         "2.2026.504.842-1.azl3"),
    ("4e24625d98b9084045453b9883d6c1716f4df10fbfe1920746e4288a6bbe5425", "openssl", "CVE-2024-9143"):
        ("Shipped BN_GF2m_poly2arr lacks the odd-polynomial guard and degree limit added by the OpenSSL fix",
         "1.43.0-1470"),
}
ASSOCIATED_FIXES = {
    ("a4526cadafa66f599b59eda9ef0829397d9161712434b25a8bc6be19ead53baf", "c-ares", "CVE-2026-69184"):
        "2.2026.921.1606-1.azl3",
    ("a4526cadafa66f599b59eda9ef0829397d9161712434b25a8bc6be19ead53baf", "c-ares", "CVE-2026-69186"):
        "2.2026.921.1606-1.azl3",
}
NOT_MDSD_OPENSSL = {
    "CVE-2018-0734", "CVE-2018-0735", "CVE-2019-1543", "CVE-2019-1547",
    "CVE-2019-1549", "CVE-2019-1551", "CVE-2019-1552", "CVE-2019-1563",
    "CVE-2020-1967", "CVE-2025-9231", "CVE-2025-9232",
    "CVE-2009-1390", "CVE-2009-3765", "CVE-2009-3766", "CVE-2009-3767", "CVE-2019-0190",
}
NVD_COMPONENTS = {
    "metricsext2": [("zlib", "1.3.1", "zlib", "zlib", "1.3.1")],
    "azure-mdsd": [("openssl", "1.1.1w", "openssl", "openssl", "1.1.1w"),
                   ("curl", "8.0.0-DEV", "haxx", "curl", "8.0.0"),
                   ("zlib", "1.2.11", "zlib", "zlib", "1.2.11")],
}
SHARED_RPMS = {
    "metricsext2": {"boost", "cpprest", "brotli", "glibc", "libgcc", "libstdc++", "util-linux-libs", "zlib"},
    "azure-mdsd": {"SymCrypt", "glibc", "libgcc", "libstdc++"},
}
BINARIES = {
    "metricsext2": {
        "sha256": "a4526cadafa66f599b59eda9ef0829397d9161712434b25a8bc6be19ead53baf",
        "version": "2.2026.227.1509-1.azl3",
        "components": [
            ("zlib", "1.3.1", "51b7f2abdade71cd9bb0e7a373ef2610ec6f9daf",
             "embedded version verified; upstream source identity and backports unverified"),
            ("c-ares", "1.34.6", "3ac47ee46edd8ea40370222f91613fc16c434853",
             "producer inventory and resolver code identified; exact linked source is not fully proven"),
            ("opentelemetry-cpp", "1.24.0", "a7b100834dce58c949ab9bdfccdf1613db460d1c",
             "producer inventory identified; affected exporter linkage is unverified"),
            ("boost", "1.83.0", "564e2ac16907019696cdaba8a93e3588ec596062",
             "shared RPM version identified; backports are unverified"),
            ("cpprestsdk", "2.10.19", "411a109150b270f23c8c97fa4ec9a0a4a98cdecf",
             "shared RPM version identified; backports are unverified"),
            ("brotli", "1.2.1", "db24a98aed38a16de78cc05ca15cdd8dc2a0d92d",
             "shared RPM version identified; backports are unverified"),
            ("openssl", "3.3.7", "204165c1550d3aa0f49395af654124cec2bbabf9",
             "linked SONAME identified; RPM identity and backports are unverified"),
            ("grpc", "1.71.0", "4e9357bca1408596663a218c0c608a4c0560a867",
             "producer inventory identified; exact linked source is unverified"),
            ("protobuf", "5.29.5", "f5de0a0495faa63b4186fc767324f8b9a7bf4fc4",
             "producer inventory identified; exact linked source is unverified"),
        ],
    },
    "azure-mdsd": {
        "sha256": "4e24625d98b9084045453b9883d6c1716f4df10fbfe1920746e4288a6bbe5425",
        "version": "1.40.3-1349",
        "components": [
            ("openssl", "1.1.1w", "e04bd3433fd84e1861bf258ea37928d9845e6a86",
             "embedded version verified; producer backports and affected code unverified"),
            ("curl", "8.0.0-DEV", "47ccaa4218c408e70671a2fa9caaa3caf8c1a877",
             "embedded 8.0.0-DEV; query uses 8.0.0 release commit, exact source and backports unverified"),
            ("zlib", "1.2.11", "cacf7f1d4e3d44d871b605da3b647f07d718623f",
             "producer declared source; linkage to shipped binary unverified"),
        ],
    },
}


def fetch(url, body=None):
    headers = {"Cache-Control": "no-cache"}
    if body is not None:
        body = json.dumps(body).encode()
        headers["Content-Type"] = "application/json"
    with urlopen(Request(url, data=body, headers=headers), timeout=60) as response:
        return response.read()


def advisories(commit):
    results = []
    seen = set()
    token = None
    while True:
        query = {"commit": commit}
        if token:
            query["page_token"] = token
        response = json.loads(fetch(OSV, query))
        if not isinstance(response, dict):
            raise ValueError("OSV returned a non-object response")
        page = response.get("vulns", [])
        if not isinstance(page, list) or any(
            not isinstance(item, dict) or not isinstance(item.get("id"), str) or not item["id"]
            or not isinstance(item.get("aliases", []), list)
            or any(not isinstance(alias, str) for alias in item.get("aliases", []))
            for item in page
        ):
            raise ValueError("OSV returned an invalid advisory list")
        results.extend(page)
        token = response.get("next_page_token")
        if not token:
            return results
        if not isinstance(token, str) or token in seen:
            raise ValueError("OSV returned an invalid pagination token")
        seen.add(token)


def nvd_advisories(vendor, product, version):
    cpe = f"cpe:2.3:a:{vendor}:{product}:{version}:*:*:*:*:*:*:*"
    results = []
    while True:
        query = urlencode({"cpeName": cpe, "resultsPerPage": 2000, "startIndex": len(results)})
        for attempt in range(5):
            try:
                response = json.loads(fetch("https://services.nvd.nist.gov/rest/json/cves/2.0?" + query))
                break
            except HTTPError as error:
                if error.code != 429 or attempt == 4:
                    raise
                time.sleep(6 * (2 ** attempt))
        if not isinstance(response, dict):
            raise ValueError("NVD returned a non-object response")
        page = response.get("vulnerabilities")
        total = response.get("totalResults")
        if (not isinstance(page, list) or type(total) is not int or total < 0
                or len(results) + len(page) > total or (not page and len(results) < total)):
            raise ValueError("NVD returned incomplete or invalid results")
        for entry in page:
            cve = entry.get("cve") if isinstance(entry, dict) else None
            advisory = cve.get("id") if isinstance(cve, dict) else None
            if not isinstance(advisory, str) or not re.fullmatch(r"CVE-\d{4}-\d+", advisory):
                raise ValueError("NVD returned an invalid advisory ID")
            results.append(advisory)
        if len(results) >= total:
            return results


def versions(platform):
    arch = {"linux/amd64": "x86_64", "linux/arm64": "aarch64"}[platform]
    root = FEED.format(arch=arch)
    repo = ET.fromstring(fetch(urljoin(root, "repodata/repomd.xml")))
    ns = {"r": "http://linux.duke.edu/metadata/repo",
          "p": "http://linux.duke.edu/metadata/common"}
    primary = repo.find("r:data[@type='primary']", ns)
    if primary is None:
        raise ValueError("Feed has no primary metadata")
    location = primary.find("r:location", ns)
    checksum = primary.find("r:checksum", ns)
    if location is None or checksum is None or checksum.get("type") != "sha256":
        raise ValueError("Feed primary location or SHA256 is missing")
    href = location.get("href", "")
    if not re.fullmatch(r"repodata/[0-9a-f]{64}-primary\.xml\.gz", href):
        raise ValueError("Unexpected feed primary metadata location")
    compressed = fetch(urljoin(root, href))
    if hashlib.sha256(compressed).hexdigest() != checksum.text:
        raise ValueError("Feed primary checksum mismatch")
    metadata = ET.fromstring(gzip.decompress(compressed))
    packages = {name: [] for name in BINARIES}
    for package in metadata.findall("p:package", ns):
        name = package.findtext("p:name", namespaces=ns)
        if name not in packages:
            continue
        package_arch = package.findtext("p:arch", namespaces=ns)
        version = package.find("p:version", ns)
        if package_arch != arch or version is None:
            continue
        value, release = version.get("ver", ""), version.get("rel", "")
        epoch = version.get("epoch", "")
        if not value or not release or not epoch.isdigit():
            raise ValueError(f"Invalid RPM version for {name}: {epoch}:{value}-{release}")
        packages[name].append((int(epoch), value, release))
    if any(not values for values in packages.values()):
        raise ValueError("Feed does not list both native packages")
    return {name: [(f"{epoch}:" if epoch else "") + f"{value}-{release}"
                   for epoch, value, release in values]
            for name, values in packages.items()}


def package_identities(image, platform, scan):
    packages = {name: set() for name in BINARIES}
    expected_arch = {"linux/amd64": "x86_64", "linux/arm64": "aarch64"}[platform]
    for result in scan.get("Results") or []:
        if result.get("Class") != "os-pkgs":
            continue
        for package in result.get("Packages") or []:
            name = package.get("Name")
            if name not in packages:
                continue
            if package.get("Arch") != expected_arch or not package.get("Version") or not package.get("Release"):
                raise ValueError(f"Invalid RPM identity for {name}")
            epoch = package.get("Epoch") or 0
            packages[name].add((f"{epoch}:" if str(epoch) != "0" else "")
                               + package["Version"] + "-" + package["Release"])
    identities = {}
    for name, found in packages.items():
        if len(found) > 1:
            raise ValueError(f"Multiple RPM identities for {name}")
        version = next(iter(found), None)
        if image == KNOWN_IMAGE and platform == "linux/amd64":
            if version is not None and version != BINARIES[name]["version"]:
                raise ValueError(f"RPM identity conflicts with the known shipped {name}")
            identities[name] = {"version": BINARIES[name]["version"], "sha256": BINARIES[name]["sha256"]}
        else:
            identities[name] = {"version": version, "sha256": None}
    return identities


def evaluate(image, platform, scan):
    report = {"image": image, "platform": platform,
              "evaluatedAt": datetime.now(timezone.utc).isoformat(),
              "packages": {}, "verdict": "BLOCKED",
              "coverage": "OSV commit matches plus selected zlib, OpenSSL and curl NVD CPEs; neither is exhaustive"}
    if image not in (scan.get("Metadata", {}).get("RepoDigests") or []):
        report["gap"] = "Trivy result is not bound to the requested image digest"
        return report
    config = scan.get("Metadata", {}).get("ImageConfig", {})
    if f"{config.get('os')}/{config.get('architecture')}" != platform:
        report["gap"] = "Trivy result platform does not match the requested platform"
        return report
    if not re.fullmatch(r"[^\s@]+@sha256:[0-9a-f]{64}", image):
        report["gap"] = "An immutable image digest is required"
        return report
    try:
        identities = package_identities(image, platform, scan)
    except (ValueError, KeyError) as error:
        report["gap"] = f"Cannot establish RPM identity: {error}"
        return report
    try:
        available = versions(platform)
    except (OSError, ValueError, KeyError, ET.ParseError, URLError) as error:
        available = {}
        report["feedGap"] = f"Cannot read current package feed: {error}"
    findings = [finding for result in (scan.get("Results") or []) if result.get("Class") == "os-pkgs"
                for finding in (result.get("Vulnerabilities") or [])]
    for name, spec in BINARIES.items():
        identity = identities[name]
        result = {"version": identity["version"], "sha256": identity["sha256"],
                  "availableVersions": available.get(name, []),
                  "needsNewVersion": "unknown",
                  "verdict": "BLOCKED", "candidates": [], "componentChecks": []}
        report["packages"][name] = result
        for finding in findings:
            if (finding.get("PkgName") == name
                    and finding.get("InstalledVersion") == identity["version"]):
                advisory = finding.get("VulnerabilityID")
                if not advisory:
                    result.setdefault("queryErrors", []).append("RPM finding has no advisory ID")
                    continue
                result["candidates"].append({
                    "component": finding["PkgName"], "upstreamVersion": finding["InstalledVersion"],
                    "advisory": advisory, "source": "Trivy OS package", "classification": "UNRESOLVED",
                    "reason": "RPM match does not establish that the affected code is shipped or unpatched",
                })
            elif identity["version"] and finding.get("PkgName") in SHARED_RPMS[name]:
                advisory = finding.get("VulnerabilityID")
                version = finding.get("InstalledVersion")
                if not advisory or not version:
                    result.setdefault("queryErrors", []).append("Shared RPM finding lacks an advisory ID or version")
                    continue
                row = {
                    "component": "rpm:" + finding["PkgName"], "upstreamVersion": version,
                    "advisory": advisory, "source": "Trivy OS package",
                    "classification": "POTENTIAL_ASSOCIATION",
                    "reason": "Shared RPM finding in the image; actual use by this executable is not established",
                }
                if finding.get("FixedVersion"):
                    row["fixedComponentVersion"] = finding["FixedVersion"]
                result["candidates"].append(row)
        if identity["version"] != spec["version"]:
            result["gap"] = "No embedded inventory for this RPM version"
            continue
        result["gap"] = "Embedded inventory, advisory applicability and producer backports are not complete"
        if not identity["sha256"]:
            result["gap"] += "; executable bytes are not bound to the historical component inventory"
        for component, version, commit, evidence in spec["components"]:
            try:
                entries = advisories(commit)
            except (OSError, ValueError, URLError) as error:
                result.setdefault("queryErrors", []).append(f"{component}: {error}")
                continue
            result["componentChecks"].append({"component": component, "version": version,
                                               "source": "OSV commit", "records": len(entries)})
            seen = set()
            for entry in entries:
                ids = {match.group() for value in [entry["id"], *entry.get("aliases", [])]
                       for match in [re.search(r"CVE-\d{4}-\d+", value)] if match}
                if name == "azure-mdsd" and component == "openssl":
                    ids -= NOT_MDSD_OPENSSL
                for advisory in sorted(ids - seen):
                    seen.add(advisory)
                    verification = VERIFIED_FINDINGS.get((identity["sha256"], component, advisory))
                    result["candidates"].append({
                        "component": component, "upstreamVersion": version, "upstreamCommit": commit,
                        "advisory": advisory, "source": "OSV commit",
                        "classification": "CONFIRMED" if verification else "POTENTIAL_ASSOCIATION",
                        "reason": verification[0] if verification else (
                            evidence + "; OSV commit matches can be inaccurate"
                            + ("; historical inventory is not bound to this binary" if not identity["sha256"] else "")),
                    })
                    if verification:
                        result["verdict"] = report["verdict"] = "FAIL"
                        result["needsNewVersion"] = "yes"
                        if verification[1] in result["availableVersions"]:
                            result["candidates"][-1]["fixedPackageVersion"] = verification[1]
                    else:
                        fixed = ASSOCIATED_FIXES.get((identity["sha256"], component, advisory))
                        if fixed and fixed in result["availableVersions"]:
                            result["candidates"][-1]["fixedPackageVersion"] = fixed
        for component, version, vendor, product, query_version in NVD_COMPONENTS[name]:
            try:
                entries = nvd_advisories(vendor, product, query_version)
                result["componentChecks"].append({"component": component, "version": version,
                                                   "source": "NVD CPE", "queriedVersion": query_version,
                                                   "records": len(entries)})
                existing = {row["advisory"]: row for row in result["candidates"]
                            if row["component"] == component}
                for advisory in entries:
                    if advisory in existing:
                        existing[advisory]["alsoFoundBy"] = ["NVD CPE"]
                result["candidates"].extend({
                    "component": component, "upstreamVersion": version,
                    "advisory": advisory, "source": "NVD CPE",
                    "classification": "POTENTIAL_ASSOCIATION",
                    "reason": ("NVD CPE results may include other products or incorrect ranges; "
                               "code and backports unverified"
                               + ("; release 8.0.0 queried for unverified 8.0.0-DEV source" if component == "curl" else "")
                               + ("; historical inventory is not bound to this binary" if not identity["sha256"] else "")),
                } for advisory in entries if advisory not in existing and not (
                    name == "azure-mdsd" and component == "openssl" and advisory in NOT_MDSD_OPENSSL))
                for row in result["candidates"]:
                    verification = VERIFIED_FINDINGS.get((identity["sha256"], row["component"], row["advisory"]))
                    if row["source"] == "NVD CPE" and verification:
                        row["classification"], row["reason"] = "CONFIRMED", verification[0]
                        result["verdict"] = report["verdict"] = "FAIL"
                        result["needsNewVersion"] = "yes"
                        if verification[1] in result["availableVersions"]:
                            row["fixedPackageVersion"] = verification[1]
            except (OSError, ValueError, KeyError, URLError) as error:
                result.setdefault("queryErrors", []).append(f"{component} NVD: {error}")
    return report


def emit(image, platform, scan):
    report = evaluate(image, platform, scan)
    for package, detail in report["packages"].items():
        for candidate in detail["candidates"]:
            print("SCAN_ADVISORY " + json.dumps({"package": package, **candidate}, separators=(",", ":")), flush=True)
    summary = {**report, "packages": {name: {key: value for key, value in detail.items() if key != "candidates"}
                                     for name, detail in report["packages"].items()}}
    print("NATIVE_ADVISORY_REPORT " + json.dumps(summary, separators=(",", ":")), flush=True)
    return report


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--image", required=True)
    parser.add_argument("--platform", required=True, choices=["linux/amd64", "linux/arm64"])
    parser.add_argument("--trivy-result", required=True, type=Path)
    args = parser.parse_args()
    emit(args.image, args.platform, json.loads(args.trivy_result.read_text(encoding="utf-8-sig")))
