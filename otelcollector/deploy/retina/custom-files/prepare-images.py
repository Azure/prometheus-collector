#!/usr/bin/env python3
"""Import every image rendered by a Retina chart and write digest pins."""

import argparse
import hashlib
import json
import re
import subprocess
from pathlib import Path

REGISTRY = "ciosstestimageacrmirror"
SERVER = REGISTRY + ".azurecr.io"
SUBSCRIPTION = "9b96ebbd-c57a-42d1-bbe9-b69296e4c7fb"
IMAGE_LINE = re.compile(r"""(?m)^\s*(?:-\s+)?image:\s*["']?([^\s"'#]+)""")


def run(args):
    result = subprocess.run(args, capture_output=True)
    if result.returncode:
        raise RuntimeError(
            f"{' '.join(args)} failed:\n"
            + (result.stderr or result.stdout).decode("utf-8", errors="replace")
        )
    return result.stdout


def normalize(image):
    if "/" not in image or not ("." in image.split("/", 1)[0] or ":" in image.split("/", 1)[0]):
        image = "docker.io/" + image
    if image.startswith("docker.io/") and image.count("/") == 1:
        image = image.replace("docker.io/", "docker.io/library/", 1)
    if ":" not in image.rsplit("/", 1)[-1]:
        image += ":latest"
    return image


def prepare(chart, version, output, dry_run):
    rendered = run([
        "helm", "template", "retina", str(chart), "--namespace", "kube-system",
        "--set", "operator.enabled=true", "--set", f"image.tag={version}",
        "--set", f"operator.tag={version}",
    ]).decode()
    raw_images = IMAGE_LINE.findall(rendered)
    if not raw_images:
        raise RuntimeError("Retina chart rendered no images")
    images = []
    for source in sorted(set(raw_images)):
        normalized = normalize(source)
        raw_manifest = run(["docker", "buildx", "imagetools", "inspect", "--raw", normalized])
        manifest = json.loads(raw_manifest)
        if normalized.startswith("ghcr.io/microsoft/retina/"):
            platforms = {
                (item.get("platform", {}).get("os"), item.get("platform", {}).get("architecture"))
                for item in manifest.get("manifests", [])
            }
            required = {("linux", "amd64"), ("linux", "arm64")}
            if normalized.startswith("ghcr.io/microsoft/retina/retina-agent:"):
                required.add(("windows", "amd64"))
            if not required <= platforms:
                raise RuntimeError(f"{normalized} is missing platforms {sorted(required - platforms)}")
        digest = "sha256:" + hashlib.sha256(raw_manifest).hexdigest()
        host, path_tag = normalized.split("/", 1)
        path, tag = path_tag.rsplit(":", 1)
        repo = f"firstparty/retina/{path.rsplit('/', 1)[-1]}" if host == "ghcr.io" and path.startswith("microsoft/retina/") else f"thirdparty/{host}/{path}"
        target_tag = f"{repo}:{tag}"
        metadata = [
            "az", "acr", "manifest", "show-metadata", "--registry", REGISTRY,
            "--subscription", SUBSCRIPTION, "--name", target_tag,
            "--query", "digest", "--output", "tsv",
        ]
        if not dry_run:
            current = subprocess.run(metadata, capture_output=True, text=True)
            if current.returncode and not any(
                marker in current.stderr.lower() for marker in ("manifest unknown", "tag does not exist", "is not found")
            ):
                raise RuntimeError(f"ACR lookup failed for {target_tag}:\n{current.stderr}")
            if current.returncode or current.stdout.strip() != digest:
                run([
                    "az", "acr", "import", "--name", REGISTRY, "--subscription", SUBSCRIPTION,
                    "--source", f"{normalized.split(':', 1)[0]}@{digest}",
                    "--image", target_tag, "--force",
                ])
            imported = run(metadata).decode().strip()
            if imported != digest:
                raise RuntimeError(f"ACR digest mismatch for {target_tag}: expected {digest}, got {imported}")
        images.append({
            "source": source,
            "destination": f"{SERVER}/{repo}@{digest}",
            "count": raw_images.count(source),
        })
        print(f"{'PLAN' if dry_run else 'READY'} {normalized} -> {SERVER}/{repo}@{digest}")
    output.write_text(json.dumps({"version": version, "images": images}, indent=2) + "\n", encoding="utf-8")


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--chart", type=Path, required=True)
    parser.add_argument("--version", required=True)
    parser.add_argument("--output", type=Path, required=True)
    parser.add_argument("--dry-run", action="store_true", help="resolve and write pins without querying or importing to ACR")
    args = parser.parse_args()
    prepare(args.chart, args.version, args.output, args.dry_run)
