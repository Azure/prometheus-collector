#!/usr/bin/env python3
"""Provision owned hosts, test compatible settings sequentially, and retain resources."""

import argparse
import copy
from contextlib import contextmanager
from datetime import datetime
from functools import cache
import hashlib
import json
import math
import os
from pathlib import Path
import re
import secrets
import shutil
import subprocess
import sys
import tempfile
import time
import uuid
import xml.etree.ElementTree as ET

import jsonschema
import yaml


ROOT = Path(__file__).resolve().parent
REPO = ROOT.parents[2]
TEST_SOURCE = REPO
BUILD_SUBSCRIPTION = "9b96ebbd-c57a-42d1-bbe9-b69296e4c7fb"
SUITES = ("containerstatus", "prometheusui", "operator", "querymetrics", "livenessprobe")
CAMPAIGN_LOCATIONS = {
    "default": "centralus", "classic-azure-cni": "centralus",
    "autoscaler": "eastus2", "cilium": "centralus", "kubenet": "centralus",
    "acns": "eastus2",
}
RESET = ("ama-metrics-settings-configmap", "ama-metrics-prometheus-config",
         "ama-metrics-prometheus-config-node", "ama-metrics-prometheus-config-node-windows")
COMPONENTS = {"prometheus-collector": "collector", "targetallocator": "targetallocator",
              "config-reader": "cfg"}
DIGEST = re.compile(r"^[A-Za-z0-9][A-Za-z0-9._:/-]*@sha256:[0-9a-f]{64}$")


def load(path):
    return json.loads(Path(path).read_text(encoding="utf-8-sig"))


def save(path, value):
    Path(path).parent.mkdir(parents=True, exist_ok=True)
    Path(path).write_text(json.dumps(value, indent=2) + "\n", encoding="utf-8")


def resource_path(base, value, boundary):
    result = (Path(base) / value).resolve()
    if not result.is_relative_to(Path(boundary).resolve()) or not result.is_file():
        raise ValueError(f"Resource path must be an existing file inside {boundary}: {value}")
    return result


def compatible(scenario, variant):
    return (set(variant["requires"]["clusterFeatures"]) <= set(scenario["clusterFeatures"])
            and set(variant["platforms"]) <= set(scenario["platforms"])
            and (not variant.get("helmValues") or "helmChartInstall" in scenario["clusterFeatures"]))


def scenario_settings(scenario, variants):
    selected = scenario["settingsVariants"]
    if selected == ["*"]:
        return [name for name, variant in variants.items()
                if compatible(scenario, variant) and not variant.get("helmValues")
                and not variant.get("addonParams")]
    if "*" in selected:
        raise ValueError(f"{scenario['id']}: wildcard must be the only settings selector")
    return selected


def catalog(settings):
    document = load(settings / "catalog.json")
    lifecycle = document["lifecycle"]
    if (set(lifecycle["resetConfigMaps"]) != set(RESET)
            or lifecycle["namespace"] != "kube-system"
            or not lifecycle["restoreOriginalResources"]
            or lifecycle["labelCombination"] != "OR"):
        raise ValueError("Unsupported settings reset/restore lifecycle")
    variants = {}
    schema = load(settings / "manifest.schema.json")
    for row in document["variants"]:
        manifest_path = resource_path(settings, row["manifest"], settings)
        manifest = load(manifest_path)
        jsonschema.Draft202012Validator(schema).validate(manifest)
        if manifest["id"] != row["id"] or row["id"] in variants:
            raise ValueError("Settings manifest IDs must be unique and match the catalog")
        manifest["_manifestSha256"] = hashlib.sha256(manifest_path.read_bytes()).hexdigest()
        manifest["_workflow"] = row.get("workflow")
        override = row.get("overrideWorkflowLabels", False)
        if type(override) is not bool:
            raise ValueError("overrideWorkflowLabels must be an explicit boolean")
        manifest["_overrideWorkflowLabels"] = override
        for field in ("configmaps", "customResources", "helmValues"):
            manifest[field] = [resource_path(manifest_path.parent, p, settings.parents[3])
                               for p in manifest.get(field, [])]
        variants[row["id"]] = manifest
    return variants


def validate_matrix(document, variants, check_paths=True):
    jsonschema.Draft202012Validator(load(ROOT / "matrix.schema.json")).validate(document)
    ids = set()
    for row in document["scenarios"]:
        if row["id"] in ids:
            raise ValueError(f"Duplicate scenario: {row['id']}")
        ids.add(row["id"])
        features = set(row["clusterFeatures"])
        if "windows" in row["platforms"] and features & {"cilium", "http-proxy", "kubenet"}:
            raise ValueError(f"{row['id']}: unsupported Windows network combination")
        if ("windows" in row["platforms"]) != ("windows" in features):
            raise ValueError(f"{row['id']}: Windows platform/feature mismatch")
        if check_paths and not row.get("blockedReason"):
            resource_path(ROOT, row["cluster"], ROOT)
        label_filter(row)
        for name in scenario_settings(row, variants):
            if name not in variants or not compatible(row, variants[name]):
                raise ValueError(f"{row['id']}: incompatible or absent settings variant {name}")
            if not variants[name].get("helmValues"):
                label_filter(row, variants[name])
    return document


def select_version(document):
    rows = [r for r in document.get("values", document.get("orchestrators", []))
            if not r.get("isPreview") and
            ("KubernetesOfficial" in r.get("capabilities", {}).get("supportPlan", ["KubernetesOfficial"]))]
    defaults = []
    for row in rows:
        if not (row.get("isDefault") or row.get("default")):
            continue
        versions = row.get("patchVersions", {})
        if not versions and row.get("orchestratorVersion"):
            versions = {row["orchestratorVersion"]: {}}
        for version in versions:
            if re.fullmatch(r"\d+\.\d+\.\d+", version):
                key = tuple(map(int, version.split(".")))
                defaults.append(key)
    if not defaults:
        raise ValueError("Region returned no stable official default Kubernetes version")
    return ".".join(map(str, max(defaults)))


def windows_image_compatible(host, image):
    # The approved 2022 collector also supports process isolation on 2025 hosts.
    return (".".join(host.split(".")[:3]), ".".join(image.split(".")[:3])) in {
        ("10.0.17763", "10.0.17763"), ("10.0.20348", "10.0.20348"),
        ("10.0.26100", "10.0.20348"),
    }


def candidate_receipt(document, source_sha, platforms, features=(), pools=()):
    if not re.fullmatch(r"[0-9a-f]{40}", source_sha) or document.get("source", {}).get("commit") != source_sha:
        raise ValueError("Candidate receipt must match the exact tested source SHA")
    images = document.get("images", [])
    identities = set()
    for image in images:
        if not DIGEST.fullmatch(image.get("ref", "")):
            raise ValueError("Every candidate image must be pinned by sha256 digest")
        if image.get("manifestRef") and not DIGEST.fullmatch(image["manifestRef"]):
            raise ValueError("Candidate manifestRef must be digest pinned")
        identity = (image.get("name"), image.get("os"), image.get("arch"), image.get("osVersion"))
        if identity in identities:
            raise ValueError(f"Duplicate candidate platform entry: {identity}")
        identities.add(identity)
        if image.get("os") == "windows" and not re.fullmatch(r"\d+\.\d+\.\d+\.\d+", image.get("osVersion", "")):
            raise ValueError("Windows candidates require an explicit four-part osVersion")
    required = {("collector", "linux"), ("targetallocator", "linux"), ("cfg", "linux")}
    if "windows" in platforms:
        required.add(("collector", "windows"))
    mapping = {}
    for component, system in required:
        matches = [i for i in images if i["name"] == component and i["os"] == system]
        if not matches or not any(i.get("arch") == "amd64" for i in matches):
            raise ValueError(f"Missing candidate {component}/{system}/amd64")
        if system == "linux" and "arm64" in features and not any(i.get("arch") == "arm64" for i in matches):
            raise ValueError(f"Missing candidate {component}/linux/arm64")
        refs = {i.get("manifestRef", i["ref"]) for i in matches}
        if len(refs) != 1:
            raise ValueError(f"{component}/{system} needs a common pinned multi-platform manifestRef")
        mapping[(component, system)] = refs.pop()
    families = {"Windows2019": "10.0.17763", "Windows2022": "10.0.20348", "Windows2025": "10.0.26100"}
    for pool in pools:
        if pool["osType"] == "Windows" and not any(
                image["name"] == "collector" and image["os"] == "windows"
                and image["arch"] == pool["architecture"]
                and windows_image_compatible(families[pool["osSKU"]], image.get("osVersion", "")) for image in images):
            raise ValueError(f"Missing candidate Windows build family for pool {pool['name']}/{pool['osSKU']}")
    return images, mapping


def pin_workloads(documents, mapping):
    documents = [doc for doc in documents if not (
        doc and doc.get("kind") == "DaemonSet" and doc["metadata"]["name"] == "ama-metrics-win-node"
        and ("collector", "windows") not in mapping)]
    found = set()
    for doc in documents:
        if not doc or doc.get("kind") not in {"Deployment", "DaemonSet", "StatefulSet", "Job"}:
            continue
        spec = doc["spec"]["template"]["spec"]
        system = "windows" if doc["metadata"]["name"] == "ama-metrics-win-node" else "linux"
        for container in spec.get("containers", []) + spec.get("initContainers", []):
            component = COMPONENTS.get(container["name"])
            if component:
                key = (component, system)
                if key not in mapping:
                    raise ValueError(f"No candidate image for {key}")
                container["image"] = mapping[key]
                found.add(key)
    if found != set(mapping):
        raise ValueError(f"Rendered chart did not contain all candidate components: {set(mapping) - found}")
    return documents


def clean_resource(document):
    result = copy.deepcopy(document)
    result.pop("status", None)
    metadata = result["metadata"]
    for key in ("uid", "resourceVersion", "creationTimestamp", "generation", "managedFields"):
        metadata.pop(key, None)
    annotations = metadata.get("annotations", {})
    annotations.pop("kubectl.kubernetes.io/last-applied-configuration", None)
    if not annotations:
        metadata.pop("annotations", None)
    return result


def junit_summary(path):
    root = ET.parse(path).getroot()
    cases = list(root.iter("testcase"))
    executed = [c for c in cases if c.find("skipped") is None]
    failed = [c for c in executed if c.find("failure") is not None or c.find("error") is not None]
    result = {"executed": len(executed), "failed": len(failed), "skipped": len(cases) - len(executed)}
    if not executed or failed:
        raise RuntimeError(f"JUnit must contain executed, successful tests: {path}: {result}")
    return result


@contextmanager
def execution_lock(evidence):
    with (evidence / "execution.lock").open("a+b") as handle:
        if os.name == "nt":
            import msvcrt
            if handle.tell() == 0:
                handle.write(b"0")
                handle.flush()
            handle.seek(0)
            msvcrt.locking(handle.fileno(), msvcrt.LK_NBLCK, 1)
        else:
            import fcntl
            fcntl.flock(handle, fcntl.LOCK_EX | fcntl.LOCK_NB)
        try:
            yield
        finally:
            if os.name == "nt":
                handle.seek(0)
                msvcrt.locking(handle.fileno(), msvcrt.LK_UNLCK, 1)
            else:
                fcntl.flock(handle, fcntl.LOCK_UN)


class Commands:
    def __init__(self, evidence):
        self.evidence = evidence
        evidence.mkdir(parents=True, exist_ok=True)
        history = evidence / "commands.json"
        self.stages = load(history) if history.exists() else []
        self.deadline = None

    def run(self, args, *, cwd=None, env=None, data=None, json_output=False, private=False):
        executable = shutil.which(str(args[0]))
        if not executable:
            raise RuntimeError(f"Required executable missing: {args[0]}")
        start = time.monotonic()
        timeout = min(7200, self.deadline - start) if self.deadline is not None else 7200
        if timeout <= 0:
            raise RuntimeError("Execution time budget exhausted; only cleanup may continue")
        try:
            completed = subprocess.run([executable, *map(str, args[1:])], cwd=cwd or REPO,
                                       env=env, input=data, text=True, capture_output=True, timeout=timeout)
        except subprocess.TimeoutExpired as failure:
            raise RuntimeError(f"Command timed out: {args[0]} {args[1]}") from failure
        record = {"command": list(map(str, args)), "seconds": round(time.monotonic() - start, 3),
                  "exitCode": completed.returncode}
        self.stages.append(record)
        save(self.evidence / "commands.json", self.stages)
        if not private:
            with (self.evidence / "commands.log").open("a", encoding="utf-8") as log:
                log.write(json.dumps(record) + "\n" + completed.stdout + completed.stderr + "\n")
        if completed.returncode:
            detail = "(private command; output redacted)" if private else completed.stderr[-6000:]
            if private and list(args[:4]) == ["az", "deployment", "group", "create"]:
                diagnostic = completed.stderr
                for name, value in (env or os.environ).items():
                    if value and re.search(r"password|token|secret|key", name, re.IGNORECASE):
                        diagnostic = diagnostic.replace(value, "[REDACTED]")
                (self.evidence / "deployment-error.txt").write_text(diagnostic, encoding="utf-8")
                detail = diagnostic[-6000:]
            raise RuntimeError(f"Command failed ({completed.returncode}): {args[0]} {args[1]}: {detail}")
        return json.loads(completed.stdout) if json_output else completed.stdout


RBAC_ADMIN_ROLE = "f58310d9-a9f6-439a-9e8d-f62e7b41a168"
RBAC_ALLOWED_ROLES = [
    "4d97b98b-1d4f-4787-a291-c67834d212e7",
    "b0d8363b-8ddd-447d-831f-62ca05bff136",
]
RBAC_PROXY_READER = "acdd72a7-3385-48ef-bd42-f606fba81ae7"


def rbac_condition(allowed):
    clauses = []
    for action, source in (("write", "Request"), ("delete", "Resource")):
        clauses.append(
            f"((!(ActionMatches{{'Microsoft.Authorization/roleAssignments/{action}'}})) OR "
            f"(@{source}[Microsoft.Authorization/roleAssignments:RoleDefinitionId] "
            f"ForAnyOfAnyValues:GuidEquals {{{', '.join(allowed)}}} AND "
            f"@{source}[Microsoft.Authorization/roleAssignments:PrincipalType] "
            "ForAnyOfAnyValues:StringEqualsIgnoreCase {'ServicePrincipal'}))")
    return " AND ".join(clauses)


class Host:
    def __init__(self, args, scenario, commands):
        self.args, self.scenario, self.commands = args, scenario, commands
        self.kubeconfig = args.evidence / "kubeconfig"
        self.receipt_path = args.evidence / "ownership.json"
        self.receipt = None

    def az(self, *args, **kwargs):
        return self.commands.run(["az", *args, "--subscription", self.args.subscription,
                                  "--only-show-errors"], **kwargs)

    def kube(self, *args, **kwargs):
        return self.commands.run([getattr(self.args, "kubectl", "kubectl"),
                                  "--kubeconfig", str(self.kubeconfig), *args], **kwargs)

    def remove_kubeconfig(self):
        self.kubeconfig.unlink(missing_ok=True)

    def provision(self, env, principal, principal_type, expected_pools=None):
        if self.receipt_path.exists() or self.kubeconfig.exists():
            raise ValueError("An ownership receipt or kubeconfig already exists; cleanup that execution first")
        if getattr(self.args, "reuse_existing", False):
            return self.reuse_existing(env, expected_pools)
        if self.az("group", "list", "--tag", f"scenario-run={self.args.run_id}",
                   json_output=True):
            raise ValueError("A resource group already exists for this run; refusing duplicate provisioning")
        token = str(uuid.uuid4())
        name = "ama-e2e-" + hashlib.sha256(
            (self.args.run_id + self.scenario["id"] + token).encode()).hexdigest()[:14]
        if self.az("group", "exists", "--name", name).strip() != "false":
            raise ValueError("Refusing to take over an existing resource group")
        self.receipt = {"subscription": self.args.subscription, "resourceGroup": name,
                        "runId": self.args.run_id, "scenario": self.scenario["id"], "token": token}
        save(self.receipt_path, self.receipt)
        self.az("group", "create", "--name", name, "--location", self.args.location,
                "--tags", f"scenario-token={token}", f"scenario-run={self.args.run_id}",
                f"scenario={self.scenario['id']}", "owner=ama-metrics-e2e",
                f"expiresAt={env['SCENARIO_EXPIRES_AT']}")
        if principal_type == "ServicePrincipal":
            self.wait_for_rbac(principal)
        retry_deadline = time.monotonic() + 600
        retry_delay = 5
        while True:
            try:
                result = self.az("deployment", "group", "create", "--resource-group", name,
                                 "--name", "scenario", "--parameters", str(ROOT / self.scenario["cluster"]),
                                 env=env, json_output=True, private=True)
                break
            except RuntimeError as error:
                detail = str(error).casefold()
                if (principal_type != "ServicePrincipal"
                        or ("authorizationfailed" not in detail and "authorization failed" not in detail)
                        or "microsoft.authorization/roleassignments/write" not in detail):
                    raise
                remaining = retry_deadline - time.monotonic()
                if remaining <= 0:
                    raise
                delay = min(retry_delay, remaining)
                print(f"Retrying role-assignment authorization failure in {delay:g}s", flush=True)
                time.sleep(delay)
                if time.monotonic() >= retry_deadline:
                    raise
                retry_delay = min(retry_delay * 2, 30)
        if expected_pools is not None:
            self.verify_deployment(result, expected_pools, {"expiresAt": env["SCENARIO_EXPIRES_AT"]})
        outputs = {key: value["value"] for key, value in result["properties"]["outputs"].items()}
        managed_groups = [outputs["nodeResourceGroupName"], outputs["amwManagedResourceGroupName"]]
        if any(not isinstance(group, str) or not group for group in managed_groups):
            raise RuntimeError("Deployment did not identify both managed resource groups")
        self.receipt["managedGroups"] = sorted(set(managed_groups))
        save(self.receipt_path, self.receipt)
        save(self.args.evidence / "deployment.json", outputs)
        return outputs

    def reuse_existing(self, env, expected_pools):
        if not expected_pools:
            raise ValueError("Retry reuse requires the preflighted pool inventory")
        groups = self.az("group", "list", "--tag", f"scenario-run={self.args.run_id}",
                         json_output=True)
        if not isinstance(groups, list) or len(groups) != 1:
            raise ValueError("Retry reuse requires exactly one existing resource group for this run")
        group = groups[0]
        name, tags = group.get("name"), group.get("tags") or {}
        token = tags.get("scenario-token")
        try:
            expires = datetime.fromisoformat(tags["expiresAt"].replace("Z", "+00:00"))
        except (KeyError, ValueError):
            raise ValueError("Retry resource group has no valid expiry") from None
        if (not name or not token or expires.tzinfo is None or expires.timestamp() <= time.time()
                or group.get("location", "").casefold() != self.args.location.casefold()):
            raise ValueError("Retry resource group is expired or has an invalid identity")
        self.receipt = {"subscription": self.args.subscription, "resourceGroup": name,
                        "runId": self.args.run_id, "scenario": self.scenario["id"],
                        "token": token}
        owned = self.assert_owned()
        if owned.get("location", "").casefold() != self.args.location.casefold():
            raise ValueError("Retry resource group location changed")
        deployment = self.az("deployment", "group", "show", "--resource-group", name,
                             "--name", "scenario", json_output=True)
        self.verify_deployment(deployment, expected_pools, tags)
        outputs = {key: value["value"] for key, value in deployment["properties"]["outputs"].items()}
        cluster = self.az("aks", "show", "--resource-group", name, "--name", outputs["clusterName"],
                          json_output=True)
        if (cluster.get("id", "").casefold() != outputs["clusterId"].casefold()
                or cluster.get("location", "").casefold() != self.args.location.casefold()
                or cluster.get("provisioningState") != "Succeeded"
                or (cluster.get("powerState") or {}).get("code") != "Running"
                or cluster.get("kubernetesVersion") != env["SCENARIO_KUBERNETES_VERSION"]
                or cluster.get("nodeResourceGroup") != outputs["nodeResourceGroupName"]
                or (cluster.get("identity") or {}).get("type") != outputs["identityType"]):
            raise ValueError("Retry AKS cluster does not match the retained deployment contract")
        managed_groups = [outputs["nodeResourceGroupName"], outputs["amwManagedResourceGroupName"]]
        if any(not isinstance(value, str) or not value for value in managed_groups):
            raise ValueError("Retained deployment does not identify both managed resource groups")
        self.receipt["managedGroups"] = sorted(set(managed_groups))
        save(self.receipt_path, self.receipt)
        save(self.args.evidence / "deployment.json", outputs)
        save(self.args.evidence / "retry-reuse.json", {
            "status": "verified", "resourceGroup": name, "runId": self.args.run_id,
            "scenario": self.scenario["id"], "location": self.args.location,
        })
        return outputs

    def verify_deployment(self, deployment, expected_pools, tags=None):
        properties = deployment.get("properties", {})
        if properties.get("provisioningState") != "Succeeded":
            raise ValueError("Retained candidate deployment is not successful")
        outputs = {key: value["value"] for key, value in properties.get("outputs", {}).items()}
        resource_tags = outputs.get("resourceTags") or {}
        expiry = (tags or {}).get("expiresAt")
        if (not expiry or resource_tags.get("expiresAt") != expiry
                or resource_tags.get("owner") != "ama-metrics-e2e"
                or resource_tags.get("scenario") != self.scenario["id"]
                or resource_tags.get("run") != self.args.run_id
                or outputs.get("location") != self.args.location
                or (expected_pools is not None and outputs.get("expectedPools") != expected_pools)
                or outputs.get("addonEnabled") is not False
                or outputs.get("installationMode") != "candidate-addon"):
            raise ValueError("Retained deployment outputs do not match the candidate contract")

    def wait_for_rbac(self, principal):
        uuid.UUID(principal)
        scope = f"/subscriptions/{self.args.subscription}/resourceGroups/{self.receipt['resourceGroup']}"
        allowed = list(RBAC_ALLOWED_ROLES)
        if "custom-ca" in self.scenario["clusterFeatures"]:
            allowed.append(RBAC_PROXY_READER)
        condition = rbac_condition(allowed)
        command = (
            "$deadline = [DateTimeOffset]::UtcNow.AddMinutes(10)\n"
            "$delay = 5\n"
            "do {\n"
            f"  $output = & az role assignment create --subscription {self.args.subscription} "
            f"--assignee-object-id {principal} --assignee-principal-type ServicePrincipal "
            f"--role {RBAC_ADMIN_ROLE} --scope {scope} --condition-version 2.0 "
            f"--condition \"{condition}\" 2>&1\n"
            "  $exitCode = $LASTEXITCODE\n"
            "  if ($exitCode -eq 0) { $output; break }\n"
            "  $detail = $output | Out-String\n"
            "  if ($detail -notmatch '(?i)authorizationfailed' -or "
            "$detail -notmatch '(?i)microsoft.authorization/roleassignments/write' -or "
            "[DateTimeOffset]::UtcNow -ge $deadline) { throw $detail }\n"
            "  $remaining = [int][Math]::Floor(($deadline - [DateTimeOffset]::UtcNow).TotalSeconds)\n"
            "  if ($remaining -le 0) { throw $detail }\n"
            "  Start-Sleep -Seconds ([Math]::Min($delay, $remaining))\n"
            "  $delay = [Math]::Min($delay * 2, 30)\n"
            "} while ($true)")
        proof = {"scope": scope, "principalId": principal, "roleDefinitionId": RBAC_ADMIN_ROLE,
                 "allowedRoleDefinitionIds": allowed, "condition": condition, "status": "waiting"}
        self.receipt["rbacBootstrapScope"] = scope
        save(self.receipt_path, self.receipt)
        save(self.args.evidence / "rbac-bootstrap.json", proof)
        (self.args.evidence / "rbac-bootstrap.ps1").write_text(command + "\n", encoding="utf-8")
        print("Administrator action required within 20 minutes; run in an authorized Azure session:\n"
              + command, flush=True)
        deadline = time.monotonic() + 1200
        while True:
            self.assert_owned()
            assignments = self.az("role", "assignment", "list", "--resource-group",
                                  self.receipt["resourceGroup"], json_output=True)
            for assignment in assignments:
                if (assignment.get("scope", "").casefold() == scope.casefold()
                        and assignment.get("principalId", "").casefold() == principal.casefold()
                        and assignment.get("roleDefinitionId", "").split("/")[-1] == RBAC_ADMIN_ROLE
                        and assignment.get("conditionVersion") == "2.0"
                        and re.sub(r"\s+", "", assignment.get("condition") or "") == re.sub(r"\s+", "", condition)):
                    self.receipt["rbacDelegationId"] = assignment["id"]
                    save(self.receipt_path, self.receipt)
                    save(self.args.evidence / "rbac-bootstrap.json",
                         {**proof, "status": "granted", "assignmentId": assignment["id"]})
                    return
            if time.monotonic() >= deadline:
                raise RuntimeError("Scoped administrator RBAC bootstrap not confirmed within 20 minutes")
            time.sleep(30)

    def assert_owned(self):
        if self.receipt is None:
            self.receipt = load(self.receipt_path)
        if self.receipt["subscription"] != self.args.subscription:
            raise ValueError("Cleanup subscription does not match the ownership receipt")
        group = self.az("group", "show", "--name", self.receipt["resourceGroup"], json_output=True)
        expected = {"scenario-token": self.receipt["token"], "scenario-run": self.receipt["runId"],
                    "scenario": self.receipt["scenario"], "owner": "ama-metrics-e2e"}
        if any(group.get("tags", {}).get(k) != v for k, v in expected.items()):
            raise ValueError("Resource group ownership changed; refusing mutation/deletion")
        return group

    def cleanup(self):
        if self.receipt is None and not self.receipt_path.exists():
            return
        try:
            if self.receipt is None:
                self.receipt = load(self.receipt_path)
            group = self.receipt["resourceGroup"]
            if self.receipt["subscription"] != self.args.subscription:
                raise ValueError("Cleanup subscription mismatch")
            discovery_error = None
            exists = self.az("group", "exists", "--name", group).strip() != "false"
            if exists:
                self.assert_owned()
            if "managedGroups" not in self.receipt:
                try:
                    groups = self.az("group", "list", "--query", "[?managedBy != null].{name:name,managedBy:managedBy}",
                                     json_output=True, private=True)
                    parent = f"/subscriptions/{self.args.subscription}/resourceGroups/{group}/".casefold()
                    self.receipt["managedGroups"] = [g["name"] for g in groups
                                                    if g["managedBy"].casefold().startswith(parent)]
                    save(self.receipt_path, self.receipt)
                except (RuntimeError, ValueError, OSError, KeyError) as failure:
                    discovery_error = str(failure)
            if exists:
                self.az("group", "delete", "--name", group, "--yes")
                if self.az("group", "exists", "--name", group).strip() != "false":
                    raise RuntimeError(f"Resource group deletion not confirmed: {group}")
            deadline = time.monotonic() + 600
            for managed in self.receipt.get("managedGroups", []):
                while self.az("group", "exists", "--name", managed).strip() != "false":
                    if time.monotonic() >= deadline:
                        raise RuntimeError(f"Managed resource group deletion not confirmed: {managed}")
                    time.sleep(20)
            if discovery_error:
                save(self.args.evidence / "cleanup.json", {"deleted": False, "resourceGroup": group,
                                                         "managedGroupDiscoveryError": discovery_error})
                raise RuntimeError(f"Parent deleted but managed-group cleanup could not be verified: {discovery_error}")
            if self.receipt.get("rbacBootstrapScope"):
                assignments = self.az("role", "assignment", "list", "--all", json_output=True, private=True)
                if any(row["scope"].casefold() == self.receipt["rbacBootstrapScope"].casefold()
                       for row in assignments):
                    raise RuntimeError("Resource group deleted but bootstrap role assignment still exists")
            save(self.args.evidence / "cleanup.json", {
                "deleted": True, "resourceGroup": group, "managedGroupsDeleted": self.receipt.get("managedGroups", []),
                **({"rbacDelegationRemoved": True} if self.receipt.get("rbacBootstrapScope") else {}),
            })
        finally:
            self.remove_kubeconfig()

    def retain(self):
        if self.receipt is None and not self.receipt_path.exists():
            self.remove_kubeconfig()
            return {"retained": False, "reason": "No ownership receipt"}
        try:
            if self.receipt is None:
                self.receipt = load(self.receipt_path)
            group = self.receipt["resourceGroup"]
            if self.receipt["subscription"] != self.args.subscription:
                raise ValueError("Retention subscription mismatch")
            if self.az("group", "exists", "--name", group).strip() == "false":
                result = {"retained": False, "resourceGroup": group, "managedResourceGroups": []}
            else:
                parent = self.assert_owned()
                groups = self.az("group", "list", "--query",
                                 "[?managedBy != null].{name:name,managedBy:managedBy}",
                                 json_output=True, private=True)
                parent_id = f"/subscriptions/{self.args.subscription}/resourceGroups/{group}/".casefold()
                managed = {name for name in self.receipt.get("managedGroups", [])}
                managed.update(g["name"] for g in groups
                               if g["managedBy"].casefold().startswith(parent_id))
                managed = sorted(managed)
                resource_groups = []
                for name in [group, *managed]:
                    if name != group and self.az("group", "exists", "--name", name).strip() == "false":
                        continue
                    details = parent if name == group else self.az(
                        "group", "show", "--name", name, json_output=True, private=True)
                    resources = self.az(
                        "resource", "list", "--resource-group", name, "--query",
                        "[].{id:id,name:name,type:type,location:location,sku:sku,kind:kind}",
                        json_output=True, private=True)
                    resource_groups.append({
                        "id": details["id"], "name": name, "tags": details.get("tags", {}),
                        "resources": resources,
                    })
                inventoried = {row["name"] for row in resource_groups}
                missing = [name for name in managed if name not in inventoried]
                result = {
                    "retained": not missing, "subscription": self.args.subscription,
                    "resourceGroup": group, "runId": self.receipt.get("runId"),
                    "scenario": self.receipt.get("scenario"), "managedResourceGroups": managed,
                    "missingManagedResourceGroups": missing,
                    "inventory": resource_groups,
                    **({"rbacBootstrap": {
                        "scope": self.receipt["rbacBootstrapScope"],
                        "assignmentId": self.receipt.get("rbacDelegationId"),
                    }} if self.receipt.get("rbacBootstrapScope") else {}),
                }
            save(self.args.evidence / "retained-resources.json", result)
            return result
        finally:
            self.remove_kubeconfig()

    def revoke_bootstrap(self):
        run = load(self.args.evidence / "result.json")
        retained = load(self.args.evidence / "retained-resources.json")
        proof = load(self.args.evidence / "rbac-bootstrap.json")
        group = retained["resourceGroup"]
        scope = f"/subscriptions/{self.args.subscription}/resourceGroups/{group}"
        assignment_id = proof["assignmentId"]
        if not isinstance(assignment_id, str):
            raise ValueError("Bootstrap assignment ID is invalid")
        assignment_prefix = scope + "/providers/Microsoft.Authorization/roleAssignments/"
        try:
            uuid.UUID(assignment_id.rsplit("/", 1)[-1])
        except ValueError as failure:
            raise ValueError("Bootstrap assignment ID is invalid") from failure
        if not assignment_id.casefold().startswith(assignment_prefix.casefold()):
            raise ValueError("Bootstrap assignment ID is outside the retained resource group")
        admin_role = RBAC_ADMIN_ROLE
        allowed = proof["allowedRoleDefinitionIds"]
        if allowed not in (RBAC_ALLOWED_ROLES, RBAC_ALLOWED_ROLES + [RBAC_PROXY_READER]):
            raise ValueError("Bootstrap receipt contains an unapproved role allowlist")
        condition = proof["condition"]
        if (run.get("status") != "PASS" or not retained.get("retained")
                or retained.get("subscription") != self.args.subscription
                or retained.get("runId") != run.get("runId")
                or retained.get("scenario") != run.get("scenario")
                or proof.get("status") != "granted" or proof.get("scope", "").casefold() != scope.casefold()
                or retained.get("rbacBootstrap", {}).get("scope", "").casefold() != scope.casefold()
                or retained.get("rbacBootstrap", {}).get("assignmentId", "").casefold() != assignment_id.casefold()
                or proof.get("roleDefinitionId", "").casefold() != admin_role
                or re.sub(r"\s+", "", condition) != re.sub(r"\s+", "", rbac_condition(allowed))):
            raise ValueError("Bootstrap revocation requires the exact successful retained-run receipt")
        parent = next((row for row in retained["inventory"] if row.get("name") == group), None)
        if not parent or not parent.get("tags", {}).get("scenario-token"):
            raise ValueError("Retained resource-group ownership tags are missing")
        self.receipt = {
            "subscription": self.args.subscription, "resourceGroup": group,
            "runId": run["runId"], "scenario": run["scenario"],
            "token": parent["tags"]["scenario-token"],
        }
        self.assert_owned()
        if self.az("account", "show", "--query", "user.type").strip().casefold() != "user":
            raise ValueError("Bootstrap revocation requires the authorized user Azure identity")
        assignments = self.az("role", "assignment", "list", "--resource-group", group,
                              json_output=True, private=True)
        matches = [row for row in assignments
                   if row.get("id", "").casefold() == assignment_id.casefold()]
        if len(matches) > 1:
            raise RuntimeError("Bootstrap assignment ID is not unique at the retained resource group")
        if matches:
            assignment = matches[0]
            if (assignment.get("scope", "").casefold() != scope.casefold()
                    or assignment.get("principalId", "").casefold() != proof["principalId"].casefold()
                    or assignment.get("roleDefinitionId", "").rsplit("/", 1)[-1].casefold() != admin_role
                    or assignment.get("conditionVersion") != "2.0"
                    or re.sub(r"\s+", "", assignment.get("condition") or "") != re.sub(r"\s+", "", condition)):
                raise ValueError("Bootstrap assignment does not match the recorded approved grant")
        allowed_roles = {role.casefold() for role in allowed}
        narrower = {row["id"] for row in assignments
                    if row.get("roleDefinitionId", "").rsplit("/", 1)[-1].casefold() in allowed_roles}
        if matches:
            self.az("role", "assignment", "delete", "--ids", assignment_id)
        remaining = self.az("role", "assignment", "list", "--resource-group", group,
                            json_output=True, private=True)
        remaining_ids = {row["id"].casefold() for row in remaining}
        if assignment_id.casefold() in remaining_ids:
            raise RuntimeError("Bootstrap role assignment removal was not confirmed")
        if not {item.casefold() for item in narrower} <= remaining_ids:
            raise RuntimeError("A narrower workload role assignment changed during bootstrap revocation")
        save(self.args.evidence / "rbac-bootstrap-revocation.json", {
            "removed": True, "assignmentId": assignment_id, "scope": scope,
            "verifiedAbsent": True, "alreadyAbsent": not matches,
            "preservedRoleAssignmentIds": sorted(narrower),
        })


@cache
def ginkgo_labels(source):
    root = source / "otelcollector/test/ginkgo-e2e"
    text = (root / "utils/constants.go").read_text()
    known = set(re.findall(r'^\s+\w+\s+=\s+"([^"]+)"', text, re.MULTILINE))
    for path in root.rglob("*_test.go"):
        for arguments in re.findall(r"\bLabel\(([^)]*)\)", path.read_text()):
            known.update(re.findall(r'"([^"]+)"', arguments))
    return known


def label_filter(scenario, variant=None):
    known = ginkgo_labels(TEST_SOURCE)
    allowed = set(scenario["ginkgoLabels"])
    if variant:
        labels = variant["ginkgoLabels"]
        if not labels:
            raise ValueError("A settings slot requires explicit Ginkgo labels; empty must not run all tests")
        if set(labels) - known:
            raise ValueError(f"Unknown settings Ginkgo labels: {set(labels) - known}")
        return " || ".join(labels)
    if allowed - known:
        raise ValueError(f"Unknown cluster Ginkgo labels: {allowed - known}")
    denied = sorted(known - allowed)
    return "!(" + " || ".join(denied) + ")" if denied else ""


def settings_workflow(variant):
    template = variant.get("_workflow")
    if not template or not re.fullmatch(r"[a-z0-9][a-z0-9-]*\.yaml", template):
        raise ValueError(f"{variant['id']}: an existing ConfigTests workflow template is required")
    path = resource_path(TEST_SOURCE / "otelcollector/test/testkube/config-processing-test-crs",
                         template, TEST_SOURCE)
    names = [doc["metadata"]["name"] for doc in yaml.safe_load_all(path.read_text())
             if doc and doc.get("kind") == "TestWorkflow"]
    common = set(names) & {"configprocessingcommon", "configprocessingcommonv2", "configprocessingmipfalse"}
    if len(common) == 1:
        return template, common.pop()
    if len(names) == 1:
        return template, names[0]
    raise ValueError(f"{variant['id']}: ambiguous workflow template; an explicit case is required")


def workflow_result(destination, workflow, source_sha, image):
    result = load(destination / "testkube-results-ConfigTests.json")
    if (result.get("status") != "passed" or result.get("test_source_sha") != source_sha
            or result.get("runner_image") != image):
        raise RuntimeError("TestKube result did not explicitly pass with the verified source and runner digest")
    executions = result.get("executions", [])
    if len(executions) != 1:
        raise RuntimeError("Expected exactly the one submitted TestKube execution")
    row = executions[0]
    execution_id = row.get("execution_id", "")
    if (row.get("workflow") != workflow or row.get("status") != "passed"
            or not re.fullmatch(r"[A-Za-z0-9][A-Za-z0-9._-]{0,127}", execution_id)):
        raise RuntimeError("Wrong workflow, missing exact execution ID, or non-passed execution")
    if (any(type(row.get(key)) is not int for key in ("specs", "passed", "skipped", "failed"))
            or row["specs"] <= 0 or row["passed"] <= 0 or row["failed"] != 0
            or row["skipped"] < 0 or row["passed"] > row["specs"]):
        raise RuntimeError("TestKube requires nonempty executed cases and zero failed cases")
    expected = (destination / "testkube-executions/ConfigTests" / workflow / execution_id / "artifacts").resolve()
    artifact_dir = Path(row["artifact_dir"]).resolve()
    if artifact_dir != expected or not artifact_dir.is_relative_to(destination.resolve()) or not artifact_dir.is_dir():
        raise RuntimeError("Artifacts do not belong to this submitted execution")
    reports = {}
    for name in ("junit.xml", "ginkgo.json"):
        matches = list(artifact_dir.rglob(name))
        if len(matches) != 1:
            raise RuntimeError(f"Expected exactly one {name} for execution {execution_id}")
        reports[name] = resource_path(artifact_dir, matches[0], artifact_dir)
    junit = junit_summary(reports["junit.xml"])
    suites = load(reports["ginkgo.json"])
    if (not isinstance(suites, list) or len(suites) != 1 or suites[0].get("SuiteSucceeded") is not True
            or sum(spec.get("State") == "passed" and spec.get("LeafNodeType") == "It"
                   for spec in suites[0].get("SpecReports", [])) != row["passed"]
            or any(spec.get("State") not in {"passed", "skipped", "pending"}
                   for spec in suites[0].get("SpecReports", []))):
        raise RuntimeError("Ginkgo JSON did not contain a successful suite with executed cases")
    if junit["executed"] != row["passed"]:
        raise RuntimeError("JUnit executed cases disagree with the submitted execution result")
    return {"workflow": workflow, "executionId": execution_id, "status": "passed",
            **junit, "artifacts": str(artifact_dir)}


def run_suite(host, suite, destination, expression, template="testkube-test-crs.yaml", workflow=None,
              expected_cluster=None):
    workflow = workflow or suite
    destination.mkdir(parents=True, exist_ok=False)
    source_sha = host.runner_receipt["source"]["commit"]
    image = host.runner_receipt["imageRef"]
    env = dict(os.environ, CANDIDATE_TEST_SOURCE_SHA=source_sha, CANDIDATE_RUNNER_IMAGE=image,
               SELECTED_WORKFLOW=workflow, GINKGO_LABEL_FILTER=expression,
               BUILD_ARTIFACTSTAGINGDIRECTORY=str(destination), KUBECONFIG=str(host.kubeconfig),
               EXPECTED_CLUSTER=expected_cluster or host.cluster_name, EXPECT_OPERATOR_HTTPS="true",
               EXPECT_WINDOWS_EXPORTER=str("windows" in host.scenario["platforms"]).lower())
    paths = []
    if getattr(host.args, "tools_bin", None):
        paths.append(str(host.args.tools_bin))
    if Path(host.args.kubectl).parent != Path("."):
        paths.append(str(Path(host.args.kubectl).parent))
    env["PATH"] = os.pathsep.join([*paths, os.environ.get("PATH", "")])
    target = ".scenario-" + str(uuid.uuid4()) + ".yaml"
    generated = TEST_SOURCE / "otelcollector/test/testkube" / target
    if generated.exists():
        raise ValueError("Refusing to overwrite an existing generated workflow")
    try:
        host.commands.run(["bash", "./testkube/run-testkube-workflow.sh", host.query_endpoint,
                          host.query_client_id, template, target, "false", "0", "ConfigTests", source_sha],
                         cwd=TEST_SOURCE / "otelcollector/test", env=env)
        return workflow_result(destination, workflow, source_sha, image)
    finally:
        if generated.exists():
            generated.unlink()


def settings_slot(host, variant):
    if variant.get("helmValues"):
        raise ValueError("Customer Helm install adapter is not implemented; refusing addon substitution")
    if variant["addonParams"]:
        raise ValueError("Nonempty addonParams require an explicit addon-to-candidate chart mapping")
    template, workflow = settings_workflow(variant)
    expression = label_filter(host.scenario, variant)
    if ("windows" not in host.scenario["platforms"]
            and "config-processing-mip-false" in variant["ginkgoLabels"]):
        expression = f"({expression}) && !windows"
    save(host.args.evidence / variant["id"] / "selection.json", {
        "template": template, "workflow": workflow, "ginkgoLabelFilter": expression,
        "overrideWorkflowLabels": variant.get("_overrideWorkflowLabels", False),
    })
    sources = {path: path.read_text(encoding="utf-8-sig")
               for path in variant["configmaps"] + variant["customResources"]}
    documents = [doc for text in sources.values() for doc in yaml.safe_load_all(text) if doc]
    save(host.args.evidence / variant["id"] / "applied-resources.json", [
        {"source": str(path), "appliedTextSha256": hashlib.sha256(text.encode()).hexdigest()}
        for path, text in sources.items()
    ])
    identities = [("ConfigMap", name, "kube-system") for name in RESET]
    for doc in documents:
        identity = (doc["kind"], doc["metadata"]["name"],
                    doc["metadata"].get("namespace", "default"))
        if identity not in identities:
            identities.append(identity)
    originals = {}
    for kind, name, namespace in identities:
        output = host.kube("get", kind, name, "-n", namespace, "--ignore-not-found", "-o", "json",
                           private=True)
        if kind == "Namespace" and output.strip():
            raise ValueError(f"Fixture namespace already exists; refusing mutation: {name}")
        originals[(kind, name, namespace)] = clean_resource(json.loads(output)) if output.strip() else None
    error = None
    try:
        for name in RESET:
            host.kube("delete", "configmap", name, "-n", "kube-system", "--ignore-not-found")
        for text in sources.values():
            host.kube("apply", "-f", "-", data=text, private=True)
        time.sleep(host.args.settle_seconds)
        result = run_suite(host, "configprocessing",
                           host.args.evidence / variant["id"] / "configprocessing",
                           expression, template=template, workflow=workflow)
        if set(variant["ginkgoLabels"]) & {"config-processing-customer-settings-v1",
                                         "config-processing-customer-settings-v2"}:
            result["aliasIngestion"] = run_suite(
                host, "querymetrics", host.args.evidence / variant["id"] / "alias-ingestion",
                "cluster-identity", expected_cluster="Customer_Config_POC",
            )
        if variant["id"] == "collector-controls":
            result["ksmAllowlist"] = run_suite(
                host, "querymetrics", host.args.evidence / variant["id"] / "ksm-allowlist",
                "ksm-allowlist",
            )
        if variant["id"] == "minimal-ingestion-disabled":
            result["mipIngestion"] = run_suite(
                host, "querymetrics", host.args.evidence / variant["id"] / "mip-ingestion",
                "mip-disabled",
            )
        if variant["id"] == "windows":
            result["nodeConfigMapIngestion"] = run_suite(
                host, "querymetrics", host.args.evidence / variant["id"] / "node-configmap-ingestion",
                "config-processing-settings-node-config-map",
            )
        return result
    except (RuntimeError, ValueError) as failure:
        error = failure
        raise
    finally:
        try:
            for (kind, name, namespace), original in reversed(list(originals.items())):
                if original is None:
                    host.kube("delete", kind, name, "-n", namespace, "--ignore-not-found")
                else:
                    host.kube("replace", "--force", "-f", "-", data=json.dumps(original), private=True)
                readback = host.kube("get", kind, name, "-n", namespace, "--ignore-not-found",
                                     "-o", "json", private=True)
                actual = clean_resource(json.loads(readback)) if readback.strip() else None
                if actual != original:
                    raise RuntimeError(f"Baseline restoration mismatch: {kind}/{name}")
        except RuntimeError as restore_failure:
            raise RuntimeError(f"Restoration failed; no next variant allowed. Original: {error}; "
                               f"restore: {restore_failure}") from restore_failure


def render_chart(commands, evidence, outputs, source_sha, mapping):
    source = REPO / "otelcollector/deploy/addon-chart/azure-monitor-metrics-addon"
    rendered = evidence / "chart-source"
    shutil.copytree(source, rendered)
    chart_text = (source / "Chart-template.yaml").read_text()
    chart = yaml.safe_load(re.sub(r"\$\{(\w+)\}", lambda m: {
        "HELM_CHART_NAME": "ama-metrics", "HELM_SEMVER": "0.1.0",
        "IMAGE_TAG": "candidate-" + source_sha[:12],
        "MCR_REGISTRY": "mcr.microsoft.com", "MCR_REPOSITORY_HELM_DEPENDENCIES": "",
    }[m[1]], chart_text))
    chart.pop("dependencies", None)
    (rendered / "Chart.yaml").write_text(yaml.safe_dump(chart))
    values = (source / "values-template.yaml").read_text()
    replacements = {
        "IMAGE_TAG": "candidate-" + source_sha[:12],
        "MCR_REPOSITORY": "/azuremonitor/containerinsights/cidev/prometheus-collector/images",
        "ARC_EXTENSION": "false", "INCLUDE_DEPENDENT_CHARTS": "false",
        "AKS_REGION": outputs["location"], "AKS_RESOURCE_ID": outputs["clusterId"],
        "ARC_RESOURCE_ID": "", "ARC_REGION": "",
    }
    values = re.sub(r"\$\{(\w+)\}", lambda m: replacements[m[1]], values)
    values = yaml.safe_load(values)
    values["global"]["commonGlobals"]["Versions"]["Kubernetes"] = outputs["kubernetesVersion"]
    (rendered / "values.yaml").write_text(yaml.safe_dump(values, sort_keys=False))
    raw = commands.run(["helm", "template", "ama-metrics", str(rendered),
                        "--namespace", "kube-system"], private=True)
    documents = pin_workloads(list(yaml.safe_load_all(raw)), mapping)
    target = evidence / "candidate-chart"
    (target / "templates").mkdir(parents=True)
    (target / "files").mkdir()
    (target / "Chart.yaml").write_text(yaml.safe_dump(chart))
    (target / "files/manifests.yaml").write_text(yaml.safe_dump_all(documents))
    (target / "templates/resources.yaml").write_text('{{ .Files.Get "files/manifests.yaml" }}\n')
    return target


def audit_images(host, images):
    nodes = host.kube("get", "nodes", "-o", "json", json_output=True)["items"]
    node_info = {n["metadata"]["name"]: n["status"]["nodeInfo"] for n in nodes}
    pods = host.kube("get", "pods", "-n", "kube-system", "-o", "json", json_output=True)["items"]
    evidence = []
    found = set()
    for pod in pods:
        node = pod["spec"].get("nodeName")
        if node not in node_info:
            continue
        info = node_info[node]
        statuses = {c["name"]: c for c in pod.get("status", {}).get("containerStatuses", [])}
        for container in pod["spec"]["containers"]:
            component = COMPONENTS.get(container["name"])
            if component is None:
                continue
            candidates = [i for i in images if i["name"] == component
                          and i["os"] == info["operatingSystem"] and i["arch"] == info["architecture"]]
            if info["operatingSystem"] == "windows":
                candidates = [i for i in candidates if i.get("osVersion")
                              and windows_image_compatible(info["kernelVersion"], i["osVersion"])]
            status = statuses.get(container["name"], {})
            runtime = status.get("imageID", "")
            if not candidates or not status.get("ready") or not any(
                    runtime.endswith(i["ref"].split("@")[1])
                    and container["image"] in {i["ref"], i.get("manifestRef")}
                    for i in candidates):
                raise RuntimeError(f"Missing, unready or wrong candidate imageID: {pod['metadata']['name']}/{container['name']}")
            found.add((component, info["operatingSystem"], node))
            evidence.append({"pod": pod["metadata"]["name"], "node": node, "component": component,
                             "os": info["operatingSystem"], "arch": info["architecture"],
                             "kernel": info["kernelVersion"], "image": container["image"],
                             "imageID": runtime})
    for node, info in node_info.items():
        if ("collector", info["operatingSystem"], node) not in found:
            raise RuntimeError(f"No verified candidate collector on node {node}")
    for component in ("targetallocator", "cfg"):
        if not any(row["component"] == component for row in evidence):
            raise RuntimeError(f"No verified candidate {component}")
    if not node_info:
        raise RuntimeError("No nodes observed")
    return evidence


def verify_pool_inventory(expected, actual, nodes):
    observed = {pool["name"]: {key.casefold(): value for key, value in pool.items()} for pool in actual}
    if set(observed) != {pool["name"] for pool in expected}:
        raise RuntimeError("Deployed pool names do not match the compiled inventory")
    evidence = []
    for pool in expected:
        row = observed[pool["name"]]
        fields = {"ostype": pool["osType"], "ossku": pool["osSKU"], "vmsize": pool["vmSize"],
                  "mode": pool["mode"], "enablefips": pool["fips"], "enableautoscaling": pool["autoscaling"]}
        if any(row.get(key) != value for key, value in fields.items()):
            raise RuntimeError(f"Deployed pool properties disagree with compiled inventory: {pool['name']}")
        if str((row.get("upgradesettings") or {}).get("maxSurge")) != pool["maxSurge"]:
            raise RuntimeError(f"Deployed maxSurge is not the preflighted value: {pool['name']}")
        if (pool["autoscaling"] and (row.get("mincount"), row.get("maxcount")) != (pool["minCount"], pool["maxCount"])
                or not pool["autoscaling"] and row.get("count") != pool["count"]):
            raise RuntimeError(f"Deployed pool capacity disagrees with compiled inventory: {pool['name']}")
        members = [node for node in nodes
                   if node["metadata"]["labels"].get("kubernetes.azure.com/agentpool") == pool["name"]]
        minimum = pool["minCount"] if pool["autoscaling"] else pool["count"]
        maximum = (pool["maxCount"] if pool["autoscaling"] else pool["count"]) + int(pool["maxSurge"])
        if not minimum <= len(members) <= maximum or any(
                node["status"]["nodeInfo"]["architecture"] != pool["architecture"]
                or node["status"]["nodeInfo"]["operatingSystem"] != pool["osType"].lower() for node in members):
            raise RuntimeError(f"Missing nodes or incorrect node platform in pool: {pool['name']}")
        evidence.append({"expected": pool, "observed": row, "nodes": [
            {"name": node["metadata"]["name"], **node["status"]["nodeInfo"]} for node in members
        ]})
    return evidence


def verify_ingestion(host, outputs, started):
    token = host.commands.run(["az", "account", "get-access-token", "--resource",
                               "https://prometheus.monitor.azure.com", "--query", "accessToken",
                               "-o", "tsv"], private=True).strip()
    if not token:
        raise RuntimeError("Azure CLI returned no AMW query token")
    marker = host.receipt["token"]
    query = f'min(timestamp(myapp_temperature{{scenario_run="{marker}"}}))'
    script = 'IFS= read -r token; exec curl --fail-with-body --silent --show-error --max-time 60 -H "Authorization: Bearer $token" --get --data-urlencode "$1" "$2"'
    last_error = "No fresh run-specific sample"
    for attempt in range(30):
        try:
            response = host.kube("exec", "-i", "deployment/ama-metrics", "-n", "kube-system",
                                 "-c", "prometheus-collector", "--", "sh", "-c", script,
                                 "scenario-query", "query=" + query,
                                 outputs["queryEndpoint"].rstrip("/") + "/api/v1/query",
                                 data=token + "\n", private=True, json_output=True)
            results = response.get("data", {}).get("result", [])
            if (response.get("status") != "success"
                    or response.get("data", {}).get("resultType") != "vector" or len(results) != 1):
                raise RuntimeError("AMW did not return one successful run-specific result")
            observed = float(results[0]["value"][1])
            if not math.isfinite(observed) or not started <= observed <= time.time() + 60:
                raise RuntimeError("AMW sample is absent, stale or from the future")
            evidence = {"status": "fresh", "marker": marker, "sampleTimestamp": observed,
                        "probeStarted": started, "query": query, "attempts": attempt + 1,
                        "queryOrigin": "candidate Linux collector pod", "endpoint": outputs["queryEndpoint"]}
            save(host.args.evidence / "ingestion.json", evidence)
            return evidence
        except RuntimeError as failure:
            last_error = str(failure)
            if host.commands.deadline is not None and time.monotonic() + 10 >= host.commands.deadline:
                break
            if attempt < 29:
                time.sleep(10)
    raise RuntimeError(f"Fresh collector ingestion was not verified: {last_error}")


def compiled_parameters(args, scenario, commands, version):
    env = deployment_env(args, version, proxy="http-proxy" in scenario["clusterFeatures"])
    envelope = commands.run(["az", "bicep", "build-params", "--file", str(ROOT / scenario["cluster"]),
                             "--stdout"], env=env, json_output=True, private=True)
    template = (json.loads(envelope["templateJson"]) if envelope.get("templateJson") else
                commands.run(["az", "bicep", "build", "--file", str(ROOT / "clusters/aks.bicep"),
                              "--stdout"], env=env, json_output=True, private=True))
    values = {name: definition["defaultValue"] for name, definition in template["parameters"].items()
              if "defaultValue" in definition}
    values.update({name: definition["value"]
                   for name, definition in json.loads(envelope["parametersJson"])["parameters"].items()})
    return values


def capacity_plan(parameters, scenario, version):
    pools = parameters["nodePools"]
    if not pools or {pool["osType"].lower() for pool in pools} != set(scenario["platforms"]):
        raise ValueError("Compiled pool platforms do not match the scenario")
    for feature, observed in (("arm64", any(p["architecture"] == "arm64" for p in pools)),
                              ("fips", any(p["fips"] for p in pools))):
        if (feature in scenario["clusterFeatures"]) != observed:
            raise ValueError(f"Compiled pools disagree with scenario feature: {feature}")
    capacity = []
    for pool in pools:
        sku = pool["osSKU"]
        minor = tuple(map(int, version.split(".")[:2]))
        if ((sku == "Windows2022" and minor >= (1, 37))
                or (sku == "Windows2019" and minor >= (1, 33))):
            raise ValueError(f"{sku} unsupported with selected Kubernetes {version}; no downgrade")
        count = pool["maxCount"] if pool["autoscaling"] else pool["count"]
        surge = str(pool.get("maxSurge", ""))
        if type(count) is not int or count < 1 or not re.fullmatch(r"[1-9][0-9]*", surge):
            raise ValueError("Compiled pools require positive counts and explicit integer maxSurge")
        capacity.append({"name": pool["name"], "vmSize": pool["vmSize"],
                         "architecture": pool["architecture"], "nodes": count, "surge": int(surge)})
    if parameters["proxyEnabled"]:
        count = parameters["proxyVmCount"]
        if type(count) is not int or count < 1:
            raise ValueError("Proxy capacity must be explicit in compiled parameters")
        capacity.append({"name": "proxy", "vmSize": parameters["proxyVmSize"], "nodes": count, "surge": 0})
    return capacity


def verify_capacity(capacity, skus, usage, location):
    by_name = {sku["name"]: sku for sku in skus}
    required = {"cores": 0}
    observed = []
    for entry in capacity:
        sku = by_name.get(entry["vmSize"])
        if not sku or location.casefold() not in {v.casefold() for v in sku["locations"]}:
            raise ValueError(f"SKU unavailable in {location}: {entry['vmSize']}")
        for restriction in sku.get("restrictions", []):
            locations = restriction.get("values", []) + restriction.get("restrictionInfo", {}).get("locations", [])
            if restriction["type"] == "Location" and (not locations or location.casefold() in
                                                     {v.casefold() for v in locations}):
                raise ValueError(f"SKU subscription restriction: {entry['vmSize']}/{location}")
        capabilities = {c["name"]: c["value"] for c in sku["capabilities"]}
        architecture = {"x64": "amd64", "arm64": "arm64"}[capabilities["CpuArchitectureType"].casefold()]
        if entry.get("architecture") and entry["architecture"] != architecture:
            raise ValueError(f"SKU architecture mismatch: {entry['name']}/{entry['vmSize']}")
        cores = int(capabilities["vCPUs"]) * (entry["nodes"] + entry["surge"])
        family = sku["family"].casefold()
        required[family] = required.get(family, 0) + cores
        required["cores"] += cores
        observed.append({**entry, "architecture": architecture, "family": sku["family"], "peakVcpus": cores})
    limits = {row["name"]["value"].casefold(): row for row in usage}
    quota = []
    for name, cores in required.items():
        row = limits.get(name)
        if not row:
            raise ValueError(f"Missing regional/family quota: {name}")
        current, limit = int(str(row["currentValue"])), int(str(row["limit"]))
        if current < 0 or limit < 0 or current + cores > limit:
            raise ValueError(f"Insufficient regional/family quota: {name}, requested additional {cores} vCPUs")
        quota.append({"name": name, "current": current, "limit": limit, "additional": cores})
    return {"compute": observed, "quota": quota, "capacityReserved": False}


def regional_capacity(args, capacity, commands):
    names = sorted({entry["vmSize"] for entry in capacity})
    if any(not re.fullmatch(r"[A-Za-z0-9_]+", name) for name in names):
        raise ValueError("Unsupported VM SKU name in compiled capacity plan")
    query = "[?contains([" + ",".join("'" + name + "'" for name in names) + "], name)]"
    skus = commands.run(["az", "vm", "list-skus", "--subscription", args.subscription, "--location",
                         args.location, "--resource-type", "virtualMachines", "--all", "--query", query,
                         "--only-show-errors", "-o", "json"], json_output=True, private=True)
    usage = commands.run(["az", "vm", "list-usage", "--subscription", args.subscription,
                          "--location", args.location, "--only-show-errors", "-o", "json"],
                         json_output=True, private=True)
    save(args.evidence / "capacity-inputs.json", {"skus": skus, "usage": usage})
    return verify_capacity(capacity, skus, usage, args.location)


def preflight(args, scenario, commands):
    if args.subscription != BUILD_SUBSCRIPTION:
        raise ValueError("This POC is restricted to the approved build subscription")
    versions = commands.run(["az", "aks", "get-versions", "--subscription", args.subscription,
                             "--location", args.location, "--only-show-errors", "-o", "json"],
                            json_output=True)
    version = select_version(versions)
    if "secrets-watch-enforcement" in scenario["clusterFeatures"] and tuple(map(int, version.split(".")[:2])) < (1, 37):
        raise ValueError("Unsupported scenario: secret watch enforcement requires Kubernetes>=1.37; no version override")
    parameters = compiled_parameters(args, scenario, commands, version)
    capacity = capacity_plan(parameters, scenario, version)
    result = {"scenario": scenario["id"], "subscription": args.subscription, "location": args.location,
              "kubernetesVersion": version, "upgradeChannel": "stable", "expectedPools": parameters["nodePools"],
              **regional_capacity(args, capacity, commands)}
    save(args.evidence / "preflight.json", result)
    return result


def campaign_preflight(args, matrix, commands):
    output = args.evidence / "campaign-preflight.json"
    if output.exists():
        raise ValueError("Campaign admission requires new evidence; do not reuse a capacity snapshot")
    if args.subscription != BUILD_SUBSCRIPTION or type(args.build_id) is not int or args.build_id <= 0:
        raise ValueError("Campaign admission requires the approved subscription and positive producer --build-id")
    selected = list(CAMPAIGN_LOCATIONS) if args.scenario == "all" else [args.scenario]
    if any(name not in CAMPAIGN_LOCATIONS for name in selected):
        raise ValueError("Unsupported campaign scenario")
    source = source_commit(commands, REPO, ["otelcollector/test/scenarios"])
    placements = {name: CAMPAIGN_LOCATIONS[name] for name in selected}
    regions = {}
    for location in sorted(set(placements.values())):
        regional = copy.copy(args)
        regional.location = location
        regional.evidence = args.evidence / location
        regional.evidence.mkdir(parents=True, exist_ok=True)
        versions = commands.run(["az", "aks", "get-versions", "--subscription", args.subscription,
                                 "--location", location, "--only-show-errors", "-o", "json"],
                                json_output=True)
        version = select_version(versions)
        capacity = []
        for name in selected:
            if placements[name] != location:
                continue
            row = next((row for row in matrix["scenarios"] if row["id"] == name), None)
            if not row or row.get("blockedReason"):
                raise ValueError(f"Campaign scenario is missing or blocked: {name}")
            parameters = compiled_parameters(regional, row, commands, version)
            capacity.extend({**pool, "scenario": name}
                            for pool in capacity_plan(parameters, row, version))
        regions[location] = {"kubernetesVersion": version,
                             **regional_capacity(regional, capacity, commands)}
    result = {"status": "PASS", "sourceSha": source, "buildId": args.build_id,
              "subscription": args.subscription, "placements": placements,
              "regions": regions, "capacityReserved": False}
    save(output, result)
    return result


def deployment_env(args, version, proxy=False):
    env = dict(os.environ, SCENARIO_RUN_NAME=args.run_id, SCENARIO_LOCATION=args.location,
               SCENARIO_KUBERNETES_VERSION=version, SCENARIO_OWNER="ama-metrics-e2e",
               SCENARIO_EXPIRES_AT=time.strftime("%Y-%m-%dT%H:%M:%SZ",
                                                time.gmtime(time.time() + args.max_run_seconds + 1800)),
               SCENARIO_WINDOWS_PASSWORD=secrets.token_urlsafe(32) + "!aA1")
    if proxy:
        executable = shutil.which("ssh-keygen")
        if not executable:
            raise RuntimeError("ssh-keygen is required to mint an owned proxy VM public key")
        with tempfile.TemporaryDirectory(prefix="ama-proxy-key-") as directory:
            key = Path(directory) / "key"
            subprocess.run([executable, "-q", "-t", "rsa", "-b", "3072", "-N", "", "-C", "ama-metrics-e2e",
                            "-f", str(key)], capture_output=True, check=True, timeout=60)
            env["SCENARIO_PROXY_SSH_PUBLIC_KEY"] = key.with_suffix(".pub").read_text().strip()
    return env


def compile_profiles(args, commands):
    env = deployment_env(args, "1.36.5", proxy=True)
    checks = []
    for pattern, operation in (("*.bicep", "build"), ("*.bicepparam", "build-params")):
        for path in sorted((ROOT / "clusters").glob(pattern)):
            commands.run(["az", "bicep", operation, "--file", str(path), "--stdout"],
                          env=env, private=True)
            checks.append({"file": str(path.relative_to(ROOT)), "status": "compiled",
                           "sha256": hashlib.sha256(path.read_bytes()).hexdigest()})
    if not checks:
        raise ValueError("No Bicep profiles found")
    save(args.evidence / "compilation.json", {
        "checks": checks, "liveValidation": False,
        "inputs": "Compilation-only version; not deployment parameters or network proof",
    })


def prepare_testkube(args, commands):
    if not args.testkube_version or not re.fullmatch(r"\d+\.\d+\.\d+", args.testkube_version):
        raise ValueError("--testkube-version must be an explicit chart version before provisioning")
    destination = args.evidence / "testkube-chart"
    destination.mkdir()
    commands.run(["helm", "pull", "testkube", "--repo", "https://kubeshop.github.io/helm-charts",
                  "--version", args.testkube_version, "--destination", str(destination)])
    package = destination / f"testkube-{args.testkube_version}.tgz"
    metadata = yaml.safe_load(commands.run(["helm", "show", "chart", str(package)]))
    if metadata["version"] != args.testkube_version:
        raise ValueError("Downloaded TestKube chart version does not match the requested version")
    save(args.evidence / "testkube-chart.json", {
        "version": metadata["version"], "appVersion": metadata["appVersion"],
        "sha256": hashlib.sha256(package.read_bytes()).hexdigest(),
        "testExecution": "not submitted; chart download is not execution evidence",
    })
    account = commands.run(["az", "account", "show", "--subscription", args.subscription],
                           json_output=True, private=True)
    user = account["user"]
    if user["type"] not in {"user", "servicePrincipal"}:
        raise ValueError(f"Unsupported Azure CLI identity type: {user['type']}")
    lookup = ["az", "ad", "signed-in-user", "show"] if user["type"] == "user" else [
        "az", "ad", "sp", "show", "--id", user["name"]]
    principal = commands.run([*lookup, "--query", "id", "-o", "tsv"], private=True).strip()
    uuid.UUID(principal)
    return package, principal, "User" if user["type"] == "user" else "ServicePrincipal"


def bootstrap_testkube(host, outputs, package, principal, principal_type):
    group = host.receipt["resourceGroup"]
    identity = host.az("identity", "create", "--resource-group", group,
                       "--name", "ama-e2e-tests", json_output=True)
    # This subscription also has a broader custom role with the same display name.
    role = "b0d8363b-8ddd-447d-831f-62ca05bff136"
    # The fresh private-query probe forwards the caller's token into the collector pod.
    for object_id, kind in ((identity["principalId"], "ServicePrincipal"), (principal, principal_type)):
        host.az("role", "assignment", "create", "--assignee-object-id", object_id,
                "--assignee-principal-type", kind, "--role", role, "--scope", outputs["workspaceId"])
    cluster = host.az("aks", "show", "--resource-group", group, "--name", outputs["clusterName"],
                      json_output=True)
    if (cluster.get("autoUpgradeProfile") or {}).get("upgradeChannel") != "stable":
        raise RuntimeError("The effective AKS auto-upgrade channel must be stable")
    if "managed-acns" in host.scenario["clusterFeatures"]:
        advanced = ((cluster.get("networkProfile") or {}).get("advancedNetworking") or {})
        if (advanced.get("enabled") is not True
                or (advanced.get("observability") or {}).get("enabled") is not True):
            raise RuntimeError("AKS must report managed advanced networking and observability enabled")
        save(host.args.evidence / "managed-acns.json", {
            "advancedNetworkingEnabled": advanced["enabled"],
            "observabilityEnabled": advanced["observability"]["enabled"],
        })
    save(host.args.evidence / "effective-upgrade.json", {
        "kubernetesVersion": cluster["kubernetesVersion"],
        "autoUpgradeProfile": cluster["autoUpgradeProfile"],
    })
    issuer = (cluster.get("oidcIssuerProfile") or {}).get("issuerUrl")
    if not issuer or not (cluster.get("securityProfile", {}).get("workloadIdentity") or {}).get("enabled"):
        raise RuntimeError("The host must enable OIDC and workload identity")
    host.az("identity", "federated-credential", "create", "--resource-group", group,
            "--identity-name", identity["name"], "--name", "testkube",
            "--issuer", issuer, "--subject", "system:serviceaccount:testkube:testkube-api-server-tests-job",
            "--audience", "api://AzureADTokenExchange")
    host.commands.run(["helm", "upgrade", "--install", "testkube", str(package),
                      "--kubeconfig", str(host.kubeconfig), "--namespace", "testkube",
                      "--create-namespace", "--values", str(TEST_SOURCE / "otelcollector/test/testkube/values.yaml"),
                      "--wait", "--timeout", "20m"], private=True)
    host.kube("annotate", "serviceaccount", "testkube-api-server-tests-job", "-n", "testkube",
              f"azure.workload.identity/client-id={identity['clientId']}", "--overwrite")
    host.kube("apply", "-f", str(TEST_SOURCE / "otelcollector/test/testkube/api-server-permissions.yaml"))
    deployments = host.kube("get", "deployments", "-n", "testkube", "-o", "json", json_output=True)["items"]
    if not deployments:
        raise RuntimeError("TestKube has no deployments")
    for deployment in deployments:
        host.kube("rollout", "status", "deployment/" + deployment["metadata"]["name"],
                  "-n", "testkube", "--timeout=15m")
    workflows = host.kube("api-resources", "--api-group=testworkflows.testkube.io", "--no-headers")
    if not re.search(r"^testworkflows\s", workflows, re.MULTILINE):
        raise RuntimeError("TestKube workflow API was not installed")
    save(host.args.evidence / "testkube-bootstrap.json", {
        "status": "installed", "clientId": identity["clientId"], "issuer": issuer,
        "serviceAccount": "testkube-api-server-tests-job",
        "requiredWorkflowPodLabel": {"azure.workload.identity/use": "true"},
        "workloadIdentityExecution": "not submitted; bootstrap is not authentication proof",
    })
    host.query_client_id = identity["clientId"]
    host.query_endpoint = outputs["queryEndpoint"]
    host.cluster_name = outputs["clusterName"]
    return cluster


def source_commit(commands, root, paths):
    source = commands.run(["git", "-C", str(root), "rev-parse", "HEAD"]).strip()
    if not re.fullmatch(r"[0-9a-f]{40}", source):
        raise ValueError("A full committed source SHA is required")
    if commands.run(["git", "-C", str(root), "status", "--porcelain", "--", *paths]).strip():
        raise ValueError("Candidate source has uncommitted changes; provenance cannot be established")
    return source


def verified_candidate(args, commands):
    if type(args.build_id) is not int or args.build_id <= 0:
        raise ValueError("--build-id must identify a protected candidate build before provisioning")
    verifier = args.receipt_tools_root / ".pipelines/pr-e2e/fetch_candidate.py"
    if not verifier.is_file():
        raise ValueError("Authenticated protected runner-receipt verifier is missing from --receipt-tools-root")
    source = source_commit(commands, REPO, ["otelcollector/deploy/addon-chart"])
    test_source = source_commit(commands, TEST_SOURCE, [
        "otelcollector/test/ginkgo-e2e", "otelcollector/test/test-cluster-yamls",
        "otelcollector/test/testkube", "internal/referenceapp",
    ])
    destination = args.evidence / "verified-candidate"
    commands.run([sys.executable, str(verifier), "--build-id", str(args.build_id),
                  "--source-commit", source, "--output", str(destination)], private=True)
    candidate = load(destination / "candidate-build.json")
    runner_path = destination / "sre-testkube-runner/runner.json"
    runner = load(runner_path)
    expected = candidate["testRunner"]
    if (candidate["source"]["commit"] != source or runner["source"] != expected["testSource"]
            or runner["source"]["commit"] != test_source or runner["imageRef"] != expected["imageRef"]
            or hashlib.sha256(runner_path.read_bytes()).hexdigest() != expected["receiptSha256"]):
        raise ValueError("Verified candidate output does not match product/test checkouts and protected runner bytes")
    for image in candidate["images"]:
        if image["name"] == "config-reader":
            image["name"] = "cfg"
        image["manifestRef"] = image["indexRef"]
    return candidate, runner


def execute(args, scenario, variants, commands):
    candidate, runner_receipt = verified_candidate(args, commands)
    selected_settings = scenario_settings(scenario, variants)
    for name in selected_settings:
        variant = variants[name]
        if variant.get("helmValues") or variant["addonParams"]:
            raise ValueError(f"{name}: customer Helm/addon parameter adapter is required before provisioning")
        label_filter(scenario, variant)
    label_filter(scenario)
    source = candidate["source"]["commit"]
    resolved = preflight(args, scenario, commands)
    images, mapping = candidate_receipt(candidate, source, scenario["platforms"],
                                       scenario["clusterFeatures"], resolved["expectedPools"])
    testkube = prepare_testkube(args, commands)
    save(args.evidence / "settings-provenance.json", {
        name: {
            "manifestSha256": variants[name]["_manifestSha256"],
            "resources": [
                {"path": str(path), "sha256": hashlib.sha256(path.read_bytes()).hexdigest()}
                for field in ("configmaps", "customResources", "helmValues")
                for path in variants[name].get(field, [])
            ],
        } for name in selected_settings
    })
    env = deployment_env(args, resolved["kubernetesVersion"], proxy="http-proxy" in scenario["clusterFeatures"])
    host = Host(args, scenario, commands)
    host.runner_receipt = runner_receipt
    outcome = {"scenario": scenario["id"], "runId": args.run_id, "executionEngine": "testkube",
               "sourceSha": source, "status": "RUNNING", "settings": {}}
    commands.deadline = time.monotonic() + args.max_run_seconds
    try:
        outputs = host.provision(env, testkube[1], testkube[2], resolved["expectedPools"])
        host.assert_owned()
        group, cluster = host.receipt["resourceGroup"], outputs["clusterName"]
        host.az("aks", "get-credentials", "--resource-group", group, "--name", cluster,
                "--file", str(host.kubeconfig), "--admin", private=True)
        host.kube("wait", "nodes", "--all", "--for=condition=Ready", "--timeout=20m")
        state = bootstrap_testkube(host, outputs, *testkube)
        nodes = host.kube("get", "nodes", "-o", "json", json_output=True)["items"]
        save(args.evidence / "pool-inventory.json",
             verify_pool_inventory(resolved["expectedPools"], state["agentPoolProfiles"], nodes))
        host.az("aks", "update", "--resource-group", group, "--name", cluster,
                "--enable-azure-monitor-metrics", "--azure-monitor-workspace-resource-id", outputs["workspaceId"])
        host.kube("wait", "--for=create", "secret/aad-msi-auth-token",
                  "-n", "kube-system", "--timeout=15m", private=True)
        host.az("aks", "update", "--resource-group", group, "--name", cluster,
                "--disable-azure-monitor-metrics")
        state = host.az("aks", "show", "--resource-group", group, "--name", cluster, json_output=True)
        outputs["kubernetesVersion"] = state["kubernetesVersion"]
        if ((state.get("azureMonitorProfile") or {}).get("metrics") or {}).get("enabled"):
            raise RuntimeError("Managed addon is still enabled; refusing competing Helm ownership")
        deadline = time.monotonic() + 600
        managed = {"ama-metrics", "ama-metrics-node", "ama-metrics-win-node",
                   "ama-metrics-operator-targets", "ama-metrics-ksm"}
        while True:
            remaining = host.kube("get", "deployments,daemonsets", "-n", "kube-system",
                                  "-o", "json", json_output=True)["items"]
            if not any(resource["metadata"]["name"] in managed for resource in remaining):
                break
            if time.monotonic() >= deadline:
                raise RuntimeError("Managed addon workloads remain; refusing competing Helm ownership")
            time.sleep(10)
        chart = render_chart(commands, args.evidence, outputs, source, mapping)
        commands.run(["helm", "upgrade", "--install", "ama-metrics", str(chart),
                      "--kubeconfig", str(host.kubeconfig), "--namespace", "kube-system",
                      "--wait", "--timeout", "20m"])
        for resource in ("deployment/ama-metrics", "deployment/ama-metrics-operator-targets", "daemonset/ama-metrics-node"):
            host.kube("rollout", "status", resource, "-n", "kube-system", "--timeout=15m")
        if "windows" in scenario["platforms"]:
            host.kube("rollout", "status", "daemonset/ama-metrics-win-node", "-n", "kube-system", "--timeout=20m")
        save(args.evidence / "images-before.json", audit_images(host, images))
        for name in selected_settings:
            outcome["settings"][name] = settings_slot(host, variants[name])
        # Health suites depend on the repository's existing reference-app scrape fixtures.
        for name in ("ama-metrics-prometheus-config-configmap.yaml",
                     "ama-metrics-prometheus-config-node-configmap.yaml",
                     "ama-metrics-prometheus-config-node-windows-configmap.yaml",
                     "ama-metrics-settings-configmap.yaml"):
            host.kube("apply", "-f", str(TEST_SOURCE / "otelcollector/test/test-cluster-yamls/configmaps" / name))
        references = ["prometheus-reference-app.yaml"]
        if "windows" in scenario["platforms"]:
            references.append("win-prometheus-reference-app.yaml")
        for name in references:
            host.kube("apply", "-f", str(TEST_SOURCE / "internal/referenceapp" / name))
        monitors = list(yaml.safe_load_all(
            (TEST_SOURCE / "otelcollector/test/test-cluster-yamls/customresources/prometheus-reference-app.yaml").read_text()))
        probe_started = time.time()
        marked = 0
        for monitor in monitors:
            if not monitor or monitor.get("kind") not in {"PodMonitor", "ServiceMonitor"}:
                continue
            for field in ("endpoints", "podMetricsEndpoints"):
                for endpoint in monitor["spec"].get(field, []):
                    endpoint.setdefault("metricRelabelings", []).append({
                        "action": "replace", "targetLabel": "scenario_run", "replacement": host.receipt["token"],
                    })
                    marked += 1
        if not marked:
            raise RuntimeError("No reference-monitor endpoint was marked for fresh ingestion proof")
        host.kube("apply", "-f", "-", data=yaml.safe_dump_all(monitors), private=True)
        time.sleep(args.settle_seconds)
        outcome["suites"] = {}
        for suite in SUITES:
            outcome["suites"][suite] = run_suite(host, suite, args.evidence / "health" / suite,
                                                label_filter(scenario))
        outcome["ingestion"] = verify_ingestion(host, outputs, probe_started)
        save(args.evidence / "images-after.json", audit_images(host, images))
        outcome["status"] = "TESTS_PASSED"
    except (RuntimeError, ValueError, KeyError, OSError, ET.ParseError, yaml.YAMLError) as failure:
        outcome.update(status="FAIL", error=str(failure))
        raise
    finally:
        commands.deadline = None
        try:
            outcome["resources"] = host.retain()
            if outcome["status"] == "TESTS_PASSED":
                if not outcome["resources"].get("retained"):
                    raise RuntimeError("Successful tests but retained resource inventory is incomplete")
                outcome["status"] = "PASS"
        except (RuntimeError, ValueError, OSError, KeyError) as failure:
            outcome.update(status="FAIL", retentionError=str(failure))
            raise
        finally:
            save(args.evidence / "result.json", outcome)
    return outcome


def main(argv=None):
    global REPO, TEST_SOURCE
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("action", choices=("validate", "compile", "plan", "preflight", "campaign-preflight", "run",
                                           "cleanup", "revoke-bootstrap"))
    parser.add_argument("--matrix", type=Path, default=ROOT / "matrix.json")
    parser.add_argument("--settings-root", type=Path, default=ROOT / "settings")
    parser.add_argument("--source-root", type=Path, default=REPO,
                        help="Existing clean candidate source checkout; no checkout or branch changes are performed")
    parser.add_argument("--test-source-root", type=Path,
                        help="Existing clean test-source checkout matching the verified runner receipt")
    parser.add_argument("--scenario", default="default")
    parser.add_argument("--subscription", default=BUILD_SUBSCRIPTION)
    parser.add_argument("--location", default="eastus2")
    parser.add_argument("--run-id", default=str(uuid.uuid4()))
    parser.add_argument("--build-id", type=int, help="Successful pipeline-440 candidate build with protected receipts")
    parser.add_argument("--reuse-existing", action="store_true",
                        help="Reuse only the exact, unexpired resource group owned by this validation run")
    parser.add_argument("--receipt-tools-root", type=Path, default=ROOT.parents[2],
                        help="Trusted tools checkout providing .pipelines/pr-e2e/fetch_candidate.py")
    parser.add_argument("--testkube-version", help="Explicit TestKube Helm chart version; required for run")
    parser.add_argument("--kubectl", default="kubectl", help="kubectl executable name or absolute path")
    parser.add_argument("--tools-bin", type=Path, help="Directory containing scoped TestKube CLI/jq executables")
    parser.add_argument("--evidence", type=Path, required=True)
    parser.add_argument("--settle-seconds", type=int, default=360)
    parser.add_argument("--max-run-seconds", type=int, default=14400)
    args = parser.parse_args(argv)
    REPO = args.source_root.resolve()
    TEST_SOURCE = (args.test_source_root or REPO).resolve()
    args.evidence = args.evidence.resolve()
    uuid.UUID(args.run_id)
    if args.settle_seconds < 0 or args.max_run_seconds <= 0:
        raise ValueError("settle-seconds must be nonnegative and max-run-seconds must be positive")
    commands = Commands(args.evidence)
    if args.action == "run" and not args.build_id:
        raise ValueError("--build-id must identify a protected candidate build before provisioning")
    if args.action == "cleanup":
        with execution_lock(args.evidence):
            Host(args, {}, commands).cleanup()
        return 0
    if args.action == "revoke-bootstrap":
        with execution_lock(args.evidence):
            Host(args, {}, commands).revoke_bootstrap()
        return 0
    variants = catalog(args.settings_root.resolve())
    matrix = validate_matrix(load(args.matrix), variants)
    if args.action == "validate":
        save(args.evidence / "validation.json", {"scenarios": len(matrix["scenarios"]), "variants": list(variants)})
        return 0
    if args.action == "compile":
        compile_profiles(args, commands)
        return 0
    if args.action == "campaign-preflight":
        campaign_preflight(args, matrix, commands)
        return 0
    scenario = next((s for s in matrix["scenarios"] if s["id"] == args.scenario), None)
    if not scenario:
        raise ValueError(f"Unknown scenario: {args.scenario}")
    if scenario.get("blockedReason"):
        raise ValueError(f"BLOCKED: {scenario['blockedReason']}")
    if args.action == "plan":
        save(args.evidence / "plan.json", {"hostCount": 1, "scenario": scenario,
                                         "settingsSlots": scenario_settings(scenario, variants),
                                         "candidateRequired": True, "coverageGaps": matrix["coverageGaps"]})
    elif args.action == "preflight":
        preflight(args, scenario, commands)
    else:
        with execution_lock(args.evidence):
            execute(args, scenario, variants, commands)
    return 0


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except (ValueError, RuntimeError, KeyError, OSError, ET.ParseError, yaml.YAMLError,
            jsonschema.ValidationError) as error:
        print(f"ERROR: {error}", file=sys.stderr)
        raise SystemExit(1)
