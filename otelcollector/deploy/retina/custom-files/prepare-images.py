#!/usr/bin/env python3
"""Verify Retina's pinned MCR images and write post-renderer mappings."""

import argparse
import json
import re
import subprocess
import urllib.request
from pathlib import Path

ACCEPT = (
    "application/vnd.oci.image.index.v1+json,"
    "application/vnd.docker.distribution.manifest.list.v2+json,"
    "application/vnd.oci.image.manifest.v1+json"
)
IMAGES = {
    "retina-agent": (
        "d44424c1d0a3521306dc5e2e1a6601817976797a25f603ff78cdcc7a0df7df52",
        2,
        {("linux", "amd64"), ("linux", "arm64"), ("windows", "amd64")},
    ),
    "retina-init": (
        "0b67e8b12309d4babda12a5f8efa6590580e01fc2bfa04b6bb2660bd34d3ef6f",
        1,
        {("linux", "amd64"), ("linux", "arm64")},
    ),
    "retina-operator": (
        "e553efee5223501dfa1bb3e0ebb324a57f771fb3a2874b3cd686b316a8df7e63",
        1,
        {("linux", "amd64"), ("linux", "arm64")},
    ),
}
IMAGE_LINE = re.compile(r"""(?m)^\s*(?:-\s+)?image:\s*["']?([^\s"'#]+)""")


def get_manifest(name, version):
    request = urllib.request.Request(
        f"https://mcr.microsoft.com/v2/containernetworking/{name}/manifests/{version}",
        headers={"Accept": ACCEPT},
    )
    with urllib.request.urlopen(request, timeout=30) as response:
        return response.headers.get("Docker-Content-Digest"), json.load(response)


def prepare(chart, version, output):
    if version != "v1.3.2":
        raise ValueError(f"No MCR digest pins are defined for Retina {version}")
    command = [
        "helm", "template", "retina", str(chart), "--namespace", "kube-system",
        "--kube-version", "1.30.0", "--set", "operator.enabled=true",
        "--set", f"image.tag={version}", "--set", f"operator.tag={version}",
    ]
    result = subprocess.run(command, capture_output=True)
    if result.returncode:
        raise RuntimeError(result.stderr.decode("utf-8", errors="replace"))
    sources = IMAGE_LINE.findall(result.stdout.decode())
    images = []
    for name, (expected_digest, count, required_platforms) in IMAGES.items():
        source = f"ghcr.io/microsoft/retina/{name}:{version}"
        if sources.count(source) != count:
            raise ValueError(f"Expected {count} references to {source}, found {sources.count(source)}")
        actual_digest, manifest = get_manifest(name, version)
        if actual_digest != f"sha256:{expected_digest}":
            raise ValueError(f"Unexpected MCR digest for {name}: {actual_digest}")
        platforms = {
            (item.get("platform", {}).get("os"), item.get("platform", {}).get("architecture"))
            for item in manifest.get("manifests", [])
        }
        if not required_platforms <= platforms:
            raise ValueError(f"{name} is missing MCR platforms {sorted(required_platforms - platforms)}")
        images.append({
            "source": source,
            "destination": f"mcr.microsoft.com/containernetworking/{name}@sha256:{expected_digest}",
            "count": count,
        })
        print(f"READY {source} -> {images[-1]['destination']}")
    output.write_text(json.dumps({"version": version, "images": images}, indent=2) + "\n", encoding="utf-8")


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--chart", type=Path, required=True)
    parser.add_argument("--version", required=True)
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()
    prepare(args.chart, args.version, args.output)
