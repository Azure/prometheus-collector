#!/usr/bin/env python3
"""Provision owned hosts, test compatible settings sequentially, and retain resources."""

import argparse
import base64
import copy
from contextlib import contextmanager, ExitStack
from datetime import datetime, timedelta, timezone
from functools import cache
import hashlib
import inspect
import ipaddress
import json
import math
import os
from pathlib import Path
import re
import secrets
import shutil
import shlex
import signal
import socket
import ssl
import subprocess
import sys
import tempfile
import time
import xml.etree.ElementTree as ET
from urllib.parse import urlencode, urlparse, urlsplit
from urllib.request import ProxyHandler, Request, build_opener, urlopen
import uuid

import jsonschema
import yaml


ROOT = Path(__file__).resolve().parent
REPO = ROOT.parents[2]
sys.path.insert(0, str(REPO / ".pipelines/pr-e2e/scan"))
from scan_trivy import resolve_tag

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
            and set(variant["platforms"]) <= set(scenario["platforms"]))


def scenario_settings(scenario, variants):
    selected = scenario["settingsVariants"]
    if selected == ["*"]:
        return [name for name, variant in variants.items()
                if compatible(scenario, variant) and not variant.get("addonParams")]
    if "*" in selected:
        raise ValueError(f"{scenario['id']}: wildcard must be the only settings selector")
    return selected


def catalog(settings):
    document = load(settings / "catalog.json")
    if document.get("pathBase") != "variant-directory":
        raise ValueError("Settings fixture paths must be relative to their variant folders")
    lifecycle = document["lifecycle"]
    reset_configmaps = lifecycle.get("resetConfigMaps")
    settings_configmap = lifecycle.get("settingsConfigMap")
    if (not isinstance(reset_configmaps, list) or not reset_configmaps
            or any(not isinstance(name, str) or not name for name in reset_configmaps)
            or len(set(reset_configmaps)) != len(reset_configmaps)
            or settings_configmap not in reset_configmaps
            or not isinstance(lifecycle.get("namespace"), str) or not lifecycle["namespace"]
            or not lifecycle["restoreOriginalResources"]
            or lifecycle["labelCombination"] != "OR"):
        raise ValueError("Invalid settings reset/restore lifecycle")
    variants = {}
    schema = load(settings / "variant.schema.json")
    for row in document["variants"]:
        jsonschema.Draft202012Validator(schema).validate(row)
        if row["id"] in variants:
            raise ValueError("Settings variant IDs must be unique")
        variant = dict(row)
        variant["_variantSha256"] = hashlib.sha256(
            json.dumps(row, sort_keys=True, separators=(",", ":")).encode()).hexdigest()
        for field in ("configmaps", "customResources"):
            variant[field] = [resource_path(settings / row["id"], p, settings.parents[3])
                              for p in variant.get(field, [])]
        variants[row["id"]] = variant
    return variants, lifecycle


def health_suites():
    template = REPO / "otelcollector/test/testkube/testkube-test-crs.yaml"
    suites = []
    for document in yaml.safe_load_all(template.read_text(encoding="utf-8-sig")):
        if not document or document.get("kind") != "TestWorkflow":
            continue
        name = document.get("metadata", {}).get("name", "")
        commands = [
            shlex.split(line.strip())
            for step in document.get("spec", {}).get("steps", [])
            for line in step.get("shell", "").splitlines()
            if line.strip().startswith("ginkgo run ")
        ]
        if not commands:
            continue
        if (not re.fullmatch(r"[a-z0-9][a-z0-9-]*", name) or len(commands) != 1
                or commands[0][-1] != f"./{name}"):
            raise ValueError(f"Invalid health TestWorkflow/suite definition: {name}")
        suites.append(name)
    if not suites or len(set(suites)) != len(suites):
        raise ValueError("Health TestWorkflow names must be nonempty and unique")
    return suites


def validate_build_images(config):
    containers = config["containerRoles"]
    sources = config["tagSources"]
    required = config["requiredByPlatform"]
    source_roles = {source["component"] for source in sources}
    requirements = (
        {role for roles in required.values() for role in roles}
        | {role for roles in config["requiredOnEveryNode"].values() for role in roles}
        | set(config["requiredInCluster"])
    )
    if (len({(source["component"], source["environment"]) for source in sources}) != len(sources)
            or requirements != source_roles or not set(containers.values()) <= requirements):
        raise ValueError("Build image roles, tag sources, and runtime requirements must agree")


def validate_matrix(document, variants, check_paths=True):
    jsonschema.Draft202012Validator(load(ROOT / "matrix.schema.json")).validate(document)
    validate_build_images(document["buildImages"])
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
            label_filter(row, variants[name])
    return document


def select_version(document, scenario=None):
    scoped_secrets = scenario is not None and "secrets-watch-enforcement" in scenario["clusterFeatures"]
    rows = [r for r in document.get("values", document.get("orchestrators", []))
            if scoped_secrets or (not r.get("isPreview")
            and "KubernetesOfficial" in r.get("capabilities", {}).get(
                "supportPlan", ["KubernetesOfficial"]))]
    eligible = []
    previews = []
    for row in rows:
        if not scoped_secrets and not (row.get("isDefault") or row.get("default")):
            continue
        versions = row.get("patchVersions", {})
        if not versions and row.get("orchestratorVersion"):
            versions = {row["orchestratorVersion"]: {}}
        for version in versions:
            if re.fullmatch(r"\d+\.\d+\.\d+", version):
                key = tuple(map(int, version.split(".")))
                if not scoped_secrets or key >= (1, 37, 0):
                    eligible.append(key)
                    if scoped_secrets and row.get("isPreview") is True:
                        previews.append(key)
    if not eligible:
        if scoped_secrets:
            raise ValueError("Unsupported scenario: secret watch enforcement requires an explicitly supported "
                             "regional Kubernetes >=1.37 version")
        raise ValueError("Region returned no stable official default Kubernetes version")
    return ".".join(map(str, max(previews or eligible)))


def version_key(version):
    if not re.fullmatch(r"\d+\.\d+\.\d+", version or ""):
        raise ValueError(f"Invalid Kubernetes version: {version}")
    return tuple(map(int, version.split(".")))


def log_resolved_version(scenario, location, version):
    if "secrets-watch-enforcement" in scenario["clusterFeatures"]:
        print(f"Resolved latest regional AKS version for {location}: Kubernetes {version}", flush=True)


def windows_image_compatible(host, image):
    # The approved 2022 collector also supports process isolation on 2025 hosts.
    return (".".join(host.split(".")[:3]), ".".join(image.split(".")[:3])) in {
        ("10.0.17763", "10.0.17763"), ("10.0.20348", "10.0.20348"),
        ("10.0.26100", "10.0.20348"),
    }


def image_mapping(images, platforms, features, pools, image_config):
    identities = set()
    for image in images:
        if not DIGEST.fullmatch(image.get("ref", "")):
            raise ValueError("Every build image must be pinned by sha256 digest")
        if image.get("manifestRef") and not DIGEST.fullmatch(image["manifestRef"]):
            raise ValueError("Build image manifestRef must be digest pinned")
        identity = (image.get("name"), image.get("os"), image.get("arch"), image.get("osVersion"))
        if identity in identities:
            raise ValueError(f"Duplicate build image platform entry: {identity}")
        identities.add(identity)
        if image.get("os") == "windows" and not re.fullmatch(r"\d+\.\d+\.\d+\.\d+", image.get("osVersion", "")):
            raise ValueError("Windows build images require an explicit four-part osVersion")
    required = {(component, system)
                for system in platforms
                for component in image_config["requiredByPlatform"].get(system, [])}
    mapping = {}
    for component, system in required:
        matches = [i for i in images if i["name"] == component and i["os"] == system]
        if not matches or not any(i.get("arch") == "amd64" for i in matches):
            raise ValueError(f"Missing build image {component}/{system}/amd64")
        if system == "linux" and "arm64" in features and not any(i.get("arch") == "arm64" for i in matches):
            raise ValueError(f"Missing build image {component}/linux/arm64")
        refs = {i.get("manifestRef", i["ref"]) for i in matches}
        if len(refs) != 1:
            raise ValueError(f"{component}/{system} needs a common pinned multi-platform manifestRef")
        mapping[(component, system)] = refs.pop()
    families = {"Windows2019": "10.0.17763", "Windows2022": "10.0.20348", "Windows2025": "10.0.26100"}
    for pool in pools:
        if pool["osType"] == "Windows":
            family = families[pool["osSKU"]]
            for component in image_config["requiredByPlatform"].get("windows", []):
                if not any(image["name"] == component and image["os"] == "windows"
                           and image["arch"] == pool["architecture"]
                           and windows_image_compatible(family, image.get("osVersion", ""))
                           for image in images):
                    raise ValueError(
                        f"Missing {component} Windows build image for pool {pool['name']}/{pool['osSKU']}")
    return images, mapping


def pin_workloads(documents, mapping, container_roles):
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
            component = container_roles.get(container["name"])
            if component:
                key = (component, system)
                if key not in mapping:
                    raise ValueError(f"No build image for {key}")
                container["image"] = mapping[key]
                found.add(key)
    if found != set(mapping):
        raise ValueError(f"Rendered chart did not contain all build components: {set(mapping) - found}")
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
            if not private:
                # Workflow CLI failures can be stdout-only; never echo arbitrary workflow output.
                markers = [message for message in (
                    "Internal Access Denied", "AccessDenied", "NoSuchBucket",
                    "InvalidAccessKeyId", "SignatureDoesNotMatch", "connection refused",
                    "i/o timeout", "TestWorkflow did not pass",
                ) if message.casefold() in completed.stdout.casefold()]
                detail += ("\nstdout diagnostics: " + (", ".join(markers) or "no recognized error markers")
                           + f"; full output retained in {self.evidence / 'commands.log'}")
            if private and list(args[:4]) == ["az", "deployment", "group", "create"]:
                diagnostic = completed.stderr
                for name, value in (env or os.environ).items():
                    if value and re.search(r"password|token|secret|key", name, re.IGNORECASE):
                        diagnostic = diagnostic.replace(value, "[REDACTED]")
                (self.evidence / "deployment-error.txt").write_text(diagnostic, encoding="utf-8")
                detail = diagnostic[-6000:]
            elif private and list(args[:4]) == ["az", "role", "assignment", "list"]:
                codes = re.findall(
                    r"\b(?:AuthorizationFailed|Authorization_RequestDenied|PrincipalNotFound|"
                    r"Request_ResourceNotFound|InvalidAuthenticationToken|ExpiredAuthenticationToken)\b",
                    completed.stderr)
                if codes:
                    detail = "(private command; error codes: " + ", ".join(sorted(set(codes))) + ")"
            raise RuntimeError(f"Command failed ({completed.returncode}): {args[0]} {args[1]}: {detail}")
        return json.loads(completed.stdout) if json_output else completed.stdout


RBAC_ADMIN_ROLE = "f58310d9-a9f6-439a-9e8d-f62e7b41a168"
RBAC_MONITORING_DATA_READER = "b0d8363b-8ddd-447d-831f-62ca05bff136"
RBAC_AUTH_RETRY_TIMEOUT = 600
RBAC_AUTH_RETRY_INTERVAL = 30
RBAC_AUTH_RETRY_MAX_INTERVAL = 300
RBAC_ALLOWED_ROLES = [
    "4d97b98b-1d4f-4787-a291-c67834d212e7",
    RBAC_MONITORING_DATA_READER,
]
RBAC_PROXY_READER = "acdd72a7-3385-48ef-bd42-f606fba81ae7"
SRE_PRINCIPAL = "e2bd6e9c-e75e-438b-8f57-2bb62327f346"


def persistent_identity(scenario):
    if not re.fullmatch(r"[a-z0-9]+(?:-[a-z0-9]+)*", scenario):
        raise ValueError("Invalid persistent scenario identity")
    owner = str(uuid.uuid5(uuid.NAMESPACE_URL, "ama-metrics-persistent:" + scenario))
    return "ama-e2e-persistent-" + scenario, owner


def rbac_condition(allowed, sre=False):
    clauses = []
    for action, source in (("write", "Request"), ("delete", "Resource")):
        roles = (f"@{source}[Microsoft.Authorization/roleAssignments:RoleDefinitionId] "
                 f"ForAnyOfAnyValues:GuidEquals {{{', '.join(allowed)}}}")
        if sre and action == "write" and RBAC_PROXY_READER not in allowed:
            roles = (f"({roles} OR (@{source}[Microsoft.Authorization/roleAssignments:RoleDefinitionId] "
                     f"ForAnyOfAnyValues:GuidEquals {{{RBAC_PROXY_READER}}} AND "
                     f"@{source}[Microsoft.Authorization/roleAssignments:PrincipalId] "
                     f"ForAnyOfAnyValues:GuidEquals {{{SRE_PRINCIPAL}}}))")
        clauses.append(
            f"((!(ActionMatches{{'Microsoft.Authorization/roleAssignments/{action}'}})) OR "
            f"({roles} AND "
            f"@{source}[Microsoft.Authorization/roleAssignments:PrincipalType] "
            "ForAnyOfAnyValues:StringEqualsIgnoreCase {'ServicePrincipal'}))")
    return " AND ".join(clauses)


def sibling_rbac_condition():
    return (
        "((!(ActionMatches{'Microsoft.Authorization/roleAssignments/write'})) OR "
        f"(@Request[Microsoft.Authorization/roleAssignments:RoleDefinitionId] ForAnyOfAnyValues:GuidEquals {{{RBAC_PROXY_READER}}} AND "
        f"@Request[Microsoft.Authorization/roleAssignments:PrincipalId] ForAnyOfAnyValues:GuidEquals {{{SRE_PRINCIPAL}}} AND "
        "@Request[Microsoft.Authorization/roleAssignments:PrincipalType] ForAnyOfAnyValues:StringEqualsIgnoreCase {'ServicePrincipal'})) AND "
        "(!(ActionMatches{'Microsoft.Authorization/roleAssignments/delete'}))"
    )


class Host:
    def __init__(self, args, scenario, commands):
        self.args, self.scenario, self.commands = args, scenario, commands
        self.kubeconfig = args.evidence / "kubeconfig"
        self.receipt_path = args.evidence / "ownership.json"
        self.receipt = None
        self.reused = False
        self.network_probe_name = "scenario-network-probe-" + hashlib.sha256(
            args.run_id.encode("utf-8")).hexdigest()[:16]
        self.created_network_probe_monitor = False
        self.current_kubernetes_version = None

    def az(self, *args, **kwargs):
        return self.commands.run(["az", *args, "--subscription", self.args.subscription,
                                  "--only-show-errors"], **kwargs)

    def az_with_rbac_retry(self, *args, **kwargs):
        operation = tuple(args[:3])
        if operation not in {("deployment", "group", "create"),
                             ("role", "assignment", "create")}:
            raise ValueError("RBAC retry is limited to role assignment creation and group deployments")
        role_assignment = None
        if operation == ("role", "assignment", "create"):
            options = dict(zip(args[3::2], args[4::2]))
            role = options.get("--role", "").casefold()
            role_assignment = {
                "principalId": options.get("--assignee-object-id"),
                "principalType": options.get("--assignee-principal-type"),
                "roleDefinitionId": role,
                "scope": options.get("--scope"),
            }
            group_scope = f"/subscriptions/{self.args.subscription}/resourceGroups/{self.receipt['resourceGroup']}" if self.receipt else ""
            reader_scopes = [group_scope] + [
                f"/subscriptions/{self.args.subscription}/resourceGroups/{name}"
                for name in (self.receipt or {}).get("managedGroups", [])
            ]
            if (not role_assignment["principalId"] or not role_assignment["scope"]
                    or role_assignment["principalType"] != "ServicePrincipal"
                    or (role != RBAC_MONITORING_DATA_READER and not (
                        role == RBAC_PROXY_READER and role_assignment["principalId"].casefold() == SRE_PRINCIPAL
                        and role_assignment["scope"].casefold() in {scope.casefold() for scope in reader_scopes}))
                    or (role_assignment["principalId"].casefold() == SRE_PRINCIPAL
                        and role == RBAC_MONITORING_DATA_READER and not re.fullmatch(
                            re.escape(group_scope + "/providers/Microsoft.Monitor/accounts/")
                            + r"[A-Za-z0-9][A-Za-z0-9._-]*", role_assignment["scope"], re.IGNORECASE))):
                raise ValueError("RBAC retry is limited to Monitoring Data Reader service-principal assignments and scoped SRE Reader")
        run_deadline = self.commands.deadline
        deadline = None
        last_failure = None
        interval = RBAC_AUTH_RETRY_INTERVAL
        try:
            while True:
                if last_failure is not None:
                    if time.monotonic() >= deadline:
                        raise last_failure
                    self.commands.deadline = deadline
                try:
                    return self.az(*args, **kwargs)
                except RuntimeError as failure:
                    last_failure = failure
                    message = str(failure).casefold()
                    collision = role_assignment and "roleassignmentexists" in message
                    if (not collision and ("microsoft.authorization/roleassignments/write" not in message
                            or not any(marker in message for marker in
                                       ("authorizationfailed", "authorization failed")))):
                        raise
                    if deadline is None:
                        deadline = time.monotonic() + RBAC_AUTH_RETRY_TIMEOUT
                        if run_deadline is not None:
                            deadline = min(deadline, run_deadline)
                    self.commands.deadline = deadline
                    if role_assignment:
                        existing = self.az("role", "assignment", "list", "--scope",
                                           role_assignment["scope"], json_output=True, private=True)
                        for assignment in existing:
                            assigned_role = assignment.get("roleDefinitionId", "").rstrip("/").rsplit("/", 1)[-1]
                            if (assignment.get("principalId", "").casefold()
                                    == role_assignment["principalId"].casefold()
                                    and assignment.get("principalType", "").casefold() == "serviceprincipal"
                                    and assigned_role.casefold() == role_assignment["roleDefinitionId"]
                                    and assignment.get("scope", "").casefold()
                                    == role_assignment["scope"].casefold()
                                    and (role_assignment["principalId"].casefold() != SRE_PRINCIPAL
                                         or assignment.get("roleDefinitionId", "").casefold() == (
                                             f"/subscriptions/{self.args.subscription}/providers/Microsoft.Authorization/roleDefinitions/{role_assignment['roleDefinitionId']}"
                                         ).casefold())
                                    and assignment.get("condition") is None
                                    and assignment.get("conditionVersion") is None):
                                return assignment
                    if collision:
                        raise
                    if "if access was recently granted, please refresh your credentials" in message:
                        self.az("account", "get-access-token", "--output", "none", private=True)
                    remaining = deadline - time.monotonic()
                    if remaining <= 0:
                        raise
                    delay = min(interval, remaining)
                    print(f"Retrying role-assignment authorization failure in {delay:g}s", flush=True)
                    time.sleep(delay)
                    interval = min(interval * 2, RBAC_AUTH_RETRY_MAX_INTERVAL)
        finally:
            self.commands.deadline = run_deadline

    def kube(self, *args, **kwargs):
        return self.commands.run([getattr(self.args, "kubectl", "kubectl"),
                                  "--kubeconfig", str(self.kubeconfig), *args], **kwargs)

    def remove_kubeconfig(self):
        self.kubeconfig.unlink(missing_ok=True)

    def sre_read_assignment(self, scope, role):
        assignments = self.az("role", "assignment", "list", "--assignee-object-id", SRE_PRINCIPAL,
                              "--fill-principal-name", "false", "--scope", scope,
                              json_output=True, private=True)
        if not isinstance(assignments, list) or not all(isinstance(row, dict) for row in assignments):
            raise ValueError("SRE role assignment listing is invalid")
        matches = [row for row in assignments if row.get("scope", "").casefold() == scope.casefold()
               and row.get("principalId", "").casefold() == SRE_PRINCIPAL
               and row.get("roleDefinitionId", "").casefold() == (
                   f"/subscriptions/{self.args.subscription}/providers/Microsoft.Authorization/roleDefinitions/{role}"
               ).casefold()]
        if len(matches) > 1:
            raise ValueError("Multiple exact SRE role assignments found at the same scope")
        if matches and (matches[0].get("principalType", "").casefold() != "serviceprincipal"
                        or matches[0].get("condition") is not None
                        or matches[0].get("conditionVersion") is not None):
            raise ValueError("Existing SRE role assignment is conditional or has an invalid principal type")
        return matches[0] if matches else None

    def grant_sre_read(self, workspace=None):
        self.assert_owned()
        group_scope = f"/subscriptions/{self.args.subscription}/resourceGroups/{self.receipt['resourceGroup']}"
        scope, role = group_scope, RBAC_PROXY_READER
        if workspace is not None:
            if not isinstance(workspace, str) or not re.fullmatch(
                    re.escape(group_scope + "/providers/Microsoft.Monitor/accounts/") + r"[A-Za-z0-9][A-Za-z0-9._-]*",
                    workspace, re.IGNORECASE):
                raise ValueError("SRE workspace scope must be an AMW in the owned resource group")
            scope, role = workspace, RBAC_MONITORING_DATA_READER
        assignment = self.sre_read_assignment(scope, role)
        if not assignment:
            self.az_with_rbac_retry("role", "assignment", "create", "--assignee-object-id", SRE_PRINCIPAL,
                                    "--assignee-principal-type", "ServicePrincipal", "--role", role, "--scope", scope)
            assignment = self.sre_read_assignment(scope, role)
            if not assignment:
                raise RuntimeError(f"SRE read grant not confirmed at {scope}")
        print(f"Verified permanent SRE read grant: principal={SRE_PRINCIPAL} role={role} scope={scope}", flush=True)

    def verify_sre_managed_groups(self, outputs, principal, principal_type, retained=False):
        self.assert_owned()
        group_scope = f"/subscriptions/{self.args.subscription}/resourceGroups/{self.receipt['resourceGroup']}"
        profile = Path(self.scenario["cluster"]).stem
        cluster_id, workspace_id = outputs["clusterId"], outputs["workspaceId"]
        for resource_id, kind in ((cluster_id, "Microsoft.ContainerService/managedClusters"),
                                  (workspace_id, "Microsoft.Monitor/accounts")):
            if not isinstance(resource_id, str) or not re.fullmatch(
                    re.escape(group_scope + "/providers/" + kind + "/")
                    + r"[A-Za-z0-9][A-Za-z0-9._-]*", resource_id, re.IGNORECASE):
                raise ValueError(f"Unowned {kind} resource")
        cluster = self.az("aks", "show", "--resource-group", self.receipt["resourceGroup"],
                          "--name", outputs["clusterName"], json_output=True, private=True)
        workspace = self.az("resource", "show", "--ids", workspace_id, "--api-version", "2023-04-03",
                            json_output=True, private=True)
        for resource, expected in ((cluster, cluster_id), (workspace, workspace_id)):
            tags = resource.get("tags") or {}
            if (resource.get("id", "").casefold() != expected.casefold()
                    or tags.get("owner") != "ama-metrics-e2e" or tags.get("scenario") != profile
                    or tags.get("run") != self.receipt["runId"]):
                raise ValueError("Managed-group parent resource ownership changed")
        if cluster_id.rsplit("/", 1)[-1].casefold() != outputs["clusterName"].casefold():
            raise ValueError("Managed-group AKS identity changed")
        dce = ((workspace.get("properties") or {}).get("defaultIngestionSettings") or {}).get(
            "dataCollectionEndpointResourceId", "")
        dce_match = re.fullmatch(
            re.escape(f"/subscriptions/{self.args.subscription}/resourceGroups/")
            + r"([A-Za-z0-9][A-Za-z0-9._()-]*)/providers/Microsoft.Insights/dataCollectionEndpoints/[^/]+",
            dce, re.IGNORECASE)
        node = cluster.get("nodeResourceGroup", "")
        if (not dce_match or not re.fullmatch(r"[A-Za-z0-9][A-Za-z0-9._()-]*", node)
                or node.casefold() != outputs["nodeResourceGroupName"].casefold()
                or dce_match[1].casefold() != outputs["amwManagedResourceGroupName"].casefold()
                or node.casefold() == dce_match[1].casefold()
                or self.receipt["resourceGroup"].casefold() in {node.casefold(), dce_match[1].casefold()}):
            raise ValueError("Managed-group provider association is missing or ambiguous")
        targets = ((node, cluster_id), (dce_match[1], workspace_id))
        self.receipt["managedGroups"] = sorted(name for name, _ in targets)
        deadline = time.monotonic() + (0 if retained else RBAC_AUTH_RETRY_TIMEOUT)
        if self.commands.deadline is not None:
            deadline = min(deadline, self.commands.deadline)
        while True:
            try:
                complete = True
                for name, resource_id in targets:
                    scope = f"/subscriptions/{self.args.subscription}/resourceGroups/{name}"
                    group = self.az("group", "show", "--name", name, json_output=True)
                    if (group.get("id", "").casefold() != scope.casefold()
                            or (group.get("managedBy") and group["managedBy"].casefold() != resource_id.casefold())):
                        raise ValueError("Managed-group ownership does not match its provider resource")
                    if principal_type == "ServicePrincipal":
                        assignments = self.az("role", "assignment", "list", "--scope", scope,
                                              "--assignee-object-id", principal, "--fill-principal-name", "false",
                                              json_output=True, private=True)
                        matches = [row for row in assignments
                                   if row.get("scope", "").casefold() == scope.casefold()
                                   and row.get("principalId", "").casefold() == principal.casefold()
                                   and row.get("roleDefinitionId", "").casefold() == (
                                       f"/subscriptions/{self.args.subscription}/providers/Microsoft.Authorization/roleDefinitions/{RBAC_ADMIN_ROLE}"
                                   ).casefold()]
                        if len(matches) > 1 or any(
                                row.get("principalType", "").casefold() != "serviceprincipal"
                                or row.get("conditionVersion") != "2.0"
                                or re.sub(r"\s+", "", row.get("condition") or "") !=
                                   re.sub(r"\s+", "", sibling_rbac_condition()) for row in matches):
                            raise ValueError("Incompatible sibling bootstrap delegation")
                        if not matches:
                            complete = False
                            continue
                    assignment = self.sre_read_assignment(scope, RBAC_PROXY_READER)
                    if not assignment:
                        self.az_with_rbac_retry(
                            "role", "assignment", "create", "--assignee-object-id", SRE_PRINCIPAL,
                            "--assignee-principal-type", "ServicePrincipal", "--role", RBAC_PROXY_READER,
                            "--scope", scope)
                        assignment = self.sre_read_assignment(scope, RBAC_PROXY_READER)
                        if not assignment:
                            raise RuntimeError(f"SRE managed-group Reader not confirmed at {scope}")
                    print(f"Verified permanent SRE managed-group Reader: principal={SRE_PRINCIPAL} role={RBAC_PROXY_READER} scope={scope}", flush=True)
                if complete:
                    return True
            except RuntimeError as failure:
                if not any(marker in str(failure).casefold() for marker in
                           ("authorizationfailed", "authorization failed", "authorization_requestdenied")):
                    raise
            if time.monotonic() >= deadline:
                if retained:
                    return False
                raise RuntimeError("Managed-group SRE Reader or exact sibling delegation not confirmed; authorized administrator bootstrap is required")
            time.sleep(min(10, max(0, deadline - time.monotonic())))

    def provision(self, env, principal, principal_type, expected_pools=None):
        if self.receipt_path.exists() or self.kubeconfig.exists():
            raise ValueError("An ownership receipt or kubeconfig already exists; cleanup that execution first")
        name, owner = persistent_identity(self.scenario["id"])
        existing = self.persistent_state(env, expected_pools)
        if existing:
            if principal_type == "ServicePrincipal":
                self.wait_for_rbac(principal, retained=True)
            self.grant_sre_read()
            self.grant_sre_read(existing["workspaceId"])
            if not self.verify_sre_managed_groups(existing, principal, principal_type, retained=True):
                if principal_type != "ServicePrincipal":
                    raise RuntimeError("Managed-group SRE bootstrap requires an authorized administrator")
                self.wait_for_rbac(principal)
                self.verify_sre_managed_groups(existing, principal, principal_type)
            self.reused = True
            save(self.receipt_path, self.receipt)
            save(self.args.evidence / "deployment.json", existing)
            save(self.args.evidence / "persistent-reuse.json", {
                "status": "verified", "resourceGroup": name, "runId": self.args.run_id,
                "scenario": self.scenario["id"], "location": self.args.location,
            })
            return existing
        if "secrets-watch-enforcement" in self.scenario["clusterFeatures"]:
            raise ValueError("The retained secret-watch cluster is missing; refusing to recreate it")
        if getattr(self.args, "admitted_existing", False):
            raise ValueError("An admitted persistent cluster disappeared; refusing new provisioning")
        if principal_type != "ServicePrincipal":
            raise RuntimeError("New provisioning requires a ServicePrincipal for the independent managed-group bootstrap")
        self.receipt = {"subscription": self.args.subscription, "resourceGroup": name,
                        "runId": owner, "scenario": self.scenario["id"], "token": owner}
        save(self.receipt_path, self.receipt)
        if self.az("group", "exists", "--name", name).strip() == "false":
            self.az("group", "create", "--name", name, "--location", self.args.location,
                    "--tags", f"scenario-token={owner}", f"scenario-run={owner}",
                    f"scenario={self.scenario['id']}", "owner=ama-metrics-e2e")
        else:
            self.assert_owned()
            if self.az("aks", "list", "--resource-group", name, json_output=True):
                raise ValueError("A cluster appeared after admission; refusing deployment")
            deployments = self.az("deployment", "group", "list", "--resource-group", name, json_output=True)
            if any(row.get("properties", {}).get("provisioningState") not in ("Succeeded", "Failed", "Canceled")
                   for row in deployments):
                raise ValueError("A deployment is still active; refusing concurrent provisioning")
            if "private-api" in self.scenario["clusterFeatures"]:
                # VM creation keys are immutable; tunnel authentication still uses a fresh per-run key.
                vms = self.az("vm", "list", "--resource-group", name, json_output=True)
                if vms:
                    keys = {key["keyData"] for vm in vms
                            for key in vm["osProfile"]["linuxConfiguration"]["ssh"]["publicKeys"]}
                    if len(keys) != 1:
                        raise ValueError("Retained private VMs must have one matching initial public key")
                    env = dict(env, SCENARIO_PROXY_SSH_PUBLIC_KEY=keys.pop())
        if principal_type == "ServicePrincipal":
            self.wait_for_rbac(principal)
        self.grant_sre_read()
        group_scope = f"/subscriptions/{self.args.subscription}/resourceGroups/{name}"
        accounts = self.az("resource", "list", "--resource-group", name,
                           "--resource-type", "Microsoft.Monitor/accounts", json_output=True, private=True)
        if not isinstance(accounts, list) or len(accounts) > 1 or any(not isinstance(row, dict) for row in accounts):
            raise ValueError("Invalid or multiple AMWs in the owned resource group; refusing ambiguous deployment")
        override = []
        if accounts:
            account = accounts[0]
            scope = account.get("id", "")
            tags = account.get("tags") or {}
            if (not re.fullmatch(re.escape(group_scope + "/providers/Microsoft.Monitor/accounts/")
                                 + r"[A-Za-z0-9][A-Za-z0-9._-]*", scope, re.IGNORECASE)
                    or tags.get("owner") != "ama-metrics-e2e"
                    or tags.get("scenario") != Path(self.scenario["cluster"]).stem or tags.get("run") != owner):
                raise ValueError("Existing AMW ownership is ambiguous; refusing deployment")
            assignment = self.sre_read_assignment(scope, RBAC_MONITORING_DATA_READER)
            if assignment:
                assignment_id = assignment.get("id", "")
                if not re.fullmatch(
                        re.escape(scope + "/providers/Microsoft.Authorization/roleAssignments/")
                        + r"[0-9a-f]{8}(?:-[0-9a-f]{4}){3}-[0-9a-f]{12}", assignment_id, re.IGNORECASE):
                    raise ValueError("Existing SRE AMW assignment ID is invalid or outside the owned AMW")
                override = ["--parameters", "sreWorkspaceAssignmentId=" + assignment_id]
        env = dict(env, SCENARIO_RUN_NAME=owner)
        deploy = self.az_with_rbac_retry if principal_type == "ServicePrincipal" else self.az
        if "custom-ca" in self.scenario["clusterFeatures"]:
            prepared = deploy(
                "deployment", "group", "create", "--resource-group", name,
                "--name", "scenario-proxy-bootstrap", "--parameters", str(ROOT / self.scenario["cluster"]),
                *override,
                env=dict(env, SCENARIO_PROXY_BOOTSTRAP_ONLY="true"),
                json_output=True, private=True)
            vm = prepared["properties"]["outputs"]["proxyVmName"]["value"]
            command = self.az(
                "vm", "run-command", "show", "--resource-group", name, "--vm-name", vm,
                "--name", "proxy-bootstrap", "--expand", "instanceView", json_output=True, private=True)
            view = command.get("instanceView") or {}
            if view.get("executionState") != "Succeeded" or view.get("exitCode") != 0:
                raise RuntimeError("Proxy CA bootstrap did not complete successfully")
            certificate = view["output"].strip()
            der = ssl.PEM_cert_to_DER_cert(base64.b64decode(certificate, validate=True).decode("ascii"))
            proof = {"status": "PASS", "vm": vm, "sha256": hashlib.sha256(der).hexdigest(),
                     "reader": "authenticated Azure VM run-command", "signingKey": "retained only on VM"}
            save(self.args.evidence / "proxy-ca-bootstrap.json", proof)
            print("Proxy CA bootstrap: " + json.dumps(proof), flush=True)
            env = dict(env, SCENARIO_PROXY_BOOTSTRAP_ONLY="false",
                       SCENARIO_PROXY_TRUST_CERTIFICATE=certificate)
        result = deploy(
            "deployment", "group", "create", "--resource-group", name,
            "--name", "scenario", "--parameters", str(ROOT / self.scenario["cluster"]),
            *override,
            env=env, json_output=True, private=True)
        if expected_pools is not None:
            self.verify_deployment(result, expected_pools)
        outputs = {key: value["value"] for key, value in result["properties"]["outputs"].items()}
        managed_groups = [outputs["nodeResourceGroupName"], outputs["amwManagedResourceGroupName"]]
        if any(not isinstance(group, str) or not group for group in managed_groups):
            raise RuntimeError("Deployment did not identify both managed resource groups")
        self.receipt["managedGroups"] = sorted(set(managed_groups))
        save(self.receipt_path, self.receipt)
        save(self.args.evidence / "deployment.json", outputs)
        self.grant_sre_read(outputs["workspaceId"])
        self.verify_sre_managed_groups(outputs, principal, principal_type)
        return outputs

    def protect_private_host(self):
        self.assert_owned()
        group = self.receipt["resourceGroup"]
        locks = self.az("lock", "list", "--resource-group", group, json_output=True)
        if not any(row.get("level") == "CanNotDelete" and row.get("name") == "scenario-retain"
                   for row in locks):
            self.az("lock", "create", "--resource-group", group, "--name", "scenario-retain",
                    "--lock-type", "CanNotDelete", "--notes", "Retained ama-metrics scenario; no expiry")
        observed = self.az("lock", "show", "--resource-group", group, "--name", "scenario-retain",
                           json_output=True)
        if observed.get("level") != "CanNotDelete":
            raise RuntimeError("Private scenario deletion protection was not confirmed")
        save(self.args.evidence / "deletion-protection.json",
             {"status": "PASS", "id": observed["id"], "level": observed["level"]})

    def persistent_state(self, env, expected_pools):
        if not expected_pools:
            raise ValueError("Persistent reuse requires the preflighted pool inventory")
        name, owner = persistent_identity(self.scenario["id"])
        if self.az("group", "exists", "--name", name).strip() == "false":
            return None
        self.receipt = {"subscription": self.args.subscription, "resourceGroup": name,
                        "runId": owner, "scenario": self.scenario["id"], "token": owner}
        owned = self.assert_owned()
        if owned.get("location", "").casefold() != self.args.location.casefold():
            raise ValueError("Persistent resource group location changed")
        clusters = self.az("aks", "list", "--resource-group", name, json_output=True)
        if not clusters:
            return None
        if len(clusters) != 1:
            raise ValueError("Persistent resource group must contain exactly one cluster")
        deployment = self.az("deployment", "group", "show", "--resource-group", name,
                             "--name", "scenario", json_output=True)
        self.verify_deployment(deployment, expected_pools)
        outputs = {key: value["value"] for key, value in deployment["properties"]["outputs"].items()}
        cluster = self.az("aks", "show", "--resource-group", name, "--name", outputs["clusterName"],
                          json_output=True)
        verify_secret_watch_version(self.scenario, cluster)
        current_version = cluster.get("kubernetesVersion")
        requested_version = env["SCENARIO_KUBERNETES_VERSION"]
        upgradeable_secret_watch = (
            "secrets-watch-enforcement" in self.scenario["clusterFeatures"]
            and version_key(current_version) < version_key(requested_version))
        if current_version != requested_version and not upgradeable_secret_watch:
            raise ValueError("Persistent AKS cluster does not match the retained deployment contract")
        self.current_kubernetes_version = current_version
        network = cluster.get("networkProfile") or {}
        advanced = network.get("advancedNetworking") or {}
        if (clusters[0].get("id", "").casefold() != outputs["clusterId"].casefold()
                or cluster.get("id", "").casefold() != outputs["clusterId"].casefold()
                or cluster.get("location", "").casefold() != self.args.location.casefold()
                or cluster.get("provisioningState") != "Succeeded"
                or (cluster.get("powerState") or {}).get("code") != "Running"
                or cluster.get("nodeResourceGroup") != outputs["nodeResourceGroupName"]
                or (cluster.get("identity") or {}).get("type") != outputs["identityType"]
                or network.get("networkPlugin") != "azure"
                or network.get("networkPluginMode") != "overlay"
                or bool(advanced.get("enabled")) != ("managed-acns" in self.scenario["clusterFeatures"])
                or ("managed-acns" in self.scenario["clusterFeatures"]
                    and (advanced.get("observability") or {}).get("enabled") is not True)):
            raise ValueError("Persistent AKS cluster does not match the retained deployment contract")
        if "cilium" in self.scenario["clusterFeatures"]:
            verify_cilium_profile(cluster)
        verify_pool_configuration(expected_pools, cluster.get("agentPoolProfiles") or [])
        managed_groups = [outputs["nodeResourceGroupName"], outputs["amwManagedResourceGroupName"]]
        if any(not isinstance(value, str) or not value for value in managed_groups):
            raise ValueError("Retained deployment does not identify both managed resource groups")
        self.receipt["managedGroups"] = sorted(set(managed_groups))
        return outputs

    def verify_deployment(self, deployment, expected_pools):
        properties = deployment.get("properties", {})
        if properties.get("provisioningState") != "Succeeded":
            raise ValueError("Retained scenario deployment is not successful")
        outputs = {key: value["value"] for key, value in properties.get("outputs", {}).items()}
        resource_tags = outputs.get("resourceTags") or {}
        expected_scenario = ("linux" if "secrets-watch-enforcement" in self.scenario["clusterFeatures"]
                             else self.scenario["id"])
        if (resource_tags.get("owner") != "ama-metrics-e2e"
                or resource_tags.get("scenario") != expected_scenario
                or resource_tags.get("run") != self.receipt["runId"]
                or outputs.get("location") != self.args.location
                or (expected_pools is not None and outputs.get("expectedPools") != expected_pools)
                or outputs.get("addonEnabled") is not False
                or outputs.get("installationMode") != "candidate-addon"):
            raise ValueError("Retained deployment outputs do not match the scenario contract")

    def wait_for_rbac(self, principal, retained=False):
        uuid.UUID(principal)
        if principal.casefold() == SRE_PRINCIPAL:
            raise ValueError("The SRE identity cannot receive bootstrap administrator roles")
        scope = f"/subscriptions/{self.args.subscription}/resourceGroups/{self.receipt['resourceGroup']}"
        allowed = list(RBAC_ALLOWED_ROLES)
        condition = rbac_condition(allowed, sre=True)
        if retained:
            self.assert_owned()
            assignments = self.az("role", "assignment", "list", "--resource-group",
                                  self.receipt["resourceGroup"], json_output=True, private=True)
            matches = [row for row in assignments if row.get("scope", "").casefold() == scope.casefold()
                       and row.get("principalId", "").casefold() == principal.casefold()
                       and row.get("principalType", "").casefold() == "serviceprincipal"
                       and row.get("roleDefinitionId", "").rsplit("/", 1)[-1].casefold() == RBAC_ADMIN_ROLE
                       and row.get("conditionVersion") == "2.0"
                       and re.sub(r"\s+", "", row.get("condition") or "") in {
                           re.sub(r"\s+", "", condition),
                           re.sub(r"\s+", "", rbac_condition(allowed))}]
            if len(matches) != 1:
                raise RuntimeError("Retained bootstrap delegation is missing or incompatible with the SRE read contract; authorized administrator must update it narrowly")
            if re.sub(r"\s+", "", matches[0]["condition"]) == re.sub(r"\s+", "", condition):
                return
        command = f"$started = [DateTimeOffset]::FromUnixTimeMilliseconds({int(time.time() * 1000)})\n"
        owner = persistent_identity(self.scenario["id"])[1]
        profile = Path(self.scenario["cluster"]).stem
        replacements = {
            "SUBSCRIPTION": self.args.subscription, "GROUP": self.receipt["resourceGroup"],
            "SCOPE": scope, "SCENARIO": self.scenario["id"], "PROFILE": profile, "OWNER": owner,
            "PIPELINE": principal, "SRE": SRE_PRINCIPAL, "READER": RBAC_PROXY_READER,
            "ADMIN": RBAC_ADMIN_ROLE, "CONDITION": sibling_rbac_condition(),
            "PARENTCONDITION": condition, "LEGACYCONDITION": rbac_condition(allowed),
        }
        managed_script = r"""
$ErrorActionPreference = 'Stop'
$subscription = '@@SUBSCRIPTION@@'
$parent = '@@GROUP@@'
$parentScope = '@@SCOPE@@'
$scenario = '@@SCENARIO@@'
$profile = '@@PROFILE@@'
$owner = '@@OWNER@@'
$pipeline = '@@PIPELINE@@'
$sre = '@@SRE@@'
$reader = '@@READER@@'
$admin = '@@ADMIN@@'
$siblingCondition = '@@CONDITION@@'
$parentCondition = '@@PARENTCONDITION@@'
$legacyCondition = '@@LEGACYCONDITION@@'
function Read-Az([string[]] $arguments) {
  $json = & az @arguments --subscription $subscription --only-show-errors --output json
  if ($LASTEXITCODE -ne 0) { throw "Azure read failed: $($arguments[0..1] -join ' ')" }
  return (($json -join "`n") | ConvertFrom-Json)
}
function Has-Grant([string] $scope, [string] $identity, [string] $role, [string] $condition) {
  $rows = @(Read-Az -arguments @('role','assignment','list','--scope',$scope,'--assignee-object-id',$identity,'--fill-principal-name','false'))
  $roleId = "/subscriptions/$subscription/providers/Microsoft.Authorization/roleDefinitions/$role"
  $matches = @($rows | Where-Object { $_.scope -ieq $scope -and $_.principalId -ieq $identity -and $_.roleDefinitionId -ieq $roleId })
  if ($matches.Count -gt 1) { throw "Ambiguous assignments at $scope" }
  if (!$matches.Count) { return $false }
  $row = $matches[0]
  if ($row.principalType -ine 'ServicePrincipal') { throw "Invalid principal type at $scope" }
  if ($condition) {
    if ($row.conditionVersion -cne '2.0' -or ($row.condition -replace '\s','') -cne ($condition -replace '\s','')) {
      if ($scope -ieq $parentScope -and $identity -ieq $pipeline -and $role -ieq $admin -and
          $row.conditionVersion -ceq '2.0' -and
          ($row.condition -replace '\s','') -ceq ($legacyCondition -replace '\s','')) { return $false }
      throw "Incompatible sibling delegation at $scope"
    }
  } elseif ($null -ne $row.condition -or $null -ne $row.conditionVersion) {
    throw "Conditional SRE Reader at $scope"
  }
  return $true
}
function Ensure-Grant([string] $scope, [string] $identity, [string] $role, [string] $condition) {
  if (Has-Grant $scope $identity $role $condition) { return }
  $deadline = [DateTimeOffset]::UtcNow.AddMinutes(10)
  $delay = 5
  do {
    $args = @('role','assignment','create','--subscription',$subscription,'--assignee-object-id',$identity,
              '--assignee-principal-type','ServicePrincipal','--role',$role,'--scope',$scope,'--output','none')
    if ($condition) { $args += @('--condition-version','2.0','--condition',$condition) }
    $file = $null
    if ($scope -ieq $parentScope -and $identity -ieq $pipeline -and $role -ieq $admin) {
      $roleId = "/subscriptions/$subscription/providers/Microsoft.Authorization/roleDefinitions/$role"
      $existing = @(Read-Az -arguments @('role','assignment','list','--scope',$scope,'--assignee-object-id',$identity,'--fill-principal-name','false') |
        Where-Object { $_.scope -ieq $scope -and $_.principalId -ieq $identity -and $_.roleDefinitionId -ieq $roleId })
      if ($existing.Count) {
        $row = $existing[0]
        $prefix = "$scope/providers/Microsoft.Authorization/roleAssignments/"
        if ($existing.Count -ne 1 -or !$row.id -or
            !$row.id.StartsWith($prefix, [StringComparison]::OrdinalIgnoreCase) -or
            $row.id.Substring($prefix.Length) -notmatch '^[0-9a-fA-F]{8}(-[0-9a-fA-F]{4}){3}-[0-9a-fA-F]{12}$' -or
            $row.principalType -ine 'ServicePrincipal' -or $row.conditionVersion -cne '2.0' -or
            ($row.condition -replace '\s','') -cne ($legacyCondition -replace '\s','')) {
          throw 'Parent delegation migration would change an unapproved assignment'
        }
        $update = @{ id=$row.id; name=$row.id.Substring($prefix.Length); scope=$row.scope; principalId=$row.principalId; principalType=$row.principalType;
                     roleDefinitionId=$row.roleDefinitionId; condition=$parentCondition; conditionVersion='2.0' }
        $file = [IO.Path]::GetTempFileName()
        [IO.File]::WriteAllText($file, (ConvertTo-Json -InputObject $update -Depth 10 -Compress), [Text.UTF8Encoding]::new($false))
        $args = @('role','assignment','update','--subscription',$subscription,'--role-assignment',$file,'--output','none')
      }
    }
    $preference = $ErrorActionPreference
    try {
      $ErrorActionPreference = 'Continue'
      $output = & az @args 2>&1
      $code = $LASTEXITCODE
    } finally {
      $ErrorActionPreference = $preference
      if ($file) { Remove-Item -LiteralPath $file }
    }
    if (Has-Grant $scope $identity $role $condition) { return }
    $detail = $output | Out-String
    if ($code -ne 0 -and $detail -notmatch '(?i)RoleAssignmentExists' -and
        ($detail -notmatch '(?i)authorizationfailed' -or $detail -notmatch '(?i)roleassignments/write')) {
      throw "Sibling grant failed at ${scope}: $detail"
    }
    if ([DateTimeOffset]::UtcNow -ge $deadline) { throw "Sibling grant not confirmed at $scope" }
    Start-Sleep -Seconds $delay
    $delay = [Math]::Min($delay * 2, 30)
  } while ($true)
}
function Owned-Resource($resource, [string] $kind) {
  $prefix = "$parentScope/providers/$kind/"
  if (!$resource.id -or !$resource.id.StartsWith($prefix, [StringComparison]::OrdinalIgnoreCase) -or
      $resource.id.Substring($prefix.Length) -notmatch '^[A-Za-z0-9][A-Za-z0-9._-]*$' -or
      $resource.tags.owner -cne 'ama-metrics-e2e' -or $resource.tags.scenario -cne $profile -or
      $resource.tags.run -cne $owner) { throw "Unowned or ambiguous $kind resource" }
}
function Managed-Scope([string] $name, [string] $resourceId) {
  if (!$name -or $name -notmatch '^[A-Za-z0-9][A-Za-z0-9._()-]*$' -or $name -ieq $parent) {
    throw 'Invalid managed resource group name'
  }
  if (!(Read-Az -arguments @('group','exists','--name',$name))) { return $null }
  $group = Read-Az -arguments @('group','show','--name',$name)
  $scope = "/subscriptions/$subscription/resourceGroups/$name"
  if ($group.id -ine $scope -or ($group.managedBy -and $group.managedBy -ine $resourceId)) {
    throw "Managed group ownership mismatch: $name"
  }
  return $scope
}
$waitingForDeploymentUntil = [DateTimeOffset]::UtcNow.AddMinutes(20)
while ($true) {
  $group = Read-Az -arguments @('group','show','--name',$parent)
  if ($group.id -ine $parentScope -or $group.tags.owner -cne 'ama-metrics-e2e' -or
      $group.tags.scenario -cne $scenario -or $group.tags.'scenario-run' -cne $owner -or
      $group.tags.'scenario-token' -cne $owner -or $null -ne $group.tags.expiresAt) {
    throw 'Parent resource group ownership changed'
  }
  Ensure-Grant $parentScope $pipeline $admin $parentCondition
  $clusters = @(Read-Az -arguments @('aks','list','--resource-group',$parent))
  $accounts = @(Read-Az -arguments @('resource','list','--resource-group',$parent,'--resource-type','Microsoft.Monitor/accounts'))
  if ($clusters.Count -gt 1 -or $accounts.Count -gt 1) { throw 'Ambiguous parent AKS or AMW inventory' }
  $nodeScope = $null
  $amwScope = $null
  if ($clusters.Count) {
    $cluster = $clusters[0]
    Owned-Resource $cluster 'Microsoft.ContainerService/managedClusters'
    if ($cluster.nodeResourceGroup) { $nodeScope = Managed-Scope $cluster.nodeResourceGroup $cluster.id }
  }
  if ($accounts.Count) {
    Owned-Resource $accounts[0] 'Microsoft.Monitor/accounts'
    $account = Read-Az -arguments @('resource','show','--ids',$accounts[0].id,'--api-version','2023-04-03')
    Owned-Resource $account 'Microsoft.Monitor/accounts'
    if ($account.id -ine $accounts[0].id) { throw 'AMW identity changed' }
    $dce = $account.properties.defaultIngestionSettings.dataCollectionEndpointResourceId
    if ($dce) {
      $parts = $dce -split '/'
      if ($parts.Count -ne 9 -or $parts[1] -ine 'subscriptions' -or $parts[2] -ine $subscription -or
          $parts[3] -ine 'resourceGroups' -or $parts[5] -ine 'providers' -or
          $parts[6] -ine 'Microsoft.Insights' -or $parts[7] -ine 'dataCollectionEndpoints' -or !$parts[8]) {
        throw 'Invalid AMW managed-group association'
      }
      $amwScope = Managed-Scope $parts[4] $account.id
    }
  }
  if ($nodeScope -and $amwScope -and $nodeScope -ieq $amwScope) { throw 'Managed groups must be distinct' }
  foreach ($target in @($nodeScope, $amwScope)) {
    if ($target) {
      Ensure-Grant $target $pipeline $admin $siblingCondition
      Ensure-Grant $target $sre $reader $null
    }
  }
  if ($nodeScope -and $amwScope) { break }
  $deployments = @(Read-Az -arguments @('deployment','group','list','--resource-group',$parent))
  $deployment = @($deployments | Where-Object { $_.name -ceq 'scenario' })
  if ($deployment.Count -gt 1) { throw 'Ambiguous scenario deployment' }
  if ($deployment.Count -and !$deployment[0].properties.timestamp) { throw 'Scenario deployment timestamp is missing' }
  if ($deployment.Count -and
      [DateTimeOffset]::Parse($deployment[0].properties.timestamp) -ge $started -and
      $deployment[0].properties.provisioningState -in @('Succeeded','Failed','Canceled')) {
    throw 'Scenario deployment ended before both sibling grants were confirmed'
  }
  if ((!$deployment.Count -or [DateTimeOffset]::Parse($deployment[0].properties.timestamp) -lt $started) -and
      [DateTimeOffset]::UtcNow -ge $waitingForDeploymentUntil) {
    throw 'Scenario deployment did not start after bootstrap'
  }
  Start-Sleep -Seconds 10
}
"""
        for key, value in replacements.items():
            managed_script = managed_script.replace(f"@@{key}@@", value.replace("'", "''"))
        command += "\n" + managed_script
        proof = {"scope": scope, "principalId": principal, "roleDefinitionId": RBAC_ADMIN_ROLE,
                 "allowedRoleDefinitionIds": allowed, "condition": condition, "status": "waiting"}
        self.receipt["rbacBootstrapScope"] = scope
        save(self.receipt_path, self.receipt)
        (self.args.evidence / "rbac-bootstrap.ps1").write_text(command + "\n", encoding="utf-8")
        save(self.args.evidence / "rbac-bootstrap.json", proof)
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
                        and assignment.get("roleDefinitionId", "").rsplit("/", 1)[-1].casefold() == RBAC_ADMIN_ROLE
                        and assignment.get("conditionVersion") == "2.0"
                        and re.sub(r"\s+", "", assignment.get("condition") or "") ==
                        re.sub(r"\s+", "", rbac_condition(allowed)) and condition != rbac_condition(allowed)):
                    continue
                if (assignment.get("scope", "").casefold() == scope.casefold()
                        and assignment.get("principalId", "").casefold() == principal.casefold()
                        and assignment.get("principalType", "").casefold() == "serviceprincipal"
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
        tags = group.get("tags", {})
        if (any(tags.get(k) != v for k, v in expected.items())
                or (self.receipt["resourceGroup"].startswith("ama-e2e-persistent-") and "expiresAt" in tags)):
            raise ValueError("Resource group ownership changed; refusing mutation/deletion")
        return group

    def cleanup(self):
        if self.receipt is None and not self.receipt_path.exists():
            return
        try:
            if self.receipt is None:
                self.receipt = load(self.receipt_path)
            group = self.receipt["resourceGroup"]
            if group.startswith("ama-e2e-persistent-"):
                raise ValueError("Persistent campaign resources are not eligible for cleanup")
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
                if {"private-api", "ampls"} & set(self.scenario.get("clusterFeatures", [])):
                    self.protect_private_host()
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
                    "resourceGroup": group, "runId": getattr(self.args, "run_id", self.receipt.get("runId")),
                    "ownerRunId": self.receipt.get("runId"),
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
        retained = load(self.args.evidence / "retained-resources.json")
        group = retained["resourceGroup"]
        if group.casefold().startswith("ama-e2e-persistent-"):
            raise ValueError("Persistent campaign bootstrap grants must be retained for future runs")
        run = load(self.args.evidence / "result.json")
        proof = load(self.args.evidence / "rbac-bootstrap.json")
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
                or re.sub(r"\s+", "", condition) not in {
                    re.sub(r"\s+", "", rbac_condition(allowed)),
                    re.sub(r"\s+", "", rbac_condition(allowed, sre=True))}):
            raise ValueError("Bootstrap revocation requires the exact successful retained-run receipt")
        parent = next((row for row in retained["inventory"] if row.get("name") == group), None)
        if not parent or not parent.get("tags", {}).get("scenario-token"):
            raise ValueError("Retained resource-group ownership tags are missing")
        self.receipt = {
            "subscription": self.args.subscription, "resourceGroup": group,
            "runId": retained.get("ownerRunId", run["runId"]), "scenario": run["scenario"],
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


def tunnel_lease(run_id, claim_until=None, root="/var/lib/scenario-tunnel"):
    """VM-side lease serializing the shared tunnel key and NSG rule; the pipeline has no exclusive lock."""
    run_id = str(uuid.UUID(run_id))
    # Keep error cursors open before writes; reopening for syncfs can miss earlier writeback errors.
    script = f"""exec 7<"$(dirname {root})"
install -d -m 700 {root}
exec 9>{root}/lock
flock -w 120 9
flush_state() {{
  python3 -c 'import ctypes, os, sys
libc = ctypes.CDLL(None, use_errno=True)
for fd in map(int, sys.argv[1:]):
    if libc.syncfs(fd) != 0:
        error = ctypes.get_errno()
        raise OSError(error, os.strerror(error))' "$@"
}}
tunnel_sessions_status() {{
  if id scenario-tunnel >/dev/null 2>&1; then
    process_status=0
    pgrep -u scenario-tunnel >/dev/null 2>&1 || process_status=$?
    if [ "$process_status" -eq 0 ]; then return 0; fi
    if [ "$process_status" -ne 1 ]; then return 2; fi
  elif [ "$1" = required ]; then
    if [ ! -e {root}/account-not-created ] || [ -e /home/scenario-tunnel ]; then return 2; fi
    account_status=0
    getent passwd scenario-tunnel >/dev/null 2>&1 || account_status=$?
    if [ "$account_status" -ne 2 ]; then return 2; fi
  fi
  connections=$(ss -Htn state connected exclude time-wait '( sport = :22 )') || return 2
  if [ -n "$connections" ]; then return 0; fi
  return 1
}}
OWNER= UNTIL=0
if [ -s {root}/lease ]; then read -r OWNER UNTIL < {root}/lease; fi
case "$UNTIL" in ''|*[!0-9]*) echo 'Corrupt private API tunnel lease' >&2; exit 1;; esac
"""
    if claim_until is None:
        return script + f"""if [ "$OWNER" != '{run_id}' ]; then
  printf '%s\\n' '{run_id}' >> {root}/revoked
  flush_state 7 9
  printf 'SCENARIO_RESULT={{"owned":false}}\\n'; exit 0
fi
"""
    # A claim delayed past its own cleanup must not install a key nobody will remove.
    return script + f"""if [ -e {root}/revoked ] && grep -qxF '{run_id}' {root}/revoked; then
  printf 'SCENARIO_RESULT={{"leaseOwner":"revoked","leaseUntil":0}}\\n'; exit 0
fi
if [ -e {root}/pending ]; then
  printf 'SCENARIO_RESULT={{"leaseOwner":"unsettled","leaseUntil":0}}\\n'; exit 0
fi
if [ -n "$OWNER" ] && [ "$OWNER" != '{run_id}' ]; then
  printf 'SCENARIO_RESULT={{"leaseOwner":"%s","leaseUntil":%s}}\\n' "$OWNER" "$UNTIL"; exit 0
fi
session_status=0
tunnel_sessions_status optional || session_status=$?
if [ "$session_status" -ne 1 ]; then
  printf 'SCENARIO_RESULT={{"leaseOwner":"unsettled","leaseUntil":0}}\\n'; exit 0
fi
rm -f {root}/account-not-created
printf '%s %s\\n' '{run_id}' '{int(claim_until)}' > {root}/lease.tmp
mv {root}/lease.tmp {root}/lease
flush_state 7 9
"""


def close_private_api(host, outputs, run_id):
    run_id = str(uuid.UUID(run_id))
    vm = outputs["orchestratorVmName"]
    # Only the lease owner may touch the shared key/rule; the lease is kept until ingress is denied.
    operation = str(uuid.uuid4())
    result = vm_script(host, vm, "set -eu\n" + tunnel_lease(run_id) + f"""printf '%s\\n' '{run_id}' >> /var/lib/scenario-tunnel/revoked
if [ -d /home/scenario-tunnel/.ssh ]; then
  exec 8< /home/scenario-tunnel/.ssh
else
  exec 8< /home
fi
rm -f /home/scenario-tunnel/.ssh/authorized_keys
test ! -e /home/scenario-tunnel/.ssh/authorized_keys
if [ -e /var/lib/scenario-tunnel/pending ]; then
  touch /var/lib/scenario-tunnel/uncertain
  flush_state 7 8 9
  printf 'SCENARIO_RESULT={{"unsettled":true}}\\n'; exit 0
fi
printf '%s\\n' '{operation}' > /var/lib/scenario-tunnel/pending
flush_state 7 8 9
printf 'SCENARIO_RESULT={{"owned":true,"keyRemoved":true}}\\n'
""")
    if result.get("owned") is False:
        previous = host.args.evidence / "private-api-cleanup.json"
        if previous.exists():
            old_proof = load(previous)
            if old_proof.get("status") == "PASS" and old_proof.get("runId") == run_id:
                print("Private API cleanup: previous PASS for this run retained", flush=True)
                return
        proof = {"status": "SKIPPED", "reason": "private API tunnel lease is not held by this run"}
        (host.args.evidence / "private-api-cleanup.json").write_text(json.dumps(proof) + "\n")
        print("Private API cleanup: " + json.dumps(proof), flush=True)
        return
    if result.get("unsettled") is True:
        raise RuntimeError("Private API NSG operation is unsettled; tunnel lease retained for manual recovery")
    if result.get("keyRemoved") is not True:
        raise RuntimeError("Temporary SSH key removal was not confirmed")
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
    released = vm_script(host, vm, "set -eu\n" + tunnel_lease(run_id) + f"""if [ -e /var/lib/scenario-tunnel/uncertain ] ||
   [ "$(cat /var/lib/scenario-tunnel/pending)" != '{operation}' ]; then
  printf 'SCENARIO_RESULT={{"released":false}}\\n'; exit 0
fi
attempt=0
while [ "$attempt" -lt 18 ]; do
  session_status=0
  tunnel_sessions_status required || session_status=$?
  if [ "$session_status" -ne 0 ]; then break; fi
  attempt=$((attempt + 1))
  if [ "$attempt" -lt 18 ]; then sleep 5; fi
done
if [ "$session_status" -ne 1 ]; then
  rm -f /var/lib/scenario-tunnel/pending
  flush_state 7 9
  printf 'SCENARIO_RESULT={{"released":false,"sessionsUnsettled":true}}\\n'; exit 0
fi
rm -f /var/lib/scenario-tunnel/lease /var/lib/scenario-tunnel/pending /var/lib/scenario-tunnel/account-not-created
flush_state 7 9
printf 'SCENARIO_RESULT={{"released":true}}\\n'
""")
    if released.get("sessionsUnsettled") is True:
        raise RuntimeError("Private API tunnel sessions are active or uninspectable; tunnel lease retained")
    if released.get("released") is not True:
        raise RuntimeError("Private API tunnel lease release was not confirmed")
    proof = {"status": "PASS", "runId": run_id, "ingress": "Deny", "keyRemoved": True, "leaseReleased": True,
             "sessionsClosed": True}
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
        operation = str(uuid.uuid4())
        reserved = vm_script(host, vm, "set -eu\n" + tunnel_lease(run_id) + f"""if [ "$UNTIL" -le "$(date +%s)" ] || [ -e /var/lib/scenario-tunnel/pending ]; then
  printf 'SCENARIO_RESULT={{"reserved":false}}\\n'; exit 0
fi
printf '%s\\n' '{operation}' > /var/lib/scenario-tunnel/pending
flush_state 7 9
printf 'SCENARIO_RESULT={{"reserved":true}}\\n'
""")
        if reserved.get("reserved") is not True:
            raise RuntimeError("Private API tunnel lease cannot reserve the NSG operation")
        host.az("network", "nsg", "rule", "create", "--resource-group", group,
                "--nsg-name", nsg, "--name", "ActivePipelineTunnel", "--priority", "100",
                "--access", "Allow", "--direction", "Inbound", "--protocol", "Tcp",
                "--source-address-prefixes", prefix, "--source-port-ranges", "*",
                "--destination-address-prefixes", "*", "--destination-port-ranges", "22")
        completed = vm_script(host, vm, "set -eu\n" + tunnel_lease(run_id) + f"""if [ -e /var/lib/scenario-tunnel/uncertain ] ||
   [ "$(cat /var/lib/scenario-tunnel/pending)" != '{operation}' ]; then
  printf 'SCENARIO_RESULT={{"completed":false}}\\n'; exit 0
fi
rm -f /var/lib/scenario-tunnel/pending
flush_state 7 9
printf 'SCENARIO_RESULT={{"completed":true}}\\n'
""")
        if completed.get("completed") is not True:
            raise RuntimeError("Private API NSG operation was not safely settled; tunnel lease retained")
        print(f"Private API ingress: {prefix} -> {public_ip}:22 (direct IPv4 discovery)", flush=True)
        return prefix

    process = None
    key_attempted = False
    with tempfile.TemporaryDirectory(prefix="ama-private-api-") as directory:
        key = Path(directory) / "key"
        host.commands.run(["ssh-keygen", "-q", "-t", "ed25519", "-N", "", "-f", str(key)], private=True)
        public_key = key.with_suffix(".pub").read_text().strip()
        run_id = str(uuid.UUID(host.args.run_id))
        until = datetime.now(timezone.utc) + timedelta(seconds=host.args.max_run_seconds + 3600)
        expires = until.strftime("%Y%m%d%H%M%SZ")
        try:
            # Recovery runs in a later step without the run ID; it must release only this lease.
            (host.args.evidence / "private-api-lease.json").write_text(json.dumps({"runId": run_id}) + "\n")
            key_attempted = True
            proof = vm_script(host, vm, "set -eu\n" + tunnel_lease(run_id, until.timestamp()) + f"""if ! id scenario-tunnel >/dev/null 2>&1; then
  if ! useradd --create-home --shell /bin/bash scenario-tunnel; then
    account_status=0
    getent passwd scenario-tunnel >/dev/null 2>&1 || account_status=$?
    if [ "$account_status" -eq 2 ] && [ ! -e /home/scenario-tunnel ]; then
      touch /var/lib/scenario-tunnel/account-not-created
      flush_state 7 9
    fi
    echo 'Could not create private API tunnel account' >&2
    exit 1
  fi
fi
printf '%s\\n' 'Match User scenario-tunnel' '    AuthenticationMethods publickey' '    AllowTcpForwarding local' '    ClientAliveInterval 15' '    ClientAliveCountMax 3' 'Match all' > /etc/ssh/sshd_config.d/00-scenario-tunnel.conf
sshd -t
test "$(sshd -T -C user=scenario-tunnel,host=localhost,addr=127.0.0.1 | awk '$1 == "allowtcpforwarding" {{print $2}}')" = local
test "$(sshd -T -C user=scenario-tunnel,host=localhost,addr=127.0.0.1 | awk '$1 == "clientaliveinterval" {{print $2}}')" = 15
test "$(sshd -T -C user=scenario-tunnel,host=localhost,addr=127.0.0.1 | awk '$1 == "clientalivecountmax" {{print $2}}')" = 3
systemctl reload ssh
install -d -m 700 -o scenario-tunnel -g scenario-tunnel /home/scenario-tunnel/.ssh
printf '%s\\n' 'expiry-time="{expires}",no-agent-forwarding,no-X11-forwarding,no-pty,no-user-rc,command="/bin/false",permitopen="{fqdn}:443" {public_key}' > /home/scenario-tunnel/.ssh/authorized_keys
chown scenario-tunnel:scenario-tunnel /home/scenario-tunnel/.ssh/authorized_keys
chmod 600 /home/scenario-tunnel/.ssh/authorized_keys
IP=$(getent ahostsv4 '{fqdn}' | head -n1 | cut -d' ' -f1)
KEY=$(cat /etc/ssh/ssh_host_ed25519_key.pub)
printf 'SCENARIO_RESULT={{"privateIp":"%s","hostKey":"%s"}}\\n' "$IP" "$KEY"
""")
            if "leaseOwner" in proof:
                key_attempted = False
                (host.args.evidence / "private-api-lease.json").unlink(missing_ok=True)
                raise RuntimeError(f"Private API tunnel is leased by run {proof['leaseOwner']} until {proof['leaseUntil']}")
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
                    close_private_api(host, outputs, run_id)
            finally:
                host.commands.deadline = deadline


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
    known = ginkgo_labels(REPO)
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
    template = variant.get("workflow")
    if not template or not re.fullmatch(r"[a-z0-9][a-z0-9-]*\.yaml", template):
        raise ValueError(f"{variant['id']}: an existing ConfigTests workflow template is required")
    path = resource_path(REPO / "otelcollector/test/testkube/config-processing-test-crs",
                         template, REPO)
    names = [doc["metadata"]["name"] for doc in yaml.safe_load_all(path.read_text())
             if doc and doc.get("kind") == "TestWorkflow"]
    common = set(names) & {"configprocessingcommon", "configprocessingcommonv2"}
    if len(common) == 1:
        return template, common.pop()
    if len(names) == 1:
        return template, names[0]
    raise ValueError(f"{variant['id']}: ambiguous workflow template; an explicit case is required")


def workflow_result(destination, workflow, source_sha, image):
    result = load(destination / "testkube-results-ConfigTests.json")
    if (result.get("status") != "passed" or result.get("test_source_sha") != source_sha
            or result.get("runner_image") != image):
        raise RuntimeError("TestKube result did not explicitly pass with the expected source and runner")
    executions = result.get("executions", [])
    if len(executions) != 1:
        raise RuntimeError("Expected exactly the one submitted TestKube execution")
    row = executions[0]
    execution_id = row.get("execution_id", "")
    if (row.get("workflow") != workflow or row.get("status") != "passed"
            or not re.fullmatch(r"[A-Za-z0-9][A-Za-z0-9._-]{0,127}", execution_id)):
        raise RuntimeError("Wrong workflow, missing exact execution ID, or non-passed execution")
    expected = (destination / "testkube-executions/ConfigTests" / workflow / execution_id / "artifacts").resolve()
    artifact_dir = Path(row["artifact_dir"]).resolve()
    if artifact_dir != expected or not artifact_dir.is_relative_to(destination.resolve()) or not artifact_dir.is_dir():
        raise RuntimeError("Artifacts do not belong to this submitted execution")
    return {"workflow": workflow, "executionId": execution_id, "status": "passed",
            "artifacts": str(artifact_dir)}


def render_workflow(source, workflow, sha, image, label_filter=None, expectations=None):
    matches = [
        item for item in yaml.safe_load_all(source)
        if item and item.get("kind") == "TestWorkflow"
        and item.get("metadata", {}).get("name") == workflow
    ]
    if len(matches) != 1:
        raise ValueError("select exactly one existing TestWorkflow by metadata.name")
    document = matches[0]
    spec = document["spec"]
    if "use" in spec:
        raise ValueError("workflow must define its content, steps and pod inline")
    container_defaults = spec.get("container", {})
    spec["content"]["git"]["revision"] = sha
    spec.setdefault("pod", {}).setdefault("nodeSelector", {}).update({
        "kubernetes.io/os": "linux", "kubernetes.io/arch": "amd64",
    })
    spec["pod"].setdefault("labels", {})["azure.workload.identity/use"] = "true"
    spec["pod"]["serviceAccountName"] = "testkube-api-server-tests-job"
    paths = spec["content"]["git"].setdefault("paths", [])
    if "otelcollector/test/scenarios/settings" not in paths:
        paths.append("otelcollector/test/scenarios/settings")
    runs = 0
    for step in spec["steps"]:
        shell = step.get("shell", "")
        lines = shell.splitlines()
        for index, line in enumerate(lines):
            if not line.strip().startswith("ginkgo run "):
                continue
            if "{{" in line or "}}" in line:
                raise ValueError("unsupported workflow template expression")
            args = shlex.split(line)
            suite = args[-1]
            if not re.fullmatch(r"\./[A-Za-z][A-Za-z0-9]*", suite):
                raise ValueError("workflow must select an existing Ginkgo suite")
            options = []
            offset = 2
            while offset < len(args) - 1:
                arg = args[offset]
                if arg in {"--json-report", "--junit-report", "--label-filter"}:
                    value = args[offset + 1]
                    offset += 2
                    if arg == "--label-filter" and label_filter is None:
                        options.extend([arg, value])
                    continue
                if arg.startswith("--label-filter=") and label_filter is not None:
                    offset += 1
                    continue
                if not arg.startswith(("--json-report=", "--junit-report=")) and arg != "--fail-on-empty":
                    options.append(arg)
                offset += 1
            if label_filter is not None:
                if not label_filter.strip():
                    raise ValueError("label override cannot be empty")
                options.extend(["--label-filter", label_filter])
            options.extend(["--fail-on-empty", "--json-report=/data/reports/ginkgo.json",
                            "--junit-report=/data/reports/junit.xml", "."])
            lines[index] = (
                "GOTOOLCHAIN=\"$(go env GOVERSION)\" "
                "go run github.com/onsi/ginkgo/v2/ginkgo@"
                "\"$(go list -m github.com/onsi/ginkgo/v2 | cut -d ' ' -f 2)\" run "
                + shlex.join(options)
            )
            container = dict(container_defaults)
            container.update(step.get("container", {}))
            step["container"] = container
            container["image"] = image
            container["workingDir"] = "/data/repo/otelcollector/test/ginkgo-e2e/" + suite[2:]
            env = container.setdefault("env", [])
            values = {"GOTOOLCHAIN": "auto", "GOFLAGS": "-mod=readonly"}
            if suite == "./operator":
                values["EXPECT_OPERATOR_HTTPS"] = "true"
            if suite == "./querymetrics":
                values["SCENARIO_PROBE_IMAGE"] = image
            values.update(expectations or {})
            env[:] = [item for item in env if item["name"] not in values]
            env.extend({"name": name, "value": value} for name, value in values.items())
            if suite == "./querymetrics":
                pod_fields = {"SCENARIO_POD_NAME": "metadata.name",
                              "SCENARIO_POD_NAMESPACE": "metadata.namespace",
                              "SCENARIO_POD_UID": "metadata.uid"}
                env[:] = [item for item in env if item["name"] not in pod_fields]
                env.extend({"name": name, "valueFrom": {"fieldRef": {"fieldPath": field}}}
                           for name, field in pod_fields.items())
            runs += 1
        if "go run github.com/onsi/ginkgo/v2/ginkgo@" in "\n".join(lines):
            step["shell"] = "mkdir -p /data/reports\n" + "\n".join(
                line for line in lines if line.strip() != "mkdir -p /data/reports"
            ) + "\n"
    if runs != 1:
        raise ValueError("workflow must contain exactly one Ginkgo invocation")
    spec["steps"] = [step for step in spec["steps"] if "artifacts" not in step]
    spec["steps"].append({
        "name": "Retain Ginkgo evidence", "condition": "always",
        "artifacts": {"workingDir": "/data/reports", "paths": ["**/*"]},
    })
    return document


def acquire_workflow_hold(host):
    hold = host.kube("get", "configmap", "ama-e2e-workflow-hold", "-n", "kube-system",
                     "--ignore-not-found", "-o", "json", private=True)
    run_id = str(uuid.UUID(host.args.run_id))
    if not hold.strip():
        host.kube("create", "configmap", "ama-e2e-workflow-hold", "-n", "kube-system",
                  f"--from-literal=runId={run_id}")
        return
    hold = json.loads(hold)
    if hold.get("data", {}).get("runId") != "":
        raise RuntimeError("Existing TestKube workflow hold blocks cluster reuse; verify the prior execution before clearing it")
    patch_workflow_hold(host, hold, "", run_id)


def patch_workflow_hold(host, hold, old_run_id, new_run_id):
    metadata = hold["metadata"]
    patch = [{"op": "test", "path": "/metadata/uid", "value": metadata["uid"]},
             {"op": "test", "path": "/metadata/resourceVersion", "value": metadata["resourceVersion"]},
             {"op": "test", "path": "/data/runId", "value": old_run_id},
             {"op": "replace", "path": "/data/runId", "value": new_run_id}]
    host.kube("patch", "configmap", "ama-e2e-workflow-hold", "-n", "kube-system",
              "--type=json", "-p", json.dumps(patch), private=True)


def release_workflow_hold(host):
    hold = host.kube("get", "configmap", "ama-e2e-workflow-hold", "-n", "kube-system",
                     "-o", "json", json_output=True, private=True)
    run_id = str(uuid.UUID(host.args.run_id))
    if hold.get("data", {}).get("runId") != run_id:
        raise RuntimeError("TestKube workflow hold ownership changed; refusing to release it")
    patch_workflow_hold(host, hold, run_id, "")


def terminal_execution(result, execution_id, workflow):
    return (isinstance(result, dict) and isinstance(result.get("workflow"), dict)
            and isinstance(result.get("result"), dict)
            and result.get("id") == execution_id and result["workflow"].get("name") == workflow
            and result["result"].get("status") in
            ("passed", "failed", "aborted", "timeout", "canceled", "cancelled", "skipped"))


def public_testkube_result(result):
    statuses = {"pending", "queued", "running", "paused", "passed", "failed", "aborted",
                "timeout", "canceled", "cancelled", "skipped", "unknown"}
    reasons = set("""
        image-pull-failed definition-invalid job-create-failed resource-create-failed start-failed
        queue-limit-exceeded template-missing git-auth-failed git-clone-failed unschedulable config-missing
        volume-mount-failed admission-denied initialization-timeout process-killed step-timeout
        execution-timeout execution-stuck worker-resume-failed oom-killed evicted preempted node-shutdown
        container-error deadline-exceeded job-deleted service-not-ready artifact-upload-failed git-unreachable
        fail-fast exit-code child-workflow-failed queue-timeout queued-too-long transition-timeout
        stop-not-confirmed abort-all superseded trigger-abort user-cancel force-cancel unknown
    """.split())

    def safe_step(step):
        if not isinstance(step, dict):
            return {}
        safe = {}
        status = step.get("status")
        if isinstance(status, str) and status in statuses:
            safe["status"] = status
        reason = step.get("errorReason")
        if isinstance(reason, str) and reason:
            safe["errorReason"] = reason if reason in reasons else "other"
        for field, lower, upper in (("exitCode", -1, 255), ("attempts", 0, 1000)):
            value = step.get(field)
            if type(value) is int and lower <= value <= upper:
                safe[field] = value
        return safe

    public = {"status": result["status"]}
    initialization = safe_step(result.get("initialization"))
    if initialization:
        public["initialization"] = initialization
    step_indexes = {}
    steps = result.get("steps")
    if isinstance(steps, dict):
        public_steps = []
        for reference, step in steps.items():
            safe = safe_step(step)
            if safe:
                step_indexes[reference] = len(public_steps)
                public_steps.append(safe)
        if public_steps:
            public["steps"] = public_steps
    details = result.get("statusDetails")
    if isinstance(details, dict):
        allowed = {
            "type": {"init-failure", "execution-failure", "step-failure", "user-cancel", "unknown"},
            "reason": reasons,
            "actor": {"user", "control-plane", "trigger", "fail-fast", "runner", "quality-loop", "api", "system"},
        }
        safe_details = {}
        for field, values in allowed.items():
            value = details.get(field)
            if isinstance(value, str) and value:
                safe_details[field] = value if value in values else "other"
        reference = details.get("step")
        if isinstance(reference, str) and reference in step_indexes:
            safe_details["stepIndex"] = step_indexes[reference]
        if safe_details:
            public["statusDetails"] = safe_details
    return public


def verify_evidence_path(base, path):
    if not path.is_relative_to(base) or not path.resolve().is_relative_to(base.resolve()):
        raise ValueError("TestKube evidence escaped its destination")
    current = path
    while True:
        if current.is_symlink():
            raise ValueError("TestKube evidence must not contain links")
        if current == base:
            break
        current = current.parent


@cache
def test_report_sources(source):
    root = source / "otelcollector/test/ginkgo-e2e"
    texts, files = set(), {}
    literal = r'("(?:\\.|[^"\\])*"|`[^`]*`)'
    patterns = (
        re.compile(r'\b(?:[FXP]?(?:Describe|Context|When|It|Specify|DescribeTable|Entry)|Label)\s*\(\s*' + literal),
        re.compile(r'\bRunSpecs\s*\(\s*[^,\n]+,\s*' + literal),
    )
    for path in root.rglob("*.go"):
        contents = path.read_text(encoding="utf-8")
        files[path.relative_to(source).as_posix()] = len(contents.splitlines())
        for pattern in patterns:
            for match in pattern.findall(contents):
                try:
                    value = match[1:-1] if match.startswith("`") else json.loads(match)
                except ValueError:
                    continue
                if value.isprintable() and len(value) <= 512:
                    texts.add(value)
    labels = ginkgo_labels(source) if (root / "utils/constants.go").is_file() else set()
    return texts, labels, files


def public_testkube_reports(private_artifacts, staged_artifacts):
    reports = {"junit.xml": [], "ginkgo.json": []}
    for path in private_artifacts.rglob("*"):
        verify_evidence_path(private_artifacts, path)
        if path.is_file() and path.name in reports:
            reports[path.name].append(path)
    if (any(len(paths) != 1 for paths in reports.values())
            or reports["junit.xml"][0].parent != reports["ginkgo.json"][0].parent):
        raise RuntimeError("Expected one JUnit report and one Ginkgo report")
    try:
        junit = ET.parse(reports["junit.xml"][0]).getroot()
        suites = load(reports["ginkgo.json"][0])
    except (OSError, UnicodeError, ET.ParseError, json.JSONDecodeError):
        raise RuntimeError("Invalid JUnit or Ginkgo report") from None
    if junit.tag not in {"testsuite", "testsuites"} or not isinstance(suites, list) or not suites:
        raise RuntimeError("Invalid JUnit or Ginkgo report")

    texts, labels, files = test_report_sources(REPO)

    def location(value):
        if not isinstance(value, dict):
            return None
        filename, line = value.get("FileName"), value.get("LineNumber")
        if not isinstance(filename, str) or type(line) is not int:
            return None
        filename = filename.replace("\\", "/")
        for name, lines in files.items():
            if (filename == name or filename.endswith("/" + name)) and 0 < line <= lines:
                return {"FileName": name, "LineNumber": line}
        return None

    def safe_labels(values):
        if not isinstance(values, list):
            return []
        return [value for value in values if isinstance(value, str) and value in labels]

    states = {"passed", "failed", "skipped", "pending", "aborted", "panicked", "interrupted", "timedout"}
    failures = {"failed", "aborted", "panicked", "interrupted", "timedout"}
    node_types = {"It", "BeforeSuite", "AfterSuite", "SynchronizedBeforeSuite", "SynchronizedAfterSuite",
                  "ReportBeforeSuite", "ReportAfterSuite"}
    cases = iter(junit.iter("testcase"))
    output = ET.Element("testsuites")
    sanitized = []
    total = {"tests": 0, "failures": 0, "errors": 0, "skipped": 0}
    reports_passed = True
    ran_test = False
    for suite_index, suite in enumerate(suites, 1):
        if (not isinstance(suite, dict) or type(suite.get("SuiteSucceeded")) is not bool
                or not isinstance(suite.get("SpecReports"), list)):
            raise RuntimeError("Invalid Ginkgo suite")
        name = suite.get("SuiteDescription")
        name = name if isinstance(name, str) and name in texts else f"suite-{suite_index}"
        safe_suite = {"SuiteDescription": name, "SuiteSucceeded": suite["SuiteSucceeded"], "SpecReports": []}
        safe_suite["SuiteLabels"] = safe_labels(suite.get("SuiteLabels"))
        xml_suite = ET.SubElement(output, "testsuite", {"name": name})
        counts = {"tests": 0, "failures": 0, "errors": 0, "skipped": 0}
        reports_passed &= suite["SuiteSucceeded"]
        for index, spec in enumerate(suite["SpecReports"], 1):
            case = next(cases, None)
            if (case is None or not isinstance(spec, dict) or not isinstance(spec.get("State"), str)
                    or spec["State"] not in states):
                raise RuntimeError("JUnit and Ginkgo reports disagree")
            state = spec["State"]
            outcome = "failed" if state in failures else "skipped" if state in {"skipped", "pending"} else "passed"
            tags = [child.tag for child in case if child.tag in {"failure", "error", "skipped"}]
            actual = "failed" if any(tag in {"failure", "error"} for tag in tags) else "skipped" if tags else "passed"
            if (outcome != actual or ("skipped" in tags and actual == "failed")
                    or ("status" in case.attrib and case.get("status") != state)):
                raise RuntimeError("JUnit and Ginkgo reports disagree")
            node = spec.get("LeafNodeType")
            node = node if isinstance(node, str) and node in node_types else "spec"
            leaf = spec.get("LeafNodeText")
            leaf = leaf if isinstance(leaf, str) and leaf in texts else f"spec-{index}"
            if node != "It" and not spec.get("LeafNodeText"):
                leaf = node
            groups = spec.get("ContainerHierarchyTexts")
            groups = groups if isinstance(groups, list) else []
            groups = [value if isinstance(value, str) and value in texts else f"group-{i}"
                      for i, value in enumerate(groups, 1)]
            safe_spec = {"State": state, "LeafNodeType": node, "LeafNodeText": leaf,
                         "ContainerHierarchyTexts": groups, "LeafNodeLabels": safe_labels(spec.get("LeafNodeLabels"))}
            hierarchy_labels = spec.get("ContainerHierarchyLabels")
            if isinstance(hierarchy_labels, list):
                safe_spec["ContainerHierarchyLabels"] = [safe_labels(values) for values in hierarchy_labels]
            leaf_location = location(spec.get("LeafNodeLocation"))
            if leaf_location:
                safe_spec["LeafNodeLocation"] = leaf_location
            failure = spec.get("Failure")
            failure_location = location(failure.get("Location")) if isinstance(failure, dict) else None
            if state in failures and failure_location:
                safe_spec["Failure"] = {"Location": failure_location}
            duration = spec.get("RunTime")
            if type(duration) is int and 0 <= duration <= 604800 * 10**9:
                safe_spec["RunTime"] = duration
            safe_suite["SpecReports"].append(safe_spec)

            xml_case = ET.SubElement(xml_suite, "testcase", {
                "name": " ".join([*groups, leaf]), "classname": name, "status": state,
                "time": f"{safe_spec.get('RunTime', 0) / 10**9:.3f}",
            })
            if actual == "failed":
                tag = "error" if "error" in tags else "failure"
                message = "Test failed; diagnostic output omitted"
                if failure_location:
                    message = f"Test failed at {failure_location['FileName']}:{failure_location['LineNumber']}"
                ET.SubElement(xml_case, tag, {"message": message, "type": state})
                counts["errors" if tag == "error" else "failures"] += 1
                reports_passed = False
            elif actual == "skipped":
                ET.SubElement(xml_case, "skipped")
                counts["skipped"] += 1
            ran_test |= node == "It" and state == "passed"
            counts["tests"] += 1
        safe_suite["Counts"] = dict(counts)
        if not suite["SuiteSucceeded"] and not (counts["failures"] or counts["errors"]):
            xml_case = ET.SubElement(xml_suite, "testcase", {
                "name": "suite-level-failure", "classname": name, "status": "failed", "time": "0",
            })
            ET.SubElement(xml_case, "failure", {"message": "Ginkgo reported a suite-level failure"})
            counts["tests"] += 1
            counts["failures"] += 1
            safe_suite["SuiteLevelFailure"] = True
        xml_suite.attrib.update({key: str(value) for key, value in counts.items()})
        for key, value in counts.items():
            total[key] += value
        sanitized.append(safe_suite)
    if next(cases, None) is not None or not any(suite["SpecReports"] for suite in sanitized):
        raise RuntimeError("JUnit and Ginkgo reports disagree")
    output.attrib.update({key: str(value) for key, value in total.items()})
    staged_artifacts.mkdir()
    save(staged_artifacts / "ginkgo.json", sanitized)
    (staged_artifacts / "junit.xml").write_text(ET.tostring(output, encoding="unicode"), encoding="utf-8")
    return reports_passed and ran_test


def publish_testkube_evidence(destination, workflow, host=None, env=None):
    private_root = destination / ".azure" / "testkube-executions" / "ConfigTests" / workflow
    verify_evidence_path(destination, private_root)
    submissions = list(private_root.glob("*/submission.json"))
    if len(submissions) != 1:
        raise ValueError(f"Expected one TestKube submission record for {workflow}, found {len(submissions)}")
    submission_path = submissions[0]
    case_dir = submission_path.parent
    execution_file = case_dir / "execution.json"
    verify_evidence_path(destination, submission_path)
    verify_evidence_path(destination, execution_file)
    submission = load(submission_path)
    execution_id = submission.get("id") if isinstance(submission, dict) else None
    if (not isinstance(execution_id, str)
            or not re.fullmatch(r"[A-Za-z0-9][A-Za-z0-9_-]{0,127}", execution_id)
            or case_dir.name != execution_id):
        raise ValueError("TestKube submission returned no valid execution ID")
    execution = load(execution_file)
    if not terminal_execution(execution, execution_id, workflow):
        raise ValueError("Execution evidence does not match the submitted terminal TestKube workflow")
    status = execution["result"]["status"]
    public_case = destination / "testkube-executions" / "ConfigTests" / workflow / execution_id
    verify_evidence_path(destination, public_case)
    public_artifacts = public_case / "artifacts"
    public_execution = {
        "id": execution_id, "workflow": {"name": workflow}, "result": public_testkube_result(execution["result"]),
    }
    record = {"workflow": workflow, "execution_id": execution_id,
              "status": "passed" if status == "passed" else "failed",
              "artifact_dir": str(public_artifacts.resolve())}
    if public_case.exists():
        for name, expected in (("submission.json", {"id": execution_id}),
                               ("execution.json", public_execution), ("record.json", record)):
            path = public_case / name
            verify_evidence_path(destination, path)
            if load(path) != expected:
                raise ValueError("Existing TestKube execution evidence does not match the submission")
        verify_evidence_path(destination, public_artifacts)
        if public_artifacts.exists() and not public_artifacts.is_dir():
            raise ValueError("TestKube artifacts must be a directory")
        if public_artifacts.is_dir():
            for path in public_artifacts.rglob("*"):
                verify_evidence_path(public_artifacts, path)
    artifact_error = None
    reports_passed = False
    artifacts_available = False
    staging = case_dir / ("publication-" + str(uuid.uuid4()))
    staged_artifacts = staging / "artifacts"
    try:
        try:
            staging.mkdir()
            private_artifacts = case_dir / "artifacts"
            if host is not None:
                # A timed-out launcher may still be writing its original download.
                private_artifacts = case_dir / ("settled-artifacts-" + str(uuid.uuid4()))
                private_artifacts.mkdir()
                deadline = host.commands.deadline
                host.commands.deadline = time.monotonic() + 300
                try:
                    host.commands.run([host.args.kubectl, "testkube", "download", "artifacts", execution_id,
                                       "--download-dir", str(private_artifacts), "--format", "folder"],
                                      env=env, private=True)
                finally:
                    host.commands.deadline = deadline
            verify_evidence_path(destination, private_artifacts)
            if not private_artifacts.is_dir():
                raise RuntimeError("TestKube artifact directory is missing")
            reports_passed = public_testkube_reports(private_artifacts, staged_artifacts)
        except (OSError, RuntimeError) as failure:
            artifact_error = failure
        if public_artifacts.exists():
            try:
                public_artifacts.rename(case_dir / ("previous-artifacts-" + str(uuid.uuid4())))
            except OSError as failure:
                artifact_error = failure
        if not public_case.exists():
            metadata = case_dir / ("metadata-" + str(uuid.uuid4()))
            metadata.mkdir()
            try:
                save(metadata / "submission.json", {"id": execution_id})
                save(metadata / "execution.json", public_execution)
                save(metadata / "record.json", record)
                public_case.parent.mkdir(parents=True, exist_ok=True)
                verify_evidence_path(destination, public_case)
                metadata.rename(public_case)
            finally:
                if metadata.exists():
                    shutil.rmtree(metadata)
        if artifact_error is None:
            try:
                verify_evidence_path(destination, public_artifacts)
                staged_artifacts.rename(public_artifacts)
                artifacts_available = True
            except OSError as failure:
                artifact_error = failure
    finally:
        if staging.exists():
            try:
                shutil.rmtree(staging)
            except OSError:
                print("Private report staging cleanup failed; partial reports remain unpublished", flush=True)
    result_path = destination / "testkube-results-ConfigTests.json"
    verify_evidence_path(destination, result_path)
    result = load(result_path)
    result["executions"] = [record]
    result["status"] = "passed" if record["status"] == "passed" and artifacts_available and reports_passed else "failed"
    result["artifact_status"] = "available" if artifacts_available else "unavailable"
    result_temp = result_path.with_name("." + result_path.name + "." + str(uuid.uuid4()))
    try:
        save(result_temp, result)
        os.replace(result_temp, result_path)
    finally:
        result_temp.unlink(missing_ok=True)
    if artifact_error is not None:
        raise RuntimeError("TestKube reports unavailable; terminal execution metadata retained") from artifact_error
    return record


def settle_workflow(host, destination, workflow, env):
    # The launcher may outlive an interrupted Popen; fence it before trusting absent markers.
    (destination / "testkube-fenced").touch()
    if not (destination / "testkube-submission-started").exists():
        return (not (destination / "testkube-mutation-started").exists()
                or (destination / "testkube-mutation-settled").exists())
    private_root = destination / ".azure" / "testkube-executions" / "ConfigTests" / workflow
    try:
        verify_evidence_path(destination, private_root)
    except ValueError as failure:
        print(f"Invalid execution evidence for {workflow}: {failure}", flush=True)
        return False
    submissions = list(private_root.glob("*/submission.json"))
    if len(submissions) != 1:
        print(f"Cannot establish whether {workflow} was submitted; refusing fixture restoration", flush=True)
        return False
    try:
        submission = submissions[0]
        verify_evidence_path(destination, submission)
        execution_id = load(submission)["id"]
        if (not re.fullmatch(r"[A-Za-z0-9][A-Za-z0-9_-]{0,127}", execution_id)
                or submission.parent.name != execution_id):
            raise ValueError("invalid execution ID")
        execution_file = submission.parent / "execution.json"
        verify_evidence_path(destination, execution_file)

        if execution_file.exists():
            try:
                if terminal_execution(load(execution_file), execution_id, workflow):
                    return True
            except (OSError, ValueError) as failure:
                print(f"Invalid execution status for {execution_id}: {failure}; attempting abort", flush=True)
        deadline = host.commands.deadline
        host.commands.deadline = time.monotonic() + 90
        try:
            try:
                host.commands.run([host.args.kubectl, "testkube", "abort", "testworkflowexecution", execution_id],
                                  env=env, private=True)
            except RuntimeError as failure:
                print(f"Unable to abort {execution_id}: {failure}; checking terminal status", flush=True)
            last_failure = None
            for attempt in range(13):
                try:
                    result = host.commands.run([host.args.kubectl, "testkube", "get", "testworkflowexecution",
                                                execution_id, "--output", "json"], env=env, json_output=True,
                                               private=True)
                    if terminal_execution(result, execution_id, workflow):
                        save(execution_file, result)
                        return True
                except (RuntimeError, ValueError, KeyError) as failure:
                    last_failure = failure
                if attempt < 12:
                    time.sleep(5)
            if last_failure:
                print(f"Unable to confirm {execution_id} stopped: {last_failure}", flush=True)
        finally:
            host.commands.deadline = deadline
    except (OSError, ValueError, KeyError, TypeError) as failure:
        print(f"Invalid execution evidence for {workflow}: {failure}", flush=True)
    print(f"TestKube execution for {workflow} is not confirmed terminal; refusing fixture restoration", flush=True)
    return False


def run_suite(host, suite, destination, expression, template="testkube-test-crs.yaml", workflow=None,
              expected_cluster=None):
    workflow = workflow or suite
    destination.mkdir(parents=True, exist_ok=False)
    source_sha = host.args.source_sha
    runner_image = os.environ.get("TEST_RUNNER_IMAGE", "")
    if not DIGEST.fullmatch(runner_image):
        raise ValueError("TEST_RUNNER_IMAGE must be an immutable ACR image digest")
    env = dict(os.environ, TEST_SOURCE_SHA=source_sha, TEST_RUNNER_IMAGE=runner_image,
               GINKGO_LABEL_FILTER=expression,
               AMW_QUERY_ENDPOINT=host.query_endpoint, AZURE_CLIENT_ID=host.query_client_id,
               BUILD_ARTIFACTSTAGINGDIRECTORY=str(destination), KUBECONFIG=str(host.kubeconfig),
               EXPECTED_CLUSTER=expected_cluster or host.cluster_name, EXPECT_OPERATOR_HTTPS="true",
               EXPECT_WINDOWS_EXPORTER=str("windows" in host.scenario["platforms"]).lower(),
               SCENARIO_RUN_ID=str(uuid.UUID(host.args.run_id)))
    paths = []
    if getattr(host.args, "tools_bin", None):
        paths.append(str(host.args.tools_bin))
    if Path(host.args.kubectl).parent != Path("."):
        paths.append(str(Path(host.args.kubectl).parent))
    env["PATH"] = os.pathsep.join([*paths, os.environ.get("PATH", "")])
    target = "scenario-" + str(uuid.uuid4()) + ".yaml"
    generated = REPO / "otelcollector/test/testkube" / target
    if generated.exists():
        raise ValueError("Refusing to overwrite an existing generated workflow")
    host.workflow_unsettled = True
    published = False
    try:
        if (not re.fullmatch(r"[a-z0-9][a-z0-9-]*", workflow)
                or not re.fullmatch(r"[A-Za-z0-9_-]+\.yaml", template)
                or not re.fullmatch(r"[A-Za-z0-9_-]+\.yaml", target)):
            raise ValueError("Invalid TestKube workflow name or filename")
        test_dir = REPO / "otelcollector/test"
        template_dir = (test_dir / "testkube/config-processing-test-crs"
                        if template.startswith("testkube-config-test-") else test_dir / "testkube")
        source = resource_path(template_dir, template, test_dir / "testkube")
        expectations = {
            name: env[name] for name in ("EXPECTED_CLUSTER", "AMW_QUERY_ENDPOINT", "AZURE_CLIENT_ID",
                                         "EXPECT_OPERATOR_HTTPS", "EXPECT_WINDOWS_EXPORTER", "SCENARIO_RUN_ID")
            if name in env
        }
        rendered = render_workflow(
            source.read_text(encoding="utf-8"), workflow, source_sha, runner_image,
            expression, expectations)
        generated.write_text(yaml.safe_dump_all([rendered], sort_keys=False), encoding="utf-8")
        result_path = destination / "testkube-results-ConfigTests.json"
        save(result_path, {
            "environment": "ConfigTests", "start_time": datetime.now().strftime("%Y-%m-%d %H:%M:%S"),
            "status": "running", "test_source_sha": source_sha,
            "runner_image": runner_image, "executions": [],
        })
        (destination / "testkube-mutation-started").touch()
        if (destination / "testkube-fenced").exists():
            raise RuntimeError("TestKube launcher was fenced by settlement")
        cwd = test_dir
        try:
            host.kube("apply", "-f", "./testkube/api-server-permissions.yaml", cwd=cwd, env=env)
            existing = host.kube("get", "-f", f"./testkube/{target}", "--ignore-not-found", "-o", "name",
                                 cwd=cwd, env=env)
            for resource in existing.splitlines():
                if resource:
                    host.kube("annotate", "--namespace", "testkube", resource,
                              "kubectl.kubernetes.io/last-applied-configuration-", cwd=cwd, env=env)
            host.kube("apply", "-f", f"./testkube/{target}", cwd=cwd, env=env)
        except RuntimeError as error:
            raise RuntimeError(f"Failed to apply TestKube workflow: {error}") from error
        (destination / "testkube-mutation-settled").touch()
        (destination / "testkube-submission-started").touch()
        # TestKube responses can contain environment values; the publisher excludes .azure.
        execution_root = destination / ".azure" / "testkube-executions" / "ConfigTests" / workflow
        verify_evidence_path(destination, execution_root)
        execution_root.mkdir(parents=True, exist_ok=True)
        runner = test_dir / "testkube" / "run-testkube-workflow.sh"
        host.commands.run(["bash", str(runner), "--run-existing", workflow, str(execution_root)],
                          cwd=cwd, env=env, private=True)
        publish_testkube_evidence(destination, workflow)
        published = True
        result = workflow_result(destination, workflow, source_sha, runner_image)
        print("TestKube execution evidence: " + json.dumps(result), flush=True)
        return result
    except (RuntimeError, ValueError) as failure:
        raise RuntimeError(
            f"TestKube suite failed: suite={suite}, workflow={workflow}, evidence={destination}: {failure}"
        ) from failure
    finally:
        try:
            host.workflow_unsettled = not settle_workflow(host, destination, workflow, env)
            if host.workflow_unsettled:
                raise RuntimeError(f"TestKube execution or setup mutation for {workflow} is not confirmed stopped")
            if not published and (destination / "testkube-submission-started").exists():
                publish_testkube_evidence(destination, workflow, host, env)
        finally:
            if generated.exists():
                generated.unlink()


def enable_otlp_settings(text, lifecycle):
    documents = list(yaml.safe_load_all(text))
    for document in documents:
        if (document and document.get("kind") == "ConfigMap"
                and document["metadata"]["name"] == lifecycle["settingsConfigMap"]
                and document["metadata"].get("namespace") == lifecycle["namespace"]):
            data = document.setdefault("data", {})
            if data.get("schema-version") == "v2":
                cluster = yaml.safe_load(data.get("cluster-metrics", "")) or {}
                cluster["opentelemetry-metrics"] = "enabled = true"
                # The product parser recognizes only literal section headers ending in ": |-".
                data["cluster-metrics"] = "\n".join(
                    f"{name}: |-\n" + "\n".join(f"  {line}" for line in value.splitlines())
                    for name, value in cluster.items())
            else:
                data["opentelemetry-metrics"] = "enabled = true"
    return yaml.safe_dump_all(documents)


def snapshot(host, identities):
    originals = {}
    for kind, name, namespace in identities:
        output = host.kube("get", kind, name, "-n", namespace, "--ignore-not-found", "-o", "json",
                           private=True)
        originals[(kind, name, namespace)] = clean_resource(json.loads(output)) if output.strip() else None
    return originals


def restore(host, originals, preserve_settings=False, settings_identity=None):
    restore_failures = []
    for (kind, name, namespace), original in reversed(list(originals.items())):
        if kind == "Namespace" and original is not None:
            continue
        try:
            if original is None:
                for attempt in range(4):
                    try:
                        host.kube("delete", kind, name, "-n", namespace, "--ignore-not-found")
                        break
                    except RuntimeError as failure:
                        if "ServiceUnavailable" not in str(failure) or attempt == 3:
                            raise
                        delay = 2 ** (attempt + 1)
                        print(f"Retrying settings-slot cleanup after Kubernetes ServiceUnavailable in {delay}s",
                              flush=True)
                        time.sleep(delay)
            elif preserve_settings and (kind, name, namespace) == settings_identity:
                current = host.kube("get", kind, name, "-n", namespace,
                                    "-o", "json", json_output=True, private=True)
                original_with_version = copy.deepcopy(original)
                original_with_version["metadata"]["resourceVersion"] = current["metadata"]["resourceVersion"]
                host.kube("replace", "-f", "-", data=json.dumps(original_with_version), private=True)
            else:
                host.kube("replace", "--force", "-f", "-", data=json.dumps(original), private=True)
            readback = host.kube("get", kind, name, "-n", namespace, "--ignore-not-found",
                                 "-o", "json", private=True)
            actual = clean_resource(json.loads(readback)) if readback.strip() else None
            if actual != original:
                raise RuntimeError(f"Baseline restoration mismatch: {kind}/{name}")
        except (RuntimeError, ValueError, OSError, KeyError) as restore_failure:
            restore_failures.append((kind, name, restore_failure))
    if restore_failures:
        details = "; ".join(f"{kind}/{name}: {failure}" for kind, name, failure in restore_failures)
        raise RuntimeError(f"Restoration failed; no next variant allowed: {details}") from restore_failures[0][2]


def restart_collectors(host):
    resources = ["deployment/ama-metrics", "daemonset/ama-metrics-node",
                 "deployment/ama-metrics-operator-targets"]
    if "windows" in host.scenario["platforms"]:
        resources.append("daemonset/ama-metrics-win-node")
    for resource in resources:
        host.kube("rollout", "restart", resource, "-n", "kube-system")
    for resource in resources:
        timeout = "20m" if resource == "daemonset/ama-metrics-win-node" else "15m"
        host.kube("rollout", "status", resource, "-n", "kube-system", f"--timeout={timeout}")


def settings_slot(host, variant, lifecycle):
    if variant["addonParams"]:
        raise ValueError("Nonempty addonParams require an explicit addon-to-test-chart mapping")
    template, workflow = settings_workflow(variant)
    expression = label_filter(host.scenario, variant)
    save(host.args.evidence / variant["id"] / "selection.json", {
        "template": template, "workflow": workflow, "ginkgoLabelFilter": expression,
        "overrideWorkflowLabels": variant.get("overrideWorkflowLabels", False),
    })
    sources = {path: path.read_text(encoding="utf-8-sig")
               for path in variant["configmaps"] + variant["customResources"]}
    preserve_settings = {"ampls", "http-proxy"} <= set(host.scenario["clusterFeatures"])
    settings_identity = ("ConfigMap", lifecycle["settingsConfigMap"], lifecycle["namespace"])
    if preserve_settings:
        sources = {path: enable_otlp_settings(text, lifecycle) for path, text in sources.items()}

    def replace_settings(document):
        current = host.kube("get", *settings_identity[:2], "-n", settings_identity[2],
                            "-o", "json", json_output=True, private=True)
        document["metadata"]["resourceVersion"] = current["metadata"]["resourceVersion"]
        host.kube("replace", "-f", "-", data=json.dumps(document), private=True)
    documents = [doc for text in sources.values() for doc in yaml.safe_load_all(text) if doc]
    save(host.args.evidence / variant["id"] / "applied-resources.json", [
        {"source": str(path), "appliedTextSha256": hashlib.sha256(text.encode()).hexdigest()}
        for path, text in sources.items()
    ])
    identities = [("ConfigMap", name, lifecycle["namespace"]) for name in lifecycle["resetConfigMaps"]]
    for doc in documents:
        identity = (doc["kind"], doc["metadata"]["name"],
                    doc["metadata"].get("namespace", "default"))
        if identity not in identities:
            identities.append(identity)
    originals = snapshot(host, identities)
    for (kind, name, _), original in originals.items():
        if kind == "Namespace" and original is not None:
            raise RuntimeError(f"Fixture namespace {name} already exists; refusing to reuse it")
    if preserve_settings and originals[settings_identity] is None:
        raise RuntimeError("AMPLS-proxy settings baseline is missing; refusing settings transition")
    # Set before the first mutation so an interrupt anywhere below cannot release the hold.
    host.cleanup_failed = True
    error = None
    try:
        for name in lifecycle["resetConfigMaps"]:
            if preserve_settings and name == settings_identity[1]:
                continue
            host.kube("delete", "configmap", name, "-n", lifecycle["namespace"], "--ignore-not-found")
        for text in sources.values():
            resources = [doc for doc in yaml.safe_load_all(text) if doc]
            if preserve_settings:
                for doc in list(resources):
                    if (doc["kind"], doc["metadata"]["name"], doc["metadata"].get("namespace")) == settings_identity:
                        replace_settings(doc)
                        resources.remove(doc)
            if resources:
                host.kube("apply", "-f", "-", data=yaml.safe_dump_all(resources, sort_keys=False), private=True)
        restart_collectors(host)
        result = run_suite(host, "configprocessing",
                           host.args.evidence / variant["id"] / "configprocessing",
                           expression, template=template, workflow=workflow)
        if set(variant["ginkgoLabels"]) & {"config-processing-customer-settings-v1",
                                         "config-processing-customer-settings-v2"}:
            query_labels = "cluster-identity"
            if {"managed-acns", "cilium"} <= set(host.scenario["clusterFeatures"]):
                query_labels += " || managed-acns-cilium"
            result["aliasIngestion"] = run_suite(
                host, "querymetrics", host.args.evidence / variant["id"] / "alias-ingestion",
                query_labels, expected_cluster="Customer_Config_POC",
            )
        if variant["id"] == "collector-controls":
            result["ksmAllowlist"] = run_suite(
                host, "querymetrics", host.args.evidence / variant["id"] / "ksm-allowlist",
                "ksm-allowlist",
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
        run_deadline = host.commands.deadline
        host.commands.deadline = time.monotonic() + 600
        try:
            if getattr(host, "workflow_unsettled", False):
                raise RuntimeError("TestKube execution not confirmed stopped; fixture restoration is unsafe")
            cleanup_failure = None
            if "config-processing-scenario-deny-fs-access" in variant["ginkgoLabels"]:
                try:
                    host.kube("delete", "servicemonitors.azmonitoring.coreos.com,podmonitors.azmonitoring.coreos.com",
                              "-n", "default", "-l",
                              f"ama-metrics.azure.com/scenario-run={uuid.UUID(host.args.run_id)}",
                              "--ignore-not-found", "--wait=true", "--timeout=5m")
                except (RuntimeError, ValueError, OSError, KeyError) as failure:
                    cleanup_failure = failure
            try:
                restore(host, originals, preserve_settings,
                        ("ConfigMap", lifecycle["settingsConfigMap"], lifecycle["namespace"]))
            except (RuntimeError, ValueError, OSError, KeyError) as failure:
                if cleanup_failure:
                    raise RuntimeError(f"Monitor cleanup failed: {cleanup_failure}; restoration failed: {failure}") from failure
                raise
            if cleanup_failure:
                raise RuntimeError(f"Monitor cleanup failed: {cleanup_failure}") from cleanup_failure
            host.cleanup_failed = False
        except (RuntimeError, ValueError, OSError, KeyError) as restore_failure:
            raise RuntimeError(f"Original: {error}; {restore_failure}") from restore_failure
        finally:
            host.commands.deadline = run_deadline


def render_chart(commands, evidence, outputs, source_sha, mapping, container_roles, otlp=False):
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
    if otlp:
        values["AppmonitoringAgent"]["isOpenTelemetryMetricsEnabled"] = True
    (rendered / "values.yaml").write_text(yaml.safe_dump(values, sort_keys=False))
    raw = commands.run(["helm", "template", "ama-metrics", str(rendered),
                        "--namespace", "kube-system"], private=True)
    documents = pin_workloads(list(yaml.safe_load_all(raw)), mapping, container_roles)
    if outputs.get("proxyEndpoint"):
        configure_proxy(documents, outputs)
    target = evidence / "rendered-chart"
    (target / "templates").mkdir(parents=True)
    (target / "files").mkdir()
    (target / "Chart.yaml").write_text(yaml.safe_dump(chart))
    (target / "files/manifests.yaml").write_text(yaml.safe_dump_all(documents))
    (target / "templates/resources.yaml").write_text('{{ .Files.Get "files/manifests.yaml" }}\n')
    return target


def configure_ampls_cluster_proxy(host, outputs):
    group = host.receipt["resourceGroup"]
    cluster = outputs["clusterName"]
    profile = host.az("aks", "show", "--resource-group", group, "--name", cluster,
                      json_output=True, private=True).get("httpProxyConfig") or {}
    if profile.get("enabled") is False or profile.get("httpProxy") != outputs["proxyEndpoint"] or \
            profile.get("httpsProxy") != outputs["proxyEndpoint"] or not isinstance(profile.get("noProxy"), list):
        raise ValueError("Retained cluster proxy configuration does not match the owned proxy")
    endpoints = []
    for key in ("dceConfigurationEndpoint", "dceMetricsIngestionEndpoint", "queryEndpoint"):
        parsed = urlsplit(outputs[key])
        if parsed.scheme != "https" or not parsed.hostname or not re.fullmatch(r"[a-zA-Z0-9.-]+", parsed.hostname):
            raise ValueError(f"Invalid private monitoring endpoint: {key}")
        endpoints.append(parsed.hostname)
    if not endpoints[0].endswith(".handler.control.monitor.azure.com"):
        raise ValueError("Unexpected AMCS configuration endpoint")
    bypass = list(dict.fromkeys([*profile["noProxy"], "handler.control.monitor.azure.com",
                                ".handler.control.monitor.azure.com", *endpoints]))
    if bypass != profile["noProxy"]:
        config = {key: profile[key] for key in ("httpProxy", "httpsProxy", "trustedCa") if profile.get(key)}
        config["noProxy"] = bypass
        with tempfile.TemporaryDirectory(prefix="ama-proxy-config-") as directory:
            path = Path(directory) / "proxy.json"
            save(path, config)
            host.az("aks", "update", "--resource-group", group, "--name", cluster,
                    "--http-proxy-config", str(path), private=True)
        profile = host.az("aks", "show", "--resource-group", group, "--name", cluster,
                          json_output=True, private=True).get("httpProxyConfig") or {}
    if not set(bypass).issubset(set(profile.get("noProxy") or [])):
        raise RuntimeError("AKS did not retain the required private endpoint proxy exclusions")
    outputs["proxyNoProxy"] = profile["noProxy"]
    outputs["proxyNoProxyCsv"] = ",".join(profile["noProxy"])
    save(host.args.evidence / "deployment.json", outputs)


def configure_proxy(documents, outputs):
    endpoint = outputs["proxyEndpoint"]
    exclusions = outputs["proxyNoProxyCsv"]
    if endpoint != "http://10.226.0.4:3128" or not exclusions or "*" in exclusions:
        raise ValueError("Proxy adapter requires the owned endpoint and scoped NO_PROXY")
    certificate = outputs.get("proxyTrustedCa")
    if certificate:
        certificate = base64.b64decode(certificate, validate=True).decode("ascii")
        if not certificate.startswith("-----BEGIN CERTIFICATE-----"):
            raise ValueError("Proxy trust must be a public PEM certificate")
        documents.append({
            "apiVersion": "v1", "kind": "Secret",
            "metadata": {"name": "scenario-proxy-ca", "namespace": "kube-system"},
            "stringData": {"proxy-cert.crt": certificate},
        })
        documents.append({
            "apiVersion": "v1", "kind": "ConfigMap",
            "metadata": {"name": "scenario-me-config-overrides", "namespace": "kube-system"},
            "data": {
                name: json.dumps(dict(load(REPO / "otelcollector/metricextension" / name),
                                      validateCertificates=2))
                for name in ("me.config", "me_ds.config")
            },
        })
    collectors = 0
    for document in documents:
        if document.get("kind") not in {"Deployment", "DaemonSet"}:
            continue
        spec = document["spec"]["template"]["spec"]
        for container in spec["containers"]:
            if container["name"] != "prometheus-collector":
                continue
            collectors += 1
            environment = {
                "HTTP_PROXY": endpoint, "http_proxy": endpoint,
                "HTTPS_PROXY": endpoint, "https_proxy": endpoint,
                "NO_PROXY": exclusions, "no_proxy": exclusions,
            }
            container["env"] = [entry for entry in container.get("env", [])
                                if entry["name"] not in environment]
            container["env"].extend({"name": name, "value": value}
                                    for name, value in environment.items())
            if certificate:
                config_file = "me.config" if document["kind"] == "Deployment" else "me_ds.config"
                container.setdefault("volumeMounts", []).extend([
                    {"name": "scenario-proxy-ca", "mountPath": "/anchors/proxy", "readOnly": True},
                    {"name": "scenario-me-config-overrides", "mountPath": f"/usr/sbin/{config_file}",
                     "subPath": config_file, "readOnly": True},
                ])
        if certificate and any(c["name"] == "prometheus-collector" for c in spec["containers"]):
            spec.setdefault("volumes", []).extend([
                {"name": "scenario-proxy-ca", "secret": {"secretName": "scenario-proxy-ca"}},
                {"name": "scenario-me-config-overrides",
                 "configMap": {"name": "scenario-me-config-overrides"}},
            ])
    if collectors < 2:
        raise ValueError("Proxy adapter did not find both replica and node collectors")


def proxy_routes(log, started, collector_ips, ingestion_hostname, bypass_ips,
                 require_post=False, forbid_post=False):
    if require_post and forbid_post:
        raise ValueError("Proxy route proof cannot require and forbid DCE POSTs")
    connects, posts = [], []
    for line in log.splitlines():
        fields = line.split()
        if len(fields) != 6:
            raise RuntimeError("Unexpected owned Squid route log format")
        timestamp, source, method, destination, status, hierarchy = fields
        if float(timestamp) < started or source not in collector_ips:
            continue
        if destination in bypass_ips:
            raise RuntimeError("NO_PROXY target appeared in the collector's proxy route log")
        if destination != ingestion_hostname:
            continue
        if method == "POST":
            if forbid_post:
                raise RuntimeError("DCE POST reached the proxy during CA distrust")
            if status == "200" and hierarchy.startswith("HIER_DIRECT/"):
                posts.append({"timestamp": float(timestamp), "source": source,
                              "destination": destination, "method": method, "status": int(status)})
        elif (method == "CONNECT" and status == "200"
              and hierarchy.startswith("HIER_DIRECT/")):
            connects.append({"timestamp": float(timestamp), "source": source,
                             "destination": destination, "method": method, "status": int(status)})
    routes = posts if require_post else connects if forbid_post else connects + posts
    if not routes:
        raise RuntimeError("No matching fresh collector-to-ingestion route in the owned proxy log")
    return routes


def wait_for_proxy_routes(read_log, started, collector_ips, ingestion_hostname, bypass_ips,
                          require_post=False, forbid_post=False):
    deadline = time.monotonic() + 120
    while True:
        try:
            return proxy_routes(read_log(), started, collector_ips, ingestion_hostname, bypass_ips,
                                require_post=require_post, forbid_post=forbid_post)
        except RuntimeError as error:
            if str(error) != "No matching fresh collector-to-ingestion route in the owned proxy log":
                raise
            remaining = deadline - time.monotonic()
            if remaining <= 0:
                raise
            time.sleep(min(5, remaining))


def verify_me_configuration(host, proof, started):
    immutable_id = proof["immutableDcrId"]
    if not re.fullmatch(r"dcr-[0-9a-f]{32}", immutable_id):
        raise RuntimeError("ME configuration proof requires the independently verified immutable DCR ID")
    try:
        pods = host.kube("get", "pods", "-n", "kube-system", "-o", "json",
                         json_output=True, private=True)["items"]
    except RuntimeError as failure:
        raise RuntimeError(f"AMPLS_ME_CONFIGURATION stage=pod-inventory: {failure}") from failure
    observed = []
    for pod in pods:
        if not any(c["name"] == "prometheus-collector" for c in pod["spec"]["containers"]):
            continue
        status = next(c for c in pod["status"]["containerStatuses"] if c["name"] == "prometheus-collector")
        container_started = datetime.fromisoformat(
            status["state"]["running"]["startedAt"].replace("Z", "+00:00")).timestamp()
        if not status["ready"] or container_started < started:
            raise RuntimeError("ME configuration proof requires a fresh ready collector")
        command = r'''set -eu
enabled=false; mdsd=0; me=0; localControl=0
for proc in /proc/[0-9]*; do
  name=$(cat "$proc/comm" 2>/dev/null) || continue
  case "$name" in
    mdsd) mdsd=$((mdsd + 1));;
    MetricsExtensio*)
      me=$((me + 1))
      if tr '\0' '\n' < "$proc/environ" | grep -qx 'AZMON_FULL_OTLP_ENABLED=true'; then
        enabled=true
      fi
      if tr '\0' '\n' < "$proc/cmdline" | grep -qx -- '-LocalControlChannel'; then
        localControl=$((localControl + 1))
      fi;;
  esac
done
printf '%s\n' "$enabled"
printf 'mdsd=%s me=%s localControl=%s\n' "$mdsd" "$me" "$localControl"
if test -s "/etc/mdsd.d/config-cache/me/dcrs/$1"; then
  stat -c '%Y %s' "/etc/mdsd.d/config-cache/me/dcrs/$1"
else
  printf 'configuration-missing\n'
fi
'''
        for attempt in range(60):
            try:
                lines = host.kube("exec", pod["metadata"]["name"], "-n", "kube-system",
                                  "-c", "prometheus-collector", "--", "sh", "-c", command,
                                  "me-configuration-proof", immutable_id, private=True).splitlines()
            except RuntimeError as failure:
                raise RuntimeError(
                    f"AMPLS_ME_CONFIGURATION stage=collector-exec pod={pod['metadata']['name']}: {failure}"
                ) from failure
            mode = (re.fullmatch(r"mdsd=(\d+) me=(\d+) localControl=(\d+)", lines[1])
                    if len(lines) > 1 and lines[0] in {"true", "false"} else None)
            if len(lines) != 3 or not mode:
                break
            if lines[0] == "true" and mode.groups() == ("0", "1", "0") \
                    and lines[2] != "configuration-missing":
                break
            starting = mode.groups() == ("0", "0", "0")
            waiting_for_configuration = (lines[0] == "true" and mode.groups() == ("0", "1", "0")
                                         and lines[2] == "configuration-missing")
            if not starting and not waiting_for_configuration:
                break
            if attempt < 59:
                time.sleep(5)
        if len(lines) != 3 or lines[:2] != ["true", "mdsd=0 me=1 localControl=0"]:
            detail = (f"otlpEnabled={lines[0]} mdsd={mode[1]} me={mode[2]} localControl={mode[3]}"
                      if mode else f"malformed proof output lines={len(lines)}")
            raise RuntimeError(
                f"AMPLS_ME_CONFIGURATION stage=mode-check pod={pod['metadata']['name']}: {detail}"
            )
        if lines[2] == "configuration-missing":
            raise RuntimeError("ME did not download the owned DCR configuration within five minutes")
        modified, size = map(int, lines[2].split())
        if modified < int(container_started) or size <= 0:
            raise RuntimeError("ME configuration cache must be freshly downloaded, not inherited")
        item = {"pod": pod["metadata"]["name"], "uid": pod["metadata"]["uid"],
                "configurationContainerId": status.get("containerID"), "containerStarted": container_started,
                "configurationModified": modified, "configurationBytes": size,
                "mdsdRunning": False, "meLocalControl": False}
        observed.append(item)
    if len(observed) < 2:
        raise RuntimeError("ME configuration proof requires replica and node collectors")
    result = {"status": "PASS", "immutableDcrId": immutable_id, "collectors": observed}
    print("AMPLS_ME_CONFIGURATION " + json.dumps(result), flush=True)
    return result


def network_probe_monitor(host, marker):
    monitor = {
        "apiVersion": "azmonitoring.coreos.com/v1", "kind": "ServiceMonitor",
        "metadata": {"name": host.network_probe_name, "namespace": "default"},
        "spec": {"selector": {"matchLabels": {"app": "prometheus-reference-app"}},
                 "endpoints": [{"port": "weather-app", "interval": "15s",
                                "proxyFromEnvironment": True,
                                "metricRelabelings": [
                                    {"action": "keep", "sourceLabels": ["__name__"], "regex": "myapp_temperature"},
                                    {"action": "replace", "targetLabel": "scenario_run", "replacement": marker},
                                ]}]},
    }
    resource = "servicemonitors.azmonitoring.coreos.com"
    existing = host.kube("get", resource, host.network_probe_name, "-n", "default",
                         "--ignore-not-found", "-o", "name").strip()
    if existing and not host.created_network_probe_monitor:
        raise RuntimeError("Run-specific proxy-from-environment scrape monitor already exists; refusing to overwrite")
    if host.created_network_probe_monitor:
        host.kube("apply", "-f", "-", data=yaml.safe_dump(monitor))
    else:
        host.kube("create", "-f", "-", data=yaml.safe_dump(monitor))
        host.created_network_probe_monitor = True
    observed = host.kube("get", resource, host.network_probe_name, "-n", "default",
                         "-o", "json", json_output=True, private=True)
    if observed["spec"] != monitor["spec"]:
        raise RuntimeError("Run-specific proxy-from-environment scrape monitor did not read back")


def verify_proxy(host, outputs, started, phase="default", ampls_proof=None):
    if phase not in {"default", "trusted-ca", "untrusted-ca", "recovered-ca"}:
        raise ValueError("Unknown proxy proof phase")
    if ampls_proof is not None and phase != "default":
        raise ValueError("AMPLS public-dependency proof cannot replace a custom-CA phase")
    pods = host.kube("get", "pods", "-n", "kube-system", "-o", "json",
                     json_output=True, private=True)["items"]
    sources = set()
    for pod in pods:
        if not any(c["name"] == "prometheus-collector" for c in pod["spec"]["containers"]):
            continue
        environment = host.kube(
            "exec", pod["metadata"]["name"], "-n", "kube-system", "-c", "prometheus-collector",
            "--", "sh", "-c", 'printf "%s\\n%s\\n%s\\n" "$HTTP_PROXY" "$HTTPS_PROXY" "$NO_PROXY"',
            private=True)
        expected = [outputs["proxyEndpoint"], outputs["proxyEndpoint"], outputs["proxyNoProxyCsv"]]
        if environment.splitlines() != expected:
            raise RuntimeError("Running collector does not have the explicit proxy/NO_PROXY environment")
        sources.update((pod["status"]["podIP"], pod["status"]["hostIP"]))
    if not sources:
        raise RuntimeError("No running collector source addresses for proxy proof")
    references = host.kube("get", "pods", "-n", "default", "-l", "app=prometheus-reference-app",
                           "-o", "json", json_output=True, private=True)["items"]
    bypass = {pod["status"]["podIP"] for pod in references}
    if not bypass:
        raise RuntimeError("No reference scrape endpoint for NO_PROXY proof")
    endpoint = outputs["dceMetricsIngestionEndpoint"]
    if "ampls" in host.scenario["clusterFeatures"]:
        if ampls_proof is not None:
            bypass.update(ip for addresses in ampls_proof["endpointAddresses"].values() for ip in addresses)
            hosts = ampls_proof["hosts"]
        else:
            hosts = {
                "configuration": urlsplit(outputs["dceConfigurationEndpoint"]).hostname,
                "ingestion": urlsplit(outputs["dceMetricsIngestionEndpoint"]).hostname,
                "query": urlsplit(outputs["queryEndpoint"]).hostname,
            }
        bypass.update(host for host in hosts.values() if host)
        endpoint = "https://management.azure.com/metadata/endpoints?api-version=2020-06-01"
        host.kube("exec", "deployment/ama-metrics", "-n", "kube-system", "-c", "prometheus-collector",
                  "--", "curl", "--fail", "--silent", "--show-error", "--connect-timeout", "10",
                  "--max-time", "30", "--output", "/dev/null", endpoint)
    if phase in {"default", "trusted-ca"}:
        for attempt in range(3):
            try:
                host.kube("exec", "deployment/ama-metrics", "-n", "kube-system", "-c", "prometheus-collector",
                          "--", "sh", "-c",
                          'set +e; curl --noproxy "*" --connect-timeout 10 --max-time 15 -sS -o /dev/null "$1"; '
                          'result=$?; test "$result" -eq 7 || test "$result" -eq 28',
                          "direct-egress-denial", endpoint)
                break
            except RuntimeError as error:
                if not str(error).startswith("Command failed (137):") or attempt == 2:
                    raise
                print("Retrying interrupted direct-egress denial probe after exec exit 137 in 5s", flush=True)
                time.sleep(5)
    require_post = phase in {"trusted-ca", "recovered-ca"}
    forbid_post = phase == "untrusted-ca"
    script = ("python3 - <<'PY'\nimport json, time\nfrom pathlib import Path\n"
              + inspect.getsource(proxy_routes) + "\n" + inspect.getsource(wait_for_proxy_routes)
              + "\ntry:\n"
              + "    routes = wait_for_proxy_routes(lambda: Path('/var/log/squid/access.log').read_text(), "
              + f"{started!r}, "
              + f"{sources!r}, {urlparse(endpoint).hostname!r}, {bypass!r}, "
              + f"require_post={require_post!r}, forbid_post={forbid_post!r})\n"
              + "    result = {'status': 'PASS', 'successfulRequests': len(routes), "
              + "'first': routes[0], 'last': routes[-1], 'noProxyRoutes': 0}\n"
              + "except (RuntimeError, ValueError) as error:\n"
              + "    result = {'status': 'FAIL', 'error': str(error)}\n"
              + "print('SCENARIO_RESULT=' + json.dumps(result))\nPY")
    result = vm_script(host, outputs["proxyVmName"], script)
    if result.get("status") != "PASS":
        raise RuntimeError("Collector proxy route proof failed: " + result.get("error", "missing result"))
    result.update(phase=phase, directWebDenied=phase in {"default", "trusted-ca"},
                  scrapeProxyFromEnvironment=True,
                  bypassTargets=sorted(bypass), logSource="complete owned Squid access.log")
    result["publicDependencyHostname" if "ampls" in host.scenario["clusterFeatures"]
           else "ingestionHostname"] = urlparse(endpoint).hostname
    name = "proxy-proof.json" if phase == "default" else f"proxy-{phase}-proof.json"
    save(host.args.evidence / name, result)
    print(f"Collector {phase} proxy proof: " + json.dumps(result), flush=True)
    return result


def audit_images(host, images, image_config):
    deadline = time.monotonic() + 60
    while True:
        nodes = host.kube("get", "nodes", "-o", "json", json_output=True, private=True)["items"]
        node_info = {n["metadata"]["name"]: n["status"]["nodeInfo"] for n in nodes}
        pods = host.kube("get", "pods", "-n", "kube-system", "-o", "json",
                         json_output=True, private=True)["items"]
        evidence = []
        found = set()
        pending = None
        for pod in pods:
            node = pod["spec"].get("nodeName")
            if node not in node_info:
                continue
            info = node_info[node]
            statuses = {c["name"]: c for c in pod.get("status", {}).get("containerStatuses", [])}
            for container in pod["spec"]["containers"]:
                component = image_config["containerRoles"].get(container["name"])
                if component is None:
                    continue
                candidates = [i for i in images if i["name"] == component
                              and i["os"] == info["operatingSystem"] and i["arch"] == info["architecture"]]
                if info["operatingSystem"] == "windows":
                    candidates = [i for i in candidates if i.get("osVersion")
                                  and windows_image_compatible(info["kernelVersion"], i["osVersion"])]
                matching = [i for i in candidates if container["image"] in {i["ref"], i.get("manifestRef")}]
                status = statuses.get(container["name"], {})
                runtime = status.get("imageID", "")
                runtime_matches = any(
                    any(runtime.endswith(ref.split("@")[1]) for ref in (i["ref"], i.get("manifestRef")) if ref)
                    for i in matching)
                identity = f"{pod['metadata']['name']}/{container['name']}"
                if not candidates or not matching or (runtime and not runtime_matches):
                    raise RuntimeError(f"Missing, unready or wrong build imageID: {identity}")
                if not status.get("ready"):
                    pending = pending or identity
                    continue
                if not runtime_matches:
                    raise RuntimeError(f"Missing, unready or wrong build imageID: {identity}")
                found.add((component, info["operatingSystem"], node))
                evidence.append({"pod": pod["metadata"]["name"], "node": node, "component": component,
                                 "os": info["operatingSystem"], "arch": info["architecture"],
                                 "kernel": info["kernelVersion"], "image": container["image"],
                                 "imageID": runtime})
        if pending:
            if time.monotonic() >= deadline:
                raise RuntimeError(f"Missing, unready or wrong build imageID: {pending}")
            time.sleep(2)
            continue
        for node, info in node_info.items():
            system = info["operatingSystem"].lower()
            for component in image_config["requiredOnEveryNode"].get(system, []):
                if (component, info["operatingSystem"], node) not in found:
                    raise RuntimeError(f"No verified build image for {component} on node {node}")
        for component in image_config["requiredInCluster"]:
            if not any(row["component"] == component for row in evidence):
                raise RuntimeError(f"No verified build image for {component}")
        if not node_info:
            raise RuntimeError("No nodes observed")
        return evidence


def verify_pool_configuration(expected, actual):
    observed = {pool["name"]: {key.casefold(): value for key, value in pool.items()} for pool in actual}
    if set(observed) != {pool["name"] for pool in expected}:
        raise RuntimeError("Deployed pool names do not match the compiled inventory")
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
    return observed


def verify_pool_inventory(expected, actual, nodes):
    observed = verify_pool_configuration(expected, actual)
    evidence = []
    for pool in expected:
        row = observed[pool["name"]]
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


def mark_health_monitors(monitors, marker):
    marked = 0
    for monitor in monitors:
        if not monitor or monitor.get("kind") not in {"PodMonitor", "ServiceMonitor"}:
            continue
        for field in ("endpoints", "podMetricsEndpoints"):
            for endpoint in monitor["spec"].get(field, []):
                rules = endpoint.get("metricRelabelings", [])
                keep = [rule for rule in rules if rule.get("action") == "keep"
                        and rule.get("sourceLabels") == ["__name__"] and rule.get("regex") == "up"]
                if len(keep) != 1:
                    raise ValueError("Reference monitor must keep up before adding the fresh ingestion metric")
                keep[0]["regex"] = "up|myapp_temperature"
                rules.append({"action": "replace", "targetLabel": "scenario_run", "replacement": marker})
                marked += 1
    if not marked:
        raise ValueError("No reference-monitor endpoint was marked for fresh ingestion proof")
    return monitors


def verify_ingestion(host, outputs, started):
    token = host.commands.run(["az", "account", "get-access-token", "--resource",
                               "https://prometheus.monitor.azure.com", "--query", "accessToken",
                               "-o", "tsv"], private=True).strip()
    if not token:
        raise RuntimeError("Azure CLI returned no AMW query token")
    marker = host.args.run_id
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
                        "queryOrigin": "Linux collector pod", "endpoint": outputs["queryEndpoint"]}
            save(host.args.evidence / "ingestion.json", evidence)
            print("Collector ingestion proof: " + json.dumps(evidence), flush=True)
            return evidence
        except RuntimeError as failure:
            last_error = str(failure)
            if host.commands.deadline is not None and time.monotonic() + 10 >= host.commands.deadline:
                break
            if attempt < 29:
                time.sleep(10)
    raise RuntimeError(f"Fresh collector ingestion was not verified: {last_error}")


def initialize_ampls(host, outputs, enabled):
    if host.reused:
        return
    # AKS CLI disable deletes DCRs/DCRAs still owned by this private-link fixture.
    if enabled is True:
        host.az("resource", "update", "--ids", outputs["clusterId"], "--api-version", "2024-10-01",
                "--set", "properties.azureMonitorProfile.metrics.enabled=false")
    host.az("resource", "update", "--ids", outputs["dceId"], "--api-version", "2023-03-11",
            "--set", "properties.networkAcls.publicNetworkAccess=Disabled")
    for association, field, target in (
            ("ruleAssociationId", "dataCollectionRuleId", "dcrId"),
            ("configurationAssociationId", "dataCollectionEndpointId", "dceId")):
        host.az("rest", "--method", "put", "--url",
                f"https://management.azure.com{outputs[association]}?api-version=2023-03-11",
                "--body", json.dumps({"properties": {field: outputs[target]}}))


def verify_ampls_control_plane(host, outputs):
    def get(resource, version):
        return host.az("rest", "--method", "get", "--url",
                       f"https://management.azure.com{resource}?api-version={version}",
                       json_output=True)["properties"]

    def same(actual, expected):
        return isinstance(actual, str) and actual.casefold() == expected.casefold()

    dce = get(outputs["dceId"], "2023-03-11")
    dcr = get(outputs["dcrId"], "2023-03-11")
    rule = get(outputs["ruleAssociationId"], "2023-03-11")
    configuration = get(outputs["configurationAssociationId"], "2023-03-11")
    workspace = get(outputs["workspaceId"], "2023-04-03")
    if (not same(rule.get("dataCollectionRuleId"), outputs["dcrId"])
            or not same(configuration.get("dataCollectionEndpointId"), outputs["dceId"])
            or not same(dcr.get("dataCollectionEndpointId"), outputs["dceId"])
            or not same(workspace.get("defaultIngestionSettings", {}).get(
                "dataCollectionEndpointResourceId"), outputs["workspaceDefaultDceId"])
            or workspace.get("publicNetworkAccess") != "Disabled"
            or dce.get("networkAcls", {}).get("publicNetworkAccess") != "Disabled"):
        raise RuntimeError("AMPLS requires the exact DCR/DCE associations and disabled public access")
    destinations = dcr.get("destinations", {}).get("monitoringAccounts", [])
    names = {row["name"] for row in destinations
             if same(row.get("accountResourceId"), outputs["workspaceId"])}
    if not any("Microsoft-PrometheusMetrics" in row.get("streams", [])
               and set(row.get("destinations", [])) & names for row in dcr.get("dataFlows", [])):
        raise RuntimeError("AMPLS DCR must route Prometheus metrics to this workspace")
    scope = get(outputs["amplsId"], "2021-07-01-preview")
    modes = scope.get("accessModeSettings", {})
    if (modes.get("ingestionAccessMode") != "PrivateOnly"
            or modes.get("queryAccessMode") != "PrivateOnly" or modes.get("exclusions")):
        raise RuntimeError("AMPLS ingestion and query must be PrivateOnly without exclusions")
    for name, resource in (("cluster-dce", outputs["dceId"]),
                           ("workspace-dce", outputs["workspaceDefaultDceId"])):
        linked = get(outputs["amplsId"] + "/scopedResources/" + name, "2021-07-01-preview")
        if not same(linked.get("linkedResourceId"), resource):
            raise RuntimeError("AMPLS is missing an independently verified scoped DCE")
    endpoints = {}
    for kind, resource, target, group in (
            ("monitor", outputs["monitorPrivateEndpointId"], outputs["amplsId"], "azuremonitor"),
            ("query", outputs["queryPrivateEndpointId"], outputs["workspaceId"], "prometheusMetrics")):
        endpoint = get(resource, "2024-05-01")
        connections = endpoint.get("privateLinkServiceConnections", [])
        if (endpoint.get("provisioningState") != "Succeeded" or len(connections) != 1
                or not same(connections[0]["properties"].get("privateLinkServiceId"), target)
                or connections[0]["properties"].get("groupIds") != [group]
                or connections[0]["properties"].get(
                    "privateLinkServiceConnectionState", {}).get("status") != "Approved"
                or not same(endpoint.get("subnet", {}).get("id"),
                            outputs["vnetId"] + "/subnets/endpoints")):
            raise RuntimeError(f"AMPLS {kind} endpoint must be approved on the owned VNet")
        addresses = {
            row["properties"]["privateIPAddress"]
            for nic in endpoint.get("networkInterfaces", [])
            for row in get(nic["id"], "2024-05-01").get("ipConfigurations", [])
        }
        if not addresses or any(ipaddress.ip_address(ip) not in ipaddress.ip_network("10.225.0.0/24")
                                for ip in addresses):
            raise RuntimeError(f"AMPLS {kind} endpoint has no owned private NIC addresses")
        endpoints[kind] = sorted(addresses)
    return {
        "status": "verified", "dcrId": outputs["dcrId"], "dceId": outputs["dceId"],
        "ruleAssociationId": outputs["ruleAssociationId"],
        "configurationAssociationId": outputs["configurationAssociationId"],
        "accessModeSettings": modes, "endpointAddresses": endpoints,
        "hosts": {
            "configuration": urlsplit(dce["configurationAccess"]["endpoint"]).hostname,
            "ingestion": urlsplit(dce["metricsIngestion"]["endpoint"]).hostname,
            "query": urlsplit(outputs["queryEndpoint"]).hostname,
        },
        "immutableDcrId": dcr["immutableId"],
    }


def verify_ampls_private_dns(host, proof):
    pods = host.kube("get", "pods", "-n", "kube-system", "-o", "json",
                     json_output=True, private=True)["items"]
    selected = {}
    for pod in pods:
        labels = pod["metadata"].get("labels", {})
        kind = ("linux" if labels.get("rsName") == "ama-metrics"
                else "windows" if labels.get("dsName") == "ama-metrics-win-node" else None)
        if kind and pod.get("status", {}).get("phase") == "Running":
            selected.setdefault(kind, pod["metadata"]["name"])
    if set(selected) != set(host.scenario["platforms"]):
        raise RuntimeError("AMPLS DNS proof requires a running collector on each selected platform")
    observed = {}
    for platform, pod in selected.items():
        observed[platform] = {}
        for kind, hostname in proof["hosts"].items():
            if not hostname or not re.fullmatch(r"[a-zA-Z0-9.-]+", hostname):
                raise RuntimeError("AMPLS endpoint hostname is missing or invalid")
            if platform == "windows":
                command = ["powershell", "-NoProfile", "-NonInteractive", "-Command",
                           f"[System.Net.Dns]::GetHostAddresses('{hostname}') | "
                           "ForEach-Object { $_.IPAddressToString }"]
                addresses = host.kube("exec", pod, "-n", "kube-system", "-c",
                                      "prometheus-collector", "--", *command).split()
            else:
                answer = host.kube("exec", pod, "-n", "kube-system", "-c",
                                   "prometheus-collector", "--", "/usr/sbin/busybox", "nslookup", hostname)
                # Ignore the DNS server's address, which precedes the first answer name.
                addresses = re.findall(r"^Address(?: \d+)?:\s*(\S+)", answer.split("Name:", 1)[-1],
                                       re.MULTILINE) if "Name:" in answer else []
            allowed = proof["endpointAddresses"]["query" if kind == "query" else "monitor"]
            if not addresses or not set(addresses) <= set(allowed):
                raise RuntimeError(f"AMPLS {platform} {kind} DNS does not resolve exclusively to its approved PE")
            observed[platform][kind] = {"host": hostname, "addresses": sorted(set(addresses))}
    return observed


def verify_ampls_public_denial(host, outputs, proof):
    evidence = {}
    for kind, audience, url in (
            ("query", "https://prometheus.monitor.azure.com",
             outputs["queryEndpoint"].rstrip("/") + "/api/v1/query?query=up"),
            ("ingestion", "https://monitor.azure.com",
             "https://" + proof["hosts"]["ingestion"] + "/dataCollectionRules/"
             + proof["immutableDcrId"] + "/streams/Microsoft-PrometheusMetrics/api/v1/write?api-version=2023-04-24")):
        hostname = urlsplit(url).hostname
        addresses = sorted({row[4][0] for row in socket.getaddrinfo(
            hostname, 443, family=socket.AF_INET, type=socket.SOCK_STREAM)})
        if not addresses or any(not ipaddress.ip_address(ip).is_global for ip in addresses):
            raise RuntimeError("AMPLS public-bypass proof requires a public-DNS orchestrator")
        if kind == "ingestion":
            # The runner has query-only RBAC; use the collector's existing ingestion identity.
            secret = host.kube("get", "secret/aad-msi-auth-token", "-n", "kube-system",
                               "-o", "json", private=True, json_output=True)
            token = json.loads(base64.b64decode(secret["data"]["token"]))["access_token"].strip()
        else:
            token = host.commands.run(["az", "account", "get-access-token", "--resource", audience,
                                       "--query", "accessToken", "-o", "tsv"], private=True).strip()
        if not token:
            raise RuntimeError("AMPLS public-bypass probe requires an authenticated token")
        script = (
            'IFS= read -r token; '
            'curl --noproxy "*" --silent --show-error --max-time 60 '
            '-H "Authorization: Bearer $token" --resolve "$1" -w "\\n%{http_code}" "$2"'
        )
        if kind == "ingestion":
            # Empty Snappy WriteRequest; no synthetic metric is written by the negative probe.
            script = script.replace("curl --", "printf '\\000' | curl --", 1) + (
                ' -H "Content-Type: application/x-protobuf" -H "Content-Encoding: snappy" '
                '-H "X-Prometheus-Remote-Write-Version: 0.1.0" --data-binary @-'
            )
        attempts = []
        for address in addresses:
            resolve = f"{hostname}:443:{address}"
            try:
                response = host.commands.run(["bash", "-c", script, "public-bypass", resolve, url],
                                             data=token + "\n", private=True)
            except RuntimeError as error:
                raise RuntimeError(f"AMPLS public {kind} bypass transport failed for {address}: {error}") from error
            body, status = response.rsplit("\n", 1)
            if (status.strip() != "403" or not re.search(
                    r"public network|private[\s-]+(?:link|endpoint)|network access", body, re.IGNORECASE)):
                raise RuntimeError(f"AMPLS public {kind} bypass did not return an explicit network denial: "
                                   f"HTTP {status.strip()} {body[:1000]}")
            attempts.append({"address": address, "httpStatus": 403, "body": body})
        evidence[kind] = {"host": hostname, "origin": "public-DNS pipeline orchestrator",
                          "addressFamily": "IPv4", "attempts": attempts}
    return evidence


def verify_ampls(host, outputs):
    protection = host.az("lock", "show", "--name", "scenario-retain",
                         "--resource-group", host.receipt["resourceGroup"], json_output=True)
    if protection.get("level") != "CanNotDelete":
        raise RuntimeError("AMPLS retained resource group has no confirmed deletion lock")
    print("AMPLS_DELETION_PROTECTION " + json.dumps(protection), flush=True)
    proof = verify_ampls_control_plane(host, outputs)
    proof["deletionProtection"] = protection
    proof["privateDns"] = verify_ampls_private_dns(host, proof)
    proof["publicBypass"] = verify_ampls_public_denial(host, outputs, proof)
    (host.args.evidence / "ampls.json").write_text(json.dumps(proof, indent=2) + "\n", encoding="utf-8")
    print("AMPLS_PROOF " + json.dumps(proof), flush=True)
    return proof


DCE_CERTIFICATE_ERROR = re.compile(
    r'PublicationEndpoint:\s*"https://[^"]*\.metrics\.ingest\.monitor\.azure\.com[^"]*"'
    r'.*(?:certificate verify failed|certificate signed by unknown authority|SSL peer certificate|'
    r'unable to get local issuer|CERTIFICATE_VERIFY_FAILED|CERT_E_UNTRUSTEDROOT|0x800b0109|'
    r'Error in SSL handshake)', re.IGNORECASE)


def require_certificate_rejection(result):
    lines = str(result).splitlines()
    match = re.fullmatch(r"CURL_EXIT_CODE=(\d+)", lines[-1]) if lines else None
    if not match:
        raise RuntimeError(
            "Proxy CA distrust probe did not fail certificate verification (non-numeric curl result)")
    code = match.group(1)
    curl_stderr = "\n".join(lines[:-1]).strip()
    if code != "60":
        raise RuntimeError(
            f"Proxy CA distrust probe did not fail certificate verification (curl exit code {code}): "
            f"{curl_stderr or 'no curl stderr'}")
    return {"curlExitCode": int(code), "certificateRejected": True, "curlStderr": curl_stderr}


def failure_proof(scrapes, errors, observations, started):
    if not any(row["timestamp"] >= started and row["health"] == "up" for row in scrapes):
        raise RuntimeError("CA negative phase has no successful fresh collector scrape")
    if errors < 1:
        raise RuntimeError("CA negative phase has no collector certificate-validation error")
    if len(observations) < 3 or observations[-1]["time"] - observations[0]["time"] < 60:
        raise RuntimeError("CA negative phase did not observe a full minute without ingestion")
    if any(row["result"] for row in observations):
        raise RuntimeError("Samples ingested despite deliberate proxy CA distrust")
    return {"status": "PASS", "successfulScrapes": len(scrapes), "certificateErrors": errors,
            "absentIngestionObservations": len(observations),
            "observationSeconds": observations[-1]["time"] - observations[0]["time"]}


def verify_ca_recovery(host, outputs):
    originals = []
    marker = host.args.run_id + "-untrusted-ca"
    collectors = [
        pod for pod in host.kube("get", "pods", "-n", "kube-system", "-o", "json",
                                 json_output=True, private=True)["items"]
        if any(container["name"] == "prometheus-collector"
               for container in pod["spec"]["containers"])
    ]
    if not collectors:
        raise RuntimeError("No running collector pods to verify the MetricsExtension override")
    config_proof = []
    for pod in collectors:
        owners = pod["metadata"].get("ownerReferences", [])
        if any(owner.get("kind") == "ReplicaSet" for owner in owners):
            config_file = "me.config"
        elif any(owner.get("kind") == "DaemonSet" for owner in owners):
            config_file = "me_ds.config"
        else:
            raise RuntimeError("Cannot determine the MetricsExtension config file for a collector pod")
        config_path = f"/usr/sbin/{config_file}"
        config = host.kube(
            "exec", pod["metadata"]["name"], "-n", "kube-system", "-c", "prometheus-collector",
            "--", "cat", config_path, private=True)
        try:
            overrides = json.loads(config)
        except (TypeError, json.JSONDecodeError) as error:
            raise RuntimeError("MetricsExtension config override is not valid JSON") from error
        if overrides.get("validateCertificates") != 2:
            raise RuntimeError("MetricsExtension proxy certificate validation override is not active")
        config_proof.append({"pod": pod["metadata"]["name"], "configOverridesPath": config_path,
                             "validateCertificates": 2})
    print("MetricsExtension proxy certificate validation: " + json.dumps({
        "scope": "proxy-custom-ca", "collectors": config_proof,
    }), flush=True)

    token = host.commands.run([
        "az", "account", "get-access-token", "--resource", "https://prometheus.monitor.azure.com",
        "--query", "accessToken", "-o", "tsv"], private=True).strip()
    if not token:
        raise RuntimeError("CA negative probe requires an authenticated independent AMW reader")
    proxy_endpoint = outputs.get("proxyEndpoint")
    if not proxy_endpoint:
        raise RuntimeError("CA negative probe requires the owned proxy endpoint")
    query = outputs["queryEndpoint"].rstrip("/") + "/api/v1/query?" + urlencode({
        "query": f'myapp_temperature{{scenario_run="{marker}"}}',
    })
    started = time.time()
    try:
        for resource in ("deployment/ama-metrics", "daemonset/ama-metrics-node"):
            document = host.kube("get", resource, "-n", "kube-system", "-o", "json",
                                 json_output=True, private=True)
            containers = document["spec"]["template"]["spec"]["containers"]
            index = next(i for i, container in enumerate(containers) if container["name"] == "prometheus-collector")
            mounts = containers[index].get("volumeMounts", [])
            trust_mounts = {"scenario-proxy-ca", "anchors-mariner", "anchors-ubuntu"}
            if not trust_mounts.issubset({mount.get("name") for mount in mounts}):
                raise RuntimeError("CA failure adapter is missing an owned or node proxy CA trust mount")
            # AKS exposes its trustedCa through these node anchor mounts too.
            untrusted = [mount for mount in mounts if mount.get("name") not in trust_mounts]
            patch = {"op": "replace", "path": f"/spec/template/spec/containers/{index}/volumeMounts"}
            originals.append((resource, dict(patch, value=mounts)))
            host.kube("patch", resource, "-n", "kube-system", "--type=json",
                      "-p", json.dumps([dict(patch, value=untrusted)]), private=True)
        for resource, _ in originals:
            host.kube("rollout", "status", resource, "-n", "kube-system", "--timeout=5m")
        # Rollout completion can leave old trusted pods terminating.
        for pod in collectors:
            host.kube("wait", "--for=delete", "pod/" + pod["metadata"]["name"],
                      "-n", "kube-system", "--timeout=5m")
        # A new marker cannot be satisfied by samples queued before CA removal.
        started = time.time()
        ca_probes = []
        trust_anchor = "/etc/pki/ca-trust/source/anchors/proxy-cert.crt"
        trust_bundle = "/etc/pki/tls/certs/ca-bundle.crt"
        untrusted_pods = host.kube(
            "get", "pods", "-n", "kube-system", "-o", "json", json_output=True, private=True)["items"]
        for pod in untrusted_pods:
            collector = next((container for container in pod["spec"]["containers"]
                              if container["name"] == "prometheus-collector"), None)
            if collector is None:
                continue
            if any(mount.get("name") == "scenario-proxy-ca"
                   for mount in collector.get("volumeMounts", [])):
                raise RuntimeError("Collector still mounts the owned proxy CA during distrust")
            reset_trust = (
                f'set -e; rm -f -- {trust_anchor}; update-ca-trust; '
                f'if [ -e {trust_anchor} ]; then echo PROXY_CA_ANCHOR_REMAINS; exit 1; fi; '
                'anchors=$(trust list --filter=ca-anchors); '
                'if printf "%s\\n" "$anchors" | grep -Fq "Retained AKS scenario proxy CA"; then '
                'echo PROXY_CA_STILL_TRUSTED; exit 1; fi; '
                'echo PROXY_CA_TRUST_RESET_PASS')
            host.kube(
                "exec", pod["metadata"]["name"], "-n", "kube-system", "-c", "prometheus-collector",
                "--", "sh", "-c", reset_trust, "scenario-ca-trust-reset")
            result = host.kube(
                "exec", pod["metadata"]["name"], "-n", "kube-system", "-c", "prometheus-collector",
                "--", "sh", "-c",
                'set +e; curl --noproxy "" --proxy "$1" --connect-timeout 10 --max-time 30 '
                f'--cacert {trust_bundle} --silent --show-error --output /dev/null "$2" 2>&1; '
                'code=$?; printf "\\nCURL_EXIT_CODE=%s\\n" "$code"',
                "scenario-ca-probe", proxy_endpoint, outputs["dceMetricsIngestionEndpoint"])
            ca_probes.append({"pod": pod["metadata"]["name"],
                              "trustAnchorCleared": True, "proxyCaAbsentFromTrustList": True,
                              **require_certificate_rejection(result)})
        if not ca_probes:
            raise RuntimeError("No collectors available for the proxy CA distrust probe")
        print("Proxy CA distrust probe: " + json.dumps({
            "status": "PASS", "scope": "proxy-custom-ca", "trustAnchor": trust_anchor,
            "trustBundle": trust_bundle, "collectors": ca_probes,
        }), flush=True)
        network_probe_monitor(host, marker)
        time.sleep(90)
        observations = []
        for attempt in range(3):
            with urlopen(Request(query, headers={"Authorization": "Bearer " + token}), timeout=60) as response:
                result = json.load(response)
            if result.get("status") != "success" or result.get("data", {}).get("resultType") != "vector":
                raise RuntimeError("Independent AMW query did not succeed during CA distrust")
            observations.append({"time": time.time(), "result": result["data"]["result"]})
            if attempt < 2:
                time.sleep(35)
        proxy_negative = verify_proxy(host, outputs, started, phase="untrusted-ca")
        pods = host.kube("get", "pods", "-n", "kube-system", "-o", "json",
                         json_output=True, private=True)["items"]
        scrapes, errors = [], 0
        for pod in pods:
            if not any(c["name"] == "prometheus-collector" for c in pod["spec"]["containers"]):
                continue
            name = pod["metadata"]["name"]
            targets = host.kube(
                "exec", name, "-n", "kube-system", "-c", "prometheus-collector", "--",
                "curl", "--fail", "--silent", "--show-error", "--noproxy", "*", "--max-time", "30",
                "http://localhost:9090/api/v1/targets", json_output=True, private=True)
            for target in targets.get("data", {}).get("activeTargets", []):
                if host.network_probe_name in target.get("scrapePool", ""):
                    scrapes.append({"health": target["health"],
                                    "timestamp": datetime.fromisoformat(target["lastScrape"].replace("Z", "+00:00")).timestamp()})
            log = host.kube("exec", name, "-n", "kube-system", "-c", "prometheus-collector",
                            "--", "cat", "/MetricsExtensionConsoleDebugLog.log", private=True)
            errors += sum(1 for line in log.splitlines()
                          if DCE_CERTIFICATE_ERROR.search(line))
        proof = failure_proof(scrapes, errors, observations, started)
        proof["proxy"] = proxy_negative
        print("Collector CA distrust proof: " + json.dumps(proof), flush=True)
    finally:
        deadline, host.commands.deadline = host.commands.deadline, None
        try:
            errors = []
            for resource, patch in originals:
                try:
                    host.kube("patch", resource, "-n", "kube-system", "--type=json",
                              "-p", json.dumps([patch]), private=True)
                    host.kube("rollout", "status", resource, "-n", "kube-system", "--timeout=10m")
                    document = host.kube("get", resource, "-n", "kube-system", "-o", "json",
                                         json_output=True, private=True)
                    index = int(patch["path"].split("/")[-2])
                    actual = document["spec"]["template"]["spec"]["containers"][index]["volumeMounts"]
                    if actual != patch["value"]:
                        raise RuntimeError("Collector proxy CA trust mounts were not restored exactly")
                except (RuntimeError, ValueError, KeyError) as error:
                    errors.append(f"{resource}: {error}")
            if errors:
                raise RuntimeError("Collector CA restoration failed: " + "; ".join(errors))
        finally:
            host.commands.deadline = deadline
    recovered = time.time()
    network_probe_monitor(host, host.args.run_id)
    recovery_ingestion = verify_ingestion(host, outputs, recovered)
    recovery_proxy = verify_proxy(host, outputs, recovered, phase="recovered-ca")
    proof["recovery"] = {"ingestion": recovery_ingestion, "proxy": recovery_proxy}
    (host.args.evidence / "proxy-ca-proof.json").write_text(json.dumps(proof, indent=2) + "\n")
    print("Collector CA recovery proof: " + json.dumps(proof), flush=True)
    return proof


def compiled_parameters(args, scenario, commands, version):
    env = deployment_env(args, version, proxy="private-api" in scenario["clusterFeatures"])
    envelope = commands.run(["az", "bicep", "build-params", "--file", str(ROOT / scenario["cluster"]),
                             "--stdout"], env=env, json_output=True, private=True)
    template = (json.loads(envelope["templateJson"]) if envelope.get("templateJson") else
                commands.run(["az", "bicep", "build", "--file", str(ROOT / "clusters/aks.bicep"),
                              "--stdout"], env=env, json_output=True, private=True))
    values = {name: definition["defaultValue"] for name, definition in template["parameters"].items()
              if "defaultValue" in definition}
    values.update({name: definition["value"]
                   for name, definition in json.loads(envelope["parametersJson"])["parameters"].items()})
    if "secrets-watch-enforcement" in scenario["clusterFeatures"] and values.get("scenarioId") != "linux":
        raise ValueError("Secret-watch compiled scenarioId must be linux")
    return values


def verify_secret_watch_version(scenario, cluster, expected=None):
    if "secrets-watch-enforcement" in scenario["clusterFeatures"]:
        actual = cluster.get("kubernetesVersion")
        try:
            supported = version_key(actual) >= (1, 37, 0)
        except ValueError:
            supported = False
        if not supported:
            raise ValueError("Secret-watch cluster must run Kubernetes >=1.37")
        if expected is not None and actual != expected:
            raise ValueError(f"Secret-watch cluster must run the selected Kubernetes {expected}")


def verify_pool_parameters(parameters, scenario, version):
    pools = parameters["nodePools"]
    if not pools or {pool["osType"].lower() for pool in pools} != set(scenario["platforms"]):
        raise ValueError("Compiled pool platforms do not match the scenario")
    for feature, observed in (("arm64", any(p["architecture"] == "arm64" for p in pools)),
                              ("fips", any(p["fips"] for p in pools))):
        if (feature in scenario["clusterFeatures"]) != observed:
            raise ValueError(f"Compiled pools disagree with scenario feature: {feature}")
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
    if parameters["proxyEnabled"]:
        count = parameters["proxyVmCount"]
        if type(count) is not int or count < 1:
            raise ValueError("Proxy count must be explicit in compiled parameters")


def active_subscription(commands, requested=None):
    subscription = commands.run(["az", "account", "show", "--query", "id", "-o", "tsv"],
                                 private=True).strip()
    try:
        uuid.UUID(subscription)
    except ValueError as error:
        raise ValueError("Azure CLI did not return an active subscription ID") from error
    if requested and requested.casefold() != subscription.casefold():
        raise ValueError("Requested subscription does not match the active Azure CLI subscription")
    return subscription


def preflight(args, scenario, commands):
    versions = commands.run(["az", "aks", "get-versions", "--subscription", args.subscription,
                             "--location", args.location, "--only-show-errors", "-o", "json"],
                            json_output=True)
    version = select_version(versions, scenario)
    log_resolved_version(scenario, args.location, version)
    parameters = compiled_parameters(args, scenario, commands, version)
    verify_pool_parameters(parameters, scenario, version)
    retained = Host(args, scenario, commands)
    existing = bool(retained.persistent_state(
        {"SCENARIO_KUBERNETES_VERSION": version}, parameters["nodePools"]))
    if "secrets-watch-enforcement" in scenario["clusterFeatures"] and not existing:
        raise ValueError("The retained secret-watch cluster is missing; refusing to recreate it")
    upgrade_plan = None
    if (existing and "secrets-watch-enforcement" in scenario["clusterFeatures"]
            and retained.current_kubernetes_version != version):
        upgrade_plan = {"action": "aks-upgrade-in-place", "from": retained.current_kubernetes_version,
                        "to": version}
    result = {"scenario": scenario["id"], "subscription": args.subscription, "location": args.location,
              "kubernetesVersion": version, "upgradeChannel": "stable", "expectedPools": parameters["nodePools"],
              "existing": existing}
    if "secrets-watch-enforcement" in scenario["clusterFeatures"]:
        result["upgradePlan"] = upgrade_plan
    save(args.evidence / "preflight.json", result)
    return result


def deployment_env(args, version, proxy=False):
    env = dict(os.environ, SCENARIO_RUN_NAME=args.run_id, SCENARIO_LOCATION=args.location,
               SCENARIO_KUBERNETES_VERSION=version, SCENARIO_OWNER="ama-metrics-e2e",
               SCENARIO_WINDOWS_PASSWORD=secrets.token_urlsafe(32) + "!aA1")
    if proxy:
        executable = shutil.which("ssh-keygen")
        if not executable:
            raise RuntimeError("ssh-keygen is required to mint an owned private/proxy VM public key")
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


def verify_cilium_profile(cluster):
    network = cluster.get("networkProfile") or {}
    advanced = network.get("advancedNetworking") or {}
    if (network.get("networkPlugin") != "azure"
            or network.get("networkDataplane") != "cilium"
            or advanced.get("enabled") is not True
            or (advanced.get("observability") or {}).get("enabled") is not True
            or (advanced.get("security") or {}).get("enabled") is not True):
        raise RuntimeError("AKS must report the managed Cilium ACNS dataplane, observability and security enabled")


def verify_cilium_lock(host):
    lock = host.az("lock", "show", "--resource-group", host.receipt["resourceGroup"],
                   "--name", "retain-acns-cilium", json_output=True)
    if lock.get("level") != "CanNotDelete":
        raise RuntimeError("The persistent ACNS Cilium resource group requires a CanNotDelete lock")
    print("Deletion protection verified: " + lock["id"], flush=True)


def upgrade_retained_secret_watch(host, outputs, plan):
    if ("secrets-watch-enforcement" not in host.scenario["clusterFeatures"]
            or plan.get("action") != "aks-upgrade-in-place"):
        raise ValueError("Unsupported retained-cluster upgrade plan")
    current = host.current_kubernetes_version
    target = plan.get("to")
    if current == target:
        return
    if (current != plan.get("from") or version_key(current) >= version_key(target)
            or version_key(target) < (1, 37, 0)):
        raise ValueError("Retained secret-watch cluster changed after admission; refusing upgrade")
    print(f"Upgrading retained {host.scenario['id']} AKS cluster in place: {current} -> {target}", flush=True)
    host.az("aks", "upgrade", "--resource-group", host.receipt["resourceGroup"],
            "--name", outputs["clusterName"], "--kubernetes-version", target, "--yes", private=True)
    cluster = host.az("aks", "show", "--resource-group", host.receipt["resourceGroup"],
                      "--name", outputs["clusterName"], json_output=True, private=True)
    if (cluster.get("kubernetesVersion") != target or cluster.get("provisioningState") != "Succeeded"
            or (cluster.get("powerState") or {}).get("code") != "Running"):
        raise RuntimeError("Retained secret-watch AKS upgrade did not reach the admitted Kubernetes version")
    host.current_kubernetes_version = target


def bootstrap_testkube(host, outputs, package, principal, principal_type, expected_version=None):
    group = host.receipt["resourceGroup"]
    identity = host.az("identity", "create", "--resource-group", group,
                       "--name", "ama-e2e-tests", json_output=True)
    # This subscription also has a broader custom role with the same display name.
    role = RBAC_MONITORING_DATA_READER
    # The fresh private-query probe forwards the caller's token into the collector pod.
    for object_id, kind in ((identity["principalId"], "ServicePrincipal"), (principal, principal_type)):
        assignments = host.az("role", "assignment", "list", "--assignee-object-id", object_id,
                              "--fill-principal-name", "false", "--scope", outputs["workspaceId"],
                              json_output=True, private=True)
        if not any(row.get("scope", "").casefold() == outputs["workspaceId"].casefold()
                   and row.get("principalId", "").casefold() == object_id.casefold()
                   and row.get("roleDefinitionId", "").rsplit("/", 1)[-1].casefold() == role
                   and not row.get("condition") for row in assignments):
            assign = host.az_with_rbac_retry if kind == "ServicePrincipal" else host.az
            assign("role", "assignment", "create", "--assignee-object-id", object_id,
                   "--assignee-principal-type", kind, "--role", role, "--scope", outputs["workspaceId"])
    cluster = host.az("aks", "show", "--resource-group", group, "--name", outputs["clusterName"],
                      json_output=True, private=True)
    verify_secret_watch_version(host.scenario, cluster, expected_version)
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
        if "cilium" in host.scenario["clusterFeatures"]:
            verify_cilium_profile(cluster)
            host.kube("rollout", "status", "daemonset/cilium", "-n", "kube-system", "--timeout=10m")
            print("Managed ACNS proof: " + json.dumps(cluster["networkProfile"]), flush=True)
            verify_cilium_lock(host)
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
    workflow_values = None
    proxy = cluster.get("httpProxyConfig") or {}
    if proxy.get("enabled") is not False and (proxy.get("httpProxy") or proxy.get("httpsProxy")):
        for endpoint in (proxy.get("httpProxy"), proxy.get("httpsProxy")):
            if endpoint:
                try:
                    parsed = urlsplit(endpoint)
                    if (parsed.scheme not in {"http", "https"} or not parsed.hostname
                            or parsed.username is not None or parsed.password is not None
                            or parsed.query or parsed.fragment or parsed.path not in {"", "/"}):
                        raise ValueError
                except ValueError:
                    raise ValueError("Inline TestKube defaults require credential-free HTTP(S) proxy endpoints") from None
        bypass = proxy.get("effectiveNoProxy")
        if bypass is None:
            # Core CLI can omit read-only fields; use the raw ARM response.
            raw_cluster = host.az("rest", "--method", "get", "--url",
                                  cluster["id"] + "?api-version=2026-09-01",
                                  json_output=True, private=True)
            bypass = ((raw_cluster.get("properties") or {}).get("httpProxyConfig") or {}).get("effectiveNoProxy")
        if bypass is None:
            raise ValueError("AKS effectiveNoProxy is required to preserve injected bypass defaults")
        # AKS admission can omit clone init containers; workflow defaults reach them.
        environment = [
            {"name": name, "value": value}
            for key, value in (
                ("HTTP_PROXY", proxy.get("httpProxy")),
                ("HTTPS_PROXY", proxy.get("httpsProxy") or proxy.get("httpProxy")),
                ("NO_PROXY", ",".join([*bypass, "testkube-api-server:8089"])),
            ) if value is not None
            for name in (key, key.lower())
        ]
        workflow_values = yaml.safe_dump({"global": {"testWorkflows": {"globalTemplate": {
            "enabled": True, "inline": True, "spec": {"container": {"env": environment}},
        }}}})
    host.commands.run(["helm", "upgrade", "--install", "testkube", str(package),
                      "--kubeconfig", str(host.kubeconfig), "--namespace", "testkube",
                      "--create-namespace", "--values", str(REPO / "otelcollector/test/testkube/values.yaml"),
                      *(["--values", "-"] if workflow_values is not None else []),
                      # Match the cluster's .svc/.cluster.local bypasses without replacing proxy environment.
                      "--set-string", "testkube-api.storage.endpoint=testkube-minio-service-testkube.testkube.svc.cluster.local:9000",
                      "--set", "testkube-api.minio.podSecurityContext.fsGroup=1001",
                      "--set", "mongodb.nodeAffinityPreset.type=hard",
                      "--set-string", "mongodb.nodeAffinityPreset.key=kubernetes.azure.com/agentpool",
                      "--set-string", "mongodb.nodeAffinityPreset.values[0]=system",
                      "--set", "mongodb.updateStrategy.type=Recreate",
                      "--set-string", "global.testWorkflows.builtinTemplatesInstaller.nodeSelector.kubernetes\\.io/os=linux",
                      "--wait", "--timeout", "20m"], data=workflow_values, private=True)
    host.kube("annotate", "serviceaccount", "testkube-api-server-tests-job", "-n", "testkube",
              f"azure.workload.identity/client-id={identity['clientId']}", "--overwrite")
    host.kube("apply", "-f", str(REPO / "otelcollector/test/testkube/api-server-permissions.yaml"))
    deployments = host.kube("get", "deployments", "-n", "testkube", "-o", "json",
                            json_output=True, private=True)["items"]
    if not deployments:
        raise RuntimeError("TestKube has no deployments")
    for deployment in deployments:
        host.kube("rollout", "status", "deployment/" + deployment["metadata"]["name"],
                  "-n", "testkube", "--timeout=15m")
    # API readiness stays green after bucket initialization exhausts its retries.
    # Replay startup only after MinIO is ready, including on retained clusters.
    host.kube("rollout", "restart", "deployment/testkube-api-server", "-n", "testkube")
    host.kube("rollout", "status", "deployment/testkube-api-server", "-n", "testkube", "--timeout=15m")
    workflows = host.kube("api-resources", "--api-group=testworkflows.testkube.io", "--no-headers")
    if not re.search(r"^testworkflows\s", workflows, re.MULTILINE):
        raise RuntimeError("TestKube workflow API was not installed")
    if "cilium" in host.scenario["clusterFeatures"]:
        # Cilium emits DNS metrics only for inspected DNS traffic; preserve all other egress.
        policy = {
            "apiVersion": "cilium.io/v2", "kind": "CiliumNetworkPolicy",
            "metadata": {"name": "candidate-acns-dns", "namespace": "testkube"},
            "spec": {
                "endpointSelector": {"matchLabels": {"azure.workload.identity/use": "true"}},
                "egress": [
                    {"toPorts": [{"ports": [{"port": "53", "protocol": "ANY"}],
                                  "rules": {"dns": [{"matchPattern": "*"}]}}]},
                    {"toPorts": [{"ports": [
                        {"port": "1", "endPort": 52, "protocol": "ANY"},
                        {"port": "54", "endPort": 65535, "protocol": "ANY"},
                    ]}]},
                ],
            },
        }
        host.kube("apply", "-f", "-", data=json.dumps(policy))
        observed = host.kube("get", "ciliumnetworkpolicy", "candidate-acns-dns", "-n", "testkube",
                             "-o", "json", json_output=True)
        if observed.get("spec") != policy["spec"]:
            raise RuntimeError("Cilium DNS inspection policy does not match the scoped traffic proof")
        print("Managed ACNS DNS inspection proof: " + json.dumps(observed["spec"]), flush=True)
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


def resolve_build_images(args, image_config):
    images = []
    for source in image_config["tagSources"]:
        component = source["component"]
        variable = source["environment"]
        tag = os.environ[variable]
        if "truncateTo" in source:
            tag = tag[:source["truncateTo"]]
        images.extend(resolve_tag(component, tag))
    for image in images:
        image["manifestRef"] = image["indexRef"]
    save(args.evidence / "images.json", images)
    return images


def cleanup_reference_apps(host):
    host.assert_owned()
    fixtures = [
        "otelcollector/test/test-cluster-yamls/customresources/prometheus-reference-app.yaml",
        "internal/referenceapp/prometheus-reference-app.yaml",
    ]
    if "windows" in host.scenario["platforms"]:
        fixtures.append("internal/referenceapp/win-prometheus-reference-app.yaml")
    for fixture in fixtures:
        host.kube("delete", "-f", str(REPO / fixture), "-n", "default",
                  "--ignore-not-found", "--wait=true", "--timeout=5m")
    if "windows" in host.scenario["platforms"]:
        host.kube("delete", "daemonset/windows-exporter", "configmap/windows-exporter-config", "-n", "monitoring",
                  "--ignore-not-found", "--wait=true", "--timeout=5m")
        if getattr(host, "created_monitoring", False):
            host.kube("delete", "namespace", "monitoring", "--ignore-not-found", "--wait=true", "--timeout=5m")
    if "otlp" in host.scenario["clusterFeatures"]:
        host.kube("delete", "service", "candidate-otlp", "-n", "kube-system",
                  "--ignore-not-found", "--wait=true", "--timeout=5m")
    if (set(host.scenario["clusterFeatures"]) & {"private-api", "ampls"}
            and host.created_network_probe_monitor):
        resource = "servicemonitors.azmonitoring.coreos.com"
        host.kube("delete", resource, host.network_probe_name, "-n", "default",
                  "--ignore-not-found", "--wait=true", "--timeout=5m")
        remaining = host.kube("get", resource, host.network_probe_name, "-n", "default",
                              "--ignore-not-found", "-o", "name").strip()
        if remaining:
            raise RuntimeError("Run-specific network probe monitor was not removed")
        host.created_network_probe_monitor = False


def install_windows_exporter(host):
    if not host.kube("get", "namespace", "monitoring", "--ignore-not-found", "-o", "name").strip():
        host.kube("create", "namespace", "monitoring")
        host.created_monitoring = True
    host.kube("apply", "-f", str(REPO / "internal/windowsExporterInstaller/windows-exporter-daemonset.yaml"))
    host.kube("rollout", "status", "daemonset/windows-exporter", "-n", "monitoring", "--timeout=10m")


def execute(args, scenario, variants, lifecycle, image_config, suites, commands):
    features = set(scenario["clusterFeatures"])
    ampls = "ampls" in features
    ampls_proxy = ampls and "http-proxy" in features
    otlp = "otlp" in features
    enable_otlp = ampls_proxy or otlp
    managed_acns_cilium = {"managed-acns", "cilium"} <= features
    secret_watch = "secrets-watch-enforcement" in features
    args.source_sha = os.environ["BUILD_SOURCEVERSION"]
    source = args.source_sha
    build_images = resolve_build_images(args, image_config)
    selected_settings = scenario_settings(scenario, variants)
    for name in selected_settings:
        variant = variants[name]
        if variant["addonParams"]:
            raise ValueError(f"{name}: addon parameter adapter is required before provisioning")
        label_filter(scenario, variant)
    label_filter(scenario)
    resolved = preflight(args, scenario, commands)
    args.admitted_existing = resolved["existing"]
    images, mapping = image_mapping(build_images, scenario["platforms"],
                                    scenario["clusterFeatures"], resolved["expectedPools"], image_config)
    testkube = prepare_testkube(args, commands)
    save(args.evidence / "settings-provenance.json", {
        name: {
            "variantSha256": variants[name]["_variantSha256"],
            "resources": [
                {"path": str(path), "sha256": hashlib.sha256(path.read_bytes()).hexdigest()}
                for field in ("configmaps", "customResources")
                for path in variants[name].get(field, [])
            ],
        } for name in selected_settings
    })
    env = deployment_env(args, resolved["kubernetesVersion"], proxy="private-api" in scenario["clusterFeatures"])
    host = Host(args, scenario, commands)
    transport = ExitStack()
    outcome = {"scenario": scenario["id"], "runId": args.run_id, "executionEngine": "testkube",
               "sourceSha": source, "status": "RUNNING", "settings": {}}
    reference_apps_ready = False
    baseline = None
    hold_acquired = False
    cleanup_completed = False
    commands.deadline = time.monotonic() + args.max_run_seconds
    try:
        outputs = host.provision(env, testkube[1], testkube[2], resolved["expectedPools"])
        resolved["existing"] = host.reused
        host.assert_owned()
        group, cluster = host.receipt["resourceGroup"], outputs["clusterName"]
        host.az("aks", "get-credentials", "--resource-group", group, "--name", cluster,
                "--file", str(host.kubeconfig), "--admin", private=True)
        if "private-api" in scenario["clusterFeatures"]:
            transport.enter_context(private_api(host, outputs))
        acquire_workflow_hold(host)
        hold_acquired = True
        if secret_watch:
            group_scope = f"/subscriptions/{args.subscription}/resourceGroups/{host.receipt['resourceGroup']}"
            for key, resource_type in (("clusterId", "Microsoft.ContainerService/managedClusters"),
                                       ("workspaceId", "Microsoft.Monitor/accounts")):
                resource_id = outputs[key].rstrip("/")
                if not re.fullmatch(re.escape(f"{group_scope}/providers/{resource_type}/") + r"[^/]+",
                                    resource_id, re.IGNORECASE):
                    raise ValueError(f"Retained {key} is outside the owned resource group or has the wrong type")
                if key == "clusterId" and resource_id.rsplit("/", 1)[-1].casefold() != outputs["clusterName"].casefold():
                    raise ValueError("Retained clusterId does not match the cluster name")
                scopes = (f"/subscriptions/{args.subscription}", group_scope, resource_id)
                locks = host.az("lock", "list", "--resource", resource_id, json_output=True, private=True)
                if not isinstance(locks, list) or not any(
                    isinstance(lock, dict) and isinstance(lock.get("id"), str)
                    and isinstance(lock.get("name"), str) and lock["name"]
                    and lock.get("level") in ("CanNotDelete", "ReadOnly")
                    and any(lock.get("id", "").casefold() == (
                        scope + "/providers/Microsoft.Authorization/locks/" + lock["name"]
                    ).casefold() for scope in scopes)
                    for lock in locks
                ):
                    raise RuntimeError(f"Retained {key} has no effective deletion-protection lock")
            if resolved.get("upgradePlan"):
                upgrade_retained_secret_watch(host, outputs, resolved["upgradePlan"])
        if ampls_proxy:
            configure_ampls_cluster_proxy(host, outputs)
        host.kube("wait", "nodes", "--all", "--for=condition=Ready", "--timeout=20m")
        state = bootstrap_testkube(host, outputs, *testkube, expected_version=resolved["kubernetesVersion"])
        nodes = host.kube("get", "nodes", "-o", "json", json_output=True, private=True)["items"]
        save(args.evidence / "pool-inventory.json",
             verify_pool_inventory(resolved["expectedPools"], state["agentPoolProfiles"], nodes))
        handoff = not resolved["existing"] or (managed_acns_cilium
            and ((state.get("azureMonitorProfile") or {}).get("metrics") or {}).get("enabled") is True)
        enable_addon = not resolved["existing"]
        if (ampls and not host.reused) or (
                not ampls and "http-proxy" in features and "custom-ca" not in features):
            managed_enabled = bool(((state.get("azureMonitorProfile") or {}).get("metrics") or {}).get("enabled"))
            needs_token = not host.kube("get", "secret/aad-msi-auth-token", "-n", "kube-system",
                                       "--ignore-not-found", "-o", "name", private=True).strip()
            enable_addon = needs_token and not managed_enabled
            handoff = needs_token or managed_enabled
        if enable_addon:
            host.az("aks", "update", "--resource-group", group, "--name", cluster,
                    "--enable-azure-monitor-metrics", "--azure-monitor-workspace-resource-id", outputs["workspaceId"])
        if not (ampls and host.reused):
            host.kube("wait", "--for=create", "secret/aad-msi-auth-token",
                      "-n", "kube-system", "--timeout=15m", private=True)
        if ampls and not host.reused:
            initialize_ampls(host, outputs, handoff)
        elif handoff:
            if managed_acns_cilium:
                # CLI addon disable deletes the DCR/DCRAs still owned by this fixture.
                host.az("resource", "update", "--ids", outputs["clusterId"], "--api-version", "2024-10-01",
                        "--set", "properties.azureMonitorProfile.metrics.enabled=false")
            else:
                host.az("aks", "update", "--resource-group", group, "--name", cluster,
                        "--disable-azure-monitor-metrics")
        if managed_acns_cilium:
            association_url = f"https://management.azure.com{outputs['ruleAssociationId']}?api-version=2023-03-11"
            host.az("rest", "--method", "put", "--url", association_url,
                    "--body", json.dumps({"properties": {"dataCollectionRuleId": outputs["dcrId"]}}))
            observed = host.az("rest", "--method", "get", "--url", association_url, json_output=True)
            if observed.get("properties", {}).get("dataCollectionRuleId", "").casefold() != outputs["dcrId"].casefold():
                raise RuntimeError("Cilium handoff requires the owned DCR association")
            verify_cilium_lock(host)
            print("Managed ACNS monitoring fixture: owned DCR association and deletion protection verified", flush=True)
        state = host.az("aks", "show", "--resource-group", group, "--name", cluster, json_output=True)
        outputs["kubernetesVersion"] = state["kubernetesVersion"]
        verify_secret_watch_version(scenario, state, resolved["kubernetesVersion"])
        if managed_acns_cilium:
            verify_cilium_profile(state)
        if ((state.get("azureMonitorProfile") or {}).get("metrics") or {}).get("enabled"):
            raise RuntimeError("Managed addon is still enabled; refusing competing Helm ownership")
        if handoff:
            deadline = time.monotonic() + 600
            managed = {"ama-metrics", "ama-metrics-node", "ama-metrics-win-node",
                       "ama-metrics-operator-targets", "ama-metrics-ksm"}
            while True:
                remaining = host.kube("get", "deployments,daemonsets", "-n", "kube-system",
                                      "-o", "json", json_output=True, private=True)["items"]
                if not any(resource["metadata"]["name"] in managed for resource in remaining):
                    break
                if time.monotonic() >= deadline:
                    raise RuntimeError("Managed addon workloads remain; refusing competing Helm ownership")
                time.sleep(10)
        if "private-api" in scenario["clusterFeatures"] or ampls:
            host.protect_private_host()
        baseline = snapshot(host, [("ConfigMap", name, lifecycle["namespace"])
                                   for name in lifecycle["resetConfigMaps"]])
        if ampls_proxy:
            settings = enable_otlp_settings(
                (REPO / "otelcollector/test/test-cluster-yamls/configmaps/ama-metrics-settings-configmap.yaml")
                .read_text(), lifecycle)
            host.kube("apply", "-f", "-", data=settings, private=True)
        elif otlp:
            settings = yaml.safe_load(
                (REPO / "otelcollector/test/test-cluster-yamls/configmaps/ama-metrics-settings-configmap.yaml").read_text())
            settings["data"]["opentelemetry-metrics"] = "enabled = true"
            host.kube("apply", "-f", "-", data=yaml.safe_dump(settings), private=True)
        chart = render_chart(commands, args.evidence, outputs, source, mapping, image_config["containerRoles"],
                             otlp=enable_otlp)
        commands.run(["helm", "upgrade", "--install", "ama-metrics", str(chart),
                      "--kubeconfig", str(host.kubeconfig), "--namespace", "kube-system",
                      "--wait", "--timeout", "20m"])
        for resource in ("deployment/ama-metrics", "deployment/ama-metrics-operator-targets", "daemonset/ama-metrics-node"):
            host.kube("rollout", "status", resource, "-n", "kube-system", "--timeout=15m")
        if "windows" in scenario["platforms"]:
            host.kube("rollout", "status", "daemonset/ama-metrics-win-node", "-n", "kube-system", "--timeout=20m")
        reference_apps_ready = True
        cleanup_reference_apps(host)
        if "windows" in scenario["platforms"] and ampls:
            install_windows_exporter(host)
        save(args.evidence / "images-before.json", audit_images(host, images, image_config))
        if ampls and not host.reused:
            outcome["ampls"] = verify_ampls(host, outputs)
        if ampls_proxy:
            proxy_started = time.time()
            if not host.reused:
                warm = verify_me_configuration(host, outcome["ampls"], 0)
                for resource in ("deployment/ama-metrics", "daemonset/ama-metrics-node"):
                    host.kube("rollout", "restart", resource, "-n", "kube-system")
                    host.kube("rollout", "status", resource, "-n", "kube-system", "--timeout=15m")
                for pod in warm["collectors"]:
                    host.kube("wait", "--for=delete", f"pod/{pod['pod']}", "-n", "kube-system", "--timeout=5m")
                outcome["meConfiguration"] = verify_me_configuration(host, outcome["ampls"], proxy_started)
        if managed_acns_cilium:
            host.kube("apply", "-f", str(REPO / "internal/referenceapp/prometheus-reference-app.yaml"))
            host.kube("rollout", "status", "deployment/prometheus-reference-app", "--timeout=10m")
        for name in selected_settings:
            outcome["settings"][name] = settings_slot(host, variants[name], lifecycle)
        if "windows" in scenario["platforms"] and not ampls:
            install_windows_exporter(host)
        # Health suites depend on the repository's existing reference-app scrape fixtures.
        for name in ("ama-metrics-prometheus-config-configmap.yaml",
                     "ama-metrics-prometheus-config-node-configmap.yaml",
                     "ama-metrics-prometheus-config-node-windows-configmap.yaml",
                     "ama-metrics-settings-configmap.yaml"):
            if enable_otlp and name == "ama-metrics-settings-configmap.yaml":
                continue
            host.kube("apply", "-f", str(REPO / "otelcollector/test/test-cluster-yamls/configmaps" / name))
        references = ["prometheus-reference-app.yaml"]
        if "windows" in scenario["platforms"]:
            references.append("win-prometheus-reference-app.yaml")
        for name in references:
            host.kube("apply", "-f", str(REPO / "internal/referenceapp" / name))
        monitors = list(yaml.safe_load_all(
            (REPO / "otelcollector/test/test-cluster-yamls/customresources/prometheus-reference-app.yaml").read_text()))
        probe_started = time.time()
        host.kube("apply", "-f", "-", data=yaml.safe_dump_all(
            mark_health_monitors(monitors, args.run_id)), private=True)
        restart_collectors(host)
        outcome["suites"] = {}
        for suite in suites:
            outcome["suites"][suite] = run_suite(host, suite, args.evidence / "health" / suite,
                                                label_filter(scenario))
        if otlp:
            service = {
                "apiVersion": "v1", "kind": "Service",
                "metadata": {"name": "candidate-otlp", "namespace": "kube-system"},
                "spec": {"selector": {"dsName": "ama-metrics-node"},
                         "ports": [{"port": 56681, "targetPort": 56681}]},
            }
            host.kube("apply", "-f", "-", data=json.dumps(service), private=True)
            outcome["suites"]["otlp"] = run_suite(
                host, "querymetrics", args.evidence / "health" / "otlp", "scenario-otlp")
        if enable_otlp:
            outcome["suites"]["otlp-processes"] = run_suite(
                host, "containerstatus", args.evidence / "health" / "otlp-processes", "otlp && !windows")
        if "private-api" in scenario["clusterFeatures"] or ampls:
            probe_started = time.time()
            network_probe_monitor(host, args.run_id)
        if "http-proxy" in scenario["clusterFeatures"] and not ampls_proxy:
            phase = "trusted-ca" if "custom-ca" in scenario["clusterFeatures"] else "default"
            outcome["proxy"] = verify_proxy(host, outputs, probe_started, phase)
        outcome["ingestion"] = verify_ingestion(host, outputs, probe_started)
        if ampls:
            print("AMPLS_PRIVATE_INGESTION " + json.dumps(outcome["ingestion"]), flush=True)
        if ampls_proxy:
            outcome["proxy"] = verify_proxy(
                host, outputs, proxy_started, ampls_proof=outcome.get("ampls"))
        if "custom-ca" in scenario["clusterFeatures"]:
            outcome["ca"] = verify_ca_recovery(host, outputs)
        save(args.evidence / "images-after.json", audit_images(host, images, image_config))
        outcome["status"] = "TESTS_PASSED"
    except (RuntimeError, ValueError, KeyError, OSError, ET.ParseError, yaml.YAMLError) as failure:
        outcome.update(status="FAIL", error=str(failure))
        raise
    finally:
        commands.deadline = None
        try:
            if getattr(host, "workflow_unsettled", False):
                raise RuntimeError("TestKube execution not confirmed stopped; fixture restoration is unsafe")
            try:
                if baseline is not None:
                    commands.deadline = time.monotonic() + 600
                    try:
                        restore(host, baseline, preserve_settings=ampls_proxy,
                                settings_identity=("ConfigMap", lifecycle["settingsConfigMap"],
                                                   lifecycle["namespace"]))
                    finally:
                        commands.deadline = None
            finally:
                if reference_apps_ready:
                    cleanup_reference_apps(host)
            cleanup_completed = not getattr(host, "cleanup_failed", False)
        except (RuntimeError, ValueError, OSError, KeyError) as failure:
            outcome.update(status="FAIL", fixtureCleanupError=str(failure))
            raise
        finally:
            try:
                try:
                    if hold_acquired and cleanup_completed:
                        release_workflow_hold(host)
                finally:
                    try:
                        transport.close()
                    finally:
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
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("action", choices=("validate", "compile", "plan", "preflight", "run",
                                           "cleanup", "revoke-bootstrap", "close-private-api"))
    parser.add_argument("--matrix", type=Path, default=ROOT / "matrix.json")
    parser.add_argument("--settings-root", type=Path, default=ROOT / "settings")
    parser.add_argument("--scenario", default="default")
    parser.add_argument("--subscription")
    parser.add_argument("--location")
    parser.add_argument("--run-id", default=str(uuid.uuid4()))
    parser.add_argument("--testkube-version", help="Explicit TestKube Helm chart version; required for run")
    parser.add_argument("--kubectl", default="kubectl", help="kubectl executable name or absolute path")
    parser.add_argument("--tools-bin", type=Path, help="Directory containing scoped TestKube CLI/jq executables")
    parser.add_argument("--evidence", type=Path, required=True)
    parser.add_argument("--max-run-seconds", type=int, default=14400)
    args = parser.parse_args(argv)
    if args.action == "run":
        if os.environ.get("ENVIRONMENT_NAME") != "Prometheus-Collector":
            raise ValueError("Cluster scenarios require the Prometheus-Collector deployment environment")
        if args.matrix.resolve() != ROOT / "matrix.json" or args.settings_root.resolve() != ROOT / "settings":
            raise ValueError("Scenario matrix and settings must come from this checkout")
    args.evidence = args.evidence.resolve()
    uuid.UUID(args.run_id)
    if args.max_run_seconds <= 0:
        raise ValueError("max-run-seconds must be positive")
    commands = Commands(args.evidence)
    if args.action in {"preflight", "run", "cleanup", "revoke-bootstrap"} or (
            args.action == "close-private-api" and (args.evidence / "ownership.json").exists()):
        args.subscription = active_subscription(commands, args.subscription)
    if args.action == "cleanup":
        with execution_lock(args.evidence):
            Host(args, {}, commands).cleanup()
        return 0
    if args.action == "revoke-bootstrap":
        with execution_lock(args.evidence):
            Host(args, {}, commands).revoke_bootstrap()
        return 0
    if args.action == "close-private-api":
        if not (args.evidence / "ownership.json").exists():
            return 0
        scenario = next((row for row in load(args.matrix)["scenarios"] if row["id"] == args.scenario), None)
        if not scenario:
            raise ValueError(f"Unknown scenario: {args.scenario}")
        with execution_lock(args.evidence):
            host = Host(args, scenario, commands)
            host.receipt = load(args.evidence / "ownership.json")
            if host.receipt["scenario"] != args.scenario:
                raise ValueError("Private API recovery scenario differs from the ownership receipt")
            if "private-api" not in scenario["clusterFeatures"]:
                print(f"No private API to clean up for {args.scenario}")
                return 0
            host.assert_owned()
            deployment = args.evidence / "deployment.json"
            lease = args.evidence / "private-api-lease.json"
            try:
                if deployment.exists() and lease.exists():
                    close_private_api(host, load(deployment), load(lease)["runId"])
            finally:
                host.protect_private_host()
        return 0
    variants, lifecycle = catalog(args.settings_root.resolve())
    matrix = validate_matrix(load(args.matrix), variants)
    suites = health_suites()
    if args.action == "validate":
        save(args.evidence / "validation.json", {
            "scenarios": len(matrix["scenarios"]), "variants": list(variants), "healthSuites": suites})
        return 0
    if args.action == "compile":
        if args.location is None:
            args.location = matrix["scenarios"][0]["location"]
        compile_profiles(args, commands)
        return 0
    scenario = next((s for s in matrix["scenarios"] if s["id"] == args.scenario), None)
    if not scenario:
        raise ValueError(f"Unknown scenario: {args.scenario}")
    if scenario.get("blockedReason"):
        raise ValueError(f"BLOCKED: {scenario['blockedReason']}")
    if args.location is None:
        args.location = scenario["location"]
    if args.action == "plan":
        save(args.evidence / "plan.json", {"hostCount": 1, "scenario": scenario,
                                         "settingsSlots": scenario_settings(scenario, variants),
                                         "buildImagesRequired": True})
    elif args.action == "preflight":
        preflight(args, scenario, commands)
    else:
        def interrupted(signum, frame):
            raise RuntimeError(f"Scenario interrupted by signal {signum}")
        previous = {number: signal.signal(number, interrupted) for number in (signal.SIGINT, signal.SIGTERM)}
        try:
            with execution_lock(args.evidence):
                execute(args, scenario, variants, lifecycle, matrix["buildImages"], suites, commands)
        finally:
            for number, handler in previous.items():
                signal.signal(number, handler)
    return 0


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except (ValueError, RuntimeError, KeyError, OSError, ET.ParseError, yaml.YAMLError,
            jsonschema.ValidationError) as error:
        print(f"ERROR: {error}", file=sys.stderr)
        raise SystemExit(1)
