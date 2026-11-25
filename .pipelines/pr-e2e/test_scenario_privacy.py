import ast
import json
import os
import runpy
import shutil
import sys
import uuid
import xml.etree.ElementTree as ET
from contextlib import nullcontext
from pathlib import Path
from types import SimpleNamespace
from unittest.mock import patch

import pytest


RUNNER = Path(__file__).resolve().parents[2] / "otelcollector" / "test" / "scenarios" / "run-scenario.py"


def load_runner():
    with patch.dict(sys.modules, {"scan_trivy": SimpleNamespace(resolve_tag=None)}):
        return runpy.run_path(str(RUNNER))


def write_report_pair(path, status, secret, *, complete=True, missing=None):
    path.mkdir(parents=True, exist_ok=True)
    failure = (f'<failure message="{secret}">{secret}</failure>' if status == "failed"
               else "<skipped/>" if status == "skipped" else "")
    junit = (
        f'<testsuite name="example suite" tests="2">'
        f'<testcase name="example">{failure}<system-out>{secret}</system-out></testcase>'
        f'<testcase name="{secret}">{failure}</testcase></testsuite>'
    )
    location = {"FileName": "/src/otelcollector/test/ginkgo-e2e/querymetrics/suite_test.go", "LineNumber": 3}
    specs = [{
        "ContainerHierarchyTexts": ["example group"],
        "LeafNodeType": "It", "LeafNodeText": name, "State": status,
        "LeafNodeLocation": location,
        "Failure": {"Location": location, "Message": secret},
        "CapturedGinkgoWriterOutput": secret, "CapturedStdOutErr": secret,
    } for name in ("example", secret)]
    ginkgo = [{"SuiteDescription": "example suite", "SuiteSucceeded": status != "failed", "SpecReports": specs}]
    if missing != "all" and missing != "junit":
        (path / "junit.xml").write_text(junit if complete else "<testsuite>")
    if missing != "all" and missing != "ginkgo":
        (path / "ginkgo.json").write_text(json.dumps(ginkgo))
    (path / "environment.json").write_text(json.dumps({"env": [{"name": "ARBITRARY", "value": secret}]}))


def test_image_audit_keeps_derived_evidence_without_logging_pod_environment():
    runner = load_runner()
    digest = "a" * 64
    image = "example/image@sha256:" + digest
    secret = "scenario-test-secret"
    nodes = {"items": [{
        "metadata": {"name": "node"},
        "status": {"nodeInfo": {"operatingSystem": "linux", "architecture": "amd64", "kernelVersion": "6.8"}},
    }]}
    pods = {"items": [{
        "metadata": {"name": "collector"},
        "spec": {"nodeName": "node", "containers": [{
            "name": "prometheus-collector", "image": image,
            "env": [{"name": "PASSWORD", "value": secret}],
        }]},
        "status": {"containerStatuses": [{
            "name": "prometheus-collector", "ready": True, "imageID": "containerd://sha256:" + digest,
        }]},
    }]}

    def run(command, **_):
        if "nodes" in command:
            stdout = json.dumps(nodes)
        elif "pods" in command:
            stdout = json.dumps(pods)
        else:
            stdout = "safe command output"
        return SimpleNamespace(returncode=0, stdout=stdout, stderr="")

    logs = []

    class Log:
        def __enter__(self):
            return self

        def __exit__(self, *_):
            pass

        def write(self, value):
            logs.append(value)

    commands = runner["Commands"].__new__(runner["Commands"])
    commands.evidence = Path("unused-evidence")
    commands.stages = []
    commands.deadline = None
    host = runner["Host"](SimpleNamespace(evidence=commands.evidence, kubectl="kubectl", run_id="test-run"), {}, commands)
    config = {
        "containerRoles": {"prometheus-collector": "collector"},
        "requiredOnEveryNode": {"linux": ["collector"]},
        "requiredInCluster": ["collector"],
    }
    images = [{"name": "collector", "os": "linux", "arch": "amd64", "ref": image}]
    with patch("shutil.which", return_value="kubectl"), patch("subprocess.run", side_effect=run), \
            patch.object(Path, "open", return_value=Log()), \
            patch.dict(runner["Commands"].run.__globals__, {"save": lambda *_: None}):
        evidence = runner["audit_images"](host, images, config)
        commands.run(["kubectl", "version"])

    assert evidence[0]["pod"] == "collector" and evidence[0]["image"] == image
    assert secret not in json.dumps(evidence)
    assert secret not in "".join(logs)
    assert "safe command output" in "".join(logs)


def test_sensitive_scenario_reads_are_private():
    tree = ast.parse(RUNNER.read_text(encoding="utf-8"))
    kinds = {"pod", "pods", "po", "deployment", "deployments", "deploy",
             "daemonset", "daemonsets", "ds", "replicaset", "replicasets", "rs", "nodes"}
    unsafe = []
    for call in (node for node in ast.walk(tree) if isinstance(node, ast.Call)):
        if not isinstance(call.func, ast.Attribute) or not call.args:
            continue
        args = call.args
        sensitive = False
        if call.func.attr == "kube" and isinstance(args[0], ast.Constant):
            operation = args[0].value
            if operation == "get" and any(isinstance(arg, ast.Constant) and arg.value == "json" for arg in args):
                resource = args[1]
                sensitive = not isinstance(resource, ast.Constant) or any(
                    part.split("/", 1)[0] in kinds for part in str(resource.value).split(",")
                )
            elif operation == "exec":
                sensitive = any(
                    isinstance(arg, ast.Constant) and "$HTTP_PROXY" in str(arg.value)
                    or isinstance(arg, ast.Name) and arg.id == "config_path"
                    for arg in args
                )
        elif call.func.attr == "run" and isinstance(args[0], (ast.List, ast.Tuple)):
            constants = {arg.value for arg in args[0].elts if isinstance(arg, ast.Constant)}
            sensitive = ({"testkube", "testworkflowexecution", "get", "json"} <= constants
                         or {"testkube", "testworkflowexecution", "abort"} <= constants)
        if sensitive and not any(
            keyword.arg == "private" and isinstance(keyword.value, ast.Constant) and keyword.value.value is True
            for keyword in call.keywords
        ):
            unsafe.append(call.lineno)
    assert not unsafe, f"Secret-bearing scenario reads can be logged at lines: {unsafe}"


@pytest.mark.parametrize("mode", [
    "passed", "failed", "settled", "download-fails", "passed-download-fails", "write-fails", "write-recovers",
    "missing-reports", "missing-junit", "missing-ginkgo", "existing-empty", "empty-reports",
    "invalid-junit", "invalid-ginkgo", "invalid-state", "report-mismatch", "split-reports",
    "junit-status-mismatch", "skipped-only", "suite-failed",
])
def test_publication_keeps_test_results_without_raw_execution_environment(mode):
    root = Path(".scenario-privacy-" + uuid.uuid4().hex)
    root.mkdir()
    try:
        repo = root / "repo"
        templates = repo / "otelcollector" / "test" / "testkube"
        templates.mkdir(parents=True)
        (templates / "testkube-test-crs.yaml").write_text("kind: TestWorkflow\n")
        source = repo / "otelcollector" / "test" / "ginkgo-e2e" / "querymetrics" / "suite_test.go"
        source.parent.mkdir(parents=True)
        source.write_text('package querymetrics\nDescribe("example group", func() {\nIt("example", func() {})\n})\n')
        destination = root / "evidence" / "case"
        secret = "scenario-execution-secret"
        environment = [{"name": "ARBITRARY", "value": secret}]
        status = "passed" if mode in {
            "passed", "passed-download-fails", "missing-reports", "missing-junit", "missing-ginkgo",
            "existing-empty", "empty-reports", "invalid-junit", "invalid-ginkgo", "invalid-state", "report-mismatch",
            "split-reports", "junit-status-mismatch", "skipped-only", "suite-failed",
        } else "failed"
        execution = {
            "id": "execution-id",
            "workflow": {"name": "common", "spec": {"container": {"env": environment}}},
            "resolvedWorkflow": {"spec": {"container": {"env": environment}}},
            "result": {
                "status": status,
                "statusDetails": {"type": "init-failure", "reason": "unschedulable",
                                  "message": secret, "user": secret},
                "initialization": {"status": "failed", "errorReason": "unschedulable",
                                   "errorMessage": secret},
                "steps": {"step-id": {"status": "failed", "errorReason": "exit-code",
                                      "exitCode": 17, "attempts": 2, "errorMessage": secret},
                          secret: {"status": secret, "errorReason": secret}},
            },
        }
        def write_artifacts(path, complete=True):
            missing = {
                "missing-reports": "all", "missing-junit": "junit", "missing-ginkgo": "ginkgo",
                "existing-empty": "all",
            }.get(mode)
            report_status = "skipped" if mode == "skipped-only" else status
            write_report_pair(path, report_status, secret, complete=complete, missing=missing)
            if mode == "invalid-junit":
                (path / "junit.xml").write_text("<testsuite>")
            if mode == "invalid-ginkgo":
                (path / "ginkgo.json").write_text("{")
            if mode == "empty-reports":
                (path / "junit.xml").write_text("<testsuite/>")
                (path / "ginkgo.json").write_text(json.dumps([{"SuiteSucceeded": True, "SpecReports": []}]))
            if mode == "split-reports":
                (path / "nested").mkdir()
                (path / "ginkgo.json").rename(path / "nested" / "ginkgo.json")
            if mode == "junit-status-mismatch":
                junit_path = path / "junit.xml"
                junit_path.write_text(junit_path.read_text().replace('name="example"', 'name="example" status="failed"'))
            if mode in {"invalid-state", "report-mismatch", "suite-failed"}:
                report = json.loads((path / "ginkgo.json").read_text())
                if mode == "invalid-state":
                    report[0]["SpecReports"][0]["State"] = [secret]
                elif mode == "report-mismatch":
                    report[0]["SpecReports"][0]["State"] = "failed"
                else:
                    report[0]["SuiteSucceeded"] = False
                (path / "ginkgo.json").write_text(json.dumps(report))

        class Commands:
            deadline = None

            def __init__(self):
                self.private_downloads = []

            def run(self, command, **kwargs):
                if command[0] == "bash":
                    case = Path(command[-1]) / "execution-id"
                    write_artifacts(case / "artifacts", complete=mode not in {
                        "settled", "download-fails", "passed-download-fails",
                    })
                    (case / "submission.json").write_text(json.dumps({
                        "id": "execution-id", "workflow": {"spec": {"container": {"env": environment}}},
                    }))
                    if mode in {"settled", "download-fails", "passed-download-fails"}:
                        raise RuntimeError("interrupted launcher")
                    (case / "execution.json").write_text(json.dumps(execution))
                    if mode == "existing-empty":
                        public = destination / "testkube-executions" / "ConfigTests" / "common" / "execution-id"
                        (public / "artifacts").mkdir(parents=True)
                        (public / "submission.json").write_text(json.dumps({"id": "execution-id"}))
                        result = runner["public_testkube_result"](execution["result"])
                        (public / "execution.json").write_text(json.dumps({
                            "id": "execution-id", "workflow": {"name": "common"}, "result": result,
                        }))
                        (public / "record.json").write_text(json.dumps({
                            "workflow": "common", "execution_id": "execution-id", "status": "passed",
                            "artifact_dir": str((public / "artifacts").resolve()),
                        }))
                    return ""
                if "download" in command:
                    self.private_downloads.append(kwargs.get("private"))
                    write_artifacts(Path(command[command.index("--download-dir") + 1]),
                                    complete=mode not in {"download-fails", "passed-download-fails"})
                    if mode in {"download-fails", "passed-download-fails", "write-fails"}:
                        raise RuntimeError("artifact recovery failed")
                    return ""
                if "get" in command:
                    return execution
                return ""

        args = SimpleNamespace(
            source_sha="a" * 40, kubectl="kubectl", run_id=str(uuid.uuid4()), evidence=destination,
        )
        host = SimpleNamespace(
            args=args, scenario={"platforms": ["linux"]}, query_endpoint="https://example",
            query_client_id="client", cluster_name="cluster", kubeconfig=destination / "kubeconfig",
            commands=Commands(), kube=lambda *_, **__: "",
        )
        runner = load_runner()
        run_suite = runner["run_suite"]
        globals_override = {"REPO": repo.resolve(), "render_workflow": lambda *_: {"kind": "TestWorkflow"}}
        image = "example/runner@sha256:" + "b" * 64

        real_write = Path.write_text
        write_attempts = []

        def partial_write(path, text, *args, **kwargs):
            if (path.name == "junit.xml" and path.parent.name == "artifacts"
                    and path.parent.parent.name.startswith(("publication-", "report-"))):
                if mode == "write-recovers" and write_attempts:
                    return real_write(path, text, *args, **kwargs)
                write_attempts.append(True)
                real_write(path, "incomplete")
                raise OSError("artifact write failed")
            return real_write(path, text, *args, **kwargs)

        write_failure = (patch.object(Path, "write_text", new=partial_write)
                         if mode in {"write-fails", "write-recovers"} else nullcontext())
        with patch.dict(run_suite.__globals__, globals_override), \
                patch.dict(os.environ, {"TEST_RUNNER_IMAGE": image}), write_failure:
            if mode == "passed":
                assert run_suite(host, "querymetrics", destination, "", workflow="common")["status"] == "passed"
            else:
                with pytest.raises(RuntimeError):
                    run_suite(host, "querymetrics", destination, "", workflow="common")
        assert not host.workflow_unsettled
        assert host.commands.private_downloads == ([True] if mode in
                                                   {"settled", "download-fails", "passed-download-fails",
                                                    "write-fails", "write-recovers",
                                                    "missing-reports", "missing-junit", "missing-ginkgo",
                                                    "existing-empty", "empty-reports", "invalid-junit", "invalid-ginkgo",
                                                    "invalid-state", "report-mismatch", "split-reports",
                                                    "junit-status-mismatch"} else [])

        filter_path = RUNNER.parents[3] / ".pipelines" / "pr-e2e" / "build" / "filter_evidence.py"
        stage = runpy.run_path(str(filter_path))["stage"]
        publication = root / "publication"
        stage(root / "evidence", publication, environment={})
        case = publication / "case" / "testkube-executions" / "ConfigTests" / "common" / "execution-id"
        published_execution = json.loads((case / "execution.json").read_text())
        assert published_execution["result"]["status"] == status
        assert published_execution["result"]["statusDetails"]["reason"] == "unschedulable"
        assert published_execution["result"]["initialization"]["errorReason"] == "unschedulable"
        assert published_execution["result"]["steps"][0]["exitCode"] == 17
        assert published_execution["result"]["steps"][1]["errorReason"] == "other"
        if mode in {"download-fails", "passed-download-fails", "write-fails",
                    "missing-reports", "missing-junit", "missing-ginkgo", "existing-empty", "empty-reports",
                    "invalid-junit", "invalid-ginkgo", "invalid-state", "report-mismatch",
                    "split-reports", "junit-status-mismatch"}:
            assert not (case / "artifacts").exists()
            summary = json.loads((publication / "case" / "testkube-results-ConfigTests.json").read_text())
            assert summary["status"] == "failed" and summary["artifact_status"] == "unavailable"
        else:
            report = ET.parse(case / "artifacts" / "junit.xml").getroot()
            cases = list(report.iter("testcase"))
            assert len(cases) == (3 if mode == "suite-failed" else 2) and "example" in cases[0].get("name", "")
            assert int(report.get("tests")) == len(cases)
            assert secret not in cases[1].get("name", "")
            assert all((item.find("failure") is not None) == (status == "failed") for item in cases[:2])
            specs = json.loads((case / "artifacts" / "ginkgo.json").read_text())[0]["SpecReports"]
            report_status = "skipped" if mode == "skipped-only" else status
            assert [item["State"] for item in specs] == [report_status, report_status]
            assert specs[0]["LeafNodeText"] == "example" and specs[1]["LeafNodeText"] != secret
            if status == "failed":
                assert specs[0]["Failure"]["Location"]["LineNumber"] == 3
            if mode in {"skipped-only", "suite-failed"}:
                summary = json.loads((publication / "case" / "testkube-results-ConfigTests.json").read_text())
                assert summary["status"] == "failed" and summary["artifact_status"] == "available"
                if mode == "suite-failed":
                    assert cases[-1].find("failure") is not None
        contents = "".join(path.read_text(encoding="utf-8") for path in publication.rglob("*") if path.is_file())
        assert secret not in contents
    finally:
        shutil.rmtree(root)


def test_summary_write_failure_is_atomic_and_publication_can_be_reconciled():
    root = Path(".scenario-privacy-" + uuid.uuid4().hex)
    root.mkdir()
    try:
        destination = root / "case"
        case = destination / ".azure" / "testkube-executions" / "ConfigTests" / "common" / "execution-id"
        (case / "artifacts").mkdir(parents=True)
        (case / "submission.json").write_text(json.dumps({"id": "execution-id"}))
        (case / "execution.json").write_text(json.dumps({
            "id": "execution-id", "workflow": {"name": "common"}, "result": {"status": "failed"},
        }))
        write_report_pair(case / "artifacts", "failed", "summary-private-secret")
        summary = destination / "testkube-results-ConfigTests.json"
        original = {"status": "running", "executions": []}
        summary.write_text(json.dumps(original))
        publish = load_runner()["publish_testkube_evidence"]
        save = publish.__globals__["save"]
        failed = []

        def fail_summary_once(path, value):
            if "testkube-results-ConfigTests.json" in path.name and not failed:
                failed.append(True)
                path.write_text("partial")
                raise OSError("injected summary write failure")
            return save(path, value)

        with patch.dict(publish.__globals__, {"save": fail_summary_once}):
            with pytest.raises(OSError):
                publish(destination, "common")
            assert json.loads(summary.read_text()) == original
            publish(destination, "common")
        result = json.loads(summary.read_text())
        assert result["status"] == "failed"
        assert result["executions"][0]["execution_id"] == "execution-id"
        assert len(list(ET.parse(destination / "testkube-executions" / "ConfigTests" / "common"
                                 / "execution-id" / "artifacts" / "junit.xml").getroot().iter("testcase"))) == 2
    finally:
        shutil.rmtree(root)


def test_failed_partial_report_removal_never_publishes_partial_data():
    root = Path(".scenario-privacy-" + uuid.uuid4().hex)
    root.mkdir()
    try:
        destination = root / "case"
        case = destination / ".azure" / "testkube-executions" / "ConfigTests" / "common" / "execution-id"
        (case / "artifacts").mkdir(parents=True)
        (case / "submission.json").write_text(json.dumps({"id": "execution-id"}))
        (case / "execution.json").write_text(json.dumps({
            "id": "execution-id", "workflow": {"name": "common"}, "result": {"status": "passed"},
        }))
        write_report_pair(case / "artifacts", "passed", "report-private-secret")
        (destination / "testkube-results-ConfigTests.json").write_text('{"status":"running","executions":[]}')
        publish = load_runner()["publish_testkube_evidence"]
        real_rmtree = shutil.rmtree
        real_write = Path.write_text

        def partial_write(path, text, *args, **kwargs):
            if path.name == "junit.xml" and path.is_relative_to(case) and path.parent != case / "artifacts":
                real_write(path, "partial-secret")
                raise OSError("write failed")
            return real_write(path, text, *args, **kwargs)

        def fail_partial_removal(path, *args, **kwargs):
            path = Path(path)
            if path.is_relative_to(case) and path != case:
                raise OSError("cannot remove partial report")
            return real_rmtree(path, *args, **kwargs)

        with patch.object(Path, "write_text", new=partial_write), \
                patch.object(shutil, "rmtree", side_effect=fail_partial_removal):
            with pytest.raises((OSError, RuntimeError)):
                publish(destination, "common")
        public = destination / "testkube-executions" / "ConfigTests" / "common" / "execution-id"
        assert json.loads((public / "execution.json").read_text())["result"]["status"] == "passed"
        assert not (public / "artifacts").exists()
        summary = json.loads((destination / "testkube-results-ConfigTests.json").read_text())
        assert summary["status"] == "failed" and summary["artifact_status"] == "unavailable"
        filter_path = RUNNER.parents[3] / ".pipelines" / "pr-e2e" / "build" / "filter_evidence.py"
        stage = runpy.run_path(str(filter_path))["stage"]
        publication = root / "publication"
        stage(destination, publication, environment={})
        contents = "".join(path.read_text(encoding="utf-8") for path in publication.rglob("*") if path.is_file())
        assert "partial-secret" not in contents and "report-private-secret" not in contents
    finally:
        shutil.rmtree(root)


@pytest.mark.parametrize("scope,linked", [
    ("case", ("submission.json",)), ("case", ("execution.json",)),
    ("case", ("artifacts", "junit.xml")), ("destination", (".azure",)),
    ("destination", (".azure", "testkube-executions", "ConfigTests", "common")),
])
def test_linked_execution_evidence_cannot_be_published_or_used_to_abort(scope, linked):
    root = Path(".scenario-privacy-" + uuid.uuid4().hex)
    root.mkdir()
    try:
        destination = root / "case"
        case = destination / ".azure" / "testkube-executions" / "ConfigTests" / "common" / "execution-id"
        (case / "artifacts").mkdir(parents=True)
        (case / "submission.json").write_text(json.dumps({"id": "execution-id"}))
        (case / "execution.json").write_text(json.dumps({
            "id": "execution-id", "workflow": {"name": "common"}, "result": {"status": "failed"},
        }))
        (case / "artifacts" / "junit.xml").write_text("<testsuite/>")
        (destination / "testkube-submission-started").touch()
        (destination / "testkube-results-ConfigTests.json").write_text('{"executions":[]}')
        calls = []
        host = SimpleNamespace(
            args=SimpleNamespace(kubectl="kubectl"),
            commands=SimpleNamespace(deadline=None, run=lambda *args, **kwargs: calls.append(args)),
        )
        runner = load_runner()
        original_is_symlink = Path.is_symlink
        link_path = (case if scope == "case" else destination).joinpath(*linked)
        is_symlink = lambda path: path == link_path or original_is_symlink(path)
        with patch.object(Path, "is_symlink", is_symlink):
            if scope == "case" and linked[0] == "artifacts":
                with pytest.raises(ValueError, match="links"):
                    runner["publish_testkube_evidence"](destination, "common")
            else:
                assert not runner["settle_workflow"](host, destination, "common", {})
                if scope == "destination":
                    with pytest.raises(ValueError, match="links"):
                        runner["publish_testkube_evidence"](destination, "common")
        assert not calls
        assert not (destination / "testkube-executions").exists()
    finally:
        shutil.rmtree(root)
