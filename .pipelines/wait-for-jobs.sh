#!/usr/bin/env bash
# Waits for jobs in this stage to succeed. Merge jobs call this instead of dependsOn so their agent is acquired while
# the awaited jobs run; a dependsOn request only starts at their end and usually lands on a cold agent (~4-5m).
set -euo pipefail
url="${SYSTEM_COLLECTIONURI}${SYSTEM_TEAMPROJECTID}/_apis/build/builds/${BUILD_BUILDID}/timeline?api-version=7.1"
ids=$(for job in "$@"; do echo "$SYSTEM_STAGENAME.$job.__default"; done | jq -R . | jq -cs .)
while :; do
  # Job records complete ~0.1-0.2m before their Phase records.
  status=$(curl -fsSL --retry 5 --retry-all-errors -H "Authorization: Bearer $SYSTEM_ACCESSTOKEN" "$url" | jq -r --argjson ids "$ids" '
    [.records[] | select(.type == "Job" and (.identifier as $i | $ids | index($i)))]
    | if any(.state == "completed" and .result != "succeeded" and .result != "succeededWithIssues") then "failed"
      elif length == ($ids | length) and all(.state == "completed") then "done" else "wait" end')
  echo "$status"
  case "$status" in
    done) break ;;
    failed) echo "##vso[task.logissue type=error]A job this one waits for did not succeed"; exit 1 ;;
  esac
  sleep 5
done
