import copy
from contextlib import redirect_stdout
from datetime import datetime, timezone
import hashlib
import inspect
import io
import json
from pathlib import Path
import stat
import tempfile
import unittest
import urllib.parse
import urllib.request
from unittest.mock import patch
import zipfile

import fetch_candidate


def protected_scan_artifact(source, image):
    database = b"trivy database"
    result = json.dumps({"Metadata": {"RepoDigests": [image["ref"]],
                                      "ImageConfig": {"os": "linux", "architecture": "amd64"}},
                         "Results": [{"Target": "Debian 12", "Class": "os-pkgs", "Type": "debian",
                                      "Packages": [], "Vulnerabilities": []}]}).encode()
    empty_ignore = b""
    empty_config = b"{}\n"
    files = {
        "trivy/result.json": result,
    }
    metadata = {
        "exitCode": 0,
        "requestedRef": image["ref"],
        "platform": "linux/amd64",
        "version": "0.76.1",
        "dbUpdatedAt": datetime.now(timezone.utc).isoformat(),
        "finishedUtc": datetime.now(timezone.utc).isoformat(),
        "dbSha256": hashlib.sha256(database).hexdigest(),
        "resultSha256": hashlib.sha256(result).hexdigest(),
        "configuration": {
            "scanners": ["vuln"],
            "listAllPkgs": True,
            "ignoreSha256": hashlib.sha256(empty_ignore).hexdigest(),
            "configSha256": hashlib.sha256(empty_config).hexdigest(),
        },
    }
    files["trivy/metadata.json"] = json.dumps(metadata).encode()
    provenance = {
        "schemaVersion": 1,
        "producerDefinitionId": 440,
        "source": source,
        "image": image,
        "coverage": fetch_candidate.SCAN_COVERAGE,
        "files": {name: hashlib.sha256(content).hexdigest() for name, content in files.items()},
    }
    archive = io.BytesIO()
    with zipfile.ZipFile(archive, "w") as output:
        for name, content in files.items():
            output.writestr(fetch_candidate.SCAN_ARTIFACT + "/" + name, content)
        output.writestr(fetch_candidate.SCAN_ARTIFACT + "/provenance.json", json.dumps(provenance))
    return archive.getvalue()


class FetchTests(unittest.TestCase):
    def test_report_only_artifact_preserves_database_identity_without_bytes(self):
        source = {"repo": "Azure/prometheus-collector", "commit": "a" * 40, "buildId": "123"}
        image = {"name": "collector", "ref": "image@sha256:" + "b" * 64,
                 "indexRef": "image@sha256:" + "c" * 64, "os": "linux", "arch": "amd64"}
        with tempfile.TemporaryDirectory(dir=Path(__file__).parent) as directory:
            output = Path(directory) / "scan"
            fetch_candidate.extract_scan_artifact(protected_scan_artifact(source, image), output)
            metadata = fetch_candidate.validate_scan_artifact(output, {"source": source, "images": [image]})
            self.assertEqual(metadata["dbSha256"], hashlib.sha256(b"trivy database").hexdigest())
            self.assertEqual({path.relative_to(output).as_posix()
                              for path in output.rglob("*") if path.is_file()},
                             {"trivy/result.json", "trivy/metadata.json", "provenance.json"})

    def test_report_only_artifact_rejects_invalid_database_identity_and_hidden_cache(self):
        source = {"repo": "Azure/prometheus-collector", "commit": "a" * 40, "buildId": "123"}
        image = {"name": "collector", "ref": "image@sha256:" + "b" * 64,
                 "indexRef": "image@sha256:" + "c" * 64, "os": "linux", "arch": "amd64"}
        for invalid in (None, "", "not-a-hash", "A" * 64, 42, "cache"):
            with self.subTest(invalid=invalid), \
                    tempfile.TemporaryDirectory(dir=Path(__file__).parent) as directory:
                output = Path(directory) / "scan"
                fetch_candidate.extract_scan_artifact(protected_scan_artifact(source, image), output)
                provenance_path = output / "provenance.json"
                provenance = json.loads(provenance_path.read_text())
                if invalid == "cache":
                    path = output / "cache" / "db" / "trivy.db"
                    path.parent.mkdir(parents=True)
                    path.write_bytes(b"database")
                    provenance["files"]["cache/db/trivy.db"] = hashlib.sha256(path.read_bytes()).hexdigest()
                else:
                    path = output / "trivy" / "metadata.json"
                    metadata = json.loads(path.read_text())
                    metadata["dbSha256"] = invalid
                    path.write_text(json.dumps(metadata))
                    provenance["files"]["trivy/metadata.json"] = hashlib.sha256(path.read_bytes()).hexdigest()
                provenance_path.write_text(json.dumps(provenance))
                with self.assertRaises(ValueError):
                    fetch_candidate.validate_scan_artifact(output, {"source": source, "images": [image]})

    def test_scan_extraction_keeps_bounded_total_and_rejects_unsafe_members(self):
        for kind in ("total", "entry", "traversal", "symlink", "duplicate"):
            with self.subTest(kind=kind), \
                    tempfile.TemporaryDirectory(dir=Path(__file__).parent) as directory:
                raw = io.BytesIO()
                with zipfile.ZipFile(raw, "w", compression=zipfile.ZIP_DEFLATED) as archive:
                    if kind == "total":
                        archive.writestr("first", b"a" * 6)
                        archive.writestr("second", b"b" * 6)
                    elif kind == "entry":
                        archive.writestr("oversized", b"a" * 11)
                    elif kind == "traversal":
                        archive.writestr("../escaped", b"a")
                    elif kind == "symlink":
                        entry = zipfile.ZipInfo("link")
                        entry.external_attr = (stat.S_IFLNK | 0o777) << 16
                        archive.writestr(entry, "target")
                    else:
                        archive.writestr("same", b"a")
                        archive.writestr(fetch_candidate.SCAN_ARTIFACT + "/same", b"b")
                output = Path(directory) / "scan"
                with patch.object(fetch_candidate, "MAX_SCAN_ARTIFACT_BYTES", 10), \
                        self.assertRaises(ValueError):
                    fetch_candidate.extract_scan_artifact(raw.getvalue(), output)
                if kind in ("total", "entry"):
                    self.assertFalse((output / ("second" if kind == "total" else "oversized")).exists())

    def test_download_rejects_oversize_response_with_bounded_read(self):
        response = io.BytesIO(b"a" * 11)
        with patch.object(fetch_candidate.urllib.request, "build_opener") as opener, \
                patch.object(response, "read", wraps=response.read) as read:
            opener.return_value.open.return_value.__enter__.return_value = response
            with self.assertRaisesRegex(ValueError, "exceeds the allowed size"):
                fetch_candidate.download(fetch_candidate.ADO_PROJECT + "/_apis/build/test", "test-token", 10)
            read.assert_called_once_with(11)
        self.assertEqual(fetch_candidate.MAX_SCAN_ARTIFACT_BYTES, 512 * 1024 * 1024)

    def test_resolves_successful_producer_for_exact_pr_head(self):
        commit = "a" * 40
        branch = "refs/heads/sre/vuln/campaign"
        builds = {"value": [
            {"id": 121, "status": "completed", "result": "succeeded",
             "definition": {"id": 440}, "sourceVersion": commit, "sourceBranch": branch,
             "repository": {"id": "Azure/prometheus-collector", "type": "GitHub"}},
            {"id": 122, "status": "completed", "result": "succeeded",
             "definition": {"id": 440}, "sourceVersion": "b" * 40, "sourceBranch": branch,
             "repository": {"id": "Azure/prometheus-collector", "type": "GitHub"}},
            {"id": 123, "status": "completed", "result": "succeeded",
             "definition": {"id": 440}, "sourceVersion": commit, "sourceBranch": "refs/heads/main",
             "repository": {"id": "Azure/prometheus-collector", "type": "GitHub"}},
            {"id": 124, "status": "completed", "result": "failed",
             "definition": {"id": 440}, "sourceVersion": commit, "sourceBranch": branch,
             "repository": {"id": "Azure/prometheus-collector", "type": "GitHub"}},
        ]}
        received = []

        def download(url, token):
            received.append((url, token))
            return json.dumps(builds).encode()

        with patch.object(fetch_candidate, "access_token", return_value="unit-test-token"), \
                patch.object(fetch_candidate, "download", side_effect=download):
            self.assertEqual(fetch_candidate.resolve_build_id(commit, branch), 121)
        self.assertIn("definitions=440", received[0][0])
        self.assertIn(urllib.parse.quote(branch, safe=""), received[0][0])
        self.assertEqual(received[0][1], "unit-test-token")

    def test_resolver_rejects_missing_exact_source_build(self):
        with patch.object(fetch_candidate, "access_token", return_value="unit-test-token"), \
                patch.object(fetch_candidate, "download", return_value=b'{"value": []}'):
            self.assertIsNone(fetch_candidate.resolve_build_id(
                "a" * 40, "refs/heads/sre/vuln/campaign"))

    def test_authenticated_build_and_raw_receipts_bind_candidate(self):
        source = {"repo": "Azure/prometheus-collector", "commit": "a" * 40, "buildId": "123"}
        image = {
            "name": "collector",
            "ref": "mcr.microsoft.com/azuremonitor/containerinsights/cidev/prometheus-collector/images@sha256:" + "b" * 64,
            "indexRef": "mcr.microsoft.com/azuremonitor/containerinsights/cidev/prometheus-collector/images@sha256:" + "c" * 64,
            "os": "linux",
            "arch": "amd64",
        }
        build = {"status": "completed", "result": "succeeded", "id": 123,
                 "definition": {"id": 440}, "sourceVersion": source["commit"],
                 "repository": {"id": source["repo"], "type": "GitHub"},
                 "sourceBranch": "refs/heads/bot/ama-metrics-sre-test",
                 "url": fetch_candidate.ADO_PROJECT + "/_apis/build/builds/123"}
        for field, value in ((None, None), ("sourceVersion", "b" * 40),
                             ("result", "failed"), ("id", 999), ("url", "https://example.invalid/build"),
                             ("definition", {"id": 999}), ("sourceVersion", "short"),
                             ("repository", {"id": "other/repo", "type": "GitHub"}),
                             ("repository", {"id": source["repo"], "type": "TfsGit"}),
                             ("repository", {"id": source["repo"], "type": "GitHub", "name": "other/repo"}),
                             ("repository", {"name": source["repo"]}),
                             ("sourceBranch", "refs/heads/main")):
            with self.subTest(field=field), tempfile.TemporaryDirectory() as directory:
                candidate_build = copy.deepcopy(build)
                if field:
                    candidate_build[field] = value
                received = []
                raw_runner = b'{"imageRef":"immutable-runner"}\n'

                def download(url, token, max_bytes=16 * 1024 * 1024):
                    self.assertEqual(token, "unit-test-token")
                    self.assertTrue(url.startswith(build["url"]))
                    received.append(url)
                    if "artifacts?" not in url:
                        return json.dumps(candidate_build).encode()
                    name = url.split("artifactName=")[1].split("&")[0]
                    if name == fetch_candidate.SCAN_ARTIFACT:
                        return protected_scan_artifact(source, image)
                    archive = io.BytesIO()
                    with zipfile.ZipFile(archive, "w") as file:
                        filename = "runner.json" if name == "sre-testkube-runner" else "push.json"
                        file.writestr(name + "/" + filename, raw_runner if filename == "runner.json" else b'{}')
                    return archive.getvalue()

                output = Path(directory) / "evidence"
                with patch.object(fetch_candidate, "access_token", return_value="unit-test-token"), \
                        patch.object(fetch_candidate, "download", side_effect=download), \
                        patch.object(fetch_candidate.build_receipt, "emit", return_value={
                            "source": source, "buildResult": "succeeded", "images": [image],
                            "testRunner": {"imageRef": "immutable-runner"}}) as emit:
                    if field:
                        with self.assertRaises(ValueError):
                            fetch_candidate.fetch(123, source["commit"], output)
                        emit.assert_not_called()
                        self.assertFalse(output.exists())
                        self.assertEqual(len(received), 1)
                    else:
                        receipt = fetch_candidate.fetch(123, source["commit"], output)
                        self.assertEqual(receipt["source"], source)
                        self.assertEqual(len(received), 7)
                        self.assertEqual(emit.call_args.args[3], hashlib.sha256(raw_runner).hexdigest())
                        self.assertEqual((output / "sre-testkube-runner" / "runner.json").read_bytes(), raw_runner)
                        self.assertEqual(json.loads((output / "candidate-build.json").read_text()), receipt)
                        self.assertTrue((output / fetch_candidate.SCAN_ARTIFACT / "provenance.json").is_file())
                        self.assertEqual(fetch_candidate.load_verified(output, 123, source["commit"]), receipt)
                        with patch.object(fetch_candidate, "fetch", return_value=receipt), \
                                patch("sys.argv", ["fetch_candidate.py", "--build-id", "123",
                                     "--source-commit", source["commit"], "--output", str(output)]):
                            stdout = io.StringIO()
                            with redirect_stdout(stdout):
                                self.assertEqual(fetch_candidate.main(), 0)
                        cli = json.loads(stdout.getvalue())
                        metadata = json.loads((output / fetch_candidate.SCAN_ARTIFACT / "trivy" / "metadata.json").read_text())
                        self.assertEqual(cli["trivyVersion"], metadata["version"])
                        template = Path(__file__).parents[1] / "azure-pipeline-config-tests.yml"
                        self.assertIn('json.load(sys.stdin)["trivyVersion"]', template.read_text())
                        (output / fetch_candidate.SCAN_ARTIFACT / "trivy" / "result.json").write_text("tampered")
                        with self.assertRaisesRegex(ValueError, "hash mismatch"):
                            fetch_candidate.load_verified(output, 123, source["commit"])
                        with self.assertRaises(FileExistsError):
                            fetch_candidate.fetch(123, source["commit"], output)

    def test_artifact_has_exactly_the_expected_file(self):
        for names, valid in ((["runner.json"], True), (["sre-testkube-runner/runner.json"], True),
                             (["sre-testkube-runner/runner.json",
                               "sre-testkube-runner/_manifest/spdx_2.2/manifest.spdx.json"], True),
                             (["../runner.json"], False), (["runner.json", "other.json"], False), ([], False)):
            with self.subTest(names=names):
                raw = io.BytesIO()
                with zipfile.ZipFile(raw, "w") as archive:
                    for name in names:
                        archive.writestr(name, b"{}")
                if valid:
                    self.assertEqual(fetch_candidate.receipt_file(
                        raw.getvalue(), "sre-testkube-runner", "runner.json"), b"{}")
                else:
                    with self.assertRaises(ValueError):
                        fetch_candidate.receipt_file(raw.getvalue(), "sre-testkube-runner", "runner.json")

    def test_redirect_never_forwards_auth_to_blob_host(self):
        request = urllib.request.Request("https://dev.azure.com/artifact", headers={"Authorization": "Bearer test"})
        handler = fetch_candidate.ArtifactRedirect()
        for url in ("https://dev.azure.com/next",
                    "https://" + fetch_candidate.ADO_ARTIFACT_HOST + "/content",
                    "https://example.vsblob.vsassets.io/signed"):
            redirected = handler.redirect_request(request, None, 302, "", {}, url)
            trusted = urllib.parse.urlsplit(url).netloc in ("dev.azure.com", fetch_candidate.ADO_ARTIFACT_HOST)
            self.assertEqual(redirected.get_header("Authorization"), "Bearer test" if trusted else None)
        with self.assertRaises(ValueError):
            handler.redirect_request(request, None, 302, "", {}, "http://example.invalid/")

    def test_source_binding_test_rejects_broken_guard(self):
        original = fetch_candidate.fetch
        code = inspect.getsource(original)
        old = 'source["commit"] != commit'
        self.assertIn(old, code)
        exec(code.replace(old, "False"), vars(fetch_candidate))
        try:
            result = unittest.TestResult()
            FetchTests("test_authenticated_build_and_raw_receipts_bind_candidate").run(result)
        finally:
            fetch_candidate.fetch = original
        self.assertTrue(result.failures, result.errors)
        self.assertFalse(result.errors)

    def test_template_contract_rejects_missing_trivy_version(self):
        original = fetch_candidate.main
        code = inspect.getsource(original)
        field = ',\n                      "trivyVersion": scan["version"]'
        self.assertIn(field, code)
        exec(code.replace(field, ""), vars(fetch_candidate))
        try:
            result = unittest.TestResult()
            FetchTests("test_authenticated_build_and_raw_receipts_bind_candidate").run(result)
        finally:
            fetch_candidate.main = original
        self.assertTrue(result.errors)
        self.assertIn("trivyVersion", result.errors[0][1])


if __name__ == "__main__":
    unittest.main()
