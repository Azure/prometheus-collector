#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/../.."
state=$(mktemp -d)
trap 'rm -rf "$state"' EXIT
export STATE=$state SYSTEM_ACCESSTOKEN=x SYSTEM_COLLECTIONURI=x SYSTEM_TEAMPROJECTID=x BUILD_BUILDID=7 SYSTEM_STAGENAME=Build

# Each poll returns the next space-separated record list from $POLLS; records are job:state[:result].
curl() {
  local polls=($POLLS) n records=() r job st res
  n=$(cat "$STATE/n" 2>/dev/null || echo 0); echo $((n + 1)) >"$STATE/n"
  IFS=, read -ra recs <<<"${polls[$n]}"
  for r in "${recs[@]}"; do
    IFS=: read -r job st res <<<"$r"
    records+=("{\"type\":\"Job\",\"identifier\":\"Build.$job.__default\",\"state\":\"$st\",\"result\":\"$res\"}")
  done
  (IFS=,; echo "{\"records\":[${records[*]}]}")
}
sleep() { :; }
export -f curl sleep

run() { rm -f "$STATE/n"; POLLS=$1 bash .pipelines/wait-for-jobs.sh A B >/dev/null; }

run 'A:completed:succeeded A:completed:succeeded,B:inProgress A:completed:succeeded,B:completed:succeededWithIssues' || { echo "Succeeded jobs were not accepted"; exit 1; }
[[ $(cat "$state/n") == 3 ]] || { echo "Stopped before every job completed"; exit 1; }
if run 'A:completed:succeeded,B:completed:failed'; then echo "A failed job was accepted"; exit 1; fi
if run 'A:completed:succeeded,B:completed:canceled'; then echo "A canceled job was accepted"; exit 1; fi
echo "Wait for jobs tests passed"
