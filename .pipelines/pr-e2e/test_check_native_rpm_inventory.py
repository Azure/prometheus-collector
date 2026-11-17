import json
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest


class NativeRpmInventoryTest(unittest.TestCase):
    def test_inventory_and_cve_require_exact_rpm_identity(self):
        script = Path(__file__).with_name("check_native_rpm_inventory.py")
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            manifest = root / "manifest.tsv"
            scan = root / "scan.json"
            manifest.write_text("metricsext2\t1-2\t1\t1\tVendor\t(none)\t1\tx86_64\t1\tsource.rpm\n")
            package = {"Name": "metricsext2", "Version": "1", "Release": "2", "Epoch": 1, "Arch": "x86_64"}
            finding = {"PkgName": "metricsext2", "InstalledVersion": "1:1-2",
                       "VulnerabilityID": "CVE-2026-1234"}
            for packages, findings, expected in (([package], [finding], True),
                                                  ([], [finding], False),
                                                  ([package], [], False),
                                                  ([{**package, "Epoch": 0}], [finding], False),
                                                  ([{**package, "Arch": "aarch64"}], [finding], False)):
                scan.write_text(json.dumps({"Results": [{"Class": "os-pkgs", "Packages": packages,
                                                         "Vulnerabilities": findings}]}))
                result = subprocess.run([sys.executable, "-O", str(script), str(scan), str(manifest),
                                         "--expect-cve", "CVE-2026-1234"], capture_output=True, text=True)
                self.assertEqual(result.returncode == 0, expected, result.stderr)


if __name__ == "__main__":
    unittest.main()
