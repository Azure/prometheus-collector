#!/usr/bin/env bash
set -euo pipefail

: "${COMET_IMAGE:?COMET_IMAGE is required}"
: "${COMET_RESULTS_DIR:?COMET_RESULTS_DIR is required}"
: "${SYSTEM_ACCESSTOKEN:?System.AccessToken is required}"

scanner_image="mcr.microsoft.com/tvm-containers/shavsa/comet:v1-linux-amd64"
mkdir -p "$COMET_RESULTS_DIR"

platform="$(docker image inspect --format '{{.Os}}/{{.Architecture}}' "$COMET_IMAGE")"
if [[ "$platform" != "linux/amd64" ]]; then
  echo "Expected a local Linux AMD64 image; found '$platform'." >&2
  exit 1
fi
target_digest="$(docker image inspect --format '{{json .RepoDigests}}' "$COMET_IMAGE" | python3 -c 'import json,sys; print(json.load(sys.stdin)[0])')"
printf 'CoMET target: %s\nCoMET target digest: %s\n' "$COMET_IMAGE" "$target_digest"

docker pull "$scanner_image"
scanner_digest="$(docker image inspect --format '{{json .RepoDigests}}' "$scanner_image" | python3 -c 'import json,sys; print(json.load(sys.stdin)[0])')"
printf 'CoMET scanner: %s\nCoMET scanner digest: %s\n' "$scanner_image" "$scanner_digest"

oidc_url="${SYSTEM_COLLECTIONURI}${SYSTEM_TEAMPROJECTID}/_apis/distributedtask/hubs/build/plans/${SYSTEM_PLANID}/jobs/${SYSTEM_JOBID}/oidctoken?api-version=7.1-preview.1"
oidc_response="$(curl --fail --silent --show-error \
  --request POST \
  --header "Authorization: Bearer ${SYSTEM_ACCESSTOKEN}" \
  --header 'Content-Type: application/json' \
  --data '{}' \
  "$oidc_url")"
oidc_token="$(python3 -c 'import json,sys; print(json.load(sys.stdin)["oidcToken"])' <<<"$oidc_response")"
unset oidc_response
if [[ -z "$oidc_token" ]]; then
  echo "Azure DevOps returned an empty CoMET OIDC token." >&2
  exit 1
fi
echo "##vso[task.setsecret]$oidc_token"

docker_volume="comet-internal-${BUILD_BUILDID}-${SYSTEM_JOBATTEMPT}"
docker volume create "$docker_volume" >/dev/null
set +e
docker run --rm \
  --volume /var/run/docker.sock:/var/run/docker.sock \
  --volume "${COMET_RESULTS_DIR}:/results" \
  --volume "${docker_volume}:/internal" \
  --env EnvStage=prod \
  "$scanner_image" scan \
  --force-manifest-download \
  --oidc-token "$oidc_token" \
  --docker-shared-volume "$docker_volume" \
  --scan-result-path /results \
  "$COMET_IMAGE"
scan_exit_code=$?
set -e
unset oidc_token

if [[ ! -s "$COMET_RESULTS_DIR/result.xml" || ! -s "$COMET_RESULTS_DIR/result.json" ]]; then
  echo "CoMET did not produce both non-empty result.xml and result.json." >&2
  exit 1
fi
python3 - "$COMET_RESULTS_DIR/result.xml" "$COMET_RESULTS_DIR/result.json" <<'PY'
import json
import sys
import xml.etree.ElementTree as ET

xml_path, json_path = sys.argv[1:]
root = ET.parse(xml_path).getroot()
values = {
    element.tag.rsplit("}", 1)[-1]: (element.text or "").strip().lower()
    for element in root.iter()
}
if values.get("PARTIAL_REPORT_FLAG") != "false":
    raise SystemExit("CoMET XML is missing PARTIAL_REPORT_FLAG=false.")
if values.get("SCA_SCAN_ENABLED") != "true":
    raise SystemExit("CoMET XML does not confirm SCA_SCAN_ENABLED=true.")

results = json.load(open(json_path, encoding="utf-8"))
vulnerabilities = results.get("vulnerabilities", [])
app_count = sum(v.get("resultType") == "AppPackage" for v in vulnerabilities)
os_count = sum(v.get("resultType") == "OSPackage" for v in vulnerabilities)
print(f"CoMET XML is complete and SCA-enabled; JSON has {app_count} AppPackage and {os_count} OSPackage findings.")
PY

if [[ "$scan_exit_code" -eq 1 ]]; then
  echo "##vso[task.logissue type=warning]CoMET returned findings (exit 1); inspect the published XML and JSON reports."
elif [[ "$scan_exit_code" -ne 0 ]]; then
  echo "CoMET scanner failed with exit code $scan_exit_code." >&2
  exit "$scan_exit_code"
fi
