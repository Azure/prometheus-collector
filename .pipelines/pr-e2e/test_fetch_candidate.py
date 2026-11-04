import copy
import hashlib
import inspect
import io
import json
from pathlib import Path
import tempfile
import unittest
import urllib.parse
import urllib.request
from unittest.mock import patch
import zipfile

import fetch_candidate


class FetchTests(unittest.TestCase):
    def test_authenticated_build_and_raw_receipts_bind_candidate(self):
        source = {"repo": "Azure/prometheus-collector", "commit": "a" * 40, "buildId": "123"}
        build = {"status": "completed", "result": "succeeded", "id": 123,
                 "definition": {"id": 440}, "sourceVersion": source["commit"],
                 "repository": {"name": source["repo"]},
                 "sourceBranch": "refs/heads/bot/ama-metrics-sre-test",
                 "url": fetch_candidate.ADO_PROJECT + "/_apis/build/builds/123"}
        for field, value in ((None, None), ("sourceVersion", "b" * 40),
                             ("result", "failed"), ("id", 999), ("url", "https://example.invalid/build"),
                             ("definition", {"id": 999}), ("sourceVersion", "short"),
                             ("repository", {"name": "other/repo"}), ("sourceBranch", "refs/heads/main")):
            with self.subTest(field=field), tempfile.TemporaryDirectory() as directory:
                candidate_build = copy.deepcopy(build)
                if field:
                    candidate_build[field] = value
                received = []
                raw_runner = b'{"imageRef":"immutable-runner"}\n'

                def download(url, token):
                    self.assertEqual(token, "unit-test-token")
                    self.assertTrue(url.startswith(build["url"]))
                    received.append(url)
                    if "artifacts?" not in url:
                        return json.dumps(candidate_build).encode()
                    name = url.split("artifactName=")[1].split("&")[0]
                    archive = io.BytesIO()
                    with zipfile.ZipFile(archive, "w") as file:
                        filename = "runner.json" if name == "sre-testkube-runner" else "push.json"
                        file.writestr(name + "/" + filename, raw_runner if filename == "runner.json" else b'{}')
                    return archive.getvalue()

                output = Path(directory) / "evidence"
                with patch.object(fetch_candidate, "access_token", return_value="unit-test-token"), \
                        patch.object(fetch_candidate, "download", side_effect=download), \
                        patch.object(fetch_candidate.build_receipt, "emit", return_value={
                            "source": source, "testRunner": {"imageRef": "immutable-runner"}}) as emit:
                    if field:
                        with self.assertRaises(ValueError):
                            fetch_candidate.fetch(123, source["commit"], output)
                        emit.assert_not_called()
                        self.assertFalse(output.exists())
                        self.assertEqual(len(received), 1)
                    else:
                        receipt = fetch_candidate.fetch(123, source["commit"], output)
                        self.assertEqual(receipt["source"], source)
                        self.assertEqual(len(received), 6)
                        self.assertEqual(emit.call_args.args[3], hashlib.sha256(raw_runner).hexdigest())
                        self.assertEqual((output / "sre-testkube-runner" / "runner.json").read_bytes(), raw_runner)
                        self.assertEqual(json.loads((output / "candidate-build.json").read_text()), receipt)
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


if __name__ == "__main__":
    unittest.main()
