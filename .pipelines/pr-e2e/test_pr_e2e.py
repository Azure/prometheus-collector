import hashlib
import json
import os
from datetime import datetime, timezone
from pathlib import Path
import runpy
import shutil
from types import SimpleNamespace
from tempfile import TemporaryDirectory
from unittest.mock import patch

import yaml

from filter_evidence import stage
import scan_qualys
from scan_trivy import compare, main, scan_published_image


def test_published_image_scan_retains_the_selected_child(tmp_path):
    images = [
        {"os": "linux", "arch": "amd64", "ref": "repo@sha256:" + "a" * 64},
        {"os": "linux", "arch": "arm64", "ref": "repo@sha256:" + "b" * 64},
    ]
    image = "containerinsightsprod.azurecr.io/public/azuremonitor/containerinsights/cidev/prometheus-collector/images:tag"
    output = tmp_path / "artifact"

    def fake_trivy(command, **kwargs):
        if "--version" in command:
            return SimpleNamespace(returncode=0, stdout=json.dumps({
                "Version": "test-version",
                "VulnerabilityDB": {"UpdatedAt": datetime.now(timezone.utc).isoformat()},
            }))
        if "convert" in command:
            return SimpleNamespace(returncode=0)
        assert command[command.index("--scanners") + 1] == "vuln,secret"
        assert "--list-all-pkgs" in command
        assert "--ignore-unfixed" in command
        assert command[command.index("--format") + 1] == "json"
        assert kwargs["env"]["TRIVY_DB_REPOSITORY"] == "mirror"
        assert kwargs["env"]["TRIVY_JAVA_DB_REPOSITORY"] == "java-mirror"
        assert "TRIVY_SECRET_CONFIG" not in kwargs["env"]
        result = Path(command[command.index("--output") + 1])
        result.parent.mkdir(parents=True, exist_ok=True)
        result.write_text(json.dumps({
            "Metadata": {
                "RepoDigests": [images[0]["ref"]],
                "ImageConfig": {"os": "linux", "architecture": "amd64"},
                "OS": {"Family": "alpine", "Name": "3.20"},
            },
            "Results": [{
                "Target": images[0]["ref"] + " (alpine 3.20)",
                "Class": "os-pkgs",
                "Type": "alpine",
                "Packages": [],
                "Vulnerabilities": [],
            }],
        }))
        database = Path(command[command.index("--cache-dir") + 1]) / "db"
        database.mkdir(parents=True)
        (database / "trivy.db").write_bytes(b"database")
        return SimpleNamespace(returncode=0, stdout="")

    with patch("scan_trivy.resolve_tag", return_value=images) as resolve_tag, \
            patch("scan_trivy.subprocess.run", side_effect=fake_trivy) as trivy, \
            patch.dict(os.environ, {
                "TRIVY_DB_REPOSITORY": "mirror",
                "TRIVY_JAVA_DB_REPOSITORY": "java-mirror",
                "TRIVY_SECRET_CONFIG": "ignored",
            }):
        scan_published_image(image, "linux/amd64", output)

    resolve_tag.assert_called_once_with("collector", "tag")
    assert trivy.call_count == 3
    result = json.loads((output / "trivy" / "result.json").read_text())
    assert result["Metadata"]["RepoDigests"] == [images[0]["ref"]]
    assert (output / "cache" / "db").is_dir()
    assert not (output / "scan-work").exists()
    metadata = json.loads((output / "trivy" / "metadata.json").read_text())
    assert metadata["requestedRef"] == images[0]["ref"]
    assert metadata["configuration"]["scanners"] == ["vuln", "secret"]

    baseline = tmp_path / "baseline"
    shutil.copytree(output, baseline)
    for directory, ref in ((baseline, images[1]["ref"]), (output, images[0]["ref"])):
        result_path = directory / "trivy" / "result.json"
        data = json.loads(result_path.read_text())
        data["Metadata"]["RepoDigests"] = [ref]
        data["Results"][0]["Target"] = ref + " (alpine 3.20)"
        if directory == output:
            data["Results"].append({"Target": "secret-file", "Class": "secret", "Type": "secret"})
        raw = json.dumps(data).encode()
        result_path.write_bytes(raw)
        metadata_path = directory / "trivy" / "metadata.json"
        metadata = json.loads(metadata_path.read_text())
        metadata["requestedRef"] = ref
        metadata["resultSha256"] = hashlib.sha256(raw).hexdigest()
        metadata_path.write_text(json.dumps(metadata))
    assert compare(baseline, output)["verdict"] == "PASS"


def test_comparison_cli_uses_producer_scan(tmp_path):
    baseline = "mcr.example/repo@sha256:" + "c" * 64
    producer = tmp_path / "producer"
    output = tmp_path / "comparison"
    result = {"verdict": "PASS"}
    with patch("scan_trivy.run_comparison", return_value=result) as run, \
            patch("scan_trivy.print_comparison", return_value=0) as report:
        assert main(["--baseline-image", baseline, "--producer-scan", str(producer),
                     "--platform", "linux/amd64", "--output", str(output)]) == 0
    run.assert_called_once_with(baseline, "linux/amd64", output.resolve(), producer.resolve())
    report.assert_called_once_with(result, output.resolve())


def test_stage_redacts_credential_fields(tmp_path):
    evidence = tmp_path / "private"
    evidence.mkdir()
    (evidence / "scan.log").write_text("password=hidden\n")
    output = tmp_path / "published"
    stage(evidence, output)
    assert (output / "scan.log").read_text() == "password=[REDACTED]\n"


def test_qualys_passes_token_over_stdin(tmp_path):
    token = "example-oidc-token"
    calls = []

    def fake_run(command, **kwargs):
        calls.append((command, kwargs))
        return SimpleNamespace(returncode=0, stdout="linux/amd64")

    directory = tmp_path / "qualys"
    with patch("scan_qualys.oidc_token", return_value=token), \
            patch("scan_qualys.subprocess.run", side_effect=fake_run), \
            patch("scan_qualys.report") as report:
        scan_qualys.scan("example@sha256:" + "d" * 64, directory, "scan-volume")

    scanner_calls = [(command, kwargs) for command, kwargs in calls if command[:2] == ["docker", "run"]]
    assert len(scanner_calls) == 1
    command, kwargs = scanner_calls[0]
    assert token not in command and kwargs["input"] == token + "\n"
    assert any(command[:3] == ["docker", "image", "rm"] for command, _ in calls)
    report.assert_called_once_with(directory)


def test_candidate_workflow_rendering():
    testkube = Path(__file__).resolve().parents[2] / "otelcollector" / "test" / "testkube"
    render = runpy.run_path(str(testkube.parent / "scenarios" / "run-scenario.py"))["render_workflow"]
    source = (testkube / "config-processing-test-crs" /
              "testkube-config-test-default-targets-on-crs.yaml").read_text(encoding="utf-8")
    sha = "a" * 40
    image = "mcr.example/runner@sha256:" + "b" * 64
    result = render(source, "configprocessingcommon", sha, image, "selected", {"EXPECTED_CLUSTER": "cluster"})
    spec = result["spec"]
    step = spec["steps"][0]
    assert spec["content"]["git"]["revision"] == sha
    assert step["container"]["image"] == image
    assert {"name": "EXPECTED_CLUSTER", "value": "cluster"} in step["container"]["env"]
    assert "selected" in step["shell"] and "--junit-report=/data/reports/junit.xml" in step["shell"]
    assert spec["steps"][-1]["condition"] == "always" and "artifacts" in spec["steps"][-1]
    query_source = (testkube / "testkube-test-crs.yaml").read_text(encoding="utf-8")
    query = render(query_source, "querymetrics", sha, image, "managed-acns", {"EXPECTED_CLUSTER": "cluster"})
    query_env = query["spec"]["steps"][0]["container"]["env"]
    for name, field in (("SCENARIO_POD_NAME", "metadata.name"), ("SCENARIO_POD_NAMESPACE", "metadata.namespace"),
                        ("SCENARIO_POD_UID", "metadata.uid")):
        assert {"name": name, "valueFrom": {"fieldRef": {"fieldPath": field}}} in query_env
    assert {"name": "SCENARIO_PROBE_IMAGE", "value": image} in query_env
    try:
        render(source, "missing-workflow", sha, image)
    except ValueError:
        pass
    else:
        raise AssertionError("A missing TestWorkflow was accepted")


def test_pipeline_evidence_jobs_use_build_jobs():
    template = Path(__file__).parent / "templates" / "azure-template-e2e-stages.yml"
    stages = yaml.safe_load(template.read_text(encoding="utf-8"))["stages"]
    scan = next(stage for stage in stages if stage.get("stage") == "VulnerabilityScan")
    scenarios = next(value[0] for stage in stages for key, value in stage.items()
                     if key.startswith("${{ each scenario"))
    lock, scenario = scenarios["jobs"]
    for job in (scan["jobs"][0], scenario):
        assert job["templateContext"]["outputs"]
        assert job["templateContext"].get("type", "buildJob") in ("buildJob", "agentJob")
    assert scenarios["lockBehavior"] == "sequential"
    assert lock["templateContext"] == {"type": "releaseJob", "isProduction": False}
    assert lock["environment"] == "Prometheus-Collector"
    assert scenario["dependsOn"] == lock["deployment"]


if __name__ == "__main__":
    with TemporaryDirectory() as directory:
        tmp_path = Path(directory)
        test_published_image_scan_retains_the_selected_child(tmp_path)
        test_comparison_cli_uses_producer_scan(tmp_path)
        test_stage_redacts_credential_fields(tmp_path)
        test_qualys_passes_token_over_stdin(tmp_path)
        test_candidate_workflow_rendering()
        test_pipeline_evidence_jobs_use_build_jobs()
