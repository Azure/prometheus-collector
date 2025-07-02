#!/bin/bash
set -e

# Display usage
if [ $# -ne 2 ]; then
	echo "Usage: $0 <collector_tag_version> [stable_tag_version]"
	echo "Example: $0 v0.123.0 v1.29.0"
	exit 1
fi

TAG=$1
STABLE_TAG=$2
BRANCH_NAME="${TAG}"
CURRENT_DIR=$(pwd)

# Function to extract version numbers from git tags
get_current_otel_version() {
	cd otelcollector/opentelemetry-collector-builder
	CURRENT_VERSION=$(grep -m 1 "go.opentelemetry.io/collector " go.mod | awk '{print $2}')
	cd "$CURRENT_DIR"
	echo "$CURRENT_VERSION"
}

# Get current OTel version for reference
CURRENT_OTEL_VERSION=$(get_current_otel_version)
echo "Current OTel version: $CURRENT_OTEL_VERSION"

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

# Step 5: Update golang version in azure-pipeline-build.yaml
echo "Updating golang version in azure-pipeline-build.yaml..."
GO_VERSION=$(grep "go " opentelemetry-collector-contrib/go.mod | awk '{print $2}')
# Extract current golang version from the pipeline file
CURRENT_GO_VERSION=$(grep "GOLANG_VERSION: '" ".pipelines/azure-pipeline-build.yml" | sed "s/GOLANG_VERSION: '//;s/'//g")
CURRENT_GO_MAJOR_MINOR=$(echo $CURRENT_GO_VERSION | grep -oE '^[0-9]+\.[0-9]+')
NEW_GO_MAJOR_MINOR=$(echo $GO_VERSION | grep -oE '^[0-9]+\.[0-9]+')

echo "Current Golang version in pipeline: $CURRENT_GO_VERSION"
echo "Golang version using by otelcollector go.mod: $GO_VERSION"

# Only update if major.minor version is different
if [ "$CURRENT_GO_MAJOR_MINOR" != "$NEW_GO_MAJOR_MINOR" ]; then
	echo "Updating Golang version in pipeline from $CURRENT_GO_VERSION to $GO_VERSION"
	sed -i "s/GOLANG_VERSION: '.*'/GOLANG_VERSION: '${GO_VERSION}'/g" ".pipelines/azure-pipeline-build.yml"
else
	echo "Golang major.minor version unchanged, keeping current version in pipeline"
fi

# Get CHANGELOG.md from opentelemetry-collector-contrib
echo "Fetching CHANGELOG.md from opentelemetry-collector-contrib..."
if [ -f "opentelemetry-collector-contrib/CHANGELOG.md" ]; then
	./internal/otel-upgrade-scripts/changelogsummary.sh -f ${CURRENT_OTEL_VERSION} -t ${TAG} -c opentelemetry-collector-contrib/CHANGELOG.md -o PrometheusReceiverCHANGELOG.md --name "prometheusreceiver"
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
if [ $RETURN_CODE -ne 0 ]; then
	# Branch doesn't exist, safe to create it
	git checkout tags/$TAG -b $BRANCH_NAME
else
	# Branch exists, just check out the existing branch
	git checkout $BRANCH_NAME
	echo "Branch $BRANCH_NAME already exists, using existing branch"
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
sed -i '/cfg, err := config.Load(os.Args)/i\\t// EULA statement is required for Arc extension\n\tclusterResourceId := os.Getenv("CLUSTER")\n\tif strings.EqualFold(clusterResourceId, "connectedclusters") {\n\t\setupLog.Info("MICROSOFT SOFTWARE LICENSE TERMS\\n\\nMICROSOFT Azure Arc-enabled Kubernetes\\n\\nThis software is licensed to you as part of your or your company'\''s subscription license for Microsoft Azure Services. You may only use the software with Microsoft Azure Services and subject to the terms and conditions of the agreement under which you obtained Microsoft Azure Services. If you do not have an active subscription license for Microsoft Azure Services, you may not use the software. Microsoft Azure Legal Information: https://azure.microsoft.com/en-us/support/legal/")\n\t}' otelcollector/otel-allocator/main.go
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
	./internal/otel-upgrade-scripts/changelogsummary.sh -f ${CURRENT_OTEL_VERSION} -t ${TAG} -c opentelemetry-operator/CHANGELOG.md -o TargetAllocatorCHANGELOG.md --name "target-allocator"
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

echo "Upgrade process complete!"
