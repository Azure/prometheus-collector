#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/../.."
state=$(mktemp -d)
trap 'rm -rf "$state"' EXIT
export STATE=$state SYSTEM_ACCESSTOKEN=x SYSTEM_COLLECTIONURI=x SYSTEM_TEAMPROJECTID=x BUILD_BUILDID=7

curl() {
  case "${@: -1}" in
    *timeline*)
      local states=($TAG_STATES) n
      n=$(cat "$STATE/n" 2>/dev/null || echo 0); echo $((n + 1)) >"$STATE/n"
      echo "{\"records\":[{\"type\":\"Phase\",\"identifier\":\"Build.Image_Tags_and_Ev2_Artifacts\",${states[$n]}},
        {\"type\":\"Task\",\"name\":\"Build: resolve latest stable Trivy version\",\"log\":{\"id\":17}}]}" ;;
    *logs/17*) printf '2026-11-14T03:08:38Z ##[section]Starting\r\n2026-11-14T03:08:39Z Selected Trivy version: 0.76.2\r\n' ;;
    *) return 22 ;;
  esac
}
sleep() { :; }
export -f curl sleep

run() { rm -f "$STATE/n"; TAG_STATES=$1 bash .pipelines/tag-job-trivy-version.sh; }

out=$(run '"state":"inProgress" "state":"completed","result":"succeeded"')
[[ $out == *"##vso[task.setvariable variable=TRIVY_VERSION]0.76.2" ]] || { echo "Unexpected output: $out"; exit 1; }
if run '"state":"completed","result":"failed"' >/dev/null; then echo "A failed tag job was accepted"; exit 1; fi
echo "Tag job Trivy version tests passed"
