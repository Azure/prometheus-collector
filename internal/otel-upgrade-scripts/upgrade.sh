#!/bin/bash
set -e

# Display usage
if [ $# -ne 5 ]; then
	echo "Usage: $0 <collector_tag_version> [stable_tag_version] [target_allocator_tag_version] [current_otel_version] [current_target_allocator_version]"
	echo "Example: $0 v0.123.0 v1.29.0 v0.123.1"
	exit 1
fi

TAG=$1
STABLE_TAG=$2
TA_TAG=$3
CURRENT_OTEL_VERSION=$4
CURRENT_TA_VERSION=$5
BRANCH_NAME="${TAG}"
CURRENT_DIR=$(pwd)

# Check for required tools for registry querying
REGISTRY_TOOLS_AVAILABLE=true
if ! command -v curl &> /dev/null; then
    echo "Warning: curl not found. Will use go.mod versions directly without checking registry for latest patches."
    REGISTRY_TOOLS_AVAILABLE=false
fi

if ! command -v jq &> /dev/null; then
    echo "Warning: jq not found. Will use go.mod versions directly without checking registry for latest patches."
    REGISTRY_TOOLS_AVAILABLE=false
fi

if [ "$REGISTRY_TOOLS_AVAILABLE" = true ]; then
    echo "Registry tools (curl and jq) are available. Will check for latest patch versions."
else
    echo "Registry tools not available. Install curl and jq for automatic patch version detection."
fi


echo "Current OTel version: $CURRENT_OTEL_VERSION"
echo "Current Target Allocator version: $CURRENT_TA_VERSION"

echo "Starting Otel Collector upgrade to ${STABLE_TAG}/${TAG}..."

# Step 1: Clone OpenTelemetry Collector Contrib
echo "Cloning OpenTelemetry Collector Contrib repository..."

# Check if opentelemetry-collector-contrib directory already exists
if [ ! -d "opentelemetry-collector-contrib" ]; then
	git clone --depth 1 --branch $TAG https://github.com/open-telemetry/opentelemetry-collector-contrib.git
else
	cd opentelemetry-collector-contrib
	git fetch --depth 1 origin tag $TAG
	cd "$CURRENT_DIR"
	echo "Tag exists"
fi

# Branch for matching tag
cd opentelemetry-collector-contrib
git branch | grep -q "$BRANCH_NAME" || true
RETURN_CODE=$?
echo "Return code: $RETURN_CODE"
if [ $RETURN_CODE -ne 0 ]; then
	# Branch doesn't exist, safe to create it
	git checkout tags/$TAG -b $BRANCH_NAME
else
	# Branch exists, just check out the existing branch
	git checkout $BRANCH_NAME
	echo "Branch $BRANCH_NAME already exists, using existing branch"
fi
cd "$CURRENT_DIR"

# Step 2: Update opentelemetry-collector-builder
echo "Updating opentelemetry-collector-builder..."
cd otelcollector/opentelemetry-collector-builder

# Remove indirect dependencies from go.mod
echo "Removing indirect dependencies from go.mod..."
grep -v "// indirect" go.mod > go.mod.tmp && mv go.mod.tmp go.mod

# Update go.mod to new collector version
# Replace stable packages with stable version and beta packages with beta version
sed -i -E "s|(go\.opentelemetry\.io\/collector\/[a-zA-Z0-9\/]*) v0\.[0-9]*\.[0-9]*|\1 ${TAG}|g" go.mod
sed -i -E "s|(go\.opentelemetry\.io\/collector\/[a-zA-Z0-9\/]*) v1\.[0-9]*\.[0-9]*|\1 ${STABLE_TAG}|g" go.mod
sed -i -E "s|(github\.com\/open-telemetry\/opentelemetry-collector-contrib\/[a-zA-Z0-9\/]*) v0\.[0-9]*\.[0-9]*|\1 ${TAG}|g" go.mod
sed -i -E "s|(github\.com\/open-telemetry\/opentelemetry-collector-contrib\/[a-zA-Z0-9\/]*) v1\.[0-9]*\.[0-9]*|\1 ${STABLE_TAG}|g" go.mod

# Remove indirect dependencies and then run go mod tidy
# Delete all replace directives first
sed -i '/^replace /d' go.mod

# Add back the two specific replace directives we want to keep
echo "replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver => ../prometheusreceiver" >> go.mod
echo "replace github.com/prometheus-collector/shared => ../shared" >> go.mod
go mod tidy

# Update OtelCollector Version in main.go
sed -i -E "s|(.*Version: *)\"[0-9]*\.[0-9]*\.[0-9]*\"|\1\"${TAG#v}\"|g" main.go

# Get Prometheus Version OtelCollector is using
echo "Looking for Prometheus version in go.sum..."
PROM_VERSION_IN_GO_SUM=$(grep -m 1 "github.com/prometheus/prometheus v" go.sum | awk '{print $2}')
# Check if Prometheus version has changed
echo "Current Prometheus version in PROMETHEUS_VERSION file: $(cat PROMETHEUS_VERSION)"

# Convert Prometheus version from go module format to real release version
echo "Converting Prometheus version from go module format to real release version..."
if [[ $PROM_VERSION_IN_GO_SUM =~ ^v0\.3[0-9][0-9]\.[0-9]+ ]]; then
	# Extract the padded minor version and patch version
	PADDED_MINOR=$(echo $PROM_VERSION_IN_GO_SUM | sed -E 's/^v0\.3([0-9][0-9])\..*/\1/')
	PATCH_VERSION=$(echo $PROM_VERSION_IN_GO_SUM | sed -E 's/^v0\.3[0-9][0-9]\.([0-9]+).*/\1/')
	
	# Remove leading zero from minor version if present
	MINOR_VERSION=$(echo $PADDED_MINOR | sed 's/^0//')
	
	# Create the real Prometheus version
	REAL_PROM_VERSION="3.${MINOR_VERSION}.${PATCH_VERSION}"
	echo "Converting Prometheus version from $PROM_VERSION_IN_GO_SUM to $REAL_PROM_VERSION (real release version)"
	
	# Store both versions
	echo "$PROM_VERSION_IN_GO_SUM (go module) -> $REAL_PROM_VERSION (release)"
	#echo $REAL_PROM_VERSION > PROMETHEUS_VERSION
else
	echo "Prometheus version format not recognized: $PROM_VERSION_IN_GO_SUM"
	#echo $PROM_VERSION_IN_GO_SUM > PROMETHEUS_VERSION
fi

# Copy prometheus receiver changes into our repo
cd "$CURRENT_DIR"
rm -rf otelcollector/prometheusreceiver
mkdir -p otelcollector/prometheusreceiver
cp -r opentelemetry-collector-contrib/receiver/prometheusreceiver/* otelcollector/prometheusreceiver/
rm -rf otelcollector/prometheusreceiver/testdata

# Remove replacements at the end of go.mod
cd otelcollector/prometheusreceiver
sed -i '/^require (/,/^)/b; /^replace /d' go.mod
cd "$CURRENT_DIR"

# Step 3: Build opentelemetry-collector-builder
echo "Building opentelemetry-collector-builder..."
cd otelcollector/opentelemetry-collector-builder
go mod tidy
#make otelcollector
#rm -f otelcollector
cd "$CURRENT_DIR"

# Step 4: Update and build prom-config-validator-builder
echo "Updating prom-config-validator-builder..."
cd otelcollector/prom-config-validator-builder
# Update go.mod using opentelemetry-collector-builder go.mod
cp ../opentelemetry-collector-builder/go.mod .
sed -i '1s#.*#module github.com/microsoft/prometheus-collector/otelcollector/prom-config-validator-builder#' go.mod
go mod tidy
#make
#rm -f promconfigvalidator
cd "$CURRENT_DIR"

# Step 5: Golang version will be updated later after both go.mod files are available

# Get CHANGELOG.md from opentelemetry-collector-contrib
echo "Fetching CHANGELOG.md from opentelemetry-collector-contrib..."
echo "Current OTel version: $CURRENT_OTEL_VERSION"
if [ -f "opentelemetry-collector-contrib/CHANGELOG.md" ]; then
	echo "Getting CHANGELOG.md from opentelemetry-collector-contrib for version $CURRENT_OTEL_VERSION to $TAG..."
	./internal/otel-upgrade-scripts/changelogsummary.sh -f $CURRENT_OTEL_VERSION -t $TAG -c opentelemetry-collector-contrib/CHANGELOG.md -o PrometheusReceiverCHANGELOG.md --name "prometheusreceiver"
else
	echo "CHANGELOG.md not found in opentelemetry-collector-contrib, skipping summary generation"
fi

# Step 6: Clean up - remove opentelemetry-collector-contrib repo
echo "Cleaning up: removing opentelemetry-collector-contrib repo..."
if [ -d "opentelemetry-collector-contrib" ]; then
	rm -rf opentelemetry-collector-contrib
	echo "Removed opentelemetry-collector-contrib repo"
else
	echo "Directory opentelemetry-collector-contrib does not exist, skipping cleanup"
fi

# Step 8: Update Target Allocator
echo "Updating Target Allocator..."
echo "Using tag for the TA: $TA_TAG"
TAG=${TA_TAG:-$TAG}
BRANCH_NAME="${TAG}"
echo "TAG is now set to: $TAG"
if [ ! -d "opentelemetry-operator" ]; then
	git clone --depth 1 --branch $TAG https://github.com/open-telemetry/opentelemetry-operator.git
else
	# If directory exists, fetch only the specified tag
	cd opentelemetry-operator
	git fetch --depth 1 origin tag $TAG
	cd "$CURRENT_DIR"
	echo "Tag exists"
fi

cd opentelemetry-operator
echo "Changing into directory"
# Check if branch already exists
git branch | grep -q "$BRANCH_NAME" || true
RETURN_CODE=$?
echo "Return code: $RETURN_CODE"
echo "Checking if branch $BRANCH_NAME exists for tag $TAG"
if [ $RETURN_CODE -ne 0 ]; then
	# Branch doesn't exist, safe to create it
	echo "Creating new branch $BRANCH_NAME for tag $TAG"
	git checkout tags/$TAG -b $BRANCH_NAME
else
	# Branch exists, just check out the existing branch
	echo "Branch $BRANCH_NAME already exists, using existing branch"
	git checkout $BRANCH_NAME
fi
cd "$CURRENT_DIR"

# Backup existing Dockerfile and Makefile changes
cp otelcollector/otel-allocator/Dockerfile otelcollector/Dockerfile.backup
cp otelcollector/otel-allocator/Makefile otelcollector/Makefile.backup

# Copy otel-allocator
rm -rf otelcollector/otel-allocator
mkdir -p otelcollector/otel-allocator
cp -r opentelemetry-operator/cmd/otel-allocator/* otelcollector/otel-allocator/

echo "Restoring custom Dockerfile and Makefile"
cp otelcollector/Dockerfile.backup otelcollector/otel-allocator/Dockerfile
rm otelcollector/Dockerfile.backup
cp otelcollector/Makefile.backup otelcollector/otel-allocator/Makefile
rm otelcollector/Makefile.backup

# Update flags.go
sed -i '/import (/a\\tuberzap "go.uber.org/zap"' otelcollector/otel-allocator/internal/config/flags.go
sed -i '/zapCmdLineOpts.BindFlags(zapFlagSet)/a\\tlvl := uberzap.NewAtomicLevelAt(uberzap.PanicLevel)\n\tzapCmdLineOpts.Level = &lvl' otelcollector/otel-allocator/internal/config/flags.go

# Add the Arc EULA into the main.go file
echo "Adding Arc EULA to otel-allocator main.go file..."
sed -i '/func main() {/a\\t// EULA statement is required for Arc extension\n\tclusterResourceId := os.Getenv("CLUSTER")\n\tif strings.EqualFold(clusterResourceId, "connectedclusters") {\n\t\tsetupLog.Info("MICROSOFT SOFTWARE LICENSE TERMS\\n\\nMICROSOFT Azure Arc-enabled Kubernetes\\n\\nThis software is licensed to you as part of your or your company'\''s subscription license for Microsoft Azure Services. You may only use the software with Microsoft Azure Services and subject to the terms and conditions of the agreement under which you obtained Microsoft Azure Services. If you do not have an active subscription license for Microsoft Azure Services, you may not use the software. Microsoft Azure Legal Information: https://azure.microsoft.com/en-us/support/legal/")\n\t}' otelcollector/otel-allocator/main.go
if ! grep -q "\"strings\"" otelcollector/otel-allocator/main.go; then
	sed -i '/import (/a\\t"strings"' otelcollector/otel-allocator/main.go
fi

# Update go.mod
cd otelcollector/otel-allocator
cp "$CURRENT_DIR/opentelemetry-operator/go.mod" .
sed -i '1s#.*#module github.com/open-telemetry/opentelemetry-operator/cmd/otel-allocator#' go.mod
go mod tidy
#make
#rm -f targetallocator
cd "$CURRENT_DIR"

# Step 9: Update Configuration Reader Builder
echo "Updating Configuration Reader Builder..."
cd otelcollector/configuration-reader-builder
# Extract prometheus/common version from otel-allocator
PROM_COMMON_VERSION=$(cd "$CURRENT_DIR/otelcollector/otel-allocator" && grep "github.com/prometheus/common" go.mod | awk '{print $2}')
sed -i "s#github.com/prometheus/common .*#github.com/prometheus/common $PROM_COMMON_VERSION#g" go.mod
go mod tidy
#make
#rm -f configurationreader
cd "$CURRENT_DIR"

# Get CHANGELOG.md from opentelemetry-operator
echo "Fetching CHANGELOG.md from opentelemetry-operator..."
if [ -f "opentelemetry-operator/CHANGELOG.md" ]; then
	./internal/otel-upgrade-scripts/changelogsummary.sh -f ${CURRENT_TA_VERSION} -t ${TAG} -c opentelemetry-operator/CHANGELOG.md -o TargetAllocatorCHANGELOG.md --name "target-allocator"
else
	echo "CHANGELOG.md not found in opentelemetry-operator, skipping summary generation"
fi

# Step 6: Clean up - remove opentelemetry-collector-contrib repo
echo "Cleaning up: removing opentelemetry-operator repo..."
if [ -d "opentelemetry-operator" ]; then
	rm -rf opentelemetry-operator
	echo "Removed opentelemetry-operator repo"
else
	echo "Directory opentelemetry-operator does not exist, skipping cleanup"
fi

# Step 7: Update Prometheus UI dependencies
echo "Updating Prometheus UI dependencies..."
cd "$CURRENT_DIR/otelcollector/prometheus-ui"

# Extract prometheus/common version from opentelemetry-collector-builder
PROM_COMMON_VERSION=$(grep -m 1 "github.com/prometheus/common " ../opentelemetry-collector-builder/go.mod | awk '{print $2}')
PROM_EXPORTER_VERSION=$(grep -m 1 "github.com/prometheus/exporter-toolkit " ../opentelemetry-collector-builder/go.mod | awk '{print $2}')
OTELHTTP_VERSION=$(grep -m 1 "go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp " ../opentelemetry-collector-builder/go.mod | awk '{print $2}')

echo "Found Prometheus Common version: $PROM_COMMON_VERSION"
echo "Found Prometheus Exporter Toolkit version: $PROM_EXPORTER_VERSION"
echo "Found OTel HTTP version: $OTELHTTP_VERSION"

# Update the versions in go.mod
if [ ! -z "$PROM_COMMON_VERSION" ]; then
	sed -i "s|github.com/prometheus/common v[0-9.]*|github.com/prometheus/common $PROM_COMMON_VERSION|g" go.mod
fi

# Keep prometheus/common/assets as is since it might not be in the builder's go.mod

if [ ! -z "$PROM_EXPORTER_VERSION" ]; then
	sed -i "s|github.com/prometheus/exporter-toolkit v[0-9.]*|github.com/prometheus/exporter-toolkit $PROM_EXPORTER_VERSION|g" go.mod
fi

if [ ! -z "$OTELHTTP_VERSION" ]; then
	sed -i "s|go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v[0-9.]*|go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp $OTELHTTP_VERSION|g" go.mod
fi

# Remove "// indirect" dependencies from go.mod
echo "Removing indirect dependencies from go.mod..."
grep -v "// indirect" go.mod > go.mod.tmp && mv go.mod.tmp go.mod

# Run go mod tidy to update the go.sum file
go mod tidy

echo "Prometheus UI dependencies updated successfully."

cd "$CURRENT_DIR"

# Step 7.1: Update Test Utils Prometheus dependencies
echo "Updating Test Utils Prometheus dependencies..."
cd "$CURRENT_DIR/otelcollector/test/ginkgo-e2e/utils"

# Extract prometheus versions from both opentelemetry-collector-builder and otel-allocator
OTEL_BUILDER_PROM_CLIENT_VERSION=$(grep -m 1 "github.com/prometheus/client_golang " ../../../opentelemetry-collector-builder/go.mod | awk '{print $2}')
OTEL_BUILDER_PROM_COMMON_VERSION=$(grep -m 1 "github.com/prometheus/common " ../../../opentelemetry-collector-builder/go.mod | awk '{print $2}')
OTEL_BUILDER_PROM_CLIENT_MODEL_VERSION=$(grep -m 1 "github.com/prometheus/client_model " ../../../opentelemetry-collector-builder/go.mod | awk '{print $2}')

OTEL_ALLOCATOR_PROM_CLIENT_VERSION=$(grep -m 1 "github.com/prometheus/client_golang " ../../../otel-allocator/go.mod | awk '{print $2}')
OTEL_ALLOCATOR_PROM_COMMON_VERSION=$(grep -m 1 "github.com/prometheus/common " ../../../otel-allocator/go.mod | awk '{print $2}')
OTEL_ALLOCATOR_PROM_CLIENT_MODEL_VERSION=$(grep -m 1 "github.com/prometheus/client_model " ../../../otel-allocator/go.mod | awk '{print $2}')

echo "Found Prometheus versions in opentelemetry-collector-builder:"
echo "  client_golang: $OTEL_BUILDER_PROM_CLIENT_VERSION"
echo "  common: $OTEL_BUILDER_PROM_COMMON_VERSION" 
echo "  client_model: $OTEL_BUILDER_PROM_CLIENT_MODEL_VERSION"

echo "Found Prometheus versions in otel-allocator:"
echo "  client_golang: $OTEL_ALLOCATOR_PROM_CLIENT_VERSION"
echo "  common: $OTEL_ALLOCATOR_PROM_COMMON_VERSION"
echo "  client_model: $OTEL_ALLOCATOR_PROM_CLIENT_MODEL_VERSION"

# Function to compare and select highest version
select_highest_version() {
    local version1=$1
    local version2=$2
    if [[ -z "$version1" ]]; then
        echo "$version2"
    elif [[ -z "$version2" ]]; then
        echo "$version1"
    elif [[ $version1 > $version2 ]]; then
        echo "$version1"
    else
        echo "$version2"
    fi
}

# Select the highest versions
FINAL_PROM_CLIENT_VERSION=$(select_highest_version "$OTEL_BUILDER_PROM_CLIENT_VERSION" "$OTEL_ALLOCATOR_PROM_CLIENT_VERSION")
FINAL_PROM_COMMON_VERSION=$(select_highest_version "$OTEL_BUILDER_PROM_COMMON_VERSION" "$OTEL_ALLOCATOR_PROM_COMMON_VERSION")
FINAL_PROM_CLIENT_MODEL_VERSION=$(select_highest_version "$OTEL_BUILDER_PROM_CLIENT_MODEL_VERSION" "$OTEL_ALLOCATOR_PROM_CLIENT_MODEL_VERSION")

echo "Selected final Prometheus versions for test utils:"
echo "  client_golang: $FINAL_PROM_CLIENT_VERSION"
echo "  common: $FINAL_PROM_COMMON_VERSION"
echo "  client_model: $FINAL_PROM_CLIENT_MODEL_VERSION"

# Update the versions in go.mod
if [ ! -z "$FINAL_PROM_CLIENT_VERSION" ]; then
    sed -i "s|github.com/prometheus/client_golang v[0-9.]*|github.com/prometheus/client_golang $FINAL_PROM_CLIENT_VERSION|g" go.mod
fi

if [ ! -z "$FINAL_PROM_COMMON_VERSION" ]; then
    sed -i "s|github.com/prometheus/common v[0-9.]*|github.com/prometheus/common $FINAL_PROM_COMMON_VERSION|g" go.mod
fi

if [ ! -z "$FINAL_PROM_CLIENT_MODEL_VERSION" ]; then
    sed -i "s|github.com/prometheus/client_model v[0-9.]*|github.com/prometheus/client_model $FINAL_PROM_CLIENT_MODEL_VERSION|g" go.mod
fi

# Remove "// indirect" dependencies from go.mod
echo "Removing indirect dependencies from go.mod..."
grep -v "// indirect" go.mod > go.mod.tmp && mv go.mod.tmp go.mod

# Run go mod tidy to update the go.sum file
go mod tidy

echo "Test Utils Prometheus dependencies updated successfully."

cd "$CURRENT_DIR"

# Step 7.2: Update PrometheusUI Test Prometheus dependencies
echo "Updating PrometheusUI Test Prometheus dependencies..."
cd "$CURRENT_DIR/otelcollector/test/ginkgo-e2e/prometheusui"

echo "Using final Prometheus versions for prometheusui test:"
echo "  client_golang: $FINAL_PROM_CLIENT_VERSION"
echo "  common: $FINAL_PROM_COMMON_VERSION"
echo "  client_model: $FINAL_PROM_CLIENT_MODEL_VERSION"

# Update the versions in go.mod (reusing variables from Step 7.1)
if [ ! -z "$FINAL_PROM_CLIENT_VERSION" ]; then
    sed -i "s|github.com/prometheus/client_golang v[0-9.]*|github.com/prometheus/client_golang $FINAL_PROM_CLIENT_VERSION|g" go.mod
fi

if [ ! -z "$FINAL_PROM_COMMON_VERSION" ]; then
    sed -i "s|github.com/prometheus/common v[0-9.]*|github.com/prometheus/common $FINAL_PROM_COMMON_VERSION|g" go.mod
fi

if [ ! -z "$FINAL_PROM_CLIENT_MODEL_VERSION" ]; then
    sed -i "s|github.com/prometheus/client_model v[0-9.]*|github.com/prometheus/client_model $FINAL_PROM_CLIENT_MODEL_VERSION|g" go.mod
fi

# Special handling for prometheus/prometheus (only extract these since we need them)
OTEL_ALLOCATOR_PROM_PROMETHEUS_VERSION=$(grep -m 1 "github.com/prometheus/prometheus " ../../../otel-allocator/go.mod | awk '{print $2}')
OTEL_BUILDER_PROM_PROMETHEUS_VERSION=$(grep -m 1 "github.com/prometheus/prometheus " ../../../opentelemetry-collector-builder/go.mod | awk '{print $2}')
FINAL_PROM_PROMETHEUS_VERSION=$(select_highest_version "$OTEL_BUILDER_PROM_PROMETHEUS_VERSION" "$OTEL_ALLOCATOR_PROM_PROMETHEUS_VERSION")

echo "Found prometheus/prometheus versions:"
echo "  otel-allocator: $OTEL_ALLOCATOR_PROM_PROMETHEUS_VERSION"
echo "  otel-builder: $OTEL_BUILDER_PROM_PROMETHEUS_VERSION"
echo "  selected: $FINAL_PROM_PROMETHEUS_VERSION"

if [ ! -z "$FINAL_PROM_PROMETHEUS_VERSION" ]; then
    # This regex handles both regular versions and pre-release versions with commit hashes
    # Escape special characters in the version string for sed
    ESCAPED_VERSION=$(echo "$FINAL_PROM_PROMETHEUS_VERSION" | sed 's/[[\.*^$()+?{|]/\\&/g')
    sed -i "s|github.com/prometheus/prometheus v[0-9][0-9.]*[^[:space:]]*|github.com/prometheus/prometheus $ESCAPED_VERSION|g" go.mod
fi

# Remove "// indirect" dependencies from go.mod
echo "Removing indirect dependencies from go.mod..."
grep -v "// indirect" go.mod > go.mod.tmp && mv go.mod.tmp go.mod

# Run go mod tidy to update the go.sum file
go mod tidy

echo "PrometheusUI Test Prometheus dependencies updated successfully."

cd "$CURRENT_DIR"

# Step 7.3: Run go mod tidy in all test/ginkgo-e2e subdirectories
echo "Running go mod tidy in all test/ginkgo-e2e subdirectories..."

# Start with utils directory
echo "Running go mod tidy in utils directory..."
cd "$CURRENT_DIR/otelcollector/test/ginkgo-e2e/utils"
go mod tidy
cd "$CURRENT_DIR"

# Get all subdirectories except utils
for dir in $(find "$CURRENT_DIR/otelcollector/test/ginkgo-e2e" -maxdepth 1 -type d | grep -v "/utils$" | grep -v "^$CURRENT_DIR/otelcollector/test/ginkgo-e2e$"); do
    echo "Running go mod tidy in $(basename $dir) directory..."
    cd "$dir"
    go mod tidy
    cd "$CURRENT_DIR"
done

echo "Completed go mod tidy in all test/ginkgo-e2e subdirectories"

# Step 8: Update golang version in azure-pipeline-build.yaml using highest version
echo "Updating golang version in azure-pipeline-build.yaml..."

# Function to compare version strings
version_compare() {
    if [[ $1 == $2 ]]; then
        echo "0"
    elif [[ $1 > $2 ]]; then
        echo "1"
    else
        echo "-1"
    fi
}

# Get Go versions from both go.mod files
OTEL_CONTRIB_GO_VERSION=$(grep "^go " otelcollector/opentelemetry-collector-builder/go.mod | awk '{print $2}')
OTEL_ALLOCATOR_GO_VERSION=$(grep "^go " otelcollector/otel-allocator/go.mod | awk '{print $2}')

echo "Go version from opentelemetry-collector-contrib: $OTEL_CONTRIB_GO_VERSION"
echo "Go version from otel-allocator: $OTEL_ALLOCATOR_GO_VERSION"

# Compare versions and use the highest one
COMPARISON=$(version_compare "$OTEL_CONTRIB_GO_VERSION" "$OTEL_ALLOCATOR_GO_VERSION")
if [[ $COMPARISON -eq 1 ]]; then
    GO_VERSION=$OTEL_CONTRIB_GO_VERSION
    echo "Using Go version from opentelemetry-collector-contrib: $GO_VERSION"
elif [[ $COMPARISON -eq -1 ]]; then
    GO_VERSION=$OTEL_ALLOCATOR_GO_VERSION
    echo "Using Go version from otel-allocator: $GO_VERSION"
else
    GO_VERSION=$OTEL_CONTRIB_GO_VERSION
    echo "Both versions are the same, using: $GO_VERSION"
fi

# Extract current golang version from the pipeline file
CURRENT_GO_VERSION=$(grep "GOLANG_VERSION: '" ".pipelines/azure-pipeline-build.yml" | head -1 | sed "s/.*GOLANG_VERSION: '//;s/'.*//g")
CURRENT_GO_MAJOR_MINOR=$(echo $CURRENT_GO_VERSION | grep -oE '^[0-9]+\.[0-9]+')
NEW_GO_MAJOR_MINOR=$(echo $GO_VERSION | grep -oE '^[0-9]+\.[0-9]+')

echo "Current Golang version in pipeline: $CURRENT_GO_VERSION"
echo "Selected Golang version from go.mod files: $GO_VERSION"

# Function to get latest patch version from Microsoft container registry
get_latest_patch_version() {
    local major_minor=$1
    local registry_url="mcr.microsoft.com"
    local repository="oss/go/microsoft/golang"
    
    # Check if registry tools are available
    if [ "$REGISTRY_TOOLS_AVAILABLE" = false ]; then
        echo "$GO_VERSION"
        return
    fi
    
    echo "Checking for latest patch version for Go $major_minor from $registry_url/$repository..." >&2
    
    # Use curl to query the MCR API for tags with timeout
    local api_url="https://$registry_url/v2/$repository/tags/list"
    local available_tags
    available_tags=$(timeout 30 curl -s -f "$api_url" 2>/dev/null | jq -r '.tags[]?' 2>/dev/null || true)
    
    if [ -z "$available_tags" ]; then
        echo "Warning: Could not fetch tags from registry (timeout or network issue), falling back to original version" >&2
        echo "$GO_VERSION"
        return
    fi
    
    # Filter tags that match the major.minor version pattern and find the latest patch
    local latest_patch=""
    local highest_patch=-1
    local found_versions=""
    
    while IFS= read -r tag; do
        # Skip empty lines
        [ -z "$tag" ] && continue
        
        # Check if tag matches the pattern major.minor.patch (e.g., 1.21.5)
        if [[ $tag =~ ^${major_minor}\.([0-9]+)$ ]]; then
            local patch_num="${BASH_REMATCH[1]}"
            found_versions="$found_versions $tag"
            if [ "$patch_num" -gt "$highest_patch" ]; then
                highest_patch=$patch_num
                latest_patch="$tag"
            fi
        fi
    done <<< "$available_tags"
    
    if [ -n "$latest_patch" ]; then
        echo "Found versions for $major_minor:$found_versions" >&2
        echo "Selected latest patch version: $latest_patch" >&2
        echo "$latest_patch"
    else
        echo "No matching patch versions found in registry for $major_minor, using original version" >&2
        echo "$GO_VERSION"
    fi
}

# Only update if major.minor version is different or if we need to get latest patch
if [ "$CURRENT_GO_MAJOR_MINOR" != "$NEW_GO_MAJOR_MINOR" ]; then
    echo "Major.minor version changed, checking for latest patch version..."
    FINAL_GO_VERSION=$(get_latest_patch_version "$NEW_GO_MAJOR_MINOR")
    echo "Updating Golang version in pipeline from $CURRENT_GO_VERSION to $FINAL_GO_VERSION"
    sed -i "s/^  GOLANG_VERSION: '.*'/  GOLANG_VERSION: '${FINAL_GO_VERSION}'/g" ".pipelines/azure-pipeline-build.yml"
else
    echo "Major.minor version unchanged ($NEW_GO_MAJOR_MINOR), checking if we should update to latest patch..."
    FINAL_GO_VERSION=$(get_latest_patch_version "$NEW_GO_MAJOR_MINOR")
    
    if [ "$CURRENT_GO_VERSION" != "$FINAL_GO_VERSION" ]; then
        echo "Updating to latest patch version from $CURRENT_GO_VERSION to $FINAL_GO_VERSION"
        sed -i "s/^  GOLANG_VERSION: '.*'/  GOLANG_VERSION: '${FINAL_GO_VERSION}'/g" ".pipelines/azure-pipeline-build.yml"
    else
        echo "Already using the latest patch version: $CURRENT_GO_VERSION"
    fi
fi

echo "Upgrade process complete!"
