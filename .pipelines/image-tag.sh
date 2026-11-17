# Image names shared by the tag job and the image jobs that start without waiting for it. Source from the repo root.
# The date is the run's queue time in Pacific time so every job agrees. The POSIX TZ rule is needed because
# Git Bash on Windows has no zoneinfo and silently treats TZ=America/Los_Angeles as UTC.
QUEUE_TIME=$(curl -fsSL --retry 5 --retry-all-errors -H "Authorization: Bearer $SYSTEM_ACCESSTOKEN" \
  "${SYSTEM_COLLECTIONURI}${SYSTEM_TEAMPROJECTID}/_apis/build/builds/${BUILD_BUILDID}?api-version=7.1" \
  | grep -o '"queueTime":"[^"]*"' | head -n 1 | cut -d '"' -f 4)
if [ -z "$QUEUE_TIME" ]; then
  echo "##vso[task.logissue type=error]Could not read the queue time of build $BUILD_BUILDID"
  exit 1
fi
if [ "$IS_PR" = "True" ]; then
  BRANCH_NAME=$SYSTEM_PULLREQUEST_SOURCEBRANCH
else
  BRANCH_NAME=${BUILD_SOURCEBRANCH#refs/heads/}
fi
BRANCH_NAME=$(echo $BRANCH_NAME | tr / - | tr . - | tr _ - | cut -c1-90)
COMMIT_SHA=$(echo $BUILD_SOURCEVERSION | cut -b -8)
DATE=$(TZ='PST8PDT,M3.2.0,M11.1.0' date -d "$QUEUE_TIME" +%m-%d-%Y) || exit 1
VERSION=$(tr -d '[:space:]' < otelcollector/VERSION)
SEMVER=$VERSION-$BRANCH_NAME-$DATE-$COMMIT_SHA
LINUX_IMAGE_TAG=$SEMVER
if [ "$IS_SRE_BRANCH" = "True" ]; then
  LINUX_IMAGE_TAG="sre-$BUILD_BUILDID-$SEMVER"
fi
# Truncating to 128 characters as it is required by docker
LINUX_IMAGE_TAG=$(echo "${LINUX_IMAGE_TAG}" | cut -c1-128)
# Truncating to 105 characters as it is required by docker (4 characters used in -win, 9 characters used in -ltsc2019/-ltsc2022, 10 characters used in -unsigned)
WINDOWS_IMAGE_TAG=$(echo "${LINUX_IMAGE_TAG}" | cut -c1-105)-win
WINDOWS_FULL_IMAGE_NAME=$ACR_REGISTRY$ACR_REPOSITORY:$WINDOWS_IMAGE_TAG
WINDOWS_2019_BASE_IMAGE_VERSION=ltsc2019
WINDOWS_2022_BASE_IMAGE_VERSION=ltsc2022
# Truncating to 124 characters to add the -cfg/-ccp suffix
LINUX_IMAGE_TAG_PREFIX=$(echo "${LINUX_IMAGE_TAG}" | cut -c1-124)
LINUX_CONFIG_READER_IMAGE_TAG=$LINUX_IMAGE_TAG_PREFIX-cfg
LINUX_CCP_IMAGE_TAG=$LINUX_IMAGE_TAG_PREFIX-ccp
LINUX_CONFIG_READER_FULL_IMAGE_NAME=$ACR_REGISTRY$ACR_REPOSITORY:$LINUX_CONFIG_READER_IMAGE_TAG
LINUX_CCP_FULL_IMAGE_NAME=$ACR_REGISTRY$ACR_REPOSITORY:$LINUX_CCP_IMAGE_TAG
LINUX_AMD64_FULL_IMAGE_NAME=$ACR_REGISTRY$ACR_REPOSITORY:$LINUX_IMAGE_TAG_PREFIX-amd
LINUX_ARM64_FULL_IMAGE_NAME=$ACR_REGISTRY$ACR_REPOSITORY:$LINUX_IMAGE_TAG_PREFIX-arm
#Truncating this to 112 characters to add the targetallocator suffix
TARGET_ALLOCATOR_IMAGE_TAG_PREFIX=$(echo "${LINUX_IMAGE_TAG}" | cut -c1-112)
TARGET_ALLOCATOR_IMAGE_TAG=$TARGET_ALLOCATOR_IMAGE_TAG_PREFIX-targetallocator
#Truncating this 108 to characters to add the targetallocator-arm/amd suffix
TARGET_ALLOCATOR_MULTIARCH_TAG_PREFIX=$(echo "${LINUX_IMAGE_TAG}" | cut -c1-108)
TARGET_ALLOCATOR_ARM64_IMAGE_TAG=$TARGET_ALLOCATOR_MULTIARCH_TAG_PREFIX-targetallocator-arm
TARGET_ALLOCATOR_AMD64_IMAGE_TAG=$TARGET_ALLOCATOR_MULTIARCH_TAG_PREFIX-targetallocator-amd
TARGET_ALLOCATOR_FULL_IMAGE_NAME=$ACR_REGISTRY$ACR_REPOSITORY:$TARGET_ALLOCATOR_IMAGE_TAG
TARGET_ALLOCATOR_ARM64_FULL_IMAGE_NAME=$ACR_REGISTRY$ACR_REPOSITORY:$TARGET_ALLOCATOR_ARM64_IMAGE_TAG
TARGET_ALLOCATOR_AMD64_FULL_IMAGE_NAME=$ACR_REGISTRY$ACR_REPOSITORY:$TARGET_ALLOCATOR_AMD64_IMAGE_TAG
