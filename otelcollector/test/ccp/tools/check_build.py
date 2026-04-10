#!/usr/bin/env python3
"""Check the status of a prometheus-collector CCP build in ADO.

Usage:
    python3 check_build.py <build-id>

Checks the Azure/prometheus-collector pipeline build and reports:
- Overall build status
- CCP-related stage/job status
- ORAS push status and image tag (if available)

Requires: az login (uses az account get-access-token for auth)
"""

import json
import sys
import subprocess
import base64
import urllib.request

def get_ado_token():
    return subprocess.check_output([
        "az", "account", "get-access-token",
        "--resource", "499b84ac-1321-427f-aa17-267ca6975798",
        "--query", "accessToken", "-o", "tsv"
    ], stderr=subprocess.DEVNULL).decode().strip()

def api_get(url, creds):
    req = urllib.request.Request(url)
    req.add_header("Authorization", f"Basic {creds}")
    return json.loads(urllib.request.urlopen(req).read())

def api_get_text(url, creds):
    req = urllib.request.Request(url)
    req.add_header("Authorization", f"Basic {creds}")
    return urllib.request.urlopen(req).read().decode()

def main():
    if len(sys.argv) < 2:
        print("Usage: python3 check_build.py <build-id>")
        sys.exit(1)

    build_id = sys.argv[1]
    token = get_ado_token()
    creds = base64.b64encode(f":{token}".encode()).decode()

    # Check overall build status
    build = api_get(
        f"https://github-private.visualstudio.com/azure/_apis/build/builds/{build_id}?api-version=7.1",
        creds
    )
    print(f"Build {build_id}: status={build.get('status')}, result={build.get('result', 'pending')}")

    # Check timeline for CCP-related stages and ORAS push
    data = api_get(
        f"https://github-private.visualstudio.com/azure/_apis/build/builds/{build_id}/timeline?api-version=7.1",
        creds
    )

    oras_succeeded = False
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
                oras_succeeded = True
                log_url = r.get("log", {}).get("url")
                if log_url:
                    log_text = api_get_text(log_url, creds)
                    for line in log_text.split("\n"):
                        if "cidev" in line and "ccp" in line:
                            print(f"  IMAGE_LINE: {line.strip()}")

    if oras_succeeded:
        print("\n✅ ORAS push succeeded — CCP image is available.")
    else:
        print("\n⏳ ORAS push not yet succeeded — check back later or inspect the pipeline.")

if __name__ == "__main__":
    main()
