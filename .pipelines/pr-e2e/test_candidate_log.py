import base64
import json
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

import candidate_log
from discovery_log import emit_payload, read_payload
from fetch_candidate import extract_scan_artifact
from test_fetch_candidate import protected_scan_artifact


class CandidateLogTests(unittest.TestCase):
    def test_roundtrip_rejects_binding_corruption_and_incomplete_evidence(self):
        repository = "mcr.microsoft.com/azuremonitor/containerinsights/cidev/prometheus-collector/images"
        image = {"name": "collector", "ref": repository + "@sha256:" + "b" * 64,
                 "indexRef": repository + "@sha256:" + "c" * 64, "os": "linux", "arch": "amd64"}
        source = {"repo": "Azure/prometheus-collector", "commit": "a" * 40, "buildId": "123"}
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            directory = root / "scan"
            extract_scan_artifact(protected_scan_artifact(source, image), directory)
            text = candidate_log.emit(directory)
            expected = (123, source["commit"], image["ref"], image["indexRef"])
            output = root / "decoded.json"
            result = candidate_log.read(text, *expected, output)
            self.assertEqual(result["source"], source)
            self.assertEqual(result["image"], image)
            self.assertEqual(result["result"], json.loads((directory / "trivy/result.json").read_text()))
            self.assertEqual(json.loads(output.read_text()), result)
            with self.assertRaises(FileExistsError):
                candidate_log.read(text, *expected, output)
            bad = root / "bad.json"
            for binding in ((124, *expected[1:]), (123, "d" * 40, *expected[2:]),
                            (*expected[:2], repository + "@sha256:" + "d" * 64, expected[3]),
                            (*expected[:3], repository + "@sha256:" + "d" * 64)):
                with self.subTest(binding=binding), self.assertRaises(ValueError):
                    candidate_log.read(text, *binding, bad)
                self.assertFalse(bad.exists())
            for damaged in (text + text, "\n".join(text.splitlines()[:-1]),
                            text.replace(text.split()[1], "0" * 64)):
                with self.subTest(damaged=damaged), self.assertRaises(ValueError):
                    candidate_log.read(damaged, *expected, bad)
                self.assertFalse(bad.exists())
            for change in ("missing-file", "result-hash", "definition", "coverage"):
                value = read_payload(text, candidate_log.PREFIX, candidate_log.MAX_BYTES)
                if change == "missing-file":
                    del value["files"]["trivy/result.json"]
                elif change == "result-hash":
                    value["files"]["trivy/result.json"] = base64.b64encode(b'{"Results":[]}').decode()
                else:
                    provenance = json.loads(base64.b64decode(value["files"]["provenance.json"]))
                    provenance["producerDefinitionId" if change == "definition" else "coverage"] = "wrong"
                    value["files"]["provenance.json"] = base64.b64encode(json.dumps(provenance).encode()).decode()
                broken = emit_payload(value, candidate_log.PREFIX, candidate_log.MAX_BYTES)
                with self.subTest(change=change), self.assertRaises(ValueError):
                    candidate_log.read(broken, *expected, bad)
                self.assertFalse(bad.exists())
            with patch.object(candidate_log, "MAX_BYTES", 10), self.assertRaises(ValueError):
                candidate_log.emit(directory)

    def test_binding_guard_is_exercised(self):
        original = candidate_log.validate_scan_artifact

        def ignore_binding(directory, expected):
            provenance = json.loads((directory / "provenance.json").read_text())
            return original(directory, {"source": provenance["source"], "images": [provenance["image"]]})

        with patch.object(candidate_log, "validate_scan_artifact", side_effect=ignore_binding):
            result = unittest.TestResult()
            CandidateLogTests("test_roundtrip_rejects_binding_corruption_and_incomplete_evidence").run(result)
        self.assertTrue(result.failures, "The regression must fail when expected source/image binding is bypassed")


if __name__ == "__main__":
    unittest.main()
