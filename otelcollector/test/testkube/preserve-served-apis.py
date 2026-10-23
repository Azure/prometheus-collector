import json
from pathlib import Path
import re
import sys


def preserve(manifest, baseline):
    versions = {d["metadata"]["name"]: {v["name"]: v["served"] for v in d["spec"]["versions"]}
                for d in baseline["items"]}
    documents = re.split(r"(?m)(?=^---\s*$)", manifest)
    for i, document in enumerate(documents):
        if not re.search(r"(?m)^kind: CustomResourceDefinition$", document):
            continue
        name = re.search(r"(?m)^  name: (\S+)$", document)
        if not name:
            raise ValueError("CRD name missing")
        previous = versions.get(name[1], {})
        layout = re.search(r"(?m)^  versions:\n( +)- ", document)
        if not layout:
            raise ValueError(f"Version list missing in {name[1]}")
        indent = layout[1]
        parts = re.split(rf"(?m)(?=^{indent}- \S)", document)
        found = set()
        for j, part in enumerate(parts):
            version = re.search(rf"(?m)^{indent}(?:- |  )name: (\S+)$", part)
            if not version:
                if re.search(rf"(?m)^{indent}  served:", part):
                    raise ValueError(f"Version name missing in {name[1]}")
                continue
            if version[1] in previous:
                found.add(version[1])
                served = str(previous[version[1]]).lower()
                parts[j], count = re.subn(rf"(?m)^{indent}  served: (true|false)$", indent + "  served: " + served, part)
                if count != 1:
                    raise ValueError(f"Ambiguous served flag in {name[1]}/{version[1]}")
        if found != previous.keys():
            raise ValueError(f"Existing API versions missing in {name[1]}: {previous.keys() - found}")
        documents[i] = "".join(parts)
    return "".join(documents)


if __name__ == "__main__":
    if sys.argv[1:] == ["--self-test"]:
        sample = "kind: CustomResourceDefinition\nmetadata:\n  name: tests.tests.testkube.io\nspec:\n  versions:\n  - name: v1\n    served: false\n    storage: false\n  - name: v3\n    served: true\n    storage: true\n"
        baseline = {"items": [{"metadata": {"name": "tests.tests.testkube.io"}, "spec": {"versions": [{"name": "v1", "served": True}]}}]}
        result = preserve(sample, baseline)
        assert result.count("served: true") == 2 and "storage: false" in result
        assert preserve(sample, {"items": []}) == sample
        for invalid in (sample.replace("  - name: v1", "  - missing: v1"), sample.replace("served: false", "served: 'false'")):
            try:
                preserve(invalid, baseline)
            except ValueError:
                pass
            else:
                raise AssertionError("Invalid CRD version layout was accepted")
    else:
        if len(sys.argv) != 2:
            raise SystemExit("Usage: preserve-served-apis.py <live-crds.json>")
        sys.stdout.write(preserve(sys.stdin.read(), json.loads(Path(sys.argv[1]).read_text(encoding="utf-8-sig"))))
