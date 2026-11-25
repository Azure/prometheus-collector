import json
import os
import runpy
import shutil
import subprocess
import sys
import time
import uuid
from contextlib import nullcontext
from pathlib import Path
from types import SimpleNamespace
from unittest.mock import patch

import pytest


RUNNER = Path(__file__).resolve().parents[2] / "otelcollector" / "test" / "scenarios" / "run-scenario.py"


def load_runner():
    with patch.dict(sys.modules, {"scan_trivy": SimpleNamespace(resolve_tag=None)}):
        return runpy.run_path(str(RUNNER))


def shell_result(script, root, session_status, process_name="sshd", user_present=True,
                 socket_status=1, clears_after=0, getent_status=2, expect_failure=False):
    prelude = """
flock() { :; }
id() { [ "$USER_PRESENT" = 1 ]; }
getent() { return "$GETENT_STATUS"; }
useradd() { return 1; }
PGREP_CALLS=0
pgrep() {
  PGREP_CALLS=$((PGREP_CALLS + 1))
  if [ "$CLEARS_AFTER" -gt 0 ] && [ "$PGREP_CALLS" -gt "$CLEARS_AFTER" ]; then return 1; fi
  if [ "$SESSION_STATUS" = 0 ] && [[ " $* " == *" -x sshd "* ]] && [ "$PROCESS_NAME" != sshd ]; then
    return 1
  fi
  return "$SESSION_STATUS"
}
ss() {
  if [ "$SOCKET_STATUS" = 0 ]; then echo "connected port 22"; fi
  if [ "$SOCKET_STATUS" = 2 ]; then return 2; fi
  if [ "$SOCKET_STATUS" = 3 ] && [[ " $* " != *" exclude time-wait "* ]]; then
    echo "TIME-WAIT port 22"
  fi
  return 0
}
sleep() { :; }
install() { mkdir -p "${@: -1}"; }
"""
    script = script.replace("/var/lib/scenario-tunnel", (root / "tunnel").as_posix())
    script = script.replace("/home/scenario-tunnel/.ssh", (root / "ssh").as_posix())
    script = script.replace("/home/scenario-tunnel", (root / "home").as_posix())
    script = script.replace("flush_state 7 8 9", ":").replace("flush_state 7 9", ":")
    result = subprocess.run(
        [shutil.which("bash"), "-c", prelude + script],
        capture_output=True, text=True, env={
            **os.environ, "SESSION_STATUS": str(session_status), "PROCESS_NAME": process_name,
            "USER_PRESENT": str(int(user_present)), "SOCKET_STATUS": str(socket_status),
            "CLEARS_AFTER": str(clears_after), "GETENT_STATUS": str(getent_status),
        },
    )
    if expect_failure:
        assert result.returncode != 0
        return
    assert result.returncode == 0, result.stderr
    records = [line.removeprefix("SCENARIO_RESULT=") for line in result.stdout.splitlines()
               if line.startswith("SCENARIO_RESULT=")]
    assert len(records) == 1
    return json.loads(records[0])


@pytest.mark.parametrize(("session_status", "process_name", "socket_status"), [
    (0, "sshd", 1), (0, "helper", 1), (2, "sshd", 1), (1, "sshd", 0), (1, "sshd", 2),
])
@pytest.mark.parametrize("same_owner", [False, True])
def test_tunnel_claim_rejects_live_or_uninspectable_session(session_status, process_name, socket_status, same_owner):
    root = Path(".scenario-tunnel-" + uuid.uuid4().hex)
    root.mkdir()
    try:
        tunnel = root / "tunnel"
        tunnel.mkdir()
        runner = load_runner()
        run_id = str(uuid.uuid4())
        if same_owner:
            (tunnel / "lease").write_text(f"{run_id} {int(time.time() + 300)}\n", newline="\n")
        script = runner["tunnel_lease"](run_id, time.time() + 300, tunnel.as_posix())
        script += '\nprintf \'SCENARIO_RESULT={"claimed":true}\\n\'\n'
        result = shell_result(script, root, session_status, process_name, socket_status=socket_status)
        assert result.get("leaseOwner") == "unsettled"
        assert (tunnel / "lease").exists() is same_owner
    finally:
        shutil.rmtree(root)


@pytest.mark.parametrize(("session_status", "process_name", "socket_status", "user_present",
                          "clears_after", "account_not_created", "released"), [
    (0, "sshd", 1, True, 0, False, False), (0, "helper", 1, True, 0, False, False),
    (1, "sshd", 1, True, 0, False, True), (0, "sshd", 1, True, 1, False, True),
    (1, "sshd", 3, True, 0, False, True), (2, "sshd", 1, True, 0, False, False),
    (1, "sshd", 0, True, 0, False, False), (1, "sshd", 2, True, 0, False, False),
    (1, "sshd", 1, False, 0, False, False), (1, "sshd", 1, False, 0, True, True),
])
def test_tunnel_cleanup_keeps_lease_until_sessions_are_gone(
        session_status, process_name, socket_status, user_present, clears_after, account_not_created, released):
    root = Path(".scenario-tunnel-" + uuid.uuid4().hex)
    root.mkdir()
    try:
        tunnel, ssh = root / "tunnel", root / "ssh"
        tunnel.mkdir()
        ssh.mkdir()
        run_id = str(uuid.uuid4())
        (tunnel / "lease").write_text(f"{run_id} {int(time.time() + 300)}\n", newline="\n")
        if account_not_created:
            (tunnel / "account-not-created").touch()
        (ssh / "authorized_keys").write_text("ephemeral public key\n")
        calls = []

        def az(*args, **_):
            calls.append(args)
            return {"access": "Deny"} if "show" in args else {}

        host = SimpleNamespace(args=SimpleNamespace(evidence=root), receipt={"resourceGroup": "group"}, az=az)
        runner = load_runner()
        close = runner["close_private_api"]
        vm_script = lambda _host, _vm, script: shell_result(
            script, root, session_status, process_name, user_present, socket_status, clears_after)
        with patch.dict(close.__globals__, {"vm_script": vm_script}):
            if released:
                close(host, {"orchestratorVmName": "vm", "orchestratorNsgName": "nsg"}, run_id)
            else:
                with pytest.raises(RuntimeError):
                    close(host, {"orchestratorVmName": "vm", "orchestratorNsgName": "nsg"}, run_id)
        assert any("create" in call for call in calls)
        assert not (ssh / "authorized_keys").exists()
        assert not (tunnel / "pending").exists()
        assert (tunnel / "lease").exists() is (not released)
        proof = root / "private-api-cleanup.json"
        assert proof.exists() is released
        if proof.exists():
            assert json.loads(proof.read_text())["status"] == "PASS"
    finally:
        shutil.rmtree(root)


@pytest.mark.parametrize(("user_present", "socket_status"), [(True, 1), (False, 1), (True, 3)])
def test_available_tunnel_can_be_claimed(user_present, socket_status):
    root = Path(".scenario-tunnel-" + uuid.uuid4().hex)
    root.mkdir()
    try:
        tunnel = root / "tunnel"
        tunnel.mkdir()
        run_id = str(uuid.uuid4())
        script = load_runner()["tunnel_lease"](run_id, time.time() + 300, tunnel.as_posix())
        script += '\nprintf \'SCENARIO_RESULT={"claimed":true}\\n\'\n'
        result = shell_result(script, root, 1, user_present=user_present, socket_status=socket_status)
        assert result["claimed"] is True
        assert (tunnel / "lease").read_text().startswith(run_id + " ")
    finally:
        shutil.rmtree(root)


def test_repeated_cleanup_preserves_run_bound_proof():
    root = Path(".scenario-tunnel-" + uuid.uuid4().hex)
    root.mkdir()
    try:
        tunnel, ssh = root / "tunnel", root / "ssh"
        tunnel.mkdir()
        ssh.mkdir()
        run_id = str(uuid.uuid4())
        (tunnel / "lease").write_text(f"{run_id} {int(time.time() + 300)}\n", newline="\n")
        (ssh / "authorized_keys").write_text("ephemeral public key\n")
        host = SimpleNamespace(
            args=SimpleNamespace(evidence=root), receipt={"resourceGroup": "group"},
            az=lambda *args, **_: {"access": "Deny"} if "show" in args else {},
        )
        close = load_runner()["close_private_api"]
        outputs = {"orchestratorVmName": "vm", "orchestratorNsgName": "nsg"}
        vm_script = lambda _host, _vm, script: shell_result(script, root, 1)
        with patch.dict(close.__globals__, {"vm_script": vm_script}):
            close(host, outputs, run_id)
            proof_path = root / "private-api-cleanup.json"
            original = json.loads(proof_path.read_text())
            close(host, outputs, run_id)
        assert original["status"] == "PASS"
        assert original["runId"] == run_id
        assert json.loads(proof_path.read_text()) == original
    finally:
        shutil.rmtree(root)


def test_refused_claim_does_not_close_existing_tunnel():
    root = Path(".scenario-tunnel-" + uuid.uuid4().hex)
    root.mkdir()
    try:
        kubeconfig = root / "kubeconfig"
        kubeconfig.write_text("clusters:\n- cluster:\n    server: https://fixture.azmk8s.io\n")
        closes = []

        def generate_key(command, **_):
            Path(command[-1] + ".pub").write_text("ssh-ed25519 fake\n")

        cluster = {"apiServerAccessProfile": {
            "enablePrivateCluster": True, "enablePrivateClusterPublicFqdn": False,
        }, "privateFqdn": "fixture.azmk8s.io"}
        host = SimpleNamespace(
            args=SimpleNamespace(run_id=str(uuid.uuid4()), max_run_seconds=300, evidence=root),
            scenario={"clusterFeatures": ["private-api"]}, kubeconfig=kubeconfig,
            receipt={"resourceGroup": "group"}, az=lambda *_, **__: cluster,
            commands=SimpleNamespace(run=generate_key, deadline=None),
        )
        private_api = load_runner()["private_api"]
        overrides = {
            "tempfile": SimpleNamespace(TemporaryDirectory=lambda **_: nullcontext(str(root))),
            "vm_script": lambda *_: {"leaseOwner": "unsettled", "leaseUntil": 0},
            "close_private_api": lambda *_: closes.append(True),
        }
        with patch("shutil.which", return_value="ssh"), patch.dict(private_api.__wrapped__.__globals__, overrides):
            with pytest.raises(RuntimeError, match="unsettled"):
                with private_api(host, {
                    "clusterName": "cluster", "orchestratorVmName": "vm",
                    "orchestratorNsgName": "nsg", "orchestratorPublicIp": "192.0.2.1",
                }):
                    pass
        assert not closes
        assert not (root / "private-api-lease.json").exists()
    finally:
        shutil.rmtree(root)


@pytest.mark.parametrize(("getent_status", "released"), [(2, True), (1, False)])
def test_failed_account_creation_releases_only_after_proving_absence(getent_status, released):
    root = Path(".scenario-tunnel-" + uuid.uuid4().hex)
    root.mkdir()
    try:
        (root / "tunnel").mkdir()
        (root / "ssh").mkdir()
        kubeconfig = root / "kubeconfig"
        kubeconfig.write_text("clusters:\n- cluster:\n    server: https://fixture.azmk8s.io\n")
        cluster = {"apiServerAccessProfile": {
            "enablePrivateCluster": True, "enablePrivateClusterPublicFqdn": False,
        }, "privateFqdn": "fixture.azmk8s.io"}

        def az(*args, **_):
            if args[:2] == ("aks", "show"):
                return cluster
            return {"access": "Deny"} if "show" in args else {}

        def generate_key(command, **_):
            Path(command[-1] + ".pub").write_text("ssh-ed25519 fake\n")

        host = SimpleNamespace(
            args=SimpleNamespace(run_id=str(uuid.uuid4()), max_run_seconds=300, evidence=root),
            scenario={"clusterFeatures": ["private-api"]}, kubeconfig=kubeconfig,
            receipt={"resourceGroup": "group"}, az=az,
            commands=SimpleNamespace(run=generate_key, deadline=None),
        )
        private_api = load_runner()["private_api"]

        def vm_script(_host, _vm, script):
            setup = "useradd --create-home" in script
            result = shell_result(script, root, 1, user_present=False, getent_status=getent_status,
                                  expect_failure=setup)
            if setup:
                raise RuntimeError("account creation failed")
            return result

        overrides = {
            "tempfile": SimpleNamespace(TemporaryDirectory=lambda **_: nullcontext(str(root))),
            "vm_script": vm_script,
        }
        bash = shutil.which("bash")
        with patch("shutil.which", side_effect=lambda name: bash if name == "bash" else "ssh"), \
                patch.dict(private_api.__wrapped__.__globals__, overrides):
            with pytest.raises(RuntimeError):
                with private_api(host, {
                    "clusterName": "cluster", "orchestratorVmName": "vm",
                    "orchestratorNsgName": "nsg", "orchestratorPublicIp": "192.0.2.1",
                }):
                    pass
        assert (root / "tunnel" / "lease").exists() is (not released)
        assert (root / "private-api-cleanup.json").exists() is released
    finally:
        shutil.rmtree(root)
