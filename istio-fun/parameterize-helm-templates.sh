#!/bin/bash

# Parameterize Helm chart templates to support custom namespace via values
# This implements the proper Helm templating approach using helpers

set -e

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CHART_DIR="${SCRIPT_DIR}/../otelcollector/deploy/addon-chart/azure-monitor-metrics-addon"
TEMPLATES_DIR="${CHART_DIR}/templates"
HELPERS_FILE="${TEMPLATES_DIR}/_helpers.tpl"
VALUES_FILE="${CHART_DIR}/values-template.yaml"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

print_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

echo "=========================================="
echo "Parameterizing Helm Templates"
echo "=========================================="
echo "Chart directory: ${CHART_DIR}"
echo ""

# Check if directories exist
if [ ! -d "$TEMPLATES_DIR" ]; then
    print_error "Templates directory not found: $TEMPLATES_DIR"
    exit 1
fi

# Create backup
BACKUP_DIR="${CHART_DIR}.backup-$(date +%s)"
print_info "Creating backup: ${BACKUP_DIR}"
cp -r "$CHART_DIR" "$BACKUP_DIR"

# Step 1: Create or update _helpers.tpl
print_info "Step 1: Creating/updating _helpers.tpl..."

if [ -f "$HELPERS_FILE" ]; then
    print_warning "_helpers.tpl already exists, appending to it"
    echo "" >> "$HELPERS_FILE"
fi

cat >> "$HELPERS_FILE" << 'EOF'

{{/*
Get the namespace for deployment
Defaults to kube-system for backward compatibility
*/}}
{{- define "ama-metrics.namespace" -}}
{{- .Values.namespace | default "kube-system" }}
{{- end }}

{{/*
Get the secret namespace (always kube-system for addon-token-adapter)
*/}}
{{- define "ama-metrics.secretNamespace" -}}
kube-system
{{- end }}

{{/*
Get the configmap namespace (same as deployment namespace)
*/}}
{{- define "ama-metrics.configmapNamespace" -}}
{{- include "ama-metrics.namespace" . }}
{{- end }}
EOF

print_info "✓ Helper templates added to _helpers.tpl"

# Step 2: Update values-template.yaml
print_info "Step 2: Updating values-template.yaml..."

# Check if namespace parameter already exists
if grep -q "^namespace:" "$VALUES_FILE"; then
    print_warning "namespace parameter already exists in values-template.yaml"
else
    # Add namespace parameter at the top after any comments
    TEMP_FILE=$(mktemp)
    
    # Add namespace parameter after the first non-comment line
    awk '
        BEGIN { added = 0 }
        /^[^#]/ && added == 0 { 
            print "# Custom namespace for deployment (defaults to kube-system for compatibility)"
            print "namespace: \"kube-system\""
            print ""
            added = 1
        }
        { print }
    ' "$VALUES_FILE" > "$TEMP_FILE"
    
    mv "$TEMP_FILE" "$VALUES_FILE"
    print_info "✓ Added namespace parameter to values-template.yaml"
fi

# Step 3: Update all template files
print_info "Step 3: Updating template files..."

TEMPLATE_FILES=(
    "ama-metrics-serviceAccount.yaml"
    "ama-metrics-deployment.yaml"
    "ama-metrics-daemonset.yaml"
    "ama-metrics-ksm-deployment.yaml"
    "ama-metrics-ksm-service.yaml"
    "ama-metrics-ksm-serviceaccount.yaml"
    "ama-metrics-ksm-clusterrolebinding.yaml"
    "ama-metrics-clusterRoleBinding.yaml"
    "ama-metrics-targetallocator.yaml"
    "ama-metrics-targetallocator-service.yaml"
    "ama-metrics-secret.yaml"
    "ama-metrics-collector-hpa.yaml"
    "ama-metrics-pod-disruption-budget.yaml"
    "ama-metrics-extensionIdentity.yaml"
    "ama-metrics-role.yaml"
    "ama-metrics-roleBinding.yaml"
)

# Also find any other .yaml files
ALL_TEMPLATES=$(find "$TEMPLATES_DIR" -name "*.yaml" -type f ! -name "_*")

# Parameterize namespace in metadata
print_info "  - Parameterizing metadata.namespace..."
for template in $ALL_TEMPLATES; do
    if [ -f "$template" ]; then
        # Replace hardcoded namespace in metadata with helper
        sed -i.bak 's/^  namespace: kube-system$/  namespace: {{ include "ama-metrics.namespace" . }}/g' "$template"
        
        # Also handle ClusterRoleBinding subjects namespace
        sed -i.bak 's/^    namespace: kube-system$/    namespace: {{ include "ama-metrics.namespace" . }}/g' "$template"
    fi
done

# Parameterize --configmap-namespace args
print_info "  - Parameterizing --configmap-namespace arguments..."
for template in $ALL_TEMPLATES; do
    if [ -f "$template" ]; then
        sed -i.bak 's|--configmap-namespace=kube-system|--configmap-namespace={{ include "ama-metrics.configmapNamespace" . }}|g' "$template"
    fi
done

# Keep --secret-namespace as kube-system (addon-token-adapter is always there)
print_info "  - Keeping --secret-namespace=kube-system (for addon-token-adapter)..."
# This should already be kube-system, but let's be explicit
for template in $ALL_TEMPLATES; do
    if [ -f "$template" ]; then
        # If someone changed it, revert it back
        sed -i.bak 's|--secret-namespace=.*|--secret-namespace={{ include "ama-metrics.secretNamespace" . }}|g' "$template"
    fi
done

# Clean up backup files
find "$TEMPLATES_DIR" -name "*.bak" -delete

# Step 4: Verify changes
print_info "Step 4: Verifying changes..."
echo ""

HELPER_COUNT=$(grep -c "ama-metrics.namespace" "$HELPERS_FILE" || true)
TEMPLATE_COUNT=$(grep -r "include \"ama-metrics.namespace\"" "$TEMPLATES_DIR" --include="*.yaml" | wc -l)
HARDCODED_COUNT=$(grep -r "namespace: kube-system" "$TEMPLATES_DIR" --include="*.yaml" | grep -v "secretNamespace" | wc -l || true)

echo "  ✓ Helper functions in _helpers.tpl: $HELPER_COUNT"
echo "  ✓ Template files using helpers: $TEMPLATE_COUNT"
echo "  ✓ Remaining hardcoded kube-system: $HARDCODED_COUNT (should be 0)"

if [ $HARDCODED_COUNT -gt 0 ]; then
    print_warning "Found $HARDCODED_COUNT hardcoded kube-system references"
    print_warning "Files with hardcoded namespace:"
    grep -r "namespace: kube-system" "$TEMPLATES_DIR" --include="*.yaml" -l | grep -v "secretNamespace" || true
fi

echo ""
print_info "=========================================="
print_info "Parameterization complete!"
print_info "=========================================="
echo ""
print_info "Changes made:"
echo "  1. ✓ Created/updated _helpers.tpl with namespace helpers"
echo "  2. ✓ Added namespace parameter to values-template.yaml"
echo "  3. ✓ Parameterized all template files"
echo ""
print_info "Backup location: ${BACKUP_DIR}"
echo ""
print_info "Usage:"
echo "  1. Set namespace in your values file:"
echo "     echo 'namespace: \"monitoring\"' > my-values.yaml"
echo ""
echo "  2. Or override on command line:"
echo "     helm install ama-metrics ${CHART_DIR} \\"
echo "       --set namespace=monitoring \\"
echo "       --namespace monitoring"
echo ""
echo "  3. Use with istio-configs/custom-istio-values.yaml:"
echo "     helm install ama-metrics ${CHART_DIR} \\"
echo "       --values istio-configs/custom-istio-values.yaml \\"
echo "       --namespace monitoring"
echo ""
print_warning "Note: addon-token-adapter secret will always reference kube-system"
echo ""
print_info "To restore original: cp -r ${BACKUP_DIR}/* ${CHART_DIR}/"

exit 0
