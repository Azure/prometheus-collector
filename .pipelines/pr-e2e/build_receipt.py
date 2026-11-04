"""Bind a successful ADO build and its push receipts to verified MCR child images."""
import argparse
import hashlib
import json
from pathlib import Path
import re
import urllib.request

import normalize


def registry_json(repository, kind, reference):
    registry, name = repository.split("/", 1)
    if registry not in ("mcr.microsoft.com", "docker.io") or not re.fullmatch(r"[a-z0-9._/-]+", name):
        raise ValueError("Expected a public MCR or Docker Hub repository")
    headers = {"Accept": ", ".join((
        "application/vnd.oci.image.index.v1+json",
        "application/vnd.oci.image.manifest.v1+json",
        "application/vnd.docker.distribution.manifest.list.v2+json",
        "application/vnd.docker.distribution.manifest.v2+json",
    ))}
    if registry == "docker.io":
        with urllib.request.urlopen(
                f"https://auth.docker.io/token?service=registry.docker.io&scope=repository:{name}:pull",
                timeout=60) as response:
            token = json.load(response)["token"]
        headers["Authorization"] = "Bearer " + token
        registry = "registry-1.docker.io"
    request = urllib.request.Request(
        f"https://{registry}/v2/{name}/{kind}/{reference}", headers=headers,
    )
    with urllib.request.urlopen(request, timeout=60) as response:
        raw = response.read()
    actual = "sha256:" + hashlib.sha256(raw).hexdigest()
    if reference.startswith("sha256:") and actual != reference:
        raise ValueError("Registry bytes do not match the requested digest")
    return json.loads(raw), actual


def resolve_images(name, ref):
    if not re.fullmatch(r"[^\s@]+@sha256:[0-9a-f]{64}", ref):
        raise ValueError("Expected an immutable index or manifest")
    repository, digest = ref.split("@")
    raw, _ = registry_json(repository, "manifests", digest)
    images = []
    for child in raw.get("manifests", [{"digest": digest}]):
        descriptor = child.get("platform", {})
        if (descriptor.get("os") == "unknown"
                and child.get("annotations", {}).get("vnd.docker.reference.type") == "attestation-manifest"):
            continue
        if not re.fullmatch(r"sha256:[0-9a-f]{64}", child["digest"]):
            raise ValueError("Expected immutable child manifest digest")
        child_manifest = raw if child["digest"] == digest else registry_json(repository, "manifests", child["digest"])[0]
        config, _ = registry_json(repository, "blobs", child_manifest["config"]["digest"])
        for key in ("os", "architecture"):
            if key in descriptor and descriptor[key] != config[key]:
                raise ValueError("Index platform does not match child configuration")
        image = {"name": name, "ref": repository + "@" + child["digest"],
                 "indexRef": ref, "os": config["os"], "arch": config["architecture"]}
        if image["os"] == "windows":
            image["osVersion"] = config["os.version"]
            if descriptor.get("os.version") not in (None, image["osVersion"]):
                raise ValueError("Windows index/config OS version mismatch")
        images.append(image)
    return images


def build_source(build):
    if (build["status"] != "completed" or build["result"] != "succeeded"
            or build["definition"]["id"] != 440):
        raise ValueError("Expected a successful completed pipeline 440 build")
    if build["repository"]["name"] != "Azure/prometheus-collector":
        raise ValueError("Build repository mismatch")
    if not re.fullmatch(r"refs/heads/[^/]+/ama-metrics-sre-[^/]+", build["sourceBranch"]):
        raise ValueError("Expected an SRE campaign branch build")
    if not re.fullmatch(r"[0-9a-f]{40}", build["sourceVersion"]) or int(build["id"]) <= 0:
        raise ValueError("Expected full source SHA and positive producer build ID")
    return {"repo": "Azure/prometheus-collector", "commit": build["sourceVersion"], "buildId": str(build["id"])}


def emit(build, pushes, runner, runner_sha256):
    source = build_source(build)
    images = []
    expected_artifact = build["url"] + "/artifacts?artifactName=sre-testkube-runner&api-version=7.1"
    if (runner["schemaVersion"] != 1 or runner["artifactPath"] != "runner.json"
            or not re.fullmatch(r"[0-9a-f]{64}", runner_sha256)
            or runner["source"] != source or runner["artifactUrl"].casefold() != expected_artifact.casefold()):
        raise ValueError("TestKube runner artifact is not bound to this build")
    runner_images = resolve_images("testkube-runner", runner["imageRef"])
    if (runner["images"] != runner_images
            or runner["mode"] != "reused-toolchain-source-fetched-separately"):
        raise ValueError("TestKube runner artifact disagrees with the verified image or mode")
    for pushed in pushes:
        if pushed["commit"] != source["commit"] or str(pushed["buildId"]) != source["buildId"]:
            raise ValueError("Push receipt source/build mismatch")
        ref = pushed["indexRef"]
        if not re.fullmatch(r"[^\s@]+@sha256:[0-9a-f]{64}", ref):
            raise ValueError("Push receipt must bind an immutable index or manifest")
        repository, digest = ref.split("@")
        tag = pushed["tag"]
        if not re.fullmatch(r"[a-zA-Z0-9_][a-zA-Z0-9_.-]{0,127}", tag):
            raise ValueError("Invalid published tag")
        _, tag_digest = registry_json(repository, "manifests", tag)
        if tag_digest != digest:
            raise ValueError("Published tag no longer matches the push receipt")
        images.extend(dict(image, tagRef=repository + ":" + tag) for image in resolve_images(pushed["name"], ref))
    receipt = {
        "schemaVersion": 1, "product": "ama-metrics", "source": source,
        "sourceRef": build["sourceBranch"], "buildUrl": build["_links"]["web"]["href"],
        "buildResult": build["result"], "images": images,
        "requestedQids": [], "requestedCves": [],
        "status": "built-not-e2e-validated",
        "testRunner": {"imageRef": runner["imageRef"], "testSource": source,
                       "images": runner_images, "artifactUrl": runner["artifactUrl"],
                       "artifactPath": runner["artifactPath"], "receiptSha256": runner_sha256,
                       "mode": "reused-toolchain-source-fetched-separately"},
    }
    normalize.validate_targets(receipt)
    if not runner_images or any(image["os"] != "linux" or image["arch"] not in ("amd64", "arm64")
                                for image in runner_images) or not any(image["arch"] == "amd64" for image in runner_images):
        raise ValueError("Expected a Linux amd64-capable TestKube runner")
    if len({normalize.lane_id(image) for image in images}) != len(images):
        raise ValueError("Duplicate component/platform receipts")
    required = {("collector", "linux", "amd64"), ("collector", "linux", "arm64"),
                ("targetallocator", "linux", "amd64"), ("config-reader", "linux", "amd64")}
    actual = {(image["name"], image["os"], image["arch"]) for image in images}
    if not required <= actual or not any(
            image["name"] == "collector" and image["os"] == "windows"
            and image.get("osVersion", "").startswith("10.0.20348.") for image in images):
        raise ValueError("Missing required collector/Windows2022/allocator/config-reader candidate images")
    return receipt


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--build", required=True, type=Path, help="Authenticated ADO build response")
    parser.add_argument("--pushes", required=True, type=Path, nargs="+", help="Protected per-component push receipts from that build")
    parser.add_argument("--runner-receipt", required=True, type=Path)
    parser.add_argument("--output", required=True, type=Path)
    args = parser.parse_args()
    pushes = []
    for path in args.pushes:
        value = json.loads(path.read_text(encoding="utf-8-sig"))
        pushes.extend(value if isinstance(value, list) else [value])
    runner_raw = args.runner_receipt.read_bytes()
    result = emit(json.loads(args.build.read_text(encoding="utf-8-sig")), pushes,
                  json.loads(runner_raw.decode("utf-8-sig")), hashlib.sha256(runner_raw).hexdigest())
    with args.output.open("x", encoding="utf-8") as output:
        json.dump(result, output, indent=2)
        output.write("\n")


if __name__ == "__main__":
    main()
