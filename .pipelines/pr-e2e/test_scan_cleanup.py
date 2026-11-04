import copy
from concurrent.futures import ThreadPoolExecutor
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import tempfile
import time
import unittest
import yaml


ROOT = Path(__file__).parent
PWSH = shutil.which("pwsh")
OWNER = "11111111111111111111111111111111"
VOLUME = "prscan-" + OWNER
TARGET = "example.invalid/target@sha256:" + "1" * 64
TARGET_ID = "sha256:" + "2" * 64
SCANNER_ID = "sha256:" + "3" * 64
OTHER_ID = "sha256:" + "4" * 64
HARNESS = r"""
param([string]$Scanner, [string]$StateFile)
$global:scanFakeState = Get-Content $StateFile -Raw | ConvertFrom-Json -AsHashtable
function global:New-Guid { [guid]$global:scanFakeState.owner }
if ($env:SCAN_RACE_DIRECTORY) {
    function global:New-Item {
        [CmdletBinding()]
        param([string]$ItemType, [string[]]$Path, [switch]$Force)
        $root = [IO.Path]::TrimEndingDirectorySeparator([IO.Path]::GetFullPath($env:SCAN_OUTPUT))
        if ($Path.Count -eq 1 -and $Path[0] -eq $root) {
            # Pause in the provider's ItemExists / Directory.CreateDirectory interval.
            if ([IO.Directory]::Exists($Path[0])) { throw 'Output already exists' }
            [IO.File]::WriteAllText("$env:SCAN_RACE_DIRECTORY/$($global:scanFakeState.owner).ready", '')
            $deadline = [DateTime]::UtcNow.AddSeconds(60)
            while (-not [IO.File]::Exists("$env:SCAN_RACE_DIRECTORY/release")) {
                if ([DateTime]::UtcNow -gt $deadline) { throw 'Offline race barrier timed out' }
                Start-Sleep -Milliseconds 10
            }
            return [IO.Directory]::CreateDirectory($Path[0])
        }
        Microsoft.PowerShell.Management\New-Item @PSBoundParameters
    }
}
if ('crash-after-claim' -in $global:scanFakeState.fail) {
    function global:New-Item { [Environment]::Exit(29) }
}
function global:docker {
    $a = @($args)
    $s = $global:scanFakeState
    $s.calls += ,$a
    $global:LASTEXITCODE = 0
    switch ("$($a[0]) $($a[1])") {
        'image ls' {
            if ('inventory-before' -in $s.fail -and -not $s.pulled) {
                $global:LASTEXITCODE = 19; return
            }
            return @($s.images.Keys)
        }
        'image inspect' {
            if ('inspect-target' -in $s.fail -and $a[2] -eq $s.ref) {
                $global:LASTEXITCODE = 20; return
            }
            $id = if ($a[2] -eq $s.ref) { $s.targetId } else { $a[2] }
            if (-not $s.images.ContainsKey($id)) { throw "Unknown simulated image: $id" }
            return ConvertTo-Json -InputObject @($s.images[$id]) -Depth 20
        }
        'volume ls' { return @($s.volumes.Keys) }
        'volume create' {
            $name = $a[-1]
            $label = $a[$a.IndexOf('--label') + 1].Split('=', 2)
            $s.volumes[$name] = @{Name=$name; Labels=@{$label[0]=$label[1]}}
            if ('volume-race' -in $s.fail) { $s.volumes[$name].Labels[$label[0]] = 'foreign' }
            if ('volume-create-after' -in $s.fail) { $global:LASTEXITCODE = 21 }
            return $name
        }
        'volume inspect' {
            return ConvertTo-Json -InputObject @($s.volumes[$a[2]]) -Depth 20
        }
        'volume rm' {
            if ('volume-rm' -in $s.fail) { $global:LASTEXITCODE = 22; return }
            $s.volumes.Remove($a[2]); return
        }
        'image rm' {
            if ('image-rm' -in $s.fail) { $global:LASTEXITCODE = 23; return }
            if ($a[2] -ne $s.ref) { throw 'Refusing unexpected simulated image deletion' }
            $s.images.Remove($s.targetId); return
        }
    }
    switch ($a[0]) {
        'pull' {
            if ('pull-before' -in $s.fail) { $global:LASTEXITCODE = 17; return }
            $s.pulled = $true
            $s.images[$s.targetId] = @{
                Id=$s.targetId; RepoDigests=@($s.ref); Os=$s.os; Architecture='amd64';
                OsVersion='10.0.20348.example'
            }
            if ('pull-after' -in $s.fail) { $global:LASTEXITCODE = 18 }
            return
        }
        'info' { return 'overlay2' }
        'run' {
            if ('--pull=never' -notin $a) { throw 'Scanner image must not be implicitly pulled' }
            if ('no-xml' -notin $s.fail) {
                $time = [DateTime]::UtcNow.ToString('o')
                $finding = if ('emit-present' -in $s.fail) {
                    '<VULN><QID>100001</QID><FLAGS>v</FLAGS><RESULT>offline fixture</RESULT></VULN>'
                } else { '<VULNS_NOT_FOUND><Q>100001</Q></VULNS_NOT_FOUND>' }
                "<HOST><OS>Windows</OS><PARTIAL_REPORT_FLAG>false</PARTIAL_REPORT_FLAG><SCA_SCAN_ENABLED>true</SCA_SCAN_ENABLED><VULNSIGS_VERSION>offline-signatures</VULNSIGS_VERSION><AGENT_MANIFEST_VERSION>offline-manifest</AGENT_MANIFEST_VERSION><REPORT_GENERATED_DATE>$time</REPORT_GENERATED_DATE>$finding</HOST>" |
                    Set-Content "$env:SCAN_OUTPUT/comet/result.xml"
            }
            if ('volume-owner-lost' -in $s.fail) {
                foreach ($volume in $s.volumes.Values) { $volume.Labels['pr-e2e.invocation'] = 'foreign' }
            }
            if ('unrelated-image' -in $s.fail) {
                $s.images[$s.otherId] = @{Id=$s.otherId; RepoDigests=@('unrelated.invalid/image')}
            }
            if ('run' -in $s.fail) { $global:LASTEXITCODE = 2 }
            return 'offline scan output'
        }
    }
    throw "Unsupported simulated Docker command: $($a -join ' ')"
}
function global:Invoke-RestMethod {
    if ('oidc' -in $global:scanFakeState.fail) { throw 'Simulated OIDC failure' }
    return @{oidcToken='offline-test-token'}
}
function global:trivy {
    if ('trivy' -in $global:scanFakeState.fail) { $global:LASTEXITCODE = 24; return }
    throw 'Unexpected simulated Trivy invocation'
}
if ('hash' -in $global:scanFakeState.fail) {
    function global:Get-FileHash { throw 'Simulated hashing failure' }
}
if ('warnings-stop' -in $global:scanFakeState.fail) { $WarningPreference = 'Stop' }
try {
    & $Scanner
    $global:scanFakeState.error = $null
}
catch { $global:scanFakeState.error = $_.Exception.Message }
finally {
    ConvertTo-Json -InputObject $global:scanFakeState -Depth 30 | Set-Content $StateFile
}
if ($global:scanFakeState.error) {
    Write-Error $global:scanFakeState.error -ErrorAction Continue
    exit 1
}
"""


@unittest.skipUnless(PWSH, "PowerShell 7 is required for simulated Docker cleanup checks")
class ScanCleanupTests(unittest.TestCase):
    source = None
    steps_text = None

    def run_scan(self, fail=(), existing=False, volumes=None, os_name="windows", build_id="offline-current",
                 artifact_output=None, target_ref=TARGET, source_commit="a" * 40, source_build="offline",
                 preexisting_output=None, output_path=None, race_directory=None, owner=OWNER,
                 preexisting_claim=None, publish_owner=None):
        state = {"calls": [], "fail": list(fail), "pulled": False, "ref": target_ref,
                 "targetId": TARGET_ID, "otherId": OTHER_ID, "os": os_name,
                 "owner": owner,
                 "images": {SCANNER_ID: {"Id": SCANNER_ID, "RepoDigests": ["scanner.invalid/image"]}},
                 "volumes": copy.deepcopy(volumes or {})}
        if existing:
            state["images"][TARGET_ID] = {"Id": TARGET_ID, "RepoDigests": [target_ref]}
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            output = Path(output_path) if output_path is not None else root / "output"
            (root / "state.json").write_text(json.dumps(state))
            harness = HARNESS
            if publish_owner is not None:
                template = yaml.safe_load(self.steps_text or (ROOT / "scan-steps.yml").read_text())
                guard = next((step["pwsh"] for step in template["steps"]
                              if "-VerifyOutputOwnership" in step.get("pwsh", "")), None)
                self.assertIsNotNone(guard, "Publication must execute the ownership verifier")
                harness = harness.replace("    & $Scanner\n",
                                          guard.replace(".pipelines/pr-e2e/scan.ps1", "$Scanner"))
            (root / "harness.ps1").write_text(harness)
            script = root / "scan.ps1"
            script.write_text(self.source if self.source is not None else (ROOT / "scan.ps1").read_text())
            targets = {"source": {"commit": source_commit, "buildId": source_build},
                       "images": [{"ref": target_ref, "os": os_name, "arch": "amd64",
                                   "osVersion": "10.0.20348.example"}]}
            (root / "targets.json").write_text(json.dumps(targets))
            if preexisting_output is not None:
                output.mkdir()
                for name, content in preexisting_output.items():
                    path = output / name
                    path.parent.mkdir(parents=True, exist_ok=True)
                    path.write_bytes(content)
            claim = output.with_name(output.name + ".claim")
            if preexisting_claim is not None:
                claim.write_bytes(preexisting_claim)
            env = dict(os.environ, TARGETS=str(root / "targets.json"), TARGET_INDEX="0",
                       SCANNER_IMAGE="scanner.invalid/image@sha256:" + "5" * 64,
                       SCAN_OUTPUT=str(output_path) if output_path is not None else str(output),
                       TRIVY_VERSION="offline", BUILD_BUILDID=build_id,
                       SCAN_RACE_DIRECTORY=str(race_directory) if race_directory else "",
                       SCAN_OUTPUT_OWNER=publish_owner if publish_owner is not None else "")
            result = subprocess.run([PWSH, "-NoProfile", "-NonInteractive", "-File", str(root / "harness.ps1"),
                                     "-Scanner", str(script), "-StateFile", str(root / "state.json")],
                                    env=env, capture_output=True, text=True, timeout=120)
            state = json.loads((root / "state.json").read_text(encoding="utf-8-sig"))
            state["variables"] = dict(re.findall(
                r"##vso\[task\.setvariable variable=([^\]]+)\]([^\r\n]*)", result.stdout))
            state["claim"] = claim.read_bytes() if claim.exists() else None
            provenance = output / "provenance.json"
            if provenance.exists():
                state["provenance"] = json.loads(provenance.read_text(encoding="utf-8-sig"))
            state["outputFiles"] = {str(path.relative_to(output)): path.read_bytes()
                                    for path in output.rglob("*") if path.is_file()}
            if artifact_output is not None:
                shutil.copytree(output, artifact_output)
        return state, result

    def run_contenders(self, root, trailing_separator=False):
        gate = root / "gate"
        gate.mkdir()
        with ThreadPoolExecutor(max_workers=2) as pool:
            contenders = [pool.submit(self.run_scan,
                                      output_path=str(root / "output") +
                                      (os.sep if trailing_separator and index else ""),
                                      race_directory=gate, owner=f"{index + 1:032x}")
                          for index in range(2)]
            deadline = time.monotonic() + 90
            try:
                while time.monotonic() < deadline:
                    ready = len(list(gate.glob("*.ready")))
                    if ready == 2 or (ready == 1 and any(c.done() for c in contenders)):
                        break
                    time.sleep(.01)
                else:
                    self.fail("Producer contenders did not reach the controlled interleaving")
            finally:
                (gate / "release").touch()
            return [contender.result() for contender in contenders]

    def test_competing_producers_cannot_share_output(self):
        for trailing in (False, True):
            with self.subTest(trailing_separator=trailing), tempfile.TemporaryDirectory() as directory:
                results = self.run_contenders(Path(directory), trailing_separator=trailing)
                claimed = [(state, result) for state, result in results
                           if state["variables"].get("scanOutputOwned") == "true"]
                self.assertEqual(len(claimed), 1, "Both producers claimed the same output root")
                self.assertEqual(claimed[0][1].returncode, 0, claimed[0][1].stderr)
                denied = next((state, result) for state, result in results if (state, result) not in claimed)
                self.assertNotEqual(denied[1].returncode, 0)
                self.assertEqual(denied[0]["calls"], [])
                receipt = json.loads(claimed[0][0]["claim"])
                self.assertEqual(receipt["owner"], claimed[0][0]["owner"])
                self.assertEqual(receipt["buildId"], "offline-current")

    def test_existing_partial_or_foreign_claim_blocks_without_touching_it_or_creating_output(self):
        for contents in (b"", b"{", b'{"owner":"earlier-invocation"}'):
            with self.subTest(claim=contents), tempfile.TemporaryDirectory() as directory:
                output = Path(directory) / "output"
                state, result = self.run_scan(output_path=output, preexisting_claim=contents)
                self.assertNotEqual(result.returncode, 0)
                self.assertEqual(state["calls"], [])
                self.assertEqual(state["claim"], contents)
                self.assertNotIn("scanOutputOwned", state["variables"])
                self.assertFalse(output.exists())

    def test_claim_survives_success_failure_and_abrupt_process_exit(self):
        for failure in ([], ["run"], ["crash-after-claim"]):
            with self.subTest(failure=failure), tempfile.TemporaryDirectory() as directory:
                output = Path(directory) / "output"
                before, result = self.run_scan(output_path=output, fail=failure)
                self.assertEqual(result.returncode == 0, not failure)
                self.assertIsNotNone(before["claim"])
                receipt = json.loads(before["claim"])
                self.assertEqual(receipt["owner"], OWNER)
                if output.exists():
                    output.rename(output.with_name("preserved-evidence"))
                after, denied = self.run_scan(output_path=output)
                self.assertNotEqual(denied.returncode, 0)
                self.assertEqual(after["claim"], before["claim"])
                self.assertEqual(after["calls"], [])
                self.assertFalse(output.exists())

    def test_publication_requires_matching_claim_owner_path_build_and_existing_output(self):
        with tempfile.TemporaryDirectory() as directory:
            output = Path(directory) / "output"
            original, scan = self.run_scan(output_path=output, fail=["no-xml"])
            self.assertNotEqual(scan.returncode, 0)
            owner = original["variables"]["scanOutputOwner"]
            claim = output.with_name("output.claim")
            receipt = json.loads(original["claim"])
            cases = [("valid-failed-scan-diagnostics", {}, owner, "offline-current", True),
                     ("foreign-owner", {"owner": "2" * 32}, owner, "offline-current", False),
                     ("foreign-path", {"output": str(output.parent / "other")}, owner, "offline-current", False),
                     ("stale-build", {"buildId": "earlier-build"}, owner, "offline-current", False),
                     ("non-string-owner", {"owner": [owner]}, owner, "offline-current", False),
                     ("non-string-path", {"output": [str(output)]}, owner, "offline-current", False),
                     ("non-string-build", {"buildId": ["offline-current"]}, owner, "offline-current", False),
                     ("wrong-token", {}, "3" * 32, "offline-current", False),
                     ("missing-token", {"owner": ""}, "", "offline-current", False),
                     ("wrong-build", {}, owner, "other-build", False)]
            for name, changes, token, build, expected in cases:
                with self.subTest(case=name):
                    claim.write_text(json.dumps(dict(receipt, **changes)))
                    state, result = self.run_scan(output_path=output, publish_owner=token, build_id=build)
                    self.assertEqual(result.returncode == 0, expected, result.stderr)
                    self.assertEqual(state["variables"].get("scanOutputPublishable") == "true", expected)
                    self.assertEqual(state["calls"], [])
                    self.assertEqual(state["outputFiles"], original["outputFiles"])
            for content in (b"", b"{", None):
                with self.subTest(incomplete_claim=content):
                    if content is None:
                        claim.unlink()
                    else:
                        claim.write_bytes(content)
                    state, result = self.run_scan(output_path=output, publish_owner=owner)
                    self.assertNotEqual(result.returncode, 0)
                    self.assertNotIn("scanOutputPublishable", state["variables"])
            claim.write_bytes(original["claim"])
            output.rename(output.with_name("preserved-evidence"))
            state, result = self.run_scan(output_path=output, publish_owner=owner)
            self.assertNotEqual(result.returncode, 0)
            self.assertNotIn("scanOutputPublishable", state["variables"])

    def test_preexisting_image_is_never_removed(self):
        state, result = self.run_scan(existing=True, fail=["unrelated-image"])
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn(TARGET_ID, state["images"])
        self.assertFalse(any(call[:2] == ["image", "rm"] for call in state["calls"]))
        self.assertEqual(state["volumes"], {})

    def test_real_scan_build_and_actual_arguments_are_recorded_without_coverage_claims(self):
        state, result = self.run_scan()
        self.assertEqual(result.returncode, 0, result.stderr)
        provenance = state["provenance"]
        self.assertEqual(provenance["buildId"], "offline")
        self.assertEqual(provenance["scannedInBuildId"], "offline-current")
        arguments = provenance["scanConfiguration"]["arguments"]
        run = next(call for call in state["calls"] if call[0] == "run")
        self.assertEqual(run[run.index("--force-manifest-download"):run.index("--scan-result-path")],
                         arguments)
        self.assertEqual(arguments[-1], ",".join(provenance["languagesRequested"]))
        self.assertIs(provenance["configurationVerified"], False)
        self.assertEqual(provenance["languagesVerified"], [])

    def test_missing_actual_scan_build_blocks_before_docker(self):
        state, result = self.run_scan(build_id="")
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("actual scan build ID", state["error"])
        self.assertEqual(state["calls"], [])

    def test_preexisting_output_is_preserved_and_rejected_before_docker(self):
        for contents in ({}, {"unrelated.bin": b"unrelated", "comet/result.xml": b"earlier report",
                              "provenance.json": b'{"earlier": true}',
                              "sha256sums.json": b"earlier hashes"}):
            with self.subTest(files=list(contents)):
                state, result = self.run_scan(preexisting_output=contents, fail=["no-xml"])
                self.assertNotEqual(result.returncode, 0)
                self.assertEqual(state["calls"], [])
                self.assertEqual(state["outputFiles"], {str(Path(name)): value for name, value in contents.items()})
                self.assertNotIn("variable=scanOutputOwned]true", result.stdout)

    def test_no_xml_fails_but_retains_owned_diagnostics_and_cleans_resources(self):
        state, result = self.run_scan(fail=["no-xml"])
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual(state["error"], "BLOCKED: CoMET XML output missing")
        for name in ("completion.json", "provenance.json", "sha256sums.json", str(Path("comet") / "scan.log")):
            self.assertIn(name, state["outputFiles"])
        self.assertEqual(json.loads(state["outputFiles"]["completion.json"])["exitCode"], 0)
        self.assertFalse(state["provenance"]["configurationVerified"])
        self.assertEqual(state["volumes"], {})
        self.assertNotIn(TARGET_ID, state["images"])
        self.assertIn("variable=scanOutputOwned]true", result.stdout)

    def test_new_target_and_owned_volume_are_removed_but_other_images_remain(self):
        state, result = self.run_scan(fail=["unrelated-image"])
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertNotIn(TARGET_ID, state["images"])
        self.assertIn(SCANNER_ID, state["images"])
        self.assertIn(OTHER_ID, state["images"])
        self.assertEqual(state["volumes"], {})

    def test_existing_volume_collision_is_preserved_and_never_used(self):
        volume = {"Name": VOLUME, "Labels": {"pr-e2e.invocation": "old-job"}}
        state, result = self.run_scan(existing=True, volumes={VOLUME: volume})
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("already exists", state["error"])
        self.assertEqual(state["volumes"][VOLUME], volume)
        self.assertFalse(any(call[0] == "run" or call[:2] == ["volume", "rm"] for call in state["calls"]))

    def test_early_failure_paths_clean_only_new_resources(self):
        for fail in ("pull-before", "pull-after", "inspect-target", "oidc", "volume-create-after", "run"):
            for existing in (False, True):
                with self.subTest(fail=fail, existing=existing):
                    state, result = self.run_scan(fail=[fail], existing=existing)
                    self.assertNotEqual(result.returncode, 0)
                    self.assertIsNotNone(state["error"])
                    self.assertEqual(TARGET_ID in state["images"], existing)
                    self.assertEqual(state["volumes"], {})

    def test_failed_inventory_never_pulls_or_deletes(self):
        state, result = self.run_scan(fail=["inventory-before"], existing=True)
        self.assertNotEqual(result.returncode, 0)
        self.assertIn(TARGET_ID, state["images"])
        self.assertFalse(any(call[0] == "pull" or call[1] == "rm" for call in state["calls"]))

    def test_volume_creation_race_and_lost_ownership_are_not_deleted(self):
        for fail in ("volume-race", "volume-owner-lost"):
            with self.subTest(fail=fail):
                state, result = self.run_scan(fail=[fail])
                self.assertNotEqual(result.returncode, 0)
                self.assertIn(VOLUME, state["volumes"])
                self.assertEqual(state["volumes"][VOLUME]["Labels"]["pr-e2e.invocation"], "foreign")
                self.assertFalse(any(call[:2] == ["volume", "rm"] for call in state["calls"]))
                self.assertNotIn(TARGET_ID, state["images"])

    def test_cleanup_failures_do_not_mask_primary_error_or_skip_other_cleanup(self):
        state, result = self.run_scan(fail=["run", "volume-rm", "image-rm", "hash", "warnings-stop"])
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual(state["error"], "BLOCKED: CoMET exit 2")
        for message in ("Volume cleanup:", "Image cleanup:", "Evidence hashing:"):
            self.assertIn(message, result.stdout)
        self.assertTrue(any(call[:2] == ["image", "rm"] for call in state["calls"]))

    def test_cleanup_failure_alone_fails_the_job(self):
        for fail in ("volume-rm", "image-rm", "hash"):
            with self.subTest(fail=fail):
                state, result = self.run_scan(fail=[fail])
                self.assertNotEqual(result.returncode, 0)
                self.assertIn("Scanner cleanup failed:", state["error"])
                if fail == "volume-rm":
                    self.assertNotIn(TARGET_ID, state["images"])

    def test_linux_trivy_failure_still_cleans_owned_resources(self):
        state, result = self.run_scan(fail=["trivy"], os_name="linux")
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("Trivy version", state["error"])
        self.assertNotIn(TARGET_ID, state["images"])
        self.assertEqual(state["volumes"], {})


@unittest.skipUnless(PWSH, "PowerShell 7 is required for simulated Docker cleanup checks")
class ScanCleanupMutationTests(unittest.TestCase):
    def test_cleanup_guards_fail_when_broken(self):
        source = (ROOT / "scan.ps1").read_text()
        mutations = [
            ("preexisting-image", "Where-Object { $_ -notin $imagesBefore }",
             "Where-Object { $true }", "test_preexisting_image_is_never_removed"),
            ("volume-ownership", "if ($volumeInfo.Labels.'pr-e2e.invocation' -ne $owner) {\n"
             "                    throw 'BLOCKED: refusing cleanup",
             "if ($false) {\n                    throw 'BLOCKED: refusing cleanup",
             "test_volume_creation_race_and_lost_ownership_are_not_deleted"),
            ("missing-xml", 'if (-not (Test-Path -LiteralPath "$output/comet/result.xml" -PathType Leaf))',
             "if ($false)", "test_no_xml_fails_but_retains_owned_diagnostics_and_cleans_resources"),
            ("provider-race",
             "[IO.File]::Open($claimPath, [IO.FileMode]::CreateNew, [IO.FileAccess]::Write, [IO.FileShare]::None)",
             "[IO.MemoryStream]::new()", "test_competing_producers_cannot_share_output")
        ]
        for name, old, new, test in mutations:
            with self.subTest(mutation=name):
                self.assertIn(old, source)
                mutated = type("MutatedScanCleanup", (ScanCleanupTests,),
                               {"source": source.replace(old, new)})
                result = unittest.TestResult()
                mutated(test).run(result)
                self.assertTrue(result.failures, (name, result.errors))
                self.assertFalse(result.errors, (name, result.errors))
                print("KILLED:", name)


if __name__ == "__main__":
    unittest.main()
