#!/usr/bin/env python3
"""List or mirror all images from a pinned Helm chart into the Build ACR."""

import argparse
import hashlib
import json
import re
import subprocess
import sys
import time
from pathlib import Path

REGISTRY = "ciosstestimageacrmirror"
SERVER = REGISTRY + ".azurecr.io"
SUBSCRIPTION = "9b96ebbd-c57a-42d1-bbe9-b69296e4c7fb"
REPOSITORIES = {
    "kubeshop": "https://kubeshop.github.io/helm-charts",
    "open-telemetry": "https://open-telemetry.github.io/opentelemetry-helm-charts",
}
# Same-host cache paths already configured on the Build ACR.
CACHE_HOSTS = {"ghcr.io", "quay.io", "registry.k8s.io", "mcr.microsoft.com"}


def run(args):
    result = subprocess.run(args, capture_output=True)
    if result.returncode:
        raise RuntimeError(
            f"{' '.join(args)} failed:\n"
            + (result.stderr or result.stdout).decode("utf-8", errors="replace")
        )
    return result.stdout


def normalize(image):
    image = image.strip("\"'")
    name, separator, digest = image.partition("@")
    if separator and not re.fullmatch(r"sha256:[0-9a-f]{64}", digest):
        raise ValueError(f"Invalid digest: {image}")
    first = name.split("/")[0]
    if "/" not in name or not ("." in first or ":" in first or first == "localhost"):
        name = "docker.io/" + name
    if name.startswith("docker.io/") and name.count("/") == 1:
        name = name.replace("docker.io/", "docker.io/library/", 1)
    if not separator and ":" not in name.rsplit("/", 1)[-1]:
        name += ":latest"
    return name + ("@" + digest if separator else "")


def images(rendered):
    refs = re.findall(r"""(?m)^\s*image:\s*["']?([^\s"'#]+)""", rendered)
    refs += re.findall(
        r"""(?m)^\s*-\s*name:\s*["']?[\w]*IMAGE["']?\s*\n\s+value:\s*["']?([^\s"'#]+)""",
        rendered,
    )
    return {normalize(ref) for ref in refs
            if re.fullmatch(r"[A-Za-z0-9][A-Za-z0-9._:/@-]*", ref) and not ref.endswith(":")}


def destination(image):
    return image if image.startswith(SERVER + "/") else SERVER + "/" + image


def route(image):
    host = image.split("/", 1)[0]
    return "mirror" if host == SERVER else "cache rule" if host in CACHE_HOSTS else "ACR import"


def import_image(image):
    # Inspect the raw index, not a platform-selected child; ACR imports every platform.
    digest = "sha256:" + hashlib.sha256(
        run(["docker", "buildx", "imagetools", "inspect", "--raw", image])
    ).hexdigest()
    name = image.split("@")[0]
    repository = name.rsplit(":", 1)[0] if ":" in name.rsplit("/", 1)[-1] else name
    target = name if ":" in name.rsplit("/", 1)[-1] else repository + ":sha256-" + digest[7:]
    metadata = ["az", "acr", "manifest", "show-metadata", "--registry", REGISTRY,
                "--subscription", SUBSCRIPTION, "--name", target,
                "--query", "digest", "--output", "tsv"]
    result = subprocess.run(metadata, capture_output=True, text=True)
    if result.returncode and not any(
        message in result.stderr.lower() for message in ("tag does not exist", "manifest unknown")
    ):
        raise RuntimeError(f"ACR lookup failed for {target}:\n{result.stderr}")
    if result.returncode == 0 and result.stdout.strip() == digest:
        print(f"UNCHANGED {image}")
        return
    args = ["az", "acr", "import", "--name", REGISTRY, "--subscription", SUBSCRIPTION,
            "--source", repository + "@" + digest, "--image", target, "--force"]
    for attempt in range(3):
        try:
            run(args)
            break
        except RuntimeError:
            if attempt == 2:
                raise
            time.sleep(attempt + 1)
    if run(metadata).decode().strip() != digest:
        raise RuntimeError(f"Imported digest mismatch for {target}")
    print(f"IMPORTED {image} ({digest})")


def self_check():
    assert images("image: busybox\nimage:\n  type: string\nimage: '{{ template }}'\n"
                  "- name: TESTKUBE_TW_INIT_IMAGE\n  value: kubeshop/init:2") == {
        "docker.io/library/busybox:latest", "docker.io/kubeshop/init:2",
    }
    assert route("ghcr.io/org/app:1") == "cache rule"
    assert route("docker.io/org/app:1") == "ACR import"
    assert destination(SERVER + "/docker.io/library/nats:2") == SERVER + "/docker.io/library/nats:2"
    assert normalize("nats@sha256:" + "a" * 64) == "docker.io/library/nats@sha256:" + "a" * 64
    print("self-check passed")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    for option in ("release-file", "chart", "version", "values", "mirror-values"):
        parser.add_argument("--" + option)
    parser.add_argument("--source-set", action="append", default=[])
    parser.add_argument("--release-name", default="image-mirror")
    parser.add_argument("--namespace", default="default")
    parser.add_argument("--mode", choices=("list", "mirror"), default="list")
    parser.add_argument("--self-test", action="store_true")
    args = parser.parse_args()
    if args.self_test:
        self_check()
        return
    if args.release_file:
        text = Path(args.release_file).read_text(encoding="utf-8")
        section = re.search(r"(?m)^chart:\s*\n((?:[ \t]+.*\n)+)", text)
        if not section:
            parser.error("Release file must contain a root chart section")
        chart = dict(re.findall(r"(?m)^\s+(repository|name|version):\s*(\S+)", section[1]))
        alias = next((key for key, url in REPOSITORIES.items() if url == chart["repository"]), None)
        if not alias:
            parser.error("Unsupported chart repository")
        args.chart, args.version = alias + "/" + chart["name"], chart["version"]
    if not args.chart or not args.version:
        parser.error("Pass --release-file or both --chart and --version")
    alias = args.chart.split("/")[0]
    run(["helm", "repo", "add", alias, REPOSITORIES[alias], "--force-update"])
    run(["helm", "repo", "update", alias])
    base = ["helm", "template", args.release_name, args.chart, "--version", args.version,
            "--namespace", args.namespace]
    if args.values:
        base += ["--values", args.values]
    source_args = [arg for value in args.source_set for arg in ("--set", value)]
    sources = images(run(base + source_args).decode("utf-8"))
    overlay = ["--values", args.mirror_values] if args.mirror_values else []
    mirrored = images(run(base + overlay).decode("utf-8"))
    if not sources or mirrored != {destination(image) for image in sources}:
        raise RuntimeError(
            f"Mirror render differs from source images:\n"
            f"Expected: {sorted(destination(image) for image in sources)}\nActual: {sorted(mirrored)}"
        )
    for image in sorted(sources):
        print(f"{route(image)}: {image} -> {destination(image)}")
    print(f"Images: {len(sources)}; to import: {sum(route(i) == 'ACR import' for i in sources)}; "
          f"cache rules: {sum(route(i) == 'cache rule' for i in sources)}; non-mirror: 0")
    if args.mode == "mirror":
        rules = json.loads(run(["az", "acr", "cache", "list", "--registry", REGISTRY,
                                "--resource-group", "ci-oss-test-image-acr-mirror",
                                "--subscription", SUBSCRIPTION, "--output", "json"]))
        configured = {(rule["sourceRepository"], rule["targetRepository"]) for rule in rules}
        for image in sorted(sources):
            if route(image) == "cache rule":
                pattern = image.split("/")[0] + "/*"
                if (pattern, pattern) not in configured:
                    raise RuntimeError(f"Missing same-host ACR cache rule for {image}")
            elif route(image) == "ACR import":
                import_image(image)


if __name__ == "__main__":
    try:
        main()
    except (OSError, ValueError, KeyError, RuntimeError) as error:
        sys.exit(f"ERROR: {error}")
