import shlex
import shutil
import subprocess
from pathlib import Path

import pytest
import yaml


def run_prerequisites(*, bicep_on_path=False, config_dir=None, install_fails=False):
    template = yaml.safe_load(
        (Path(__file__).parent / "templates" / "azure-template-scenario.yml").read_text()
    )
    script = next(
        step["bash"] for step in template["steps"]
        if step.get("displayName") == "Install scenario host prerequisites"
    )
    setup = r"""
PATH=/__scenario_test_no_executables__
export PATH
export HOME=/home/cloudtest
export AZURE_BICEP_USE_BINARY_FROM_PATH=true
MOCK_INSTALLED=0
sudo() {
  if [[ "$1" == install ]]; then
    printf 'COPY:%s:%s\n' "$4" "$5"
    [[ "$MOCK_INSTALLED" == 1 ]] || return 1
    bicep() { echo BICEP:verified; }
  fi
}
az() {
  printf 'AZ:%s:%s\n' "$*" "$AZURE_BICEP_USE_BINARY_FROM_PATH"
  [[ "$MOCK_AZ_FAIL" == 0 ]] || return 1
  if [[ "$AZURE_BICEP_USE_BINARY_FROM_PATH" != false ]]; then
    command -v bicep >/dev/null || return 1
    return 0
  fi
  MOCK_INSTALLED=1
}
kubectl() { :; }
unzip() { :; }
jq() { :; }
envsubst() { :; }
"""
    setup += f"\nMOCK_AZ_FAIL={int(install_fails)}\n"
    setup += (
        f"export AZURE_CONFIG_DIR={shlex.quote(config_dir)}\n"
        if config_dir is not None
        else "unset AZURE_CONFIG_DIR\n"
    )
    if bicep_on_path:
        setup += "bicep() { echo BICEP:preinstalled; }\n"
    bash = shutil.which("bash")
    assert bash
    return subprocess.run([bash, "-c", setup + script], text=True, capture_output=True)


def test_prerequisites_reuse_bicep_on_path():
    result = run_prerequisites(bicep_on_path=True)
    assert result.returncode == 0, result.stderr
    assert "BICEP:preinstalled" in result.stdout
    assert "AZ:bicep install" not in result.stdout
    assert "COPY:" not in result.stdout
    assert "##vso[task.setvariable variable=AZURE_BICEP_USE_BINARY_FROM_PATH]true" in result.stdout


@pytest.mark.parametrize(
    ("config_dir", "source"),
    [(None, "/home/cloudtest/.azure/bin/bicep"), ("/agent/config space", "/agent/config space/bin/bicep")],
)
def test_prerequisites_install_bicep_when_missing(config_dir, source):
    result = run_prerequisites(config_dir=config_dir)
    assert result.returncode == 0, result.stderr
    assert "AZ:bicep install:false" in result.stdout
    assert f"COPY:{source}:/usr/local/bin/bicep" in result.stdout
    assert "BICEP:verified" in result.stdout
    assert "##vso[task.setvariable variable=AZURE_BICEP_USE_BINARY_FROM_PATH]true" in result.stdout


def test_prerequisites_fail_when_bicep_install_fails():
    result = run_prerequisites(install_fails=True)
    assert result.returncode != 0
    assert "COPY:" not in result.stdout
    assert "##vso[task.setvariable variable=AZURE_BICEP_USE_BINARY_FROM_PATH]true" not in result.stdout
