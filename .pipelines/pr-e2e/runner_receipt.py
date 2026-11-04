"""Verify an existing pinned TestKube toolchain image and bind separately fetched test source."""
import argparse
import json
import os
from pathlib import Path
import subprocess

import build_receipt


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--image", required=True)
    parser.add_argument("--output", required=True, type=Path)
    args = parser.parse_args()
    checkout = subprocess.run(["git", "rev-parse", "HEAD"], capture_output=True, text=True, check=True).stdout.strip()
    if checkout != os.environ["BUILD_SOURCEVERSION"]:
        raise ValueError("Test source checkout differs from the pipeline source")
    images = build_receipt.resolve_images("testkube-runner", args.image)
    if not images or any(image["os"] != "linux" or image["arch"] not in ("amd64", "arm64") for image in images):
        raise ValueError("Expected a pinned Linux TestKube toolchain")
    if not any(image["arch"] == "amd64" for image in images):
        raise ValueError("TestKube runner needs Linux amd64 support")
    build_id = os.environ["BUILD_BUILDID"]
    result = {
        "schemaVersion": 1,
        "imageRef": args.image,
        "source": {"repo": "Azure/prometheus-collector", "commit": checkout, "buildId": build_id},
        "images": images,
        "artifactPath": "runner.json",
        "artifactUrl": (
            f"{os.environ['SYSTEM_COLLECTIONURI']}{os.environ['SYSTEM_TEAMPROJECTID']}"
            f"/_apis/build/builds/{build_id}/artifacts?artifactName=sre-testkube-runner&api-version=7.1"),
        "mode": "reused-toolchain-source-fetched-separately",
    }
    args.output.parent.mkdir(parents=True, exist_ok=True)
    with args.output.open("x", encoding="utf-8") as output:
        json.dump(result, output, indent=2)
        output.write("\n")


if __name__ == "__main__":
    main()
