"""Native finalizer for one receipted private runner; requires an external host."""

import argparse
import datetime as dt
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import time
import urllib.error
import urllib.request
import uuid


SUBSCRIPTION = "9b96ebbd-c57a-42d1-bbe9-b69296e4c7fb"
PROJECT = "2d36c31d-2f89-409f-9a3e-32e4e9699840"
ORG = "https://dev.azure.com/github-private"
ARM = "https://management.azure.com"
ADO_RESOURCE = "499b84ac-1321-427f-aa17-267ca6975798"
TYPES = {
    "vm": ("Microsoft.Compute/virtualMachines", "2024-07-01"),
    "disk": ("Microsoft.Compute/disks", "2024-03-02"),
    "nic": ("Microsoft.Network/networkInterfaces", "2024-05-01"),
    "nsg": ("Microsoft.Network/networkSecurityGroups", "2024-05-01"),
    "nat": ("Microsoft.Network/natGateways", "2024-05-01"),
    "ip": ("Microsoft.Network/publicIPAddresses", "2024-05-01"),
}


class GuardError(RuntimeError):
    pass


def require(condition, message):
    if not condition:
        raise GuardError(message)


def timestamp(value):
    parsed = dt.datetime.fromisoformat(value.replace("Z", "+00:00"))
    require(parsed.utcoffset() == dt.timedelta(0), "UTC timestamps required")
    return parsed.timestamp()


def validate(receipt):
    require(receipt["version"] == 1, "Unsupported receipt version")
    run = str(uuid.UUID(receipt["run"]))
    require(run == receipt["run"], "Canonical run UUID required")
    start, deadline = timestamp(receipt["started"]), timestamp(receipt["deadline"])
    require(0 < deadline - start <= 12 * 3600, "Deadline must be within 12 hours")
    rg, vnet = receipt["resourceGroup"], receipt["vnet"]
    require(re.fullmatch(r"[a-zA-Z0-9_-]{1,80}", rg), "Invalid resource group")
    require(re.fullmatch(r"[a-zA-Z0-9_-]{1,64}", vnet), "Invalid VNet")
    name = "pr-" + uuid.UUID(run).hex
    require(receipt["name"] == name, "Run-scoped name mismatch")
    root = f"/subscriptions/{SUBSCRIPTION}/resourceGroups/{rg}/providers/"
    expected = {
        kind: root + resource_type + "/" + name
        for kind, (resource_type, _) in TYPES.items()
    }
    expected["subnet"] = root + f"Microsoft.Network/virtualNetworks/{vnet}/subnets/{name}"
    required = {"vm", "disk", "nic", "nsg", "subnet"}
    require(set(receipt["resources"]) in (required, required | {"nat", "ip"}),
            "Only approved owned resource set allowed")
    for kind, resource_id in receipt["resources"].items():
        require(resource_id == expected[kind], "Resource ID outside exact run allowlist")
    for key, forbidden in (("pool", 66), ("queue", 294), ("agent", None)):
        value = receipt[key]
        require(type(value) is int and value > 0 and value != forbidden,
                "Only a new, explicitly receipted pool/queue/agent is allowed")
    require(isinstance(receipt["completed"], list), "Missing cleanup progress")
    require(set(receipt["completed"]) <= set(receipt["resources"]) |
            {"agent", "revoke", "queue", "pool"}, "Invalid cleanup progress")


def save(path, receipt):
    path = Path(path)
    next_path = path.with_name(path.name + ".next")
    with next_path.open("x", encoding="utf-8") as stream:
        json.dump(receipt, stream, indent=2)
        stream.write("\n")
        stream.flush()
        os.fsync(stream.fileno())
    os.replace(next_path, path)


class NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):
        raise GuardError("Unexpected redirect refused")


class Native:
    """Uses the external host's own Azure CLI session; never accepts copied tokens."""

    def request(self, method, url, body=None, etag=None):
        require(url.startswith(ARM + "/subscriptions/" + SUBSCRIPTION + "/") or
                url.startswith(ORG + "/"), "Endpoint outside approved providers")
        resource = ADO_RESOURCE if url.startswith(ORG + "/") else ARM + "/"
        result = subprocess.run(
            [shutil.which("az") or "az", "account", "get-access-token",
             "--subscription", SUBSCRIPTION, "--resource", resource,
             "--query", "accessToken", "-o", "tsv"],
            capture_output=True, text=True, timeout=60, check=False,
        )
        require(result.returncode == 0 and result.stdout.strip(),
                "External host Azure CLI authentication unavailable; details suppressed")
        headers = {"Authorization": "Bearer " + result.stdout.strip()}
        if body is not None:
            headers["Content-Type"] = "application/json"
        if etag:
            headers["If-Match"] = etag
        request = urllib.request.Request(
            url, data=None if body is None else json.dumps(body).encode(),
            headers=headers, method=method,
        )
        try:
            with urllib.request.build_opener(NoRedirect).open(request, timeout=60) as response:
                data = response.read()
                return json.loads(data) if data else {}
        except urllib.error.HTTPError as error:
            if error.code == 404 and method in ("GET", "DELETE"):
                return None
            raise GuardError(f"{method} failed HTTP {error.code}; response suppressed") from None
        except urllib.error.URLError:
            raise GuardError("Provider connection failed; details suppressed") from None


def endpoints(receipt):
    pool, queue, agent = receipt["pool"], receipt["queue"], receipt["agent"]
    base = ORG + "/_apis/distributedtask/pools/" + str(pool)
    project = ORG + "/" + PROJECT
    return {
        "pool": base + "?api-version=7.1",
        "agents": base + "/agents?api-version=7.1",
        "agent": base + "/agents/" + str(agent) + "?api-version=7.1",
        "queue": project + "/_apis/distributedtask/queues/" + str(queue) + "?api-version=7.1",
        "permissions": project + "/_apis/pipelines/pipelinepermissions/queue/" +
        str(queue) + "?api-version=7.1-preview.1",
        **{
            kind: ARM + resource_id + "?api-version=" +
            ("2024-05-01" if kind == "subnet" else TYPES[kind][1])
            for kind, resource_id in receipt["resources"].items()
        },
    }


def owned(receipt, kind, value):
    if value is None:
        return
    name = receipt["name"]
    if kind in ("pool", "queue", "agent"):
        require(value["id"] == receipt[kind] and value["name"] == name,
                "ADO ownership changed")
        if kind == "pool":
            require(value.get("isHosted") is False and value.get("autoProvision") is False,
                    "Only an isolated self-hosted pool is allowed")
        if kind == "queue":
            require(value["pool"]["id"] == receipt["pool"], "Queue pool changed")
        return
    require(value["id"].lower() == receipt["resources"][kind].lower(),
            "ARM resource ID changed")
    if kind == "subnet":
        properties = value["properties"]
        require(properties.get("addressPrefix") == "10.227.0.0/24" and
                properties.get("networkSecurityGroup", {}).get("id", "").lower() ==
                receipt["resources"]["nsg"].lower(), "Subnet ownership changed")
    else:
        require(value.get("tags", {}).get("privateRunnerRun") == receipt["run"] and
                value.get("tags", {}).get("privateRunnerDeadline") == receipt["deadline"],
                "ARM ownership tags changed")
        if kind == "vm":
            properties = value["properties"]
            disk = properties["storageProfile"]["osDisk"]
            interfaces = properties["networkProfile"]["networkInterfaces"]
            require(disk.get("deleteOption") == "Detach" and
                    disk["managedDisk"]["id"].lower() == receipt["resources"]["disk"].lower()
                    and not properties["storageProfile"].get("dataDisks") and
                    len(interfaces) == 1 and
                    interfaces[0]["id"].lower() == receipt["resources"]["nic"].lower() and
                    interfaces[0].get("properties", {}).get("deleteOption") == "Detach",
                    "VM deletion must not cascade to changed or unowned attachments")


def restricted(permissions, authorized):
    require(isinstance(permissions, dict) and
            permissions.get("allPipelines", {}).get("authorized") is False,
            "Open or unknown pipeline access refused")
    actual = {p["id"] for p in permissions.get("pipelines", []) if p["authorized"]}
    require(actual == ({979} if authorized else set()),
            "Queue must authorize only pipeline 979, or nobody after revocation")


def inventory(receipt, provider):
    url = (ARM + f"/subscriptions/{SUBSCRIPTION}/resourceGroups/"
           + receipt["resourceGroup"] + "/resources?api-version=2021-04-01")
    result = set()
    while url:
        page = provider.request("GET", url)
        require(page is not None, "Owned resource inventory unavailable")
        result.update(r["id"].lower() for r in page["value"]
                      if r.get("tags", {}).get("privateRunnerRun") == receipt["run"])
        url = page.get("nextLink")
        require(not url or url.startswith(ARM + f"/subscriptions/{SUBSCRIPTION}/resourceGroups/"
                                         + receipt["resourceGroup"] + "/resources?"),
                "Inventory pagination escaped resource group")
    return result


def finalize(receipt, provider, persist, now=None, sleep=time.sleep):
    """One external invocation; scheduler/store/retries must be commissioned separately."""
    validate(receipt)
    now = time.time() if now is None else now
    # Reserve 30 minutes for provider teardown; dispatch must be external.
    require(now >= max(timestamp(receipt["started"]), timestamp(receipt["deadline"]) - 1800),
            "External cleanup window not reached")
    urls = endpoints(receipt)
    order = ["agent", "revoke", "queue", "pool", "vm", "nic", "disk",
             "subnet", "nsg"] + (["nat", "ip"] if "nat" in receipt["resources"] else [])
    require(inventory(receipt, provider) <=
            {v.lower() for v in receipt["resources"].values()},
            "Unreceipted run-owned resources found; manual reconciliation required")
    # Check every remaining object's ownership before the first destructive action.
    for kind in order:
        if kind != "revoke":
            owned(receipt, kind, provider.request("GET", urls[kind]))
    pool = provider.request("GET", urls["pool"])
    if pool is not None:
        agents = provider.request("GET", urls["agents"])
        require(agents is not None and
                {a["id"] for a in agents["value"]} <= {receipt["agent"]},
                "Refusing pool deletion with unowned agents")
    if provider.request("GET", urls["queue"]) is not None:
        permissions = provider.request("GET", urls["permissions"])
        require(permissions is not None, "Queue permissions unreadable")
        # A previous invocation may have revoked access before its receipt write failed.
        actual = {p["id"] for p in permissions.get("pipelines", []) if p["authorized"]}
        restricted(permissions, bool(actual))
    for kind in order:
        if kind == "revoke":
            if provider.request("GET", urls["queue"]) is not None:
                owned(receipt, "queue", provider.request("GET", urls["queue"]))
                provider.request("PATCH", urls["permissions"], {
                    "allPipelines": {"authorized": False},
                    "pipelines": [{"id": 979, "authorized": False}],
                })
                restricted(provider.request("GET", urls["permissions"]), False)
        else:
            value = provider.request("GET", urls[kind])
            owned(receipt, kind, value)
            if value is not None:
                if kind == "pool":
                    require(provider.request("GET", urls["agents"])["value"] == [],
                            "Pool gained another agent")
                provider.request("DELETE", urls[kind], etag=value.get("etag"))
                for _ in range(60):
                    remaining = provider.request("GET", urls[kind])
                    if remaining is None:
                        break
                    owned(receipt, kind, remaining)
                    sleep(5)
                else:
                    raise GuardError("Deletion not verified; rerun external finalizer")
        if kind not in receipt["completed"]:
            receipt["completed"].append(kind)
        persist(receipt)
    require(all(provider.request("GET", urls[k]) is None
                for k in order if k != "revoke"), "Owned leftovers remain")
    require(not inventory(receipt, provider), "Run-tagged leftovers remain")
    return {"verifiedAbsent": [k for k in order if k != "revoke"],
            "authorization": "no live queue; any observed queue revoked before deletion"}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--receipt", type=Path, required=True)
    parser.add_argument("--apply", action="store_true")
    args = parser.parse_args()
    try:
        receipt = json.loads(args.receipt.read_text(encoding="utf-8"))
        validate(receipt)
        require(args.apply, "Cleanup requires explicit --apply")
        result = finalize(receipt, Native(), lambda r: save(args.receipt, r))
        print(json.dumps(result))
        return 0
    except (GuardError, KeyError, ValueError, TypeError, AttributeError,
            OSError, subprocess.SubprocessError) as error:
        # Provider bodies and exception details can contain credentials or bootstrap output.
        print(str(error) if isinstance(error, GuardError) else
              "Invalid receipt or local/provider failure; details suppressed.")
        return 2


if __name__ == "__main__":
    raise SystemExit(main())
