"""Check that Trivy recognizes the RPM identities captured for shipped native files."""
import argparse
import csv
import json
from pathlib import Path


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("scan", type=Path)
    parser.add_argument("manifest", type=Path)
    parser.add_argument("--expect-cve")
    args = parser.parse_args()
    with args.manifest.open(encoding="utf-8", newline="") as source:
        rows = list(csv.reader(source, delimiter="\t"))
    if not rows or any(len(row) != 10 for row in rows):
        raise ValueError("Missing or invalid RPM manifest")
    expected = {(row[0], (row[8] + ":" if row[8] != "0" else "") + row[1], row[7]) for row in rows}
    data = json.loads(args.scan.read_text(encoding="utf-8"))
    results = [result for result in data["Results"] if result.get("Class") == "os-pkgs"]
    actual = {(package["Name"], (str(package["Epoch"]) + ":" if package.get("Epoch") else "")
               + package["Version"] + "-" + package["Release"], package["Arch"])
              for result in results for package in result.get("Packages", [])}
    if expected - actual:
        raise ValueError(f"RPMs absent from Trivy inventory: {sorted(expected - actual)}")
    if args.expect_cve:
        identities = {(name, version) for name, version, _ in expected}
        detected = {finding["VulnerabilityID"] for result in results
                    for finding in result.get("Vulnerabilities", [])
                    if (finding["PkgName"], finding["InstalledVersion"]) in identities}
        if args.expect_cve not in detected:
            raise ValueError(f"Expected RPM vulnerability not detected: {args.expect_cve}")
    print(f"Verified {len(expected)} captured RPM identities in {len(actual)} scanned OS packages")


if __name__ == "__main__":
    main()
