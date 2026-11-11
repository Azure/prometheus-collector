"""Capture this worker's push output, not a later mutable-tag lookup."""
import argparse
import json
import os
from pathlib import Path
import re
import subprocess


def receipt(name, image, digest, commit, build_id, checkout):
    prefix = "containerinsightsprod.azurecr.io/public/azuremonitor/containerinsights/cidev/prometheus-collector/images:"
    if not image.startswith(prefix) or not re.fullmatch(r"[a-zA-Z0-9_][a-zA-Z0-9_.-]{0,127}", image[len(prefix):]):
        raise ValueError("Expected this pipeline's CI/dev image")
    if not re.fullmatch(r"sha256:[0-9a-f]{64}", digest):
        raise ValueError("Missing or malformed push digest")
    if not re.fullmatch(r"[0-9a-f]{40}", commit) or checkout != commit or not build_id.isdecimal():
        raise ValueError("Worker checkout/source/build mismatch")
    return {
        "name": name, "commit": commit, "buildId": build_id, "pushedRef": image,
        "indexRef": "mcr.microsoft.com/azuremonitor/containerinsights/cidev/prometheus-collector/images@" + digest,
        "tag": image[len(prefix):],
    }


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--name", required=True, choices=["collector", "targetallocator", "config-reader"])
    parser.add_argument("--image", required=True)
    evidence = parser.add_mutually_exclusive_group(required=True)
    evidence.add_argument("--metadata", type=Path)
    evidence.add_argument("--digest-file", type=Path)
    parser.add_argument("--output", required=True, type=Path)
    args = parser.parse_args()
    if args.metadata:
        metadata = json.loads(args.metadata.read_text(encoding="utf-8-sig"))
        digests = {value for value in (metadata.get("containerimage.digest"),
                   metadata.get("containerimage.descriptor", {}).get("digest")) if value is not None}
        if len(digests) != 1:
            raise ValueError("Push metadata must contain one unambiguous digest")
        digest = digests.pop()
    else:
        digest = args.digest_file.read_text(encoding="utf-8-sig").strip()
    checkout = subprocess.run(["git", "rev-parse", "HEAD"], capture_output=True, text=True, check=True).stdout.strip()
    result = receipt(args.name, args.image, digest, os.environ["BUILD_SOURCEVERSION"],
                     os.environ["BUILD_BUILDID"], checkout)
    args.output.parent.mkdir(parents=True, exist_ok=True)
    with args.output.open("x", encoding="utf-8") as output:
        json.dump(result, output, indent=2)
        output.write("\n")
    print("AMA_METRICS_PUSH_RECEIPT " + json.dumps(result, separators=(",", ":")), flush=True)


if __name__ == "__main__":
    main()
