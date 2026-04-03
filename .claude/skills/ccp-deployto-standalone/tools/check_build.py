#!/usr/bin/env python3
"""Check a prometheus-collector pipeline build for the CCP ORAS push stage.

Usage: python3 check_build.py <build-id>

Queries the github-private.visualstudio.com build API to check the status of
the ORAS push stage and extract the CCP image tag from its logs.

Prerequisites:
  - Azure CLI logged in (az account get-access-token)
"""
import json, sys, subprocess, base64, urllib.request

if len(sys.argv) < 2:
    print(f"Usage: {sys.argv[0]} <build-id>", file=sys.stderr)
    sys.exit(1)

build_id = sys.argv[1]
token = subprocess.check_output([
    "az", "account", "get-access-token",
    "--resource", "499b84ac-1321-427f-aa17-267ca6975798",
    "--query", "accessToken", "-o", "tsv"
], stderr=subprocess.DEVNULL).decode().strip()

creds = base64.b64encode(f":{token}".encode()).decode()

def api_get(url):
    req = urllib.request.Request(url)
    req.add_header("Authorization", f"Basic {creds}")
    return json.loads(urllib.request.urlopen(req).read())

def api_get_text(url):
    req = urllib.request.Request(url)
    req.add_header("Authorization", f"Basic {creds}")
    return urllib.request.urlopen(req).read().decode()

build = api_get(f"https://github-private.visualstudio.com/azure/_apis/build/builds/{build_id}?api-version=7.1")
print(f"Build {build_id}: status={build.get('status')}, result={build.get('result','pending')}")

data = api_get(f"https://github-private.visualstudio.com/azure/_apis/build/builds/{build_id}/timeline?api-version=7.1")

for r in data.get("records", []):
    name = r.get("name", "")
    rtype = r.get("type", "")
    state = r.get("state", "")
    result = r.get("result", "pending")
    
    if ("CCP" in name or "ccp" in name) and rtype in ("Stage", "Job"):
        print(f"  [{rtype}] {name}: state={state}, result={result}")
    if "ORAS" in name and "linuxccp" in name:
        print(f"  [ORAS] {name}: state={state}, result={result}")
        if result == "succeeded":
            log_url = r.get("log", {}).get("url")
            if log_url:
                log_text = api_get_text(log_url)
                for line in log_text.split("\n"):
                    if "cidev" in line and "ccp" in line:
                        print(f"  IMAGE_LINE: {line.strip()}")
