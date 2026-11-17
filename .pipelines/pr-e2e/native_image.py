"""Hash native executables from an immutable image without executing it."""
import hashlib
import re
import shutil
import tarfile
import tempfile
from urllib.request import urlopen

from build_receipt import registry_json


PATHS = {"metricsext2": "usr/sbin/MetricsExtension", "azure-mdsd": "usr/sbin/mdsd"}


def binary_hashes(image, platform):
    if not re.fullmatch(r"[^\s@]+@sha256:[0-9a-f]{64}", image):
        raise ValueError("Native hashes require an immutable image child")
    repository, digest = image.split("@")
    manifest, _ = registry_json(repository, "manifests", digest)
    if (not isinstance(manifest, dict) or not isinstance(manifest.get("layers"), list)
            or not isinstance(manifest.get("config"), dict)):
        raise ValueError("Invalid native image manifest")
    config_digest = manifest["config"].get("digest")
    if not isinstance(config_digest, str) or not re.fullmatch(r"sha256:[0-9a-f]{64}", config_digest):
        raise ValueError("Invalid native image config digest")
    config, _ = registry_json(repository, "blobs", config_digest)
    if not isinstance(config, dict) or f"{config.get('os')}/{config.get('architecture')}" != platform:
        raise ValueError("Native hash image platform mismatch")
    found = {}
    unresolved = set()
    for path in PATHS.values():
        parts = path.split("/")
        unresolved.update("/".join(parts[:index]) for index in range(1, len(parts)))
    for layer in reversed(manifest["layers"]):
        digest = layer.get("digest") if isinstance(layer, dict) else None
        if not isinstance(digest, str) or not re.fullmatch(r"sha256:[0-9a-f]{64}", digest):
            raise ValueError("Invalid native image layer digest")
        with tempfile.TemporaryFile() as archive:
            with urlopen(f"https://{repository.split('/', 1)[0]}/v2/"
                         f"{repository.split('/', 1)[1]}/blobs/{digest}", timeout=120) as response:
                shutil.copyfileobj(response, archive)
            archive.seek(0)
            actual = hashlib.sha256()
            for block in iter(lambda: archive.read(1024 * 1024), b""):
                actual.update(block)
            if "sha256:" + actual.hexdigest() != digest:
                raise ValueError("Native image layer digest mismatch")
            archive.seek(0)
            with tarfile.open(fileobj=archive, mode="r:*") as contents:
                members = {member.name.removeprefix("./").rstrip("/"): member for member in contents}
                if any(path.startswith("/") or any(part in (".", "..", "") for part in path.split("/"))
                       for path in members if path not in ("", ".")):
                    raise ValueError("Noncanonical native image layer path")
                for parent in tuple(unresolved):
                    if parent in members:
                        if not members[parent].isdir():
                            raise ValueError(f"Native executable has an unsupported ancestor: {parent}")
                        unresolved.remove(parent)
                    else:
                        parts = parent.split("/")
                        if any("/".join(parts[:index] + [".wh." + parts[index]]) in members
                               or "/".join(parts[:index] + [".wh..wh..opq"]) in members
                               for index in range(len(parts))):
                            raise ValueError(f"Native executable ancestor is absent after an image whiteout: {parent}")
                for name, path in PATHS.items():
                    if name in found:
                        continue
                    parts = path.split("/")
                    ancestors = ["/".join(parts[:index]) for index in range(1, len(parts))]
                    if any(parent in members and not members[parent].isdir() for parent in ancestors):
                        raise ValueError(f"Native executable has an unsupported ancestor: {path}")
                    if path in members:
                        if not members[path].isfile():
                            raise ValueError(f"Native executable is not a regular file: {path}")
                        value = hashlib.sha256()
                        with contents.extractfile(members[path]) as executable:
                            for block in iter(lambda: executable.read(1024 * 1024), b""):
                                value.update(block)
                        found[name] = value.hexdigest()
                    elif any("/".join(parts[:index] + [".wh." + parts[index]]) in members
                             for index in range(len(parts))) or any(
                                 "/".join(parts[:index] + [".wh..wh..opq"]) in members
                                 for index in range(len(parts))):
                        raise ValueError(f"Native executable is absent after an image whiteout: {path}")
        if len(found) == len(PATHS) and not unresolved:
            return found
    raise ValueError("Native executable bytes are missing from the image")
