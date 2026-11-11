"""Transfer exact collector scan evidence through authenticated private ADO logs."""
import argparse
import base64
import json
from pathlib import Path
import re
import tempfile

from discovery_log import emit_payload, read_payload
from fetch_candidate import SCAN_COVERAGE, validate_scan_artifact


PREFIX = "AMA_METRICS_CANDIDATE_SCAN_"
MAX_BYTES = 32 * 1024 * 1024
FILES = ("provenance.json", "trivy/metadata.json", "trivy/result.json")


def emit(directory):
    files = {name: base64.b64encode((directory / name).read_bytes()).decode("ascii")
             for name in FILES}
    return emit_payload({"schemaVersion": 1, "files": files}, PREFIX, MAX_BYTES)


def read(text, build_id, source_sha, image_ref, index_ref, output):
    repository = "mcr.microsoft.com/azuremonitor/containerinsights/cidev/prometheus-collector/images"
    if (build_id <= 0 or not re.fullmatch(r"[0-9a-f]{40}", source_sha)
            or any(not re.fullmatch(re.escape(repository) + r"@sha256:[0-9a-f]{64}", ref)
                   for ref in (image_ref, index_ref))):
        raise ValueError("Invalid expected candidate binding")
    value = read_payload(text, PREFIX, MAX_BYTES)
    if (value.get("schemaVersion") != 1 or not isinstance(value.get("files"), dict)
            or set(value["files"]) != set(FILES)):
        raise ValueError("Candidate scan log must contain the complete file set")
    expected_source = {"repo": "Azure/prometheus-collector", "commit": source_sha, "buildId": str(build_id)}
    image = {"name": "collector", "ref": image_ref, "indexRef": index_ref,
             "os": "linux", "arch": "amd64"}
    # Validate in isolation; a failed check must not leave success-shaped output.
    with tempfile.TemporaryDirectory() as temporary:
        directory = Path(temporary)
        for name in FILES:
            path = directory / name
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_bytes(base64.b64decode(value["files"][name], validate=True))
        metadata = validate_scan_artifact(directory, {"source": expected_source, "images": [image]})
        result = json.loads((directory / "trivy/result.json").read_text(encoding="utf-8-sig"))
        if not isinstance(result.get("Results"), list):
            raise ValueError("Candidate scan result has no results array")
        decoded = {
            "schemaVersion": 1, "source": expected_source, "producerDefinitionId": 440,
            "image": image, "coverage": SCAN_COVERAGE, "metadata": metadata, "result": result,
        }
        with output.open("x", encoding="utf-8") as destination:
            json.dump(decoded, destination, indent=2)
            destination.write("\n")
    return decoded


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--input", type=Path, required=True)
    parser.add_argument("--build-id", type=int, required=True)
    parser.add_argument("--source-sha", required=True)
    parser.add_argument("--image-ref", required=True)
    parser.add_argument("--index-ref", required=True)
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()
    read(args.input.read_text(encoding="utf-8-sig"), args.build_id, args.source_sha,
         args.image_ref, args.index_ref, args.output)
