import json
import re
import sys
from pathlib import Path

SERVER = "ciosstestimageacrmirror.azurecr.io"
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
    remaining = [match[3] for match in IMAGE_LINE.finditer(manifest)
                 if not match[3].startswith(SERVER + "/")]
    if remaining:
        raise ValueError(f"Unmirrored chart images: {', '.join(remaining)}")
    return manifest


if __name__ == "__main__":
    if sys.argv[1:] == ["--self-test"]:
        sample = 'image: "retina-agent:v1"\n- image: busybox\n'
        result = pin(sample, [
            {"source": "retina-agent:v1", "destination": SERVER + "/agent@sha256:" + "a" * 64, "count": 1},
            {"source": "busybox", "destination": SERVER + "/busybox@sha256:" + "b" * 64, "count": 1},
        ])
        assert result.count(SERVER + "/") == 2
        for invalid in (sample.replace("busybox", "other"), sample.replace("retina-agent:v1", "old")):
            try:
                pin(invalid, [
                    {"source": "retina-agent:v1", "destination": SERVER + "/agent@sha256:" + "a" * 64, "count": 1},
                    {"source": "busybox", "destination": SERVER + "/busybox@sha256:" + "b" * 64, "count": 1},
                ])
            except ValueError:
                pass
            else:
                raise AssertionError("Image drift was not rejected")
    else:
        pins = json.loads(Path(__file__).with_name("retina-image-pins.json").read_text(encoding="utf-8"))
        sys.stdout.write(pin(sys.stdin.read(), pins["images"]))
