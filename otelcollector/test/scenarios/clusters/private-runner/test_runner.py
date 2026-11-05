import copy
import json
from pathlib import Path
import shutil
import sys
import types
import unittest
import uuid

import runner


def fixture():
    run = "58457144-f0e9-4cce-b16e-964ed59d81b1"
    name = "pr-" + uuid.UUID(run).hex
    root = f"/subscriptions/{runner.SUBSCRIPTION}/resourceGroups/owned-rg/providers/"
    resources = {kind: root + value[0] + "/" + name
                 for kind, value in runner.TYPES.items() if kind not in ("nat", "ip")}
    resources["subnet"] = root + f"Microsoft.Network/virtualNetworks/owned-vnet/subnets/{name}"
    receipt = {
        "version": 1, "run": run, "name": name, "resourceGroup": "owned-rg",
        "vnet": "owned-vnet", "started": "2026-11-05T00:00:00Z",
        "deadline": "2026-11-05T12:00:00Z", "resources": resources,
        "pool": 800, "queue": 900, "agent": 1000, "completed": [],
    }
    values = {
        kind: {"id": resource_id, "etag": '"initial"',
               "tags": {"privateRunnerRun": run,
                        "privateRunnerDeadline": receipt["deadline"]},
               "properties": {}}
        for kind, resource_id in resources.items()
    }
    values["subnet"] = {"id": resources["subnet"], "etag": '"initial"', "properties": {
        "addressPrefix": "10.227.0.0/24", "networkSecurityGroup": {"id": resources["nsg"]}}}
    values["vm"]["properties"] = {
        "storageProfile": {
            "osDisk": {"deleteOption": "Detach", "managedDisk": {"id": resources["disk"]}},
        },
        "networkProfile": {"networkInterfaces": [
            {"id": resources["nic"], "properties": {"deleteOption": "Detach"}}]},
    }
    values["pool"] = {"id": receipt["pool"], "name": name,
                      "isHosted": False, "autoProvision": False}
    values["queue"] = {"id": receipt["queue"], "name": name, "pool": {"id": receipt["pool"]}}
    values["agent"] = {"id": receipt["agent"], "name": name}
    values["permissions"] = {"allPipelines": {"authorized": False},
                             "pipelines": [{"id": 979, "authorized": True}]}
    return receipt, values


class FakeNative:
    """Control-plane-only fake: the VM/agent can be completely dead."""

    def __init__(self, receipt, values):
        self.urls = runner.endpoints(receipt)
        self.values = copy.deepcopy(values)
        self.calls = []
        self.orphans = []
        self.refuse = None

    def request(self, method, url, body=None, etag=None):
        self.calls.append((method, url, copy.deepcopy(body), etag))
        if "/resources?" in url:
            return {"value": [copy.deepcopy(v) for k, v in self.values.items()
                              if k in runner.TYPES and v is not None] + self.orphans}
        kind = next(k for k, value in self.urls.items() if value == url)
        if method == "GET":
            if kind == "agents":
                return {"value": [] if self.values["agent"] is None else [self.values["agent"]]}
            return copy.deepcopy(self.values.get(kind))
        if method == "PATCH":
            if self.refuse != "revoke":
                self.values["permissions"] = copy.deepcopy(body)
            return {}
        if method == "DELETE":
            if self.refuse != kind:
                self.values[kind] = None
            return {}
        raise AssertionError("Unexpected provider operation")


class LifecycleTests(unittest.TestCase):
    def setUp(self):
        self.receipt, self.values = fixture()
        self.provider = FakeNative(self.receipt, self.values)
        self.persisted = []

    def finalize(self):
        return runner.finalize(
            self.receipt, self.provider,
            lambda value: self.persisted.append(copy.deepcopy(value)),
            now=runner.timestamp(self.receipt["deadline"]), sleep=lambda _: None)

    def assert_no_mutations(self):
        self.assertTrue(all(call[0] == "GET" for call in self.provider.calls))

    def test_deadline_before_first_mutation(self):
        with self.assertRaises(runner.GuardError):
            runner.finalize(self.receipt, self.provider, lambda _: None,
                            now=runner.timestamp(self.receipt["deadline"]) - 1801)
        self.assert_no_mutations()

    def test_twelve_hour_bound(self):
        self.receipt["deadline"] = "2026-11-05T12:00:01Z"
        with self.assertRaises(runner.GuardError):
            runner.validate(self.receipt)

    def test_external_teardown_can_start_inside_twelve_hour_bound(self):
        runner.finalize(
            self.receipt, self.provider, lambda _: None,
            now=runner.timestamp(self.receipt["deadline"]) - 1800, sleep=lambda _: None)
        self.assertIsNone(self.provider.values["vm"])

    def test_ownership_change_aborts_everything(self):
        self.provider.values["disk"]["tags"]["privateRunnerRun"] = str(uuid.uuid4())
        with self.assertRaises(runner.GuardError):
            self.finalize()
        self.assert_no_mutations()

    def test_each_ado_object_has_identity_guard(self):
        for kind in ("pool", "queue", "agent"):
            with self.subTest(kind=kind):
                value = copy.deepcopy(self.values[kind])
                value["name"] = "someone-elses-runner"
                with self.assertRaises(runner.GuardError):
                    runner.owned(self.receipt, kind, value)

    def test_shared_pool_and_foreign_resource_refused(self):
        for key, value in (("pool", 66), ("queue", 294)):
            changed = copy.deepcopy(self.receipt)
            changed[key] = value
            with self.assertRaises(runner.GuardError):
                runner.validate(changed)
        self.receipt["resources"]["vm"] += "-foreign"
        with self.assertRaises(runner.GuardError):
            runner.validate(self.receipt)

    def test_pool_only_pipeline_979(self):
        for permissions in (
            {"allPipelines": {"authorized": True}, "pipelines": []},
            {"allPipelines": {"authorized": False},
             "pipelines": [{"id": 980, "authorized": True}]},
            {"pipelines": [{"id": 979, "authorized": True}]},
        ):
            with self.subTest(permissions=permissions):
                self.provider.values["permissions"] = permissions
                with self.assertRaises(runner.GuardError):
                    self.finalize()
                self.assert_no_mutations()

    def test_vm_cannot_cascade_delete_unowned_attachments(self):
        mutations = (
            lambda p: p["storageProfile"]["osDisk"].update(deleteOption="Delete"),
            lambda p: p["storageProfile"]["osDisk"]["managedDisk"].update(id="foreign-disk"),
            lambda p: p["storageProfile"].update(dataDisks=[{"id": "foreign-disk"}]),
            lambda p: p["networkProfile"]["networkInterfaces"][0].update(id="foreign-nic"),
            lambda p: p["networkProfile"]["networkInterfaces"][0]["properties"].update(
                deleteOption="Delete"),
        )
        for mutate in mutations:
            with self.subTest(mutate=mutate):
                self.provider = FakeNative(self.receipt, self.values)
                mutate(self.provider.values["vm"]["properties"])
                with self.assertRaises(runner.GuardError):
                    self.finalize()
                self.assert_no_mutations()

    def test_external_finalizer_unregisters_revokes_and_verifies_absence(self):
        result = self.finalize()
        actions = [(method, next(k for k, u in self.provider.urls.items() if u == url))
                   for method, url, _, _ in self.provider.calls if method != "GET"]
        self.assertEqual(actions[:4], [("DELETE", "agent"), ("PATCH", "permissions"),
                                      ("DELETE", "queue"), ("DELETE", "pool")])
        self.assertEqual([kind for method, kind in actions if method == "DELETE"],
                         ["agent", "queue", "pool", "vm", "nic", "disk", "subnet", "nsg"])
        self.assertTrue(all(self.provider.values[k] is None for k in result["verifiedAbsent"]))
        self.assertEqual(len(self.persisted), 9)
        self.assertEqual(self.persisted[-1]["completed"], self.receipt["completed"])
        runner.restricted(self.provider.values["permissions"], False)
        self.finalize()

    def test_failed_unregister_never_reports_cleanup(self):
        self.provider.refuse = "agent"
        with self.assertRaises(runner.GuardError):
            self.finalize()
        self.assertIsNotNone(self.provider.values["queue"])

    def test_failed_revoke_never_deletes_queue(self):
        self.provider.refuse = "revoke"
        with self.assertRaises(runner.GuardError):
            self.finalize()
        self.assertIsNotNone(self.provider.values["queue"])
        self.assertNotIn("revoke", self.receipt["completed"])

    def test_failed_resource_delete_never_reports_absence(self):
        self.provider.refuse = "vm"
        with self.assertRaises(runner.GuardError):
            self.finalize()
        self.assertIsNotNone(self.provider.values["vm"])

    def test_crash_after_revoke_before_journal_write_is_resumable(self):
        self.provider.values["permissions"]["pipelines"][0]["authorized"] = False
        self.finalize()

    def test_partial_cleanup_retries_without_vm_or_agent(self):
        self.provider.values["agent"] = None
        self.provider.values["vm"] = None
        self.finalize()

    def test_completed_markers_never_hide_live_resources(self):
        self.receipt["completed"] = ["vm", "nic", "disk", "subnet", "nsg"]
        result = self.finalize()
        self.assertIn("vm", result["verifiedAbsent"])
        self.assertIsNone(self.provider.values["vm"])

    def test_ownership_is_rechecked_immediately_before_delete(self):
        original = self.provider.request
        disk_reads = 0

        def changed(method, url, body=None, etag=None):
            nonlocal disk_reads
            if method == "GET" and url == self.provider.urls["disk"]:
                disk_reads += 1
                if disk_reads == 2:
                    self.provider.values["disk"]["tags"]["privateRunnerRun"] = "changed"
            return original(method, url, body, etag)

        self.provider.request = changed
        with self.assertRaises(runner.GuardError):
            self.finalize()
        self.assertIsNotNone(self.provider.values["disk"])
        self.assertFalse(any(m == "DELETE" and u == self.provider.urls["disk"]
                             for m, u, _, _ in self.provider.calls))

    def test_arm_deletes_use_live_etag(self):
        self.finalize()
        for method, url, _, etag in self.provider.calls:
            if method == "DELETE" and url.startswith(runner.ARM):
                self.assertEqual(etag, '"initial"')

    def test_unreceipted_resources_block_cleanup(self):
        self.provider.orphans = [{"id": "unexpected", "tags": {
            "privateRunnerRun": self.receipt["run"]}}]
        with self.assertRaises(runner.GuardError):
            self.finalize()
        self.assert_no_mutations()

    def test_subnet_ownership_change_blocks_cleanup(self):
        self.provider.values["subnet"]["properties"]["addressPrefix"] = "10.228.0.0/24"
        with self.assertRaises(runner.GuardError):
            self.finalize()
        self.assert_no_mutations()

    def test_atomic_receipt_persistence(self):
        folder = Path(__file__).parent / (".test-state-" + uuid.uuid4().hex)
        folder.mkdir()
        try:
            path = folder / "receipt.json"
            runner.save(path, self.receipt)
            self.assertEqual(json.loads(path.read_text()), self.receipt)
            self.receipt["completed"].append("agent")
            runner.save(path, self.receipt)
            self.assertEqual(json.loads(path.read_text())["completed"], ["agent"])
            self.assertFalse(path.with_name("receipt.json.next").exists())
            path.with_name("receipt.json.next").write_text("unreconciled", encoding="utf-8")
            with self.assertRaises(FileExistsError):
                runner.save(path, self.receipt)
            self.assertEqual(path.with_name("receipt.json.next").read_text(), "unreconciled")
        finally:
            shutil.rmtree(folder)

def mutants():
    global runner
    original = runner
    source = Path(runner.__file__).read_text(encoding="utf-8")
    changes = [
        ("ownership", 'value.get("tags", {}).get("privateRunnerRun") == receipt["run"]',
         "True", "test_ownership_change_aborts_everything"),
        ("deadline", 'now >= max(timestamp(receipt["started"]), timestamp(receipt["deadline"]) - 1800)', "True",
         "test_deadline_before_first_mutation"),
        ("only-979", 'actual == ({979} if authorized else set())', "True",
         "test_pool_only_pipeline_979"),
        ("unregister", 'provider.request("DELETE", urls[kind], etag=value.get("etag"))',
         'None if kind == "agent" else provider.request("DELETE", urls[kind], etag=value.get("etag"))',
         "test_external_finalizer_unregisters_revokes_and_verifies_absence"),
        ("revoke", 'provider.request("PATCH", urls["permissions"], {',
         'provider.request("GET", urls["permissions"], {',
         "test_external_finalizer_unregisters_revokes_and_verifies_absence"),
        ("owned-delete", 'provider.request("DELETE", urls[kind], etag=value.get("etag"))',
         'None if kind == "vm" else provider.request("DELETE", urls[kind], etag=value.get("etag"))',
         "test_external_finalizer_unregisters_revokes_and_verifies_absence"),
        ("journal", "persist(receipt)", "None",
         "test_external_finalizer_unregisters_revokes_and_verifies_absence"),
        ("cascade", 'disk.get("deleteOption") == "Detach"', "True",
         "test_vm_cannot_cascade_delete_unowned_attachments"),
    ]
    survived = []
    try:
        for label, before, after, test in changes:
            assert source.count(before) == 1, label
            runner = types.ModuleType("mutant")
            exec(compile(source.replace(before, after), "<deliberate-mutant>", "exec"),
                 runner.__dict__)
            result = unittest.TestResult()
            LifecycleTests(test).run(result)
            if result.wasSuccessful():
                survived.append(label)
            print(f"{label}: {'SURVIVED' if result.wasSuccessful() else 'killed'}")
    finally:
        runner = original
    return 1 if survived else 0


if __name__ == "__main__":
    if sys.argv[1:] == ["--mutants"]:
        raise SystemExit(mutants())
    unittest.main()
