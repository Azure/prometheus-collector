"""Fetch protected pipeline-440 evidence and verify an exact candidate checkout."""
import argparse
import hashlib
import io
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import urllib.parse
import urllib.request
import zipfile

import build_receipt


ADO_PROJECT = "https://dev.azure.com/github-private/2d36c31d-2f89-409f-9a3e-32e4e9699840"
ADO_ARTIFACT_HOST = "artprodcus3.artifacts.visualstudio.com"
ARTIFACTS = ("sre-push-linux", "sre-push-windows2022", "sre-push-targetallocator",
             "sre-push-cfg", "sre-testkube-runner")


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


def download(url, token):
    if not url.startswith(ADO_PROJECT + "/_apis/build/"):
        raise ValueError("Expected this campaign's fixed Azure DevOps project")
    request = urllib.request.Request(url, headers={"Authorization": "Bearer " + token})
    with urllib.request.build_opener(ArtifactRedirect()).open(request, timeout=120) as response:
        raw = response.read(16 * 1024 * 1024 + 1)
    if len(raw) > 16 * 1024 * 1024:
        raise ValueError("Receipt response exceeds 16 MiB")
    return raw


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
    return receipt


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--build-id", required=True, type=int)
    parser.add_argument("--source-commit", required=True)
    parser.add_argument("--output", required=True, type=Path, help="New evidence directory")
    args = parser.parse_args()
    receipt = fetch(args.build_id, args.source_commit, args.output)
    print(json.dumps({"source": receipt["source"], "testRunner": receipt["testRunner"],
                      "candidateReceipt": str(args.output / "candidate-build.json")}, indent=2))


if __name__ == "__main__":
    main()
