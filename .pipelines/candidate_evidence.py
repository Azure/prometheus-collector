"""Stage filtered candidate evidence without publishing kubeconfig or scanner caches."""

import argparse
import hashlib
import json
import os
from pathlib import Path
import re


SECRET_KEY = re.compile(
    r"(?i)^(?:password|passwd|client[_-]?secret|access[_-]?token|refresh[_-]?token|"
    r"authorization|private[_-]?key|client-key-data|client-certificate-data|token)$"
)
PRIVATE_PATH = re.compile(
    r"(?i)(?:^|/)(?:kubeconfig[^/]*|[.]kube|[.]azure|[.]git|cache|db|"
    r"credentials[^/]*|[.]?env|config[.]json)(?:/|$)|[.](?:pem|key|pfx|p12)$"
)
ALLOWED_SUFFIXES = {".json", ".xml", ".log", ".txt", ".yaml", ".yml"}


def redact(text, secrets):
    for secret in secrets:
        text = text.replace(secret, "[REDACTED]")
    text = re.sub(r"-----BEGIN [^-]*PRIVATE KEY-----.*?-----END [^-]*PRIVATE KEY-----",
                  "[REDACTED PRIVATE KEY]", text, flags=re.DOTALL)
    text = re.sub(r"(?i)((?:bearer|basic)\s+)[A-Za-z0-9._~+/-]+=*", r"\1[REDACTED]", text)
    text = re.sub(r"(https?://)[^/@\s]+:[^/@\s]+@", r"\1[REDACTED]@", text)
    text = re.sub(
        r"(?i)([?&](?:sig|token|access_token|client_secret|password)=)[^&\s\"'<>]+",
        r"\1[REDACTED]", text,
    )
    return re.sub(
        r"(?im)(\b(?:password|passwd|client[_-]?secret|access[_-]?token|refresh[_-]?token|"
        r"client-key-data|client-certificate-data|token)\s*[:=]\s*)"
        r"(?:\"[^\"\r\n]*\"|'[^'\r\n]*'|[^\s,}<]+)",
        r"\1[REDACTED]", text,
    )


def filter_json(value, secrets):
    if isinstance(value, dict):
        if value.get("kind") == "Secret":
            return {"kind": "Secret", "publication": "credential resource omitted"}
        return {key: "[REDACTED]" if SECRET_KEY.fullmatch(key) else filter_json(item, secrets)
                for key, item in value.items()}
    if isinstance(value, list):
        return [filter_json(item, secrets) for item in value]
    return redact(value, secrets) if isinstance(value, str) else value


def stage(evidence, output, source_sha, producer_build_id, environment=None):
    environment = os.environ if environment is None else environment
    if output.exists():
        raise ValueError("Publication directory must be new; never mix evidence from attempts")
    evidence, output = evidence.resolve(), output.resolve()
    if output.is_relative_to(evidence) or evidence.is_relative_to(output):
        raise ValueError("Publication and private evidence directories must be separate")
    output.mkdir(parents=True)
    manifest = {
        "sourceSha": source_sha, "producerBuildId": producer_build_id,
        "validationBuildId": environment.get("BUILD_BUILDID"),
        "pipelineSourceSha": environment.get("BUILD_SOURCEVERSION"),
        "publication": "credential-filtered; hashes distinguish original and published bytes",
        "files": [], "excluded": [], "executions": [],
    }
    secrets = sorted({
        value for key, value in environment.items()
        if len(value) >= 8 and re.search(r"(?i)token|secret|password", key)
    }, key=len, reverse=True)
    try:
        if not re.fullmatch(r"[0-9a-f]{40}", source_sha) or producer_build_id <= 0:
            raise ValueError("Publication requires a full candidate source SHA and positive producer build ID")
        if not evidence.is_dir():
            raise ValueError("No candidate evidence was produced")
        for path in sorted(evidence.rglob("*")):
            relative = path.relative_to(evidence)
            name = relative.as_posix()
            if path.is_symlink() or not path.resolve().is_relative_to(evidence):
                raise ValueError(f"Refuse linked or escaped evidence: {name}")
            if not path.is_file():
                continue
            if PRIVATE_PATH.search(name) or path.suffix.lower() not in ALLOWED_SUFFIXES:
                manifest["excluded"].append(name)
                continue
            raw = path.read_bytes()
            text = raw.decode("utf-8-sig")
            if path.suffix.lower() == ".json":
                value = json.loads(text)
                if name == "pipeline-verification/candidate-build.json":
                    source = value["source"]
                    if (source["commit"] != source_sha
                            or str(source["buildId"]) != str(producer_build_id)):
                        raise ValueError("Protected receipt source/build differs from requested candidate")
                    manifest["verifiedSource"] = source
                if name == "result.json" and value.get("sourceSha") != source_sha:
                    raise ValueError("Harness result source differs from requested candidate")
                execution_path = (relative.parts[0] == "health"
                                  or (len(relative.parts) > 2
                                      and relative.parts[1] in {"configprocessing", "alias-ingestion"}
                                      and relative.parts[2] == "testkube-executions"))
                if execution_path and path.name == "execution.json":
                    manifest["executions"].append({
                        "path": name, "id": value["id"],
                        "workflow": value["workflow"]["name"],
                        "status": value.get("result", {}).get("status"),
                    })
                filtered = filter_json(value, secrets)
                published = raw if filtered == value else (json.dumps(filtered, indent=2) + "\n").encode("utf-8")
            else:
                # YAML secrets can include credentials under arbitrary user-chosen data keys.
                if re.search(r"(?m)^\s*kind:\s*Secret\s*$", text):
                    manifest["excluded"].append(name)
                    continue
                published = redact(text, secrets).encode("utf-8")
            target = output / relative
            target.parent.mkdir(parents=True, exist_ok=True)
            target.write_bytes(published)
            manifest["files"].append({
                "path": name, "originalSha256": hashlib.sha256(raw).hexdigest(),
                "publishedSha256": hashlib.sha256(published).hexdigest(),
            })
    except (ValueError, KeyError, OSError, UnicodeError) as error:
        manifest["publicationError"] = str(error)
        raise
    finally:
        (output / "publication.json").write_text(
            json.dumps(filter_json(manifest, secrets), indent=2) + "\n", encoding="utf-8")
    return manifest


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--evidence", required=True, type=Path)
    parser.add_argument("--output", required=True, type=Path)
    parser.add_argument("--source-sha", required=True)
    parser.add_argument("--producer-build-id", required=True, type=int)
    args = parser.parse_args()
    stage(args.evidence, args.output, args.source_sha, args.producer_build_id)


if __name__ == "__main__":
    main()
