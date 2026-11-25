import hashlib
from contextlib import redirect_stdout
from io import StringIO
import json
import os
from datetime import datetime, timezone
from pathlib import Path
import runpy
import shutil
import subprocess
from types import SimpleNamespace
from tempfile import TemporaryDirectory
from unittest.mock import patch

import yaml

from filter_evidence import stage
import scan_qualys
import scan_trivy
from scan_trivy import compare, main, print_comparison, scan_published_image


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


def test_comparison_cli_fails_on_findings_and_blockers(tmp_path):
    args = ["--baseline-image", "baseline", "--producer-scan", str(tmp_path / "producer"),
            "--platform", "linux/amd64", "--output", str(tmp_path / "comparison")]
    with patch("scan_trivy.run_comparison") as run, \
            patch("scan_trivy.print_comparison", side_effect=lambda result, _: {
                "PASS": 0, "FAIL": 1, "BLOCKED": 2,
            }[result["verdict"]]):
        run.return_value = {"verdict": "FAIL"}
        assert main(args) == 1
        run.return_value = {"verdict": "BLOCKED"}
        assert main(args) == 2
        run.side_effect = ValueError("scan failed")
        try:
            main(args)
        except ValueError:
            pass
        else:
            raise AssertionError("Scan errors must not be reported as findings")


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
    assert "mcr.microsoft.com/tvm-containers/shavsa/comet:v1-linux-amd64" in command
    assert token not in command and kwargs["input"] == token + "\n"
    assert kwargs["stdout"] is subprocess.DEVNULL and kwargs["stderr"] is subprocess.DEVNULL
    assert any(command[:3] == ["docker", "image", "rm"] for command, _ in calls)
    report.assert_called_once_with(directory)


def test_windows_qualys_scanner_uses_process_isolation_without_capturing_output(tmp_path):
    token = "example-oidc-token"
    calls = []

    def fake_run(command, **kwargs):
        calls.append((command, kwargs))
        if command[:3] == ["docker", "image", "inspect"]:
            return SimpleNamespace(returncode=0, stdout="windows/amd64")
        if command[:2] == ["docker", "run"]:
            return SimpleNamespace(returncode=1, stdout="token=" + token)
        return SimpleNamespace(returncode=0, stdout="")

    directory = tmp_path / "windows"
    with patch("scan_qualys.oidc_token", return_value=token), \
            patch("scan_qualys.subprocess.run", side_effect=fake_run), \
            patch("scan_qualys.report") as report:
        scan_qualys.scan("example@sha256:" + "d" * 64, directory, "scan-volume", windows=True)
    command, kwargs = next((command, kwargs) for command, kwargs in calls if command[:2] == ["docker", "run"])
    assert command[command.index("--isolation") + 1] == "process"
    assert "qualys_binaries_volume:C:\\qualys_binaries" in command
    assert "scan-volume:C:\\internal" in command
    assert "mcr.microsoft.com/tvm-containers/shavsa/comet:v1-windows-amd64" in command
    assert token not in command and kwargs["input"] == token + "\n"
    assert kwargs["stdout"] is subprocess.DEVNULL and kwargs["stderr"] is subprocess.DEVNULL
    assert token not in (directory / "scan.log").read_text(encoding="utf-8")
    report.assert_called_once_with(directory)

    directory = tmp_path / "failed-windows"
    with patch("scan_qualys.oidc_token", return_value=token), \
            patch("scan_qualys.subprocess.run", side_effect=lambda command, **kwargs:
                  SimpleNamespace(returncode=254 if command[:2] == ["docker", "run"] else 0,
                                  stdout="windows/amd64")), \
            patch("scan_qualys.report") as report:
        try:
            scan_qualys.scan("example@sha256:" + "d" * 64, directory, "scan-volume", windows=True)
        except ValueError as error:
            assert "exit code 254" in str(error)
        else:
            raise AssertionError("Windows CoMET execution failure was accepted")
    report.assert_not_called()


def test_windows_qualys_compares_matching_image_children(tmp_path):
    repository = "mcr.microsoft.com/azuremonitor/containerinsights"
    baseline = repository + "/ciprod/prometheus-collector/images@sha256:" + "a" * 64
    manifest = "sha256:" + "f" * 64
    candidates = [
        {"os": "windows", "arch": "amd64", "osVersion": f"10.0.{build}.123",
         "ref": repository + "/cidev/prometheus-collector/images@sha256:" + digest * 64}
        for build, digest in (("17763", "b"), ("20348", "c"))
    ]
    calls = []
    owner = ""

    def fake_run(command, **kwargs):
        nonlocal owner
        calls.append(command)
        if "--label" in command:
            owner = command[command.index("--label") + 1].split("=", 1)[1]
        return SimpleNamespace(stdout=owner if command[:3] == ["docker", "volume", "inspect"] else "")

    def fake_scan(image, directory, volume, windows):
        assert windows and volume == owner
        directory.mkdir()
        release = "2019" if image == candidates[0]["ref"] else "2022"
        if image == baseline:
            release = directory.parent.name
        extra = "<VULN><QID>2</QID><FLAGS>v</FLAGS></VULN>" if image != baseline else ""
        absent = "" if image != baseline else "<Q>2</Q>"
        (directory / "result.xml").write_text(
            "<HOST><PARTIAL_REPORT_FLAG>false</PARTIAL_REPORT_FLAG><SCA_SCAN_ENABLED>true</SCA_SCAN_ENABLED>"
            f"<OS>Windows Server {release}</OS><VULNSIGS_VERSION>sig</VULNSIGS_VERSION>"
            "<AGENT_MANIFEST_VERSION>manifest</AGENT_MANIFEST_VERSION>"
            "<REPORT_GENERATED_DATE>now</REPORT_GENERATED_DATE><VULN><QID>1</QID><FLAGS>v</FLAGS></VULN>"
            f"{extra}<VULNS_NOT_FOUND>{absent}</VULNS_NOT_FOUND></HOST>", encoding="utf-8")
        (directory / "result.json").write_text("{}", encoding="utf-8")

    with patch("scan_qualys.resolve_tag", return_value=candidates) as resolve, \
            patch("scan_qualys.subprocess.run", side_effect=fake_run), \
            patch("scan_qualys.scan", side_effect=fake_scan) as scan:
        for release, candidate in zip(("2019", "2022"), candidates):
            directory = tmp_path / release
            directory.mkdir()
            result = scan_qualys.run_windows(baseline, manifest, release, directory)
            assert result["candidate"]["image"] == candidate["ref"]
            assert result["verdict"] == "FINDINGS"
            assert result["qids"] == [{"qid": "1", "status": "OPEN"}, {"qid": "2", "status": "NEW"}]
            assert result["scanner"] == scan_qualys.WINDOWS_SCANNER
            assert json.loads((directory / "comparison.json").read_text()) == result
        assert scan.call_count == 4
        assert all(call.args == ("collector", manifest) for call in resolve.call_args_list)
    assert sum(command[:4] == ["docker", "volume", "rm", "qualys_binaries_volume"] for command in calls) == 2


def test_windows_qualys_requires_immutable_manifest(tmp_path):
    digest = "sha256:" + "f" * 64
    with patch("scan_trivy.registry_json", return_value=({"manifests": []}, digest)) as registry:
        assert scan_trivy.resolve_tag("collector", digest) == []
        registry.assert_called_once_with("manifests", digest)
    with patch("scan_qualys.resolve_tag") as resolve:
        try:
            scan_qualys.run_windows("baseline", "mutable-tag", "2019", tmp_path)
        except ValueError as error:
            assert "digest" in str(error)
        else:
            raise AssertionError("A mutable Windows candidate tag was accepted")
        resolve.assert_not_called()


def test_windows_qualys_does_not_remove_existing_scanner_volume(tmp_path):
    candidate = {"os": "windows", "arch": "amd64", "osVersion": "10.0.17763.123",
                 "ref": "mcr.microsoft.com/azuremonitor/containerinsights/cidev/prometheus-collector/images@sha256:" + "b" * 64}
    baseline = "mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images@sha256:" + "a" * 64
    calls = []

    def fake_run(command, **kwargs):
        calls.append(command)
        return SimpleNamespace(stdout="qualys_binaries_volume\n" if command[:3] == ["docker", "volume", "ls"] else "")

    with patch("scan_qualys.resolve_tag", return_value=[candidate]), \
            patch("scan_qualys.subprocess.run", side_effect=fake_run):
        try:
            scan_qualys.run_windows(baseline, "sha256:" + "f" * 64, "2019", tmp_path)
        except ValueError as error:
            assert "already exists" in str(error)
        else:
            raise AssertionError("An existing scanner volume was reused")
    assert not any(command[:4] == ["docker", "volume", "rm", "qualys_binaries_volume"] for command in calls)
    assert any(command[:3] == ["docker", "volume", "rm"] for command in calls)


def test_windows_qualys_cli_reports_findings_and_blocks_errors(tmp_path):
    for verdict, expected in (("FINDINGS", 0), ("BLOCKED", 2)):
        output = tmp_path / verdict
        args = ["scan_qualys.py", "--baseline-image", "baseline", "--windows-manifest-digest", "sha256:" + "f" * 64,
                "--windows-release", "2019", "--output", str(output)]
        result = {"verdict": verdict, "scope": "Windows Server 2019/amd64", "notCovered": [],
                  "qids": [{"qid": "1", "status": "OPEN"}]}
        with patch("sys.argv", args), patch("scan_qualys.run_windows", return_value=result), \
                patch("scan_qualys.stage") as stage, redirect_stdout(StringIO()) as text:
            assert scan_qualys.main() == expected
        assert '"qid": "1"' in text.getvalue()
        stage.assert_called_once()

    args[-1] = str(tmp_path / "error")
    with patch("sys.argv", args), patch("scan_qualys.run_windows", side_effect=ValueError("scanner failed")), \
            patch("scan_qualys.stage") as stage:
        try:
            scan_qualys.main()
        except ValueError as error:
            assert str(error) == "scanner failed"
        else:
            raise AssertionError("Windows scanner error was masked")
    stage.assert_called_once()


def test_comparison_logging_filters_secret_values(tmp_path):
    result = {
        "verdict": "FAIL", "scope": "test", "findingPolicy": "test",
        "baseline": {"requestedRef": "baseline"},
        "candidate": {"requestedRef": "candidate"},
        "baselineFindings": [{"status": "OPEN", "finding": {"path": "token=fixture-value",
                                                            "package": "example-secret-value"},
                              "candidateFinding": None}],
        "newFindings": [],
    }
    output = StringIO()
    with patch.dict(os.environ, {"EXAMPLE_SECRET": "example-secret-value"}), redirect_stdout(output):
        print_comparison(result, tmp_path)
    assert "fixture-value" not in output.getvalue()
    assert "example-secret-value" not in output.getvalue()
    assert "[REDACTED]" in output.getvalue()


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


def test_retained_ampls_fixture_is_not_reinitialized():
    script = Path(__file__).resolve().parents[2] / "otelcollector" / "test" / "scenarios" / "run-scenario.py"
    initialize = runpy.run_path(str(script))["initialize_ampls"]
    outputs = {key: "/" + key for key in ("clusterId", "dceId", "dcrId", "ruleAssociationId",
                                          "configurationAssociationId")}
    for reused, enabled, expected in ((True, False, 0), (True, True, 4), (False, False, 3)):
        calls = []
        initialize(SimpleNamespace(reused=reused, az=lambda *args: calls.append(args)), outputs, enabled)
        assert len(calls) == expected
        assert enabled == any("metrics.enabled=false" in call[-1] for call in calls)


def test_network_probe_monitor_isolated_from_preexisting_resources():
    script = Path(__file__).resolve().parents[2] / "otelcollector" / "test" / "scenarios" / "run-scenario.py"
    module = runpy.run_path(str(script))
    run_id = "133337-ampls-proxy-1-1"

    class FakeHost:
        def __init__(self, *existing):
            self.args = SimpleNamespace(run_id=run_id)
            self.scenario = {"platforms": [], "clusterFeatures": ["ampls"]}
            self.network_probe_name = "scenario-network-probe-" + hashlib.sha256(
                run_id.encode("utf-8")).hexdigest()[:16]
            self.created_network_probe_monitor = False
            self.existing = set(existing)
            self.monitor = None
            self.calls = []

        def assert_owned(self):
            pass

        def kube(self, *args, data=None, **kwargs):
            self.calls.append((args, data))
            if args[0] == "get":
                if "--ignore-not-found" in args:
                    name = args[2]
                    return (f"servicemonitor.azmonitoring.coreos.com/{name}"
                            if name in self.existing else "")
                return {"spec": self.monitor["spec"]}
            if args[0] in {"create", "apply"}:
                self.monitor = yaml.safe_load(data)
                self.existing.add(self.network_probe_name)
            elif args[0] == "delete" and "servicemonitors.azmonitoring.coreos.com" in args:
                self.existing.discard(args[2])
            return ""

    legacy = FakeHost("scenario-network-probe")
    module["network_probe_monitor"](legacy, run_id)
    assert legacy.network_probe_name in legacy.existing
    module["cleanup_reference_apps"](legacy)
    assert legacy.existing == {"scenario-network-probe"}
    legacy_deletes = [args for args, _ in legacy.calls
                      if args[0] == "delete" and "servicemonitors.azmonitoring.coreos.com" in args]
    assert len(legacy_deletes) == 1 and legacy.network_probe_name in legacy_deletes[0]

    occupied = FakeHost("scenario-network-probe-" + hashlib.sha256(
        run_id.encode("utf-8")).hexdigest()[:16])
    try:
        module["network_probe_monitor"](occupied, run_id)
    except RuntimeError as error:
        assert "refusing to overwrite" in str(error)
    else:
        raise AssertionError("An existing run-specific monitor was overwritten")
    module["cleanup_reference_apps"](occupied)
    assert not any(args[0] == "delete" and "servicemonitors.azmonitoring.coreos.com" in args
                   for args, _ in occupied.calls)

    owned = FakeHost()
    module["network_probe_monitor"](owned, run_id)
    assert owned.monitor["metadata"]["name"] == owned.network_probe_name
    assert owned.created_network_probe_monitor
    module["cleanup_reference_apps"](owned)
    monitor_deletes = [args for args, _ in owned.calls
                       if args[0] == "delete" and "servicemonitors.azmonitoring.coreos.com" in args]
    assert len(monitor_deletes) == 1 and owned.network_probe_name in monitor_deletes[0]
    assert not owned.created_network_probe_monitor


def test_pipeline_evidence_jobs_use_build_jobs():
    template = Path(__file__).parent / "templates" / "azure-template-e2e-stages.yml"
    stages = yaml.safe_load(template.read_text(encoding="utf-8"))["stages"]
    scan = next(stage for stage in stages if stage.get("stage") == "VulnerabilityScan")
    trivy = next(step for step in scan["jobs"][0]["steps"]
                 if step.get("displayName") == "Compare build and baseline with a shared Trivy DB")
    assert "--report-findings-only" not in trivy["bash"] and not trivy.get("continueOnError")
    windows = next(job for job in scan["jobs"] if job.get("job") == "WindowsQualys")
    assert set(windows["strategy"]["matrix"]) == {"WS2019", "WS2022"}
    assert windows["pool"]["os"] == "windows"
    assert windows["templateContext"]["outputs"][0]["condition"] == "always()"
    assert "WindowsMultiArch_Prometheus_Collector.outputs['sreWindowsManifest.digest']" in windows["variables"]["WINDOWS_MANIFEST_DIGEST"]
    pipeline = yaml.safe_load((Path(__file__).parent.parent / "azure-pipeline-build.yml").read_text(encoding="utf-8"))
    build = next(stage for stage in pipeline["extends"]["parameters"]["stages"] if stage.get("stage") == "Build")
    producer = next(job for job in build["jobs"] if job.get("job") == "WindowsMultiArch_Prometheus_Collector")
    push = next(step for step in producer["steps"] if step.get("name") == "sreWindowsManifest")
    assert "$digest = docker manifest push" in push["powershell"]
    assert "docker manifest create $(WINDOWS_FULL_IMAGE_NAME) `" in push["powershell"]
    assert '"$imageRepository@$windows2019Digest" "$imageRepository@$windows2022Digest"' in push["powershell"]
    assert "$(WINDOWS_FULL_IMAGE_NAME)-$(WINDOWS_2019_BASE_IMAGE_VERSION)" not in push["powershell"]
    assert "$(WINDOWS_FULL_IMAGE_NAME)-$(WINDOWS_2022_BASE_IMAGE_VERSION)" not in push["powershell"]
    create = push["powershell"].index("docker manifest create")
    assert "if ($LASTEXITCODE -ne 0)" in push["powershell"][create:push["powershell"].index("$digest =")]
    assert "variable=digest;isOutput=true]$digest" in push["powershell"]
    pushes = {
        job["job"]: next(step for step in job["steps"] if step.get("name") == step_name)
        for job, step_name in (
            (next(job for job in build["jobs"] if job.get("job") == "Windows2019_Prometheus_Collector"),
             "pushWindows2019"),
            (next(job for job in build["jobs"] if job.get("job") == "Windows2022_Prometheus_Collector"),
             "pushWindows2022"),
        )
    }
    for job_name, step in pushes.items():
        assert "$pushOutput = docker push " in step["powershell"]
        assert "variable=imageDigest;isOutput=true]$imageDigest" in step["powershell"]
        assert f"dependencies.{job_name}.outputs" in producer["variables"][
            f"WINDOWS_{'2019' if '2019' in job_name else '2022'}_IMAGE_DIGEST"
        ]
    scenarios = next(value[0] for stage in stages for key, value in stage.items()
                     if key.startswith("${{ each scenario"))
    scenario_stages = next(stage for stage in stages
                           if any(key.startswith("${{ each scenario") for key in stage))
    scenario_stage = next(iter(scenario_stages.values()))[0]
    assert scenario_stage["dependsOn"] == ["Build", "TestRunnerImage"]
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
        test_comparison_cli_fails_on_findings_and_blockers(tmp_path)
        test_stage_redacts_credential_fields(tmp_path)
        test_qualys_passes_token_over_stdin(tmp_path)
        test_windows_qualys_scanner_uses_process_isolation_without_capturing_output(tmp_path)
        test_windows_qualys_compares_matching_image_children(tmp_path)
        test_windows_qualys_requires_immutable_manifest(tmp_path)
        test_windows_qualys_does_not_remove_existing_scanner_volume(tmp_path)
        test_windows_qualys_cli_reports_findings_and_blocks_errors(tmp_path)
        test_comparison_logging_filters_secret_values(tmp_path)
        test_candidate_workflow_rendering()
        test_retained_ampls_fixture_is_not_reinitialized()
        test_network_probe_monitor_isolated_from_preexisting_resources()
        test_pipeline_evidence_jobs_use_build_jobs()
