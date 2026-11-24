import json
import re
import sys
from pathlib import Path

IMAGE_LINE = re.compile(r"""(?m)^(\s*(?:-\s+)?image:\s*)(["']?)([^\s"'#]+)\2(\s*)$""")


def pin(manifest, pins):
    for image in pins:
        source, destination, count = image["source"], image["destination"], image["count"]
        manifest, replaced = re.subn(
            rf"""(?m)^(\s*(?:-\s+)?image:\s*)(["']?){re.escape(source)}\2(\s*)$""",
            lambda match: match[1] + match[2] + destination + match[2] + match[3],
            manifest,
        )
        if replaced != count:
            raise ValueError(f"Expected {count} references to {source}, found {replaced}")
    remaining = [match[3] for match in IMAGE_LINE.finditer(manifest) if match[3].startswith("ghcr.io/")]
    if remaining:
        raise ValueError(f"Unpinned GHCR chart images: {', '.join(remaining)}")
    return manifest


if __name__ == "__main__":
    if sys.argv[1:] == ["--self-test"]:
        pins = [
            {"source": "ghcr.io/microsoft/retina/retina-agent:v1", "destination": "mcr.microsoft.com/containernetworking/retina-agent@sha256:" + "a" * 64, "count": 2},
            {"source": "ghcr.io/microsoft/retina/retina-init:v1", "destination": "mcr.microsoft.com/containernetworking/retina-init@sha256:" + "b" * 64, "count": 1},
            {"source": "ghcr.io/microsoft/retina/retina-operator:v1", "destination": "mcr.microsoft.com/containernetworking/retina-operator@sha256:" + "c" * 64, "count": 1},
        ]
        sample = "\n".join(f"image: {image['source']}" for image in pins for _ in range(image["count"]))
        assert pin(sample + "\nimage: busybox", pins).count("@sha256:") == 4
        for invalid in (sample.replace("v1", "v0"), sample.replace("retina-init", "other")):
            try:
                pin(invalid, pins)
            except ValueError:
                pass
            else:
                raise AssertionError("Image drift was not rejected")
        try:
            pin(sample + "\nimage: ghcr.io/other/image:v1", pins)
        except ValueError:
            pass
        else:
            raise AssertionError("Unpinned GHCR image was not rejected")
    else:
        pins = json.loads(Path(__file__).with_name("retina-image-pins.json").read_text(encoding="utf-8"))
        sys.stdout.write(pin(sys.stdin.read(), pins["images"]))
