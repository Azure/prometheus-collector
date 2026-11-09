"""Fetch protected pipeline-440 evidence and verify an exact candidate checkout."""
import argparse
import hashlib
import io
import json
import os
from pathlib import Path
from pathlib import PurePosixPath
import re
import shutil
import stat
import subprocess
import urllib.parse
import urllib.request
import zipfile

import build_receipt


ADO_PROJECT = "https://dev.azure.com/github-private/2d36c31d-2f89-409f-9a3e-32e4e9699840"
ADO_ARTIFACT_HOST = "artprodcus3.artifacts.visualstudio.com"
ARTIFACTS = ("sre-push-linux", "sre-push-windows2022", "sre-push-targetallocator",
             "sre-push-cfg", "sre-testkube-runner")
SCAN_ARTIFACT = "sre-trivy-collector"
MAX_SCAN_ARTIFACT_BYTES = 512 * 1024 * 1024
SCAN_COVERAGE = (
    "collector/linux/amd64 Trivy vulnerability discovery only; "
    "not Windows/arm64/other images/Qualys clearance"
)


class ArtifactRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, request, response, code, message, headers, url):
        if urllib.parse.urlsplit(url).scheme != "https":
            raise ValueError("Artifact redirect must remain HTTPS")
        redirected = super().redirect_request(request, response, code, message, headers, url)
        if (urllib.parse.urlsplit(url).netloc != urllib.parse.urlsplit(request.full_url).netloc
                and urllib.parse.urlsplit(url).netloc != ADO_ARTIFACT_HOST):
            redirected.remove_header("Authorization")
        return redirected


def access_token():
    token = os.environ.get("SYSTEM_ACCESSTOKEN")
    if token:
        return token
    az = shutil.which("az")
    if not az:
        raise FileNotFoundError("Azure CLI is required when SYSTEM_ACCESSTOKEN is not supplied")
    result = subprocess.run(
        [az, "account", "get-access-token", "--resource", "499b84ac-1321-427f-aa17-267ca6975798",
         "--query", "accessToken", "--output", "tsv", "--only-show-errors"],
        capture_output=True, text=True, check=True, timeout=300,
    )
    token = result.stdout.strip()
    if not token:
        raise ValueError("Azure CLI did not return an Azure DevOps access token")
    return token


def download(url, token, max_bytes=16 * 1024 * 1024):
    if not url.startswith(ADO_PROJECT + "/_apis/build/"):
        raise ValueError("Expected this campaign's fixed Azure DevOps project")
    request = urllib.request.Request(url, headers={"Authorization": "Bearer " + token})
    with urllib.request.build_opener(ArtifactRedirect()).open(request, timeout=120) as response:
        raw = response.read(max_bytes + 1)
    if len(raw) > max_bytes:
        raise ValueError("Azure DevOps response exceeds the allowed size")
    return raw


def extract_scan_artifact(raw, output):
    output.mkdir(parents=True, exist_ok=False)
    total_size = 0
    names = set()
    with zipfile.ZipFile(io.BytesIO(raw)) as archive:
        for entry in archive.infolist():
            if entry.is_dir():
                continue
            name = entry.filename
            if "\\" in name:
                raise ValueError("Scan artifact contains a nonportable path")
            if name.startswith(SCAN_ARTIFACT + "/"):
                name = name[len(SCAN_ARTIFACT) + 1:]
            if name.startswith("_manifest/"):
                continue
            relative = PurePosixPath(name)
            if (not name or relative.is_absolute()
                    or any(part in ("", ".", "..") for part in name.split("/"))):
                raise ValueError("Scan artifact contains an unsafe path")
            mode = entry.external_attr >> 16
            if stat.S_IFMT(mode) not in (0, stat.S_IFREG):
                raise ValueError("Scan artifact contains a non-regular file")
            if relative.as_posix() in names:
                raise ValueError("Scan artifact contains duplicate paths")
            names.add(relative.as_posix())
            total_size += entry.file_size
            if entry.file_size < 0 or total_size > MAX_SCAN_ARTIFACT_BYTES:
                raise ValueError("Scan artifact exceeds the allowed uncompressed size")
            target = output.joinpath(*relative.parts)
            target.parent.mkdir(parents=True, exist_ok=True)
            written = 0
            with archive.open(entry) as source, target.open("xb") as destination:
                while block := source.read(1024 * 1024):
                    written += len(block)
                    if written > entry.file_size:
                        raise ValueError("Scan artifact entry exceeds its declared size")
                    destination.write(block)
            if written != entry.file_size:
                raise ValueError("Scan artifact entry size does not match its declaration")


def validate_scan_artifact(directory, receipt):
    provenance = json.loads((directory / "provenance.json").read_text(encoding="utf-8-sig"))
    if (provenance.get("schemaVersion") != 1
            or provenance.get("producerDefinitionId") != 440
            or provenance.get("source") != receipt.get("source")
            or provenance.get("coverage") != SCAN_COVERAGE):
        raise ValueError("Collector scan provenance does not match the candidate producer")
    images = [image for image in receipt.get("images", [])
              if image.get("name") == "collector" and image.get("os") == "linux"
              and image.get("arch") == "amd64"]
    expected_image = {key: images[0].get(key) for key in ("name", "ref", "indexRef", "os", "arch")} if len(images) == 1 else None
    if provenance.get("image") != expected_image:
        raise ValueError("Collector scan is not bound to the exact candidate image")

    files = provenance.get("files")
    if not isinstance(files, dict) or not files:
        raise ValueError("Collector scan provenance has no file hashes")
    actual = set()
    for path in directory.rglob("*"):
        if path.is_symlink() or not path.resolve().is_relative_to(directory.resolve()):
            raise ValueError("Collector scan contains a linked or escaped file")
        if path.is_file() and path.name != "provenance.json":
            actual.add(path.relative_to(directory).as_posix())
    if actual != set(files) or not {
            "trivy/result.json", "trivy/metadata.json", "cache/db/trivy.db",
            "empty.ignore", "empty.yaml"} <= actual:
        raise ValueError("Collector scan files do not match the protected provenance")
    for name, digest in files.items():
        relative = PurePosixPath(name)
        if (not name or "\\" in name or relative.is_absolute()
                or any(part in ("", ".", "..") for part in name.split("/"))
                or not re.fullmatch(r"[0-9a-f]{64}", digest)):
            raise ValueError("Collector scan provenance contains an invalid file hash")
        if hashlib.sha256((directory / Path(*relative.parts)).read_bytes()).hexdigest() != digest:
            raise ValueError("Collector scan file hash mismatch")

    metadata = json.loads((directory / "trivy" / "metadata.json").read_text(encoding="utf-8-sig"))
    image = provenance["image"]
    if (metadata.get("exitCode") != 0 or metadata.get("requestedRef") != image["ref"]
            or metadata.get("platform") != "linux/amd64"
            or not isinstance(metadata.get("version"), str)
            or not re.fullmatch(r"[0-9]+\.[0-9]+\.[0-9]+", metadata["version"])):
        raise ValueError("Collector scan metadata does not match the protected image/platform")
    if (hashlib.sha256((directory / "trivy" / "result.json").read_bytes()).hexdigest()
            != metadata.get("resultSha256")
            or hashlib.sha256((directory / "cache" / "db" / "trivy.db").read_bytes()).hexdigest()
            != metadata.get("dbSha256")):
        raise ValueError("Collector scan result or database hash mismatch")
    configuration = {
        "scanners": ["vuln"], "listAllPkgs": True,
        "ignoreSha256": hashlib.sha256((directory / "empty.ignore").read_bytes()).hexdigest(),
        "configSha256": hashlib.sha256((directory / "empty.yaml").read_bytes()).hexdigest(),
    }
    if metadata.get("configuration") != configuration:
        raise ValueError("Collector scan uses an unexpected Trivy configuration")
    return metadata


def resolve_build_id(commit, branch):
    if not re.fullmatch(r"[0-9a-f]{40}", commit):
        raise ValueError("Expected full PR source SHA")
    query = urllib.parse.urlencode({
        "definitions": 440, "branchName": branch, "statusFilter": "completed",
        "resultFilter": "succeeded", "queryOrder": "finishTimeDescending",
        "$top": 100, "api-version": "7.1",
    })
    url = ADO_PROJECT + "/_apis/build/builds?" + query
    builds = json.loads(download(url, access_token())).get("value")
    if not isinstance(builds, list):
        raise ValueError("Azure DevOps returned no build list")
    matches = [build for build in builds
               if build.get("status") == "completed" and build.get("result") == "succeeded"
               and build.get("sourceVersion") == commit and build.get("sourceBranch") == branch]
    if not matches:
        return None
    selected = max(matches, key=lambda build: (build.get("finishTime") or "", int(build["id"])))
    source = build_receipt.build_source(selected)
    if source["commit"] != commit:
        raise ValueError("Resolved producer source differs from the PR head")
    return int(selected["id"])


def receipt_file(raw, artifact, filename):
    with zipfile.ZipFile(io.BytesIO(raw)) as archive:
        files = [entry for entry in archive.infolist()
                 if not entry.is_dir() and not entry.filename.startswith(
                     ("_manifest/", artifact + "/_manifest/"))]
        if (len(files) != 1 or files[0].filename not in (filename, artifact + "/" + filename)
                or files[0].file_size > 1024 * 1024):
            raise ValueError("Expected exactly the named receipt file in the protected artifact")
        return archive.read(files[0])


def fetch(build_id, commit, output):
    if build_id <= 0 or not re.fullmatch(r"[0-9a-f]{40}", commit):
        raise ValueError("Expected positive build ID and full candidate source SHA")
    token = access_token()
    build_url = f"{ADO_PROJECT}/_apis/build/builds/{build_id}"
    build_raw = download(build_url + "?api-version=7.1", token)
    build = json.loads(build_raw)
    source = build_receipt.build_source(build)
    if (source["commit"] != commit or source["buildId"] != str(build_id)
            or build["url"].casefold() != build_url.casefold()):
        raise ValueError("Authenticated build does not match the requested candidate")
    output.mkdir(parents=True, exist_ok=False)
    (output / "build.json").write_bytes(build_raw)
    pushes = []
    for artifact in ARTIFACTS:
        archive = download(
            build_url + "/artifacts?artifactName=" + artifact + "&api-version=7.1&%24format=zip", token,
        )
        filename = "runner.json" if artifact == "sre-testkube-runner" else "push.json"
        raw = receipt_file(archive, artifact, filename)
        directory = output / artifact
        directory.mkdir()
        (directory / filename).write_bytes(raw)
        if artifact != "sre-testkube-runner":
            pushed = json.loads(raw.decode("utf-8-sig"))
            pushes.extend(pushed if isinstance(pushed, list) else [pushed])
    runner_raw = (output / "sre-testkube-runner" / "runner.json").read_bytes()
    receipt = build_receipt.emit(
        build, pushes, json.loads(runner_raw.decode("utf-8-sig")), hashlib.sha256(runner_raw).hexdigest(),
    )
    with (output / "candidate-build.json").open("x", encoding="utf-8") as file:
        json.dump(receipt, file, indent=2)
        file.write("\n")
    scan_archive = download(
        build_url + "/artifacts?artifactName=" + SCAN_ARTIFACT + "&api-version=7.1&%24format=zip",
        token, MAX_SCAN_ARTIFACT_BYTES,
    )
    scan_directory = output / SCAN_ARTIFACT
    extract_scan_artifact(scan_archive, scan_directory)
    validate_scan_artifact(scan_directory, receipt)
    return receipt


def load_verified(output, build_id, commit):
    build = json.loads((output / "build.json").read_text(encoding="utf-8-sig"))
    source = build_receipt.build_source(build)
    if (source["commit"] != commit or source["buildId"] != str(build_id)
            or build["url"].casefold() != f"{ADO_PROJECT}/_apis/build/builds/{build_id}".casefold()):
        raise ValueError("Saved producer build does not match the requested candidate")
    receipt = json.loads((output / "candidate-build.json").read_text(encoding="utf-8-sig"))
    if receipt.get("source") != source or receipt.get("buildResult") != "succeeded":
        raise ValueError("Saved candidate receipt does not match the producer build")
    validate_scan_artifact(output / SCAN_ARTIFACT, receipt)
    return receipt


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--build-id", type=int)
    parser.add_argument("--source-commit", required=True)
    parser.add_argument("--source-branch")
    parser.add_argument("--resolve-producer", action="store_true")
    parser.add_argument("--output", type=Path, help="New evidence directory")
    args = parser.parse_args()
    if args.resolve_producer:
        if args.build_id is not None or args.output is not None or not args.source_branch:
            parser.error("--resolve-producer requires --source-branch and no --build-id or --output")
        print(json.dumps({"buildId": resolve_build_id(args.source_commit, args.source_branch)}))
        return 0
    if args.build_id is None or args.output is None:
        parser.error("--build-id and --output are required unless --resolve-producer is used")
    receipt = fetch(args.build_id, args.source_commit, args.output)
    scan = json.loads((args.output / SCAN_ARTIFACT / "trivy" / "metadata.json")
                      .read_text(encoding="utf-8-sig"))
    print(json.dumps({"source": receipt["source"], "testRunner": receipt["testRunner"],
                      "candidateReceipt": str(args.output / "candidate-build.json"),
                      "trivyVersion": scan["version"]}, indent=2))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
