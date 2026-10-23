from pathlib import Path
import re


root = Path(__file__).resolve().parents[3]
pipeline = (root / ".pipelines" / "azure-pipeline-build.yml").read_text()
job = pipeline.split("      - job: Arc_Conformace_Test_Image\n", 1)[1].split(
    "      - job:", 1
)[0]
argument = re.search(r'--build-arg "GOLANG_VERSION=\$\((\w+)\)"', job)
assert argument, "Arc conformance must pass its Go toolchain to the Dockerfile"
variables = dict(re.findall(r"^  (\w+): '([\d.]+)'$", pipeline, re.MULTILINE))
version = tuple(map(int, variables[argument[1]].split(".")))
modules = sorted((root / "otelcollector" / "test" / "ginkgo-e2e").glob("*/go.mod"))
assert modules, "No Ginkgo modules found"
for module in modules:
    minimum = re.search(r"^go ([\d.]+)$", module.read_text(), re.MULTILINE)
    assert minimum, f"Missing Go directive: {module}"
    required = tuple(map(int, minimum[1].split(".")))
    assert version >= required, (
        f"Arc Go {variables[argument[1]]} cannot build {module.relative_to(root)} "
        f"(requires {minimum[1]})"
    )
print(f"Arc Go {variables[argument[1]]} satisfies every Ginkgo module")
