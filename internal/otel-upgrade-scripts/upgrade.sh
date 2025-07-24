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

# Apply custom changes for asset store fixes and secret watching
echo "Applying custom changes for asset store fixes and secret watching..."
echo "These changes are based on commit 341e15c4014e14cff73c5a4b2b9dabaff8fd8aa9"
echo "Fix for watching secret updates for CRs - adds informer for secrets and updates asset store when secrets are updated/deleted"

# Apply promOperator.go changes
cat > promoperator_patch.py << 'EOF'
import re

# Read the file
with open('otelcollector/otel-allocator/internal/watcher/promOperator.go', 'r') as f:
    content = f.read()

# 1. Add metadata import after kubernetes import
if '"k8s.io/client-go/metadata"' not in content:
    content = re.sub(
        r'(\s*"k8s\.io/client-go/kubernetes"\s*\n)',
        r'\1\t"k8s.io/client-go/metadata"\n',
        content
    )

# 2. Add metadata client creation
if 'mdClient, err := metadata.NewForConfig' not in content:
    content = re.sub(
        r'(mClient, err := versioned\.NewForConfig\(cfg\.ClusterConfig\)\s*\n\s*if err != nil \{\s*\n\s*return nil, err\s*\n\s*\})',
        r'\1\n\n\tmdClient, err := metadata.NewForConfig(cfg.ClusterConfig)\n\tif err != nil {\n\t\treturn nil, err\n\t}',
        content, flags=re.DOTALL
    )

# 3. Update factory creation
content = re.sub(
    r'factory := informers\.NewMonitoringInformerFactories\(allowList, denyList, mClient, allocatorconfig\.DefaultResyncTime, nil\)',
    'monitoringInformerFactory := informers.NewMonitoringInformerFactories(allowList, denyList, mClient, allocatorconfig.DefaultResyncTime, nil)\n\tmetaDataInformerFactory := informers.NewMetadataInformerFactory(allowList, denyList, mdClient, allocatorconfig.DefaultResyncTime, nil)',
    content
)

# 4. Update getInformers call
content = re.sub(
    r'monitoringInformers, err := getInformers\(factory\)',
    'monitoringInformers, err := getInformers(monitoringInformerFactory, metaDataInformerFactory)',
    content
)

# 5. Update getInformers function signature
content = re.sub(
    r'func getInformers\(factory informers\.FactoriesForNamespaces\) \(map\[string\]\*informers\.ForResource, error\) \{',
    'func getInformers(factory informers.FactoriesForNamespaces, metaDataInformerFactory informers.FactoriesForNamespaces) (map[string]*informers.ForResource, error) {',
    content
)

# 6. Add secretInformers creation after scrapeConfigInformers
if 'secretInformers, err := informers.NewInformersForResourceWithTransform' not in content:
    content = re.sub(
        r'(scrapeConfigInformers, err := informers\.NewInformersForResource\(factory, promv1alpha1\.SchemeGroupVersion\.WithResource\(promv1alpha1\.ScrapeConfigName\)\)\s*if err != nil \{\s*return nil, err\s*\})',
        r'\1\n\n\tsecretInformers, err := informers.NewInformersForResourceWithTransform(metaDataInformerFactory, v1.SchemeGroupVersion.WithResource(string(v1.ResourceSecrets)), informers.PartialObjectMetadataStrip)\n\tif err != nil {\n\t\treturn nil, err\n\t}',
        content, flags=re.DOTALL
    )

# 7. Add secrets to return map
if 'string(v1.ResourceSecrets):' not in content:
    content = re.sub(
        r'(promv1alpha1\.ScrapeConfigName:\s+scrapeConfigInformers,)',
        r'\1\n\t\tstring(v1.ResourceSecrets):      secretInformers,',
        content
    )

# 8. Replace the event handler section with custom logic
event_handler_pattern = r'(\s+)// only send an event notification if there isn\'t one already\s*resource\.AddEventHandler\(cache\.ResourceEventHandlerFuncs\{[^}]*}\)\s*}\)\s*'
if re.search(event_handler_pattern, content, re.DOTALL):
    replacement = '''
		// Use a custom event handler for secrets since secret update requires asset store to be updated so that CRs can pick up updated secrets.
		if name == string(v1.ResourceSecrets) {
			w.logger.Info("Using custom event handler for secrets informer", "informer", name)
			// only send an event notification if there isn't one already
			resource.AddEventHandler(cache.ResourceEventHandlerFuncs{
				// these functions only write to the notification channel if it's empty to avoid blocking
				// if scrape config updates are being rate-limited
				AddFunc: func(obj interface{}) {
					select {
					case notifyEvents <- struct{}{}:
					default:
					}
				},
				UpdateFunc: func(oldObj, newObj interface{}) {
					oldMeta, _ := oldObj.(metav1.ObjectMetaAccessor)
					newMeta, _ := newObj.(metav1.ObjectMetaAccessor)
					secretName := newMeta.GetObjectMeta().GetName()
					secretNamespace := newMeta.GetObjectMeta().GetNamespace()
					_, exists, err := w.store.GetObject(&v1.Secret{
						ObjectMeta: metav1.ObjectMeta{
							Name:      secretName,
							Namespace: secretNamespace,
						},
					})
					if !exists || err != nil {
						if err != nil {
							w.logger.Error("unexpected store error when checking if secret exists, skipping update", secretName, "error", err)
							return
						}
						// if the secret does not exist in the store, we skip the update
						return
					}

					newSecret, err := w.store.GetSecretClient().Secrets(secretNamespace).Get(context.Background(), secretName, metav1.GetOptions{})

					if err != nil {
						w.logger.Error("unexpected store error when getting updated secret - ", secretName, "error", err)
						return
					}

					w.logger.Info("Updating secret in store", "newObjName", newMeta.GetObjectMeta().GetName(), "newobjnamespace", newMeta.GetObjectMeta().GetNamespace())
					if err := w.store.UpdateObject(newSecret); err != nil {
						w.logger.Error("unexpected store error when updating secret  - ", newMeta.GetObjectMeta().GetName(), "error", err)
					} else {
						w.logger.Info(
							"Successfully updated store, sending update event to notifyEvents channel",
							"oldObjName", oldMeta.GetObjectMeta().GetName(),
							"oldobjnamespace", oldMeta.GetObjectMeta().GetNamespace(),
							"newObjName", newMeta.GetObjectMeta().GetName(),
							"newobjnamespace", newMeta.GetObjectMeta().GetNamespace(),
						)
						select {
						case notifyEvents <- struct{}{}:
						default:
						}
					}
				},
				DeleteFunc: func(obj interface{}) {
					secretMeta, _ := obj.(metav1.ObjectMetaAccessor)

					secretName := secretMeta.GetObjectMeta().GetName()
					secretNamespace := secretMeta.GetObjectMeta().GetNamespace()

					// check if the secret exists in the store
					secretObj := &v1.Secret{
						ObjectMeta: metav1.ObjectMeta{
							Name:      secretName,
							Namespace: secretNamespace,
						},
					}
					_, exists, err := w.store.GetObject(secretObj)
					// if the secret does not exist in the store, we skip the delete
					if !exists || err != nil {
						if err != nil {
							w.logger.Error("unexpected store error when checking if secret exists, skipping delete", secretMeta.GetObjectMeta().GetName(), "error", err)
							return
						}
						// if the secret does not exist in the store, we skip the delete
						return
					}
					w.logger.Info("Deleting secret from store", "objName", secretMeta.GetObjectMeta().GetName(), "objnamespace", secretMeta.GetObjectMeta().GetNamespace())
					// if the secret exists in the store, we delete it
					// and send an event notification to the notifyEvents channel
					if err := w.store.DeleteObject(secretObj); err != nil {
						w.logger.Error("unexpected store error when deleting secret - ", secretMeta.GetObjectMeta().GetName(), "error", err)
						//return
					} else {
						w.logger.Info(
							"Successfully removed secret from store, sending update event to notifyEvents channel",
							"objName", secretMeta.GetObjectMeta().GetName(),
							"objnamespace", secretMeta.GetObjectMeta().GetNamespace(),
						)
						select {
						case notifyEvents <- struct{}{}:
						default:
						}
					}
				},
			})
		} else {
			w.logger.Info("Using default event handler for informer", "informer", name)
			// only send an event notification if there isn't one already
			resource.AddEventHandler(cache.ResourceEventHandlerFuncs{
				// these functions only write to the notification channel if it's empty to avoid blocking
				// if scrape config updates are being rate-limited
				AddFunc: func(obj interface{}) {
					select {
					case notifyEvents <- struct{}{}:
					default:
					}
				},
				UpdateFunc: func(oldObj, newObj interface{}) {
					select {
					case notifyEvents <- struct{}{}:
					default:
					}
				},
				DeleteFunc: func(obj interface{}) {
					select {
					case notifyEvents <- struct{}{}:
					default:
					}
				},
			})
		}
'''
    content = re.sub(event_handler_pattern, replacement, content, flags=re.DOTALL)

with open('otelcollector/otel-allocator/internal/watcher/promOperator.go', 'w') as f:
    f.write(content)
EOF

python3 promoperator_patch.py
#rm promoperator_patch.py

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

# Add custom prometheus-operator replace directive
echo "Adding custom prometheus-operator replace directive (from commit 341e15c4014e14cff73c5a4b2b9dabaff8fd8aa9)..."
echo "" >> go.mod
echo "// pointing to this fork for prometheus-operator since we need fixes for asset store which is only available from v0.84.0 of prometheus-operator" >> go.mod
echo "// targetallocator cannot upgrade to v0.84.0 because of this issue - https://github.com/open-telemetry/opentelemetry-operator/issues/4196" >> go.mod
echo "// this commit is from this repository -https://github.com/rashmichandrashekar/prometheus-operator/tree/rashmi/v0.81.0-patch-assetstore - which only has the asset store fixes on top of v0.81.0 of prometheus-operator" >> go.mod
echo "replace github.com/prometheus-operator/prometheus-operator => github.com/rashmichandrashekar/prometheus-operator v0.0.0-20250715221118-b55ea6d3c138" >> go.mod

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

echo "Upgrade process complete!"
