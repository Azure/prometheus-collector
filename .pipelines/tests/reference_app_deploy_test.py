import os
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile

import yaml


root = Path(__file__).resolve().parents[2]
template = Path(sys.argv[1]) if len(sys.argv) > 1 else root / ".pipelines/azure-template-deploy-reference-apps.yml"
script = yaml.safe_load(template.read_text(encoding="utf-8"))["steps"][0]["inputs"]["inlineScript"]
repository = "mcr.microsoft.com/azuremonitor/containerinsights/cidev/prometheus-collector/images"
images = {name: f"{repository}@sha256:{digit * 64}" for name, digit in (
    ("REFAPP_GO", "1"), ("REFAPP_PYTHON", "2"), ("REFAPP_WIN_GO", "3"), ("REFAPP_WIN_PYTHON", "4")
)}

with tempfile.TemporaryDirectory() as temp:
    directory = Path(temp)
    manifests = directory / "reference-app"
    manifests.mkdir()
    for name in ("prometheus-reference-app.yaml", "win-prometheus-reference-app.yaml"):
        shutil.copy(root / "internal/referenceapp" / name, manifests / name)
    bash = os.environ.get("BASH") or shutil.which("bash")
    assert bash
    temp_unix = subprocess.check_output([bash, "-c", "pwd"], cwd=directory, text=True).strip()
    script = (script.replace("$(Agent.TempDirectory)", temp_unix)
              .replace("$(Pipeline.Workspace)", temp_unix)
              .replace("$(System.JobId)", "refapp-test"))
    script = """az() { :; }
curl() { :; }
kubectl() {
  if [ "$1" = apply ]; then cat >> "$TEST_OUTPUT"
  elif [ "$1" = get ] && [ "$WINDOWS" = yes ]; then echo node/windows
  fi
}
""" + script
    output = f"{temp_unix}/applied"
    env = dict(os.environ, **images, REFAPP_RESOURCE_GROUP="test", REFAPP_CLUSTER_NAME="test",
               TEST_OUTPUT=output)
    for windows in ("no", "yes"):
        applied = directory / "applied"
        applied.unlink(missing_ok=True)
        subprocess.run([bash], input=script.encode(), cwd=root, env=dict(env, WINDOWS=windows), check=True)
        manifest = applied.read_text(encoding="utf-8")
        assert images["REFAPP_GO"] in manifest and images["REFAPP_PYTHON"] in manifest
        assert (images["REFAPP_WIN_GO"] in manifest) == (windows == "yes")
        assert (images["REFAPP_WIN_PYTHON"] in manifest) == (windows == "yes")
        deployments = [doc for doc in yaml.safe_load_all(manifest) if doc and doc.get("kind") == "Deployment"]
        for deployment in deployments:
            go_container = next(c for c in deployment["spec"]["template"]["spec"]["containers"]
                                if c["name"].endswith("golang"))
            assert {"name": "BASIC_AUTH_PORT", "value": "2115"} in go_container["env"]
    applied.unlink()
    result = subprocess.run([bash], input=script.encode(), cwd=root,
                            env=dict(env, WINDOWS="no", REFAPP_PYTHON="missing"), capture_output=True)
    assert result.returncode != 0 and not applied.exists()
print("Reference app deploy checks passed")
