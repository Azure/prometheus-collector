"""Scan the candidate and released collector with CoMET and compare Qualys QIDs."""
import argparse
import json
import os
from pathlib import Path
import re
import subprocess
import uuid
import xml.etree.ElementTree as ET
from urllib.request import Request, urlopen

from filter_evidence import stage
from scan_trivy import resolve_tag


SCANNER = "mcr.microsoft.com/tvm-containers/shavsa/comet:v1-linux-amd64"
WINDOWS_SCANNER = "mcr.microsoft.com/tvm-containers/shavsa/comet:v1-windows-amd64"
WINDOWS_BUILDS = {"2019": "17763", "2022": "20348"}
NOT_COVERED_WINDOWS = "Windows collector images are not scanned by this Linux job; see the Windows CoMET jobs"


def report(directory):
    xml = (directory / "result.xml").read_text(encoding="utf-8-sig")
    if "<!DOCTYPE" in xml.upper() or "<!ENTITY" in xml.upper():
        raise ValueError("XML declarations are not allowed")
    root = ET.fromstring(xml)
    if root.tag != "HOST" or root.findtext("PARTIAL_REPORT_FLAG") != "false":
        raise ValueError("Expected a complete Qualys HOST report")
    if root.findtext("SCA_SCAN_ENABLED") != "true":
        raise ValueError("Qualys SCA coverage is missing")
    metadata = {field: root.findtext(field) for field in
                ("OS", "VULNSIGS_VERSION", "AGENT_MANIFEST_VERSION", "REPORT_GENERATED_DATE")}
    if not all(metadata.values()):
        raise ValueError("Qualys report metadata is missing")
    enrichment = json.loads((directory / "result.json").read_text(encoding="utf-8-sig"))
    if not isinstance(enrichment, dict):
        raise ValueError("Expected a Qualys JSON object")
    flagged = set()
    vulns = root.findall("VULN")
    for vuln in vulns:
        qid, flags = vuln.findtext("QID"), vuln.findtext("FLAGS")
        if not qid or not re.fullmatch(r"[1-9][0-9]*", qid) or not flags:
            raise ValueError("Malformed Qualys vulnerability")
        flag_set = set(re.split(r"[\s,]+", flags.strip()))
        if not flag_set or flag_set - {"n", "u", "v"}:
            raise ValueError("Malformed Qualys vulnerability flags")
        if "v" in flag_set:
            flagged.add(qid)
    not_found = {entry.text for entry in root.findall("./VULNS_NOT_FOUND/Q")}
    if (not vulns and not not_found) or any(
            not qid or not re.fullmatch(r"[1-9][0-9]*", qid) for qid in not_found):
        raise ValueError("Empty or malformed Qualys evaluation")
    if flagged & not_found:
        raise ValueError("A Qualys QID is both flagged and not found")
    return metadata, flagged, not_found


def compare(baseline, candidate, baseline_image, candidate_image, windows_release=None):
    before_meta, before, before_absent = report(baseline)
    after_meta, after, after_absent = report(candidate)
    for field in ("VULNSIGS_VERSION", "AGENT_MANIFEST_VERSION"):
        if before_meta[field] != after_meta[field]:
            raise ValueError(f"Incomparable Qualys knowledge bases: {field}")
    if baseline_image.rsplit("@", 1)[-1] == candidate_image.rsplit("@", 1)[-1] and before != after:
        raise ValueError("The same image produced contradictory Qualys findings")
    rows = []
    for qid in sorted(before | after, key=int):
        if qid in before and qid in after:
            status = "OPEN"
        elif qid in before:
            status = "CLOSED" if qid in after_absent else "NOT_REPORTED"
        else:
            status = "NEW" if qid in before_absent else "BASELINE_NOT_REPORTED"
        rows.append({"qid": qid, "status": status})
    statuses = {row["status"] for row in rows}
    verdict = ("BLOCKED" if statuses & {"NOT_REPORTED", "BASELINE_NOT_REPORTED"}
               else "FINDINGS" if statuses & {"OPEN", "NEW"} else "COMPLETE")
    platform = f"Windows Server {windows_release}/amd64" if windows_release else "Linux/amd64"
    return {
        "scope": f"{platform} collector Qualys QIDs; not full image clearance",
        "notCovered": [] if windows_release else [NOT_COVERED_WINDOWS],
        "scanner": WINDOWS_SCANNER if windows_release else SCANNER,
        "baseline": {"image": baseline_image, "report": before_meta},
        "candidate": {"image": candidate_image, "report": after_meta},
        "policy": "Report open and new QIDs; missing QID evaluation blocks; closure requires explicit not-found",
        "verdict": verdict,
        "qids": rows,
    }


def oidc_token():
    env = os.environ
    url = (f'{env["SYSTEM_COLLECTIONURI"]}{env["SYSTEM_TEAMPROJECTID"]}/_apis/distributedtask/'
           f'hubs/build/plans/{env["SYSTEM_PLANID"]}/jobs/{env["SYSTEM_JOBID"]}/oidctoken'
           '?api-version=7.1-preview.1')
    request = Request(url, data=b"{}", method="POST", headers={
        "Authorization": "Bearer " + env["SYSTEM_ACCESSTOKEN"],
        "Content-Type": "application/json",
    })
    with urlopen(request, timeout=60) as response:
        token = json.load(response)["oidcToken"]
    if not isinstance(token, str) or not token.strip():
        raise ValueError("Azure DevOps returned an empty CoMET OIDC token")
    return token


def scan(image, directory, volume, windows=False):
    directory.mkdir()
    local_tag = f"qualys-{uuid.uuid4().hex}:scan"
    with (directory / "scan.log").open("w", encoding="utf-8") as log:
        subprocess.run(["docker", "pull", image], stdout=log, stderr=subprocess.STDOUT, check=True)
        platform = subprocess.run(
            ["docker", "image", "inspect", "--format", "{{.Os}}/{{.Architecture}}", image],
            capture_output=True, text=True, check=True,
        ).stdout.strip()
        expected = "windows/amd64" if windows else "linux/amd64"
        if platform != expected:
            raise ValueError(f"Expected a {expected} image, got {platform}")
        subprocess.run(["docker", "tag", image, local_tag], stdout=log, stderr=subprocess.STDOUT, check=True)
        try:
            token = oidc_token()
            if windows:
                script = (
                    "$ErrorActionPreference='Stop'; $token=[Console]::In.ReadLine(); "
                    "& C:\\comet\\entrypoint.ps1 scan --verbose --force-manifest-download "
                    "--oidc-token $token --min-severity-results 0 "
                    "--docker-shared-volume $env:COMET_VOLUME --scan-result-path C:\\results "
                    "$env:COMET_IMAGE; exit $LASTEXITCODE"
                )
                result = subprocess.run([
                    "docker", "run", "--rm", "--interactive", "--isolation", "process",
                    "--volume", "//./pipe/docker_engine://./pipe/docker_engine",
                    "--volume", f"{directory}:C:\\results", "--volume", f"{volume}:C:\\internal",
                    "--volume", "qualys_binaries_volume:C:\\qualys_binaries",
                    "--env", "EnvStage=prod", "--env", f"COMET_VOLUME={volume}",
                    "--env", f"COMET_IMAGE={local_tag}", "--entrypoint", "powershell.exe",
                    WINDOWS_SCANNER, "-NoProfile", "-NonInteractive", "-Command", script,
                ], input=token + "\n", text=True, stdout=subprocess.DEVNULL,
                    stderr=subprocess.DEVNULL, check=False)
            else:
                script = ('IFS= read -r token; exec /comet/entrypoint.sh scan --verbose '
                          '--force-manifest-download --oidc-token "$token" --min-severity-results 0 '
                          '--docker-shared-volume "$1" --scan-result-path /results "$2"')
                result = subprocess.run([
                    "docker", "run", "--rm", "--interactive",
                    "--volume", "/var/run/docker.sock:/var/run/docker.sock",
                    "--volume", f"{directory}:/results", "--volume", f"{volume}:/internal",
                    "--env", "EnvStage=prod", "--entrypoint", "/bin/sh", SCANNER,
                    "-c", script, "comet-auth-stdin", volume, local_tag,
                ], input=token + "\n", text=True, stdout=subprocess.DEVNULL,
                    stderr=subprocess.DEVNULL, check=False)
            (directory / "metadata.json").write_text(json.dumps({
                "image": image, "scanner": WINDOWS_SCANNER if windows else SCANNER,
                "exitCode": result.returncode,
            }, indent=2) + "\n", encoding="utf-8")
            if result.returncode not in (0, 1):
                raise ValueError(f"CoMET failed with exit code {result.returncode}")
            report(directory)
        finally:
            subprocess.run(["docker", "image", "rm", local_tag],
                           stdout=log, stderr=subprocess.STDOUT, check=True)


def run_windows(baseline, manifest_digest, release, directory):
    if not re.fullmatch(r"sha256:[0-9a-f]{64}", manifest_digest):
        raise ValueError("Expected an immutable Windows manifest digest")
    images = [image for image in resolve_tag("collector", manifest_digest)
              if image["os"] == "windows" and image["arch"] == "amd64"
              and re.fullmatch(r"10\.0\." + WINDOWS_BUILDS[release] + r"\.\d+", image["osVersion"])]
    if len(images) != 1:
        raise ValueError(f"Expected exactly one Windows Server {release} candidate image")
    candidate = images[0]["ref"]
    image_pattern = (r"mcr\.microsoft\.com/azuremonitor/containerinsights/(?:ciprod|cidev)/"
                     r"prometheus-collector/images@sha256:[0-9a-f]{64}")
    if not re.fullmatch(image_pattern, baseline) or not re.fullmatch(image_pattern, candidate):
        raise ValueError("Expected immutable Windows collector images")
    subprocess.run(["docker", "pull", WINDOWS_SCANNER], check=True)
    volume = "qualys-" + uuid.uuid4().hex
    subprocess.run(["docker", "volume", "create", volume], stdout=subprocess.DEVNULL, check=True)
    binary_volume = "qualys_binaries_volume"
    owner = volume
    owned = False
    try:
        existing = subprocess.run([
            "docker", "volume", "ls", "--filter", f"name={binary_volume}", "--format", "{{.Name}}",
        ], capture_output=True, text=True, check=True).stdout.splitlines()
        if binary_volume in existing:
            raise ValueError(f"CoMET volume {binary_volume} already exists on this agent")
        subprocess.run(["docker", "volume", "create", "--label", f"pr-e2e-owner={owner}", binary_volume],
                       stdout=subprocess.DEVNULL, check=True)
        label = '{{index .Labels "pr-e2e-owner"}}'
        actual = subprocess.run(["docker", "volume", "inspect", "--format", label, binary_volume],
                                capture_output=True, text=True, check=True).stdout.strip()
        if actual != owner:
            raise ValueError(f"CoMET volume {binary_volume} is owned by another scan")
        owned = True
        scan(baseline, directory / "baseline", volume, windows=True)
        scan(candidate, directory / "candidate", volume, windows=True)
        result = compare(directory / "baseline", directory / "candidate", baseline, candidate, release)
        if any(f"Windows Server {release}" not in result[name]["report"]["OS"]
               for name in ("baseline", "candidate")):
            raise ValueError(f"Expected Windows Server {release} Qualys reports")
        (directory / "comparison.json").write_text(json.dumps(result, indent=2) + "\n", encoding="utf-8")
        return result
    finally:
        try:
            if owned:
                actual = subprocess.run(["docker", "volume", "inspect", "--format", label, binary_volume],
                                        capture_output=True, text=True, check=True).stdout.strip()
                if actual != owner:
                    raise ValueError(f"CoMET volume {binary_volume} changed ownership")
                subprocess.run(["docker", "volume", "rm", binary_volume], stdout=subprocess.DEVNULL, check=True)
        finally:
            subprocess.run(["docker", "volume", "rm", volume], stdout=subprocess.DEVNULL, check=True)


def run(baseline, producer_scan, directory):
    metadata = json.loads((producer_scan / "trivy" / "metadata.json").read_text(encoding="utf-8-sig"))
    candidate = metadata["requestedRef"]
    image_pattern = (r"mcr\.microsoft\.com/azuremonitor/containerinsights/(?:ciprod|cidev)/"
                     r"prometheus-collector/images@sha256:[0-9a-f]{64}")
    if (metadata["platform"] != "linux/amd64" or not re.fullmatch(image_pattern, candidate)
            or not re.fullmatch(image_pattern, baseline)):
        raise ValueError("Expected immutable Linux/amd64 collector images")
    subprocess.run(["docker", "pull", SCANNER], check=True)
    volume = "qualys-" + uuid.uuid4().hex
    subprocess.run(["docker", "volume", "create", volume], stdout=subprocess.DEVNULL, check=True)
    try:
        scan(baseline, directory / "baseline", volume)
        scan(candidate, directory / "candidate", volume)
        result = compare(directory / "baseline", directory / "candidate", baseline, candidate)
        if any("windows" in result[name]["report"]["OS"].lower() for name in ("baseline", "candidate")):
            raise ValueError("Expected Linux Qualys reports")
        (directory / "comparison.json").write_text(json.dumps(result, indent=2) + "\n", encoding="utf-8")
        return result
    finally:
        attached = subprocess.run(
            ["docker", "ps", "--all", "--quiet", "--filter", f"volume={volume}"],
            capture_output=True, text=True, check=True,
        ).stdout.splitlines()
        for container in (container.strip() for container in attached):
            if container:
                subprocess.run(["docker", "rm", "--force", container],
                               stdout=subprocess.DEVNULL, check=True)
        subprocess.run(["docker", "volume", "rm", volume], stdout=subprocess.DEVNULL, check=True)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--baseline-image", required=True)
    parser.add_argument("--producer-scan", type=Path)
    parser.add_argument("--windows-manifest-digest")
    parser.add_argument("--windows-release", choices=WINDOWS_BUILDS)
    parser.add_argument("--output", required=True, type=Path)
    args = parser.parse_args()
    if args.windows_release:
        if not args.windows_manifest_digest or args.producer_scan:
            parser.error("Windows scans require --windows-manifest-digest and no --producer-scan")
    elif not args.producer_scan or args.windows_manifest_digest:
        parser.error("Linux scans require --producer-scan and no --windows-manifest-digest")
    output = args.output.resolve()
    private = output.with_name(output.name + "-private")
    private.mkdir(parents=True, exist_ok=False)
    try:
        result = (run_windows(args.baseline_image, args.windows_manifest_digest, args.windows_release, private)
                  if args.windows_release else run(args.baseline_image, args.producer_scan.resolve(), private))
        print(json.dumps({"verdict": result["verdict"], "scope": result["scope"],
                          "notCovered": result["notCovered"],
                          "comparison": str(output / "comparison.json")}, indent=2))
        if args.windows_release:
            for row in result["qids"]:
                print(json.dumps(row))
        else:
            print(f"##vso[task.logissue type=warning]Qualys: {NOT_COVERED_WINDOWS}")
        return {"COMPLETE": 0, "FINDINGS": 0, "BLOCKED": 2}[result["verdict"]]
    finally:
        stage(private, output)


if __name__ == "__main__":
    raise SystemExit(main())
