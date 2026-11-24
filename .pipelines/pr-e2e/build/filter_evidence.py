"""Stage filtered evidence without publishing kubeconfig or scanner caches."""

import argparse
import hashlib
import html
import json
import os
from pathlib import Path
import re


SECRET_FIELDS = (
    r"(?:password|passwd|secret|api[_-]?key|(?:secret[_-]?)?access[_-]?key|account[_-]?key|"
    r"connection[_-]?string|client[_-]?secret|access[_-]?token|refresh[_-]?token|"
    r"authorization|private[_-]?key|client-key-data|client-certificate-data|token)"
)
SECRET_KEY = re.compile(r"(?i)^" + SECRET_FIELDS + r"$")
CREDENTIAL_RESOURCE = re.compile(r"""(?i)\bkind["']?\s*:\s*["']?(?:secret|config)\b""")
PRIVATE_PATH = re.compile(
    r"(?i)(?:^|/)(?:kubeconfig[^/]*|[.]kube|[.]azure|[.]git|cache|db|"
    r"credentials[^/]*|[.]?env|config[.]json)(?:/|$)|[.](?:pem|key|pfx|p12)$"
)
ALLOWED_SUFFIXES = {".json", ".xml", ".log", ".txt"}


def redact(text, secrets=()):
    for secret in secrets:
        escaped = html.escape(secret)
        for value in (secret, html.escape(secret, quote=False), escaped,
                      escaped.replace("&quot;", "&#34;").replace("&#x27;", "&#39;")):
            text = text.replace(value, "[REDACTED]")
    text = re.sub(r"-----BEGIN [^-]*PRIVATE KEY-----.*?-----END [^-]*PRIVATE KEY-----",
                  "[REDACTED PRIVATE KEY]", text, flags=re.DOTALL)
    text = re.sub(r"(?i)((?:bearer|basic)\s+)[A-Za-z0-9._~+/-]+=*", r"\1[REDACTED]", text)
    text = re.sub(r"(https?://)[^/@\s]+:[^/@\s]+@", r"\1[REDACTED]@", text)
    text = re.sub(
        r"(?i)([?&](?:sig|token|access_token|client_secret|password)=)[^&\s\"'<>]+",
        r"\1[REDACTED]", text,
    )
    return re.sub(
        r"(?im)(\b" + SECRET_FIELDS + r"\s*[:=]\s*)"
        r"(?:\"[^\"\r\n]*\"|'[^'\r\n]*'|[^\s,}<]+)",
        r"\1[REDACTED]", text,
    )


def filter_json(value, secrets=()):
    if isinstance(value, dict):
        if value.get("kind") in ("Secret", "Config"):
            return {"kind": value["kind"], "publication": "credential resource omitted"}
        return {key: "[REDACTED]" if SECRET_KEY.fullmatch(key) else filter_json(item, secrets)
                for key, item in value.items()}
    if isinstance(value, list):
        return [filter_json(item, secrets) for item in value]
    if isinstance(value, str):
        if CREDENTIAL_RESOURCE.search(html.unescape(value)):
            return "[REDACTED CREDENTIAL RESOURCE]"
        return redact(value, secrets)
    return value


def stage(evidence, output, environment=None):
    environment = os.environ if environment is None else environment
    if output.exists():
        raise ValueError("Publication directory must be new; never mix evidence from attempts")
    evidence, output = evidence.resolve(), output.resolve()
    if output.is_relative_to(evidence) or evidence.is_relative_to(output):
        raise ValueError("Publication and private evidence directories must be separate")
    output.mkdir(parents=True)
    manifest = {
        "sourceSha": environment.get("BUILD_SOURCEVERSION"), "buildId": environment.get("BUILD_BUILDID"),
        "publication": "credential-filtered; hashes distinguish original and published bytes",
        "files": [], "excluded": [],
    }
    invalid = []
    try:
        if not evidence.is_dir():
            raise ValueError("No evidence was produced")
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
            try:
                text = raw.decode("utf-8-sig")
                if path.suffix.lower() == ".json":
                    value = json.loads(text)
                    filtered = filter_json(value)
                    published = raw if filtered == value else (json.dumps(filtered, indent=2) + "\n").encode("utf-8")
                else:
                    if CREDENTIAL_RESOURCE.search(html.unescape(text)):
                        manifest["excluded"].append(name)
                        continue
                    published = redact(text).encode("utf-8")
            except (UnicodeError, json.JSONDecodeError):
                manifest["excluded"].append(name)
                invalid.append(name)
                continue
            target = output / relative
            target.parent.mkdir(parents=True, exist_ok=True)
            target.write_bytes(published)
            manifest["files"].append({
                "path": name, "originalSha256": hashlib.sha256(raw).hexdigest(),
                "publishedSha256": hashlib.sha256(published).hexdigest(),
            })
        if invalid:
            raise ValueError("Invalid evidence excluded: " + ", ".join(invalid))
    except (ValueError, KeyError, OSError, UnicodeError) as error:
        manifest["publicationError"] = str(error)
        raise
    finally:
        (output / "publication.json").write_text(
            json.dumps(filter_json(manifest), indent=2) + "\n", encoding="utf-8")
    return manifest


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--evidence", required=True, type=Path)
    parser.add_argument("--output", required=True, type=Path)
    args = parser.parse_args()
    stage(args.evidence, args.output)


if __name__ == "__main__":
    main()
