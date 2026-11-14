#!/usr/bin/env bash
# Runs on the Linux tag job and the Windows image jobs, which must derive identical image names.
set -euo pipefail
cd "$(dirname "$0")/../.."

curl() { echo "{\"id\":7,\"queueTime\":\"$TEST_QUEUE_TIME\",\"startTime\":\"2001-01-01T00:00:00Z\"}"; }
SYSTEM_ACCESSTOKEN=x SYSTEM_COLLECTIONURI=x SYSTEM_TEAMPROJECTID=x SYSTEM_PULLREQUEST_SOURCEBRANCH=
IS_PR=False IS_SRE_BRANCH=False BUILD_BUILDID=7 BUILD_SOURCEBRANCH=refs/heads/main BUILD_SOURCEVERSION=3e060365abcdef
ACR_REGISTRY=registry.io ACR_REPOSITORY=/repo
version=$(cat otelcollector/VERSION)

expect() {
  TEST_QUEUE_TIME=$1
  source .pipelines/image-tag.sh
  if [ "$WINDOWS_FULL_IMAGE_NAME" != "registry.io/repo:$version-main-$2-3e060365-win" ]; then
    echo "Queue time $1 gave $WINDOWS_FULL_IMAGE_NAME, expected date $2"
    exit 1
  fi
}

# Pacific standard and daylight time, each on a UTC day after the Pacific date.
expect 2026-11-12T01:06:45.1234567Z 11-11-2026
expect 2026-07-01T06:30:00Z 06-30-2026
expect 2026-07-01T07:30:00Z 07-01-2026
echo "Image tag tests passed"
