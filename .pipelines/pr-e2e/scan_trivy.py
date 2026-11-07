"""Discover findings in one immutable Linux image without touching a Docker daemon."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
from datetime import datetime, timedelta, timezone

import normalize


def scan(image, platform, output, database_from=None):
    if not re.fullmatch(r"[^\s@]+@sha256:[0-9a-f]{64}", image):
        raise ValueError("An immutable child image digest is required")
    if platform not in ("linux/amd64", "linux/arm64"):
        raise ValueError("This discovery lane covers Linux only; not Windows OS/QID clearance")
    output.mkdir(parents=True, exist_ok=False)
    trivy = output / "trivy"
    trivy.mkdir()
    ignore = output / "empty.ignore"
    ignore.write_text("", encoding="utf-8")
    config = output / "empty.yaml"
    config.write_text("{}\n", encoding="utf-8")
    environment = {key: value for key, value in os.environ.items() if not key.upper().startswith("TRIVY_")}
    command = [
        "trivy", "--config", str(config), "--cache-dir", str(output / "cache"),
        "image", "--image-src", "remote", "--platform", platform,
        "--scanners", "vuln", "--list-all-pkgs", "--ignorefile", str(ignore),
        "--format", "json", "--exit-code", "0", "--no-progress",
        "--timeout", "20m", "--output", str(trivy / "result.json"), image,
    ]
    if database_from is not None:
        prior = json.loads((database_from / "trivy" / "metadata.json").read_text(encoding="utf-8-sig"))
        database = database_from / "cache" / "db"
        if hashlib.sha256((database / "trivy.db").read_bytes()).hexdigest() != prior["dbSha256"]:
            raise ValueError("Baseline database snapshot hash mismatch")
        shutil.copytree(database, output / "cache" / "db")
        command.insert(-1, "--skip-db-update")
    started = datetime.now(timezone.utc).isoformat()
    with (trivy / "scan.log").open("w", encoding="utf-8") as log:
        result = subprocess.run(command, env=environment, stdout=log, stderr=subprocess.STDOUT, check=False)
    metadata = {
        "exitCode": result.returncode, "startedUtc": started,
        "finishedUtc": datetime.now(timezone.utc).isoformat(),
        "requestedRef": image, "platform": platform, "command": command,
        "scope": "discovery-only; not CoMET/QID or full-image clearance",
    }
    (trivy / "metadata.json").write_text(json.dumps(metadata, indent=2) + "\n", encoding="utf-8")
    if result.returncode:
        raise ValueError(f"Trivy exited {result.returncode}; diagnostics retained in {trivy}")
    version = subprocess.run(
        ["trivy", "--cache-dir", str(output / "cache"), "--version", "--format", "json"],
        env=environment, capture_output=True, text=True, check=True,
    )
    details = json.loads(version.stdout)
    metadata.update(version=details["Version"], dbUpdatedAt=details["VulnerabilityDB"]["UpdatedAt"])
    metadata["dbSha256"] = hashlib.sha256((output / "cache" / "db" / "trivy.db").read_bytes()).hexdigest()
    metadata["configuration"] = {
        "scanners": ["vuln"], "listAllPkgs": True,
        "ignoreSha256": hashlib.sha256(ignore.read_bytes()).hexdigest(),
        "configSha256": hashlib.sha256(config.read_bytes()).hexdigest(),
    }
    database_time = normalize.utc(metadata["dbUpdatedAt"])
    now = datetime.now(timezone.utc)
    if not now - timedelta(hours=24) <= database_time <= now + timedelta(minutes=5):
        raise ValueError("Trivy database freshness is not established")
    data = json.loads((trivy / "result.json").read_text(encoding="utf-8-sig"))
    if image not in data["Metadata"]["RepoDigests"]:
        raise ValueError("Trivy result does not match the requested child digest")
    actual_os, actual_arch = platform.split("/")
    config_data = data["Metadata"]["ImageConfig"]
    if config_data["os"] != actual_os or config_data["architecture"] != actual_arch:
        raise ValueError("Trivy result platform mismatch")
    metadata["resultSha256"] = hashlib.sha256((trivy / "result.json").read_bytes()).hexdigest()
    (trivy / "metadata.json").write_text(json.dumps(metadata, indent=2) + "\n", encoding="utf-8")
    return data


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--image", required=True)
    parser.add_argument("--platform", required=True, choices=["linux/amd64", "linux/arm64"])
    parser.add_argument("--output", required=True, type=Path)
    parser.add_argument("--database-from", type=Path, help="Reuse a fresh baseline run's hashed DB snapshot")
    args = parser.parse_args()
    scan(args.image, args.platform, args.output.resolve(), args.database_from)


if __name__ == "__main__":
    main()
