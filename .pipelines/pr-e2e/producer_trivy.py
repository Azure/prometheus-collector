"""Publish the collector's existing vulnerability scan for exact-build reuse."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess
import time
import urllib.error

from build_receipt import resolve_images
from push_receipt import receipt
from scan_trivy import scan


def produce(image, metadata_path, output):
    if os.environ["SYSTEM_DEFINITIONID"] != "440":
        raise ValueError("Expected pipeline 440")
    if os.environ["SYSTEM_JOBATTEMPT"] != "1" or os.environ["SYSTEM_STAGEATTEMPT"] != "1":
        raise ValueError("Scan artifact retries are unsupported; start a new build")
    metadata = json.loads(metadata_path.read_text(encoding="utf-8-sig"))
    digests = {value for value in (
        metadata.get("containerimage.digest"),
        metadata.get("containerimage.descriptor", {}).get("digest"),
    ) if value is not None}
    if len(digests) != 1:
        raise ValueError("Expected one unambiguous BuildKit push digest")
    checkout = subprocess.run(["git", "rev-parse", "HEAD"], capture_output=True, text=True, check=True).stdout.strip()
    pushed = receipt("collector", image, digests.pop(), os.environ["BUILD_SOURCEVERSION"],
                     os.environ["BUILD_BUILDID"], checkout)
    # MCR replication can lag the authenticated ACR push; never fall back to a tag.
    for attempt in range(12):
        try:
            images = resolve_images("collector", pushed["indexRef"])
            break
        except urllib.error.HTTPError as error:
            if error.code not in (404, 429, 502, 503, 504) or attempt == 11:
                raise
            print(f"MCR publication unavailable ({error.code}); retrying immutable digest", flush=True)
            time.sleep(15)
    candidates = [entry for entry in images if entry["os"] == "linux" and entry["arch"] == "amd64"]
    if len(candidates) != 1:
        raise ValueError("Expected exactly one Linux amd64 collector child")
    selected = candidates[0]
    output.mkdir(parents=True, exist_ok=False)
    work = output / "scan-work"
    scan(selected["ref"], "linux/amd64", work)
    (output / "trivy").mkdir()
    for name in ("result.json", "metadata.json"):
        shutil.move(work / "trivy" / name, output / "trivy" / name)
    shutil.rmtree(work)
    provenance = {
        "schemaVersion": 1,
        "source": {"repo": "Azure/prometheus-collector", "commit": pushed["commit"], "buildId": pushed["buildId"]},
        "producerDefinitionId": 440,
        "image": selected,
        "coverage": "collector/linux/amd64 Trivy vulnerability discovery only; not Windows/arm64/other images/Qualys clearance",
        "files": {path.relative_to(output).as_posix(): hashlib.sha256(path.read_bytes()).hexdigest()
                  for path in sorted(output.rglob("*")) if path.is_file()},
    }
    (output / "provenance.json").write_text(json.dumps(provenance, indent=2) + "\n", encoding="utf-8")


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--image", required=True)
    parser.add_argument("--metadata", required=True, type=Path)
    parser.add_argument("--output", required=True, type=Path)
    args = parser.parse_args()
    produce(args.image, args.metadata, args.output.resolve())
