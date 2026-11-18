"""Short-lived, host-key-pinned SSH forwarding for retained private AKS hosts."""

from contextlib import contextmanager
import ipaddress
import json
from pathlib import Path
import re
import shutil
import socket
import subprocess
import tempfile
import time
from datetime import datetime, timedelta, timezone
from urllib.parse import urlparse
from urllib.request import ProxyHandler, build_opener

import yaml


def agent_prefix(value):
    address = ipaddress.ip_address(value.strip())
    if address.version != 4 or not address.is_global:
        raise ValueError("Private API ingress requires the current agent's public IPv4 /32")
    return str(address) + "/32"


def vm_script(host, name, script):
    response = host.az(
        "vm", "run-command", "invoke", "--resource-group", host.receipt["resourceGroup"],
        "--name", name, "--command-id", "RunShellScript", "--scripts", script,
        json_output=True, private=True)
    streams = []
    for row in response.get("value", []):
        if row.get("code") == "ComponentStatus/StdOut/succeeded":
            streams.append(row["message"])
        elif row.get("code") == "ProvisioningState/succeeded":
            match = re.search(r"\[stdout\]\r?\n(.*?)\r?\n\[stderr\](?:\r?\n|$)",
                              row["message"], re.DOTALL)
            if match:
                streams.append(match[1])
    stdout = "\n".join(streams)
    values = re.findall(r"^SCENARIO_RESULT=(.+)$", stdout, re.MULTILINE)
    if len(values) != 1:
        raise RuntimeError("Owned VM command did not return one successful scenario result")
    return json.loads(values[0])


def close_private_api(host, outputs):
    errors = []
    try:
        host.az("network", "nsg", "rule", "create", "--resource-group", host.receipt["resourceGroup"],
                "--nsg-name", outputs["orchestratorNsgName"], "--name", "ActivePipelineTunnel",
                "--priority", "100", "--access", "Deny", "--direction", "Inbound", "--protocol", "Tcp",
                "--source-address-prefixes", "*", "--source-port-ranges", "*",
                "--destination-address-prefixes", "*", "--destination-port-ranges", "22")
        rule = host.az("network", "nsg", "rule", "show", "--resource-group", host.receipt["resourceGroup"],
                       "--nsg-name", outputs["orchestratorNsgName"], "--name", "ActivePipelineTunnel",
                       json_output=True)
        if rule.get("access") != "Deny":
            raise RuntimeError("Temporary private API ingress was not revoked")
    except RuntimeError as error:
        errors.append(str(error))
    try:
        result = vm_script(host, outputs["orchestratorVmName"], """set -eu
rm -f /home/scenario-tunnel/.ssh/authorized_keys
test ! -e /home/scenario-tunnel/.ssh/authorized_keys
printf 'SCENARIO_RESULT={"keyRemoved":true}\\n'
""")
        if result.get("keyRemoved") is not True:
            raise RuntimeError("Temporary SSH key removal was not confirmed")
    except RuntimeError as error:
        errors.append(str(error))
    if errors:
        raise RuntimeError("Private API cleanup failed: " + "; ".join(errors))
    proof = {"status": "PASS", "ingress": "Deny", "keyRemoved": True}
    (host.args.evidence / "private-api-cleanup.json").write_text(json.dumps(proof) + "\n")
    print("Private API cleanup: " + json.dumps(proof), flush=True)


@contextmanager
def private_api(host, outputs):
    if "private-api" not in host.scenario["clusterFeatures"]:
        yield
        return
    cluster = host.az("aks", "show", "--resource-group", host.receipt["resourceGroup"],
                      "--name", outputs["clusterName"], json_output=True)
    profile = cluster.get("apiServerAccessProfile") or {}
    fqdn = cluster.get("privateFqdn", "")
    config = yaml.safe_load(host.kubeconfig.read_text())
    if (profile.get("enablePrivateCluster") is not True
            or profile.get("enablePrivateClusterPublicFqdn") is not False
            or not re.fullmatch(r"[a-z0-9.-]+\.azmk8s\.io", fqdn)
            or len(config["clusters"]) != 1
            or urlparse(config["clusters"][0]["cluster"]["server"]).hostname != fqdn):
        raise RuntimeError("Private API adapter requires a private-only AKS API and its exact kubeconfig")
    for tool in ("ssh", "ssh-keygen"):
        if not shutil.which(tool):
            raise RuntimeError(f"Private API adapter requires {tool}")
    group = host.receipt["resourceGroup"]
    vm, nsg = outputs["orchestratorVmName"], outputs["orchestratorNsgName"]
    public_ip = str(ipaddress.IPv4Address(outputs["orchestratorPublicIp"]))

    def admit_agent():
        with build_opener(ProxyHandler({})).open("https://api.ipify.org", timeout=30) as response:
            prefix = agent_prefix(response.read(128).decode("ascii"))
        host.az("network", "nsg", "rule", "create", "--resource-group", group,
                "--nsg-name", nsg, "--name", "ActivePipelineTunnel", "--priority", "100",
                "--access", "Allow", "--direction", "Inbound", "--protocol", "Tcp",
                "--source-address-prefixes", prefix, "--source-port-ranges", "*",
                "--destination-address-prefixes", "*", "--destination-port-ranges", "22")
        print(f"Private API ingress: {prefix} -> {public_ip}:22 (direct IPv4 discovery)", flush=True)
        return prefix

    process = None
    key_attempted = False
    with tempfile.TemporaryDirectory(prefix="ama-private-api-") as directory:
        key = Path(directory) / "key"
        host.commands.run(["ssh-keygen", "-q", "-t", "ed25519", "-N", "", "-f", str(key)], private=True)
        public_key = key.with_suffix(".pub").read_text().strip()
        expires = (datetime.now(timezone.utc) + timedelta(hours=9)).strftime("%Y%m%d%H%M%SZ")
        try:
            key_attempted = True
            proof = vm_script(host, vm, f"""set -eu
id scenario-tunnel >/dev/null 2>&1 || useradd --create-home --shell /bin/bash scenario-tunnel
printf '%s\\n' 'Match User scenario-tunnel' '    AuthenticationMethods publickey' '    AllowTcpForwarding local' 'Match all' > /etc/ssh/sshd_config.d/00-scenario-tunnel.conf
sshd -t
test "$(sshd -T -C user=scenario-tunnel,host=localhost,addr=127.0.0.1 | awk '$1 == "allowtcpforwarding" {{print $2}}')" = local
systemctl reload ssh
install -d -m 700 -o scenario-tunnel -g scenario-tunnel /home/scenario-tunnel/.ssh
printf '%s\\n' 'expiry-time="{expires}",no-agent-forwarding,no-X11-forwarding,no-pty,no-user-rc,command="/bin/false",permitopen="{fqdn}:443" {public_key}' > /home/scenario-tunnel/.ssh/authorized_keys
chown scenario-tunnel:scenario-tunnel /home/scenario-tunnel/.ssh/authorized_keys
chmod 600 /home/scenario-tunnel/.ssh/authorized_keys
IP=$(getent ahostsv4 '{fqdn}' | head -n1 | cut -d' ' -f1)
KEY=$(cat /etc/ssh/ssh_host_ed25519_key.pub)
printf 'SCENARIO_RESULT={{"privateIp":"%s","hostKey":"%s"}}\\n' "$IP" "$KEY"
""")
            private_ip = ipaddress.IPv4Address(proof["privateIp"])
            if not private_ip.is_private or not re.fullmatch(
                    r"ssh-ed25519 [A-Za-z0-9+/=]+(?: [^\r\n]*)?", proof["hostKey"]):
                raise RuntimeError("Orchestrator did not resolve a private API address or supply an SSH host key")
            known_hosts = Path(directory) / "known_hosts"
            known_hosts.write_text(public_ip + " " + proof["hostKey"] + "\n")
            prefix = admit_agent()
            with socket.socket() as listener:
                listener.bind(("127.0.0.1", 0))
                port = listener.getsockname()[1]
            command = [
                shutil.which("ssh"), "-N", "-T", "-i", str(key),
                "-o", "BatchMode=yes", "-o", "IdentitiesOnly=yes",
                "-o", "StrictHostKeyChecking=yes", "-o", f"UserKnownHostsFile={known_hosts}",
                "-o", "ExitOnForwardFailure=yes", "-o", "ConnectTimeout=30",
                "-o", "ServerAliveInterval=15", "-o", "ServerAliveCountMax=3",
                "-L", f"127.0.0.1:{port}:{fqdn}:443", f"scenario-tunnel@{public_ip}",
            ]
            with (Path(directory) / "ssh.log").open("w+") as log:
                process = subprocess.Popen(command, stdout=log, stderr=log)
                config["clusters"][0]["cluster"].update(
                    server=f"https://127.0.0.1:{port}", **{"tls-server-name": fqdn})
                host.kubeconfig.write_text(yaml.safe_dump(config))
                deadline = time.monotonic() + 600
                while True:
                    if process.poll() is not None:
                        log.seek(0)
                        error = log.read()
                        if "Connection timed out" not in error or time.monotonic() >= deadline:
                            raise RuntimeError("Private API tunnel exited: " + error)
                        print("Private API connect timed out; refreshing the single agent /32 before retry", flush=True)
                        prefix = admit_agent()
                        log.seek(0)
                        log.truncate()
                        process = subprocess.Popen(command, stdout=log, stderr=log)
                        continue
                    try:
                        if host.kube("get", "--raw=/readyz", "--request-timeout=10s").strip() != "ok":
                            raise RuntimeError("Private API is not ready")
                        break
                    except RuntimeError:
                        if time.monotonic() >= deadline:
                            raise
                        time.sleep(5)
                evidence = {"status": "PASS", "privateFqdn": fqdn, "privateIp": str(private_ip),
                            "orchestrator": vm, "agentPrefix": prefix, "authentication": "ephemeral SSH key",
                            "hostKeySource": "authenticated Azure VM run-command", "readyz": "ok"}
                (host.args.evidence / "private-api.json").write_text(json.dumps(evidence, indent=2) + "\n")
                print("Private API proof: " + json.dumps(evidence), flush=True)
                yield
        finally:
            if process is not None:
                process.terminate()
                try:
                    process.wait(timeout=15)
                except subprocess.TimeoutExpired:
                    process.kill()
                    process.wait(timeout=15)
            # Cleanup gets its own budget even when the scenario deadline expired.
            deadline, host.commands.deadline = host.commands.deadline, None
            try:
                if key_attempted:
                    close_private_api(host, outputs)
            finally:
                host.commands.deadline = deadline
