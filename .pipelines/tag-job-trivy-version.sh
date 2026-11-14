#!/usr/bin/env bash
# Run by image jobs that start before the tag job finishes, ahead of any push: nothing is pushed unless the tag job
# succeeded, and every scan uses the one Trivy version it resolved for the run.
set -euo pipefail
api="${SYSTEM_COLLECTIONURI}${SYSTEM_TEAMPROJECTID}/_apis/build/builds/${BUILD_BUILDID}"
get() { curl -fsSL --retry 5 --retry-all-errors -H "Authorization: Bearer $SYSTEM_ACCESSTOKEN" "$1"; }
while :; do
  timeline=$(get "$api/timeline?api-version=7.1")
  status=$(jq -r '[.records[] | select(.type == "Phase" and .identifier == "Build.Image_Tags_and_Ev2_Artifacts")][0]
    | if .state != "completed" then "wait" elif .result == "succeeded" or .result == "succeededWithIssues" then "done" else "failed" end' <<<"$timeline")
  echo "Tag job: $status"
  case "$status" in
    done) break ;;
    failed) echo "##vso[task.logissue type=error]The tag job did not succeed"; exit 1 ;;
  esac
  sleep 15
done
log_id=$(jq -r '[.records[] | select(.type == "Task" and .name == "Build: resolve latest stable Trivy version")][0].log.id' <<<"$timeline")
version=$(get "$api/logs/$log_id?api-version=7.1" | sed -n 's/.*Selected Trivy version: \([0-9.]*\).*/\1/p')
if ! [[ $version =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  echo "##vso[task.logissue type=error]No Trivy version in the tag job log"
  exit 1
fi
echo "##vso[task.setvariable variable=TRIVY_VERSION]$version"
