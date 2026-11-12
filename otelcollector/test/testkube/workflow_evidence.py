"""Render immutable candidate workflows and validate retained Ginkgo evidence."""

import argparse
import json
from pathlib import Path
import re
import shlex
import sys
import xml.etree.ElementTree as ET


def render_candidate(source, workflow, sha, image, label_filter=None, expectations=None):
    import yaml

    if not re.fullmatch(r"[0-9a-f]{40}", sha):
        raise ValueError("candidate test source must be a full lowercase Git SHA")
    if not re.fullmatch(r"[^@\s]+@sha256:[0-9a-f]{64}", image):
        raise ValueError("candidate runner must be repository@sha256:digest")
    matches = [
        item for item in yaml.safe_load_all(source)
        if item and item.get("kind") == "TestWorkflow"
        and item.get("metadata", {}).get("name") == workflow
    ]
    if len(matches) != 1:
        raise ValueError("select exactly one existing TestWorkflow by metadata.name")
    document = matches[0]
    spec = document["spec"]
    spec["content"]["git"]["revision"] = sha
    spec.setdefault("pod", {}).setdefault("nodeSelector", {}).update({
        "kubernetes.io/os": "linux", "kubernetes.io/arch": "amd64",
    })
    spec["pod"].setdefault("labels", {})["azure.workload.identity/use"] = "true"
    spec["pod"]["serviceAccountName"] = "testkube-api-server-tests-job"
    paths = spec["content"]["git"].setdefault("paths", [])
    if "otelcollector/test/scenarios/settings" not in paths:
        paths.append("otelcollector/test/scenarios/settings")
    runs = 0
    for step in spec["steps"]:
        shell = step.get("shell", "")
        lines = shell.splitlines()
        for index, line in enumerate(lines):
            if not line.strip().startswith("ginkgo run "):
                continue
            args = shlex.split(line)
            suite = args[-1]
            if not re.fullmatch(r"\./[A-Za-z][A-Za-z0-9]*", suite):
                raise ValueError("candidate workflow must select an existing Ginkgo suite")
            options = []
            offset = 2
            while offset < len(args) - 1:
                arg = args[offset]
                if arg in {"--json-report", "--junit-report", "--label-filter"}:
                    value = args[offset + 1]
                    offset += 2
                    if arg == "--label-filter" and label_filter is None:
                        options.extend([arg, value])
                    continue
                if arg.startswith("--label-filter=") and label_filter is not None:
                    offset += 1
                    continue
                if not arg.startswith(("--json-report=", "--junit-report=")) and arg != "--fail-on-empty":
                    options.append(arg)
                offset += 1
            if label_filter is not None:
                if not label_filter.strip():
                    raise ValueError("candidate label override cannot be empty")
                options.extend(["--label-filter", label_filter])
            options.extend(["--fail-on-empty", "--json-report=/data/reports/ginkgo.json",
                            "--junit-report=/data/reports/junit.xml", "."])
            lines[index] = (
                "GOTOOLCHAIN=\"$(go env GOVERSION)\" "
                "go run github.com/onsi/ginkgo/v2/ginkgo@"
                "\"$(go list -m github.com/onsi/ginkgo/v2 | cut -d ' ' -f 2)\" run "
                + shlex.join(options)
            )
            container = step["container"]
            container["image"] = image
            container["workingDir"] = "/data/repo/otelcollector/test/ginkgo-e2e/" + suite[2:]
            env = container.setdefault("env", [])
            values = {"GOTOOLCHAIN": "auto", "GOFLAGS": "-mod=readonly"}
            if suite == "./operator":
                values["EXPECT_OPERATOR_HTTPS"] = "true"
            values.update(expectations or {})
            env[:] = [item for item in env if item["name"] not in values]
            env.extend({"name": name, "value": value} for name, value in values.items())
            runs += 1
        if "go run github.com/onsi/ginkgo/v2/ginkgo@" in "\n".join(lines):
            step["shell"] = "mkdir -p /data/reports\n" + "\n".join(
                line for line in lines if line.strip() != "mkdir -p /data/reports"
            ) + "\n"
    if runs != 1:
        raise ValueError("candidate must contain exactly one Ginkgo invocation")
    spec["steps"] = [step for step in spec["steps"] if "artifacts" not in step]
    spec["steps"].append({
        "name": "Retain Ginkgo evidence", "condition": "always",
        "artifacts": {"workingDir": "/data/reports", "paths": ["**/*"]},
    })
    return document


def report_counts(artifact_dir):
    root = Path(artifact_dir).resolve()
    if any(path.is_symlink() or not path.resolve().is_relative_to(root) for path in root.rglob("*")):
        raise ValueError("artifact tree contains a symbolic link or escaped path")
    reports = {}
    for name in ("ginkgo.json", "junit.xml"):
        paths = list(root.rglob(name))
        if len(paths) != 1 or not paths[0].resolve().is_relative_to(root):
            raise ValueError(f"expected exactly one confined {name}")
        reports[name] = paths[0]
    suites = json.loads(reports["ginkgo.json"].read_text(encoding="utf-8"))
    if not isinstance(suites, list) or not suites:
        raise ValueError("missing Ginkgo suite reports")
    reports_all = [spec for suite in suites for spec in suite.get("SpecReports", [])]
    specs = [spec for spec in reports_all if spec.get("LeafNodeType") == "It"]
    states = [spec.get("State") for spec in specs]
    counts = {
        "specs": len(specs),
        "passed": states.count("passed"),
        "skipped": states.count("skipped") + states.count("pending"),
        "failed": sum(state not in {"passed", "skipped", "pending"} for state in states),
    }
    if not all(suite.get("SuiteSucceeded") is True for suite in suites):
        raise ValueError("Ginkgo suite did not succeed")
    if any(suite.get("SuiteConfig", {}).get("DryRun") for suite in suites):
        raise ValueError("dry runs are not execution evidence")
    if any(spec.get("State") not in {"passed", "skipped", "pending"} for spec in reports_all):
        raise ValueError("Ginkgo contains an unsuccessful spec or setup node")
    if not counts["passed"] or counts["failed"]:
        raise ValueError(f"no passing specs or unsuccessful Ginkgo specs: {counts}")
    cases = ET.parse(reports["junit.xml"]).getroot().findall(".//testcase")
    passed = 0
    for case in cases:
        tags = {child.tag.rsplit("}", 1)[-1] for child in case}
        if tags & {"failure", "error"}:
            raise ValueError("JUnit contains failed cases")
        passed += "skipped" not in tags and case.get("name", "").startswith("[It]")
    if not passed or passed != counts["passed"]:
        raise ValueError("JUnit and Ginkgo passing counts do not agree")
    return counts


def validate_summary(results, expected, evidence_root=None):
    actual = set()
    for path in results:
        result = json.loads(Path(path).read_text(encoding="utf-8"))
        name = result["environment"]
        if name in actual:
            raise ValueError(f"duplicate result for {name}")
        actual.add(name)
        executions = result.get("executions")
        if result.get("status") != "passed" or not executions:
            raise ValueError(f"{name}: missing executions or nonpassed result")
        for execution in executions:
            if (execution.get("status") != "passed" or not execution.get("execution_id")
                    or execution.get("passed", 0) <= 0 or execution.get("specs", 0) <= 0
                    or execution.get("failed") != 0):
                raise ValueError(f"{name}: unsuccessful or empty workflow evidence")
            artifacts = Path(execution["artifact_dir"])
            if not artifacts.is_absolute() or (evidence_root is not None
                    and not artifacts.resolve().is_relative_to(Path(evidence_root).resolve())):
                raise ValueError(f"{name}: artifact directory must be absolute and confined")
            for filename in ("submission.json", "execution.json"):
                receipt = json.loads((artifacts.parent / filename).read_text(encoding="utf-8"))
                if receipt.get("id") != execution["execution_id"]:
                    raise ValueError(f"{name}: retained execution ID does not match submission")
                if filename == "execution.json" and (
                        receipt.get("workflow", {}).get("name") != execution["workflow"]
                        or receipt.get("result", {}).get("status") != "passed"):
                    raise ValueError(f"{name}: retained workflow execution did not pass")
            counts = report_counts(artifacts)
            if any(execution.get(key) != value for key, value in counts.items()):
                raise ValueError(f"{name}: retained report counts do not match summary")
    if actual != set(expected):
        raise ValueError(f"expected scenarios {sorted(expected)}, received {sorted(actual)}")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    commands = parser.add_subparsers(dest="command", required=True)
    render = commands.add_parser("render")
    render.add_argument("source", type=Path)
    render.add_argument("workflow")
    render.add_argument("sha")
    render.add_argument("image")
    render.add_argument("--label-filter")
    render.add_argument("--expected-cluster")
    render.add_argument("--amw-query-endpoint")
    render.add_argument("--azure-client-id")
    render.add_argument("--expect-operator-https", choices=("true", "false"))
    render.add_argument("--expect-windows-exporter", choices=("true", "false"))
    reports = commands.add_parser("reports")
    reports.add_argument("artifact_dir")
    gate = commands.add_parser("gate")
    gate.add_argument("results_dir", type=Path)
    gate.add_argument("expected", nargs="+")
    args = parser.parse_args()
    try:
        if args.command == "render":
            expectations = {
                name.upper(): value for name, value in {
                    "expected_cluster": args.expected_cluster,
                    "amw_query_endpoint": args.amw_query_endpoint,
                    "azure_client_id": args.azure_client_id,
                    "expect_operator_https": args.expect_operator_https,
                    "expect_windows_exporter": args.expect_windows_exporter,
                }.items() if value is not None
            }
            result = render_candidate(args.source.read_text(encoding="utf-8"), args.workflow,
                                      args.sha, args.image, args.label_filter, expectations)
        elif args.command == "reports":
            result = report_counts(args.artifact_dir)
        else:
            validate_summary(args.results_dir.glob("testkube-results-*.json"), args.expected,
                             args.results_dir.parent / "testkube-executions")
            result = {"status": "passed", "scenarios": args.expected}
        print(json.dumps(result))
    except (ValueError, KeyError, TypeError, OSError, ET.ParseError) as error:
        print(f"TestKube evidence rejected: {error}", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
