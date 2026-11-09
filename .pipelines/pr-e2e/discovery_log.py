"""Transfer discovery JSON through private ADO logs without artifact credentials."""

import argparse
import base64
import gzip
import hashlib
import io
import json
from pathlib import Path
import re


PREFIX = "AMA_METRICS_DISCOVERY_"
MAX_BYTES = 2 * 1024 * 1024


def verify(value, build_id, source_sha, images):
    if (build_id <= 0 or not re.fullmatch(r"[0-9a-f]{40}", source_sha)
            or set(images) != {"production", "main"}
            or any(not re.fullmatch(r".+@sha256:[0-9a-f]{64}", image.get("ref", ""))
                   for image in images.values())
            or value.get("schemaVersion") != 1
            or value.get("buildId") != build_id or value.get("sourceSha") != source_sha
            or value.get("discovery", {}).get("verdict") != "DISCOVERY_ONLY"
            or value["discovery"].get("images") != images):
        raise ValueError("Discovery build/source/image binding mismatch")
    return value["discovery"]


def emit(discovery, build_id, source_sha, images):
    value = {"schemaVersion": 1, "buildId": build_id, "sourceSha": source_sha, "discovery": discovery}
    verify(value, build_id, source_sha, images)
    raw = json.dumps(value, separators=(",", ":"), ensure_ascii=True).encode("ascii")
    if len(raw) > MAX_BYTES:
        raise ValueError("Discovery log payload exceeds limit")
    digest = hashlib.sha256(raw).hexdigest()
    encoded = base64.b64encode(gzip.compress(raw, mtime=0)).decode("ascii")
    return "\n".join([PREFIX + "BEGIN " + digest,
                      *(PREFIX + "DATA " + encoded[offset:offset + 2048]
                        for offset in range(0, len(encoded), 2048)),
                      PREFIX + "END " + digest]) + "\n"


def read(text, build_id, source_sha, images):
    records = [line.split(PREFIX, 1)[1].split() for line in text.splitlines() if PREFIX in line]
    if (len(records) < 3 or any(len(row) != 2 for row in records)
            or records[0][0] != "BEGIN" or records[-1] != ["END", records[0][1]]
            or any(row[0] != "DATA" for row in records[1:-1])):
        raise ValueError("Discovery log frame is missing, duplicated or incomplete")
    encoded = "".join(row[1] for row in records[1:-1])
    if len(encoded) > MAX_BYTES * 2:
        raise ValueError("Discovery log encoding exceeds limit")
    with gzip.GzipFile(fileobj=io.BytesIO(base64.b64decode(encoded, validate=True))) as stream:
        raw = stream.read(MAX_BYTES + 1)
    if len(raw) > MAX_BYTES or hashlib.sha256(raw).hexdigest() != records[0][1]:
        raise ValueError("Discovery log checksum or length mismatch")
    return verify(json.loads(raw), build_id, source_sha, images)


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("action", choices=("emit", "read"))
    parser.add_argument("--input", type=Path, required=True)
    parser.add_argument("--manifest", type=Path, required=True)
    parser.add_argument("--build-id", type=int, required=True)
    parser.add_argument("--source-sha", required=True)
    parser.add_argument("--output", type=Path)
    args = parser.parse_args()
    images = json.loads(args.manifest.read_text(encoding="utf-8-sig"))["images"]
    text = args.input.read_text(encoding="utf-8-sig")
    if args.action == "emit":
        print(emit(json.loads(text), args.build_id, args.source_sha, images), end="")
    else:
        if args.output is None:
            parser.error("--output is required for read")
        result = read(text, args.build_id, args.source_sha, images)
        args.output.write_text(json.dumps(result, indent=2) + "\n", encoding="utf-8")
