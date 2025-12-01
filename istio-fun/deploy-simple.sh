#!/bin/bash

# Simple Helm deployment for ama-metrics
# Deploys to kube-system namespace (original templates are hardcoded)
# Run from anywhere - uses relative paths

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CHART_DIR="${SCRIPT_DIR}/../otelcollector/deploy/addon-chart/azure-monitor-metrics-addon"
NAMESPACE="${NAMESPACE:-ama-metrics-zane-test}"  # Can be overridden via environment variable
#NAMESPACE="${NAMESPACE:-kube-system}"  # Can be overridden via environment variable
img="6.24.1-zane-istio-play-12-01-2025-3948018a"

echo "=========================================="
echo "Deploying ama-metrics Helm Chart"
echo "=========================================="
echo "Namespace: ${NAMESPACE}"
echo ""

# Prepare Chart.yaml and values.yaml from templates (like official script)
echo "Preparing Helm chart files..."

# Create Chart.yaml (remove dependencies for AKS)
if [ -f "${CHART_DIR}/Chart-template.yaml" ]; then
    sed -e 's/${HELM_CHART_NAME}/ama-metrics/g' \
        -e 's/${IMAGE_TAG}/1.0.0/g' \
        "${CHART_DIR}/Chart-template.yaml" | \
    sed '/^dependencies:/,/condition: AzureMonitorMetrics\.ArcExtension/d' \
        > "${CHART_DIR}/Chart.yaml"
    echo "✓ Created Chart.yaml"
fi

# Create values.yaml with custom namespace
if [ -f "${CHART_DIR}/values-template.yaml" ]; then
    sed -e "s/\${IMAGE_TAG}/${img}/g" \
        -e 's|${MCR_REPOSITORY}|/azuremonitor/containerinsights/ciprod/prometheus-collector/images|g' \
        -e 's/${ARC_EXTENSION}/false/g' \
        -e 's/${AKS_REGION}/westeurope/g' \
        -e 's|${AKS_RESOURCE_ID}|/subscriptions/9c17527c-af8f-4148-8019-27bada0845f7/resourcegroups/zane-custom-ns/providers/Microsoft.ContainerService/managedClusters/zane-metrics-custom-ns|g' \
        -e "s|namespace: \"kube-system\"|namespace: \"${NAMESPACE}\"|g" \
        "${CHART_DIR}/values-template.yaml" > "${CHART_DIR}/values.yaml"
    echo "✓ Created values.yaml with namespace: ${NAMESPACE}"
fi

echo ""
echo "Deploying to ${NAMESPACE}..."
echo ""

# Check if namespace exists and clean up if needed
if kubectl get namespace ${NAMESPACE} >/dev/null 2>&1; then
    echo "⚠ Namespace ${NAMESPACE} already exists"
    echo "Deleting existing namespace and resources..."
    
    # Uninstall helm release first if it exists
    if helm list -n ${NAMESPACE} 2>/dev/null | grep -q ama-metrics; then
        helm uninstall ama-metrics -n ${NAMESPACE} --wait || true
    fi
    
    # Delete the namespace
    kubectl delete namespace ${NAMESPACE} --wait=true
    echo "✓ Deleted namespace ${NAMESPACE}"
    
    # Wait for namespace to be fully deleted
    echo "Waiting for namespace deletion to complete..."
    while kubectl get namespace ${NAMESPACE} >/dev/null 2>&1; do
        sleep 2
    done
    echo "✓ Namespace fully deleted"
fi

# Fresh install
echo "Installing new release..."
helm install ama-metrics ${CHART_DIR} \
    --namespace ${NAMESPACE} \
    --create-namespace \
    --values ${CHART_DIR}/values.yaml

echo ""
echo "=========================================="
echo "Deployment complete!"
echo "=========================================="
echo ""
echo "Check status:"
echo "  kubectl get pods -n ${NAMESPACE} -l rsName=ama-metrics"
echo ""
echo "View logs:"
echo "  kubectl logs -n ${NAMESPACE} -l rsName=ama-metrics -c prometheus-collector"
echo ""
echo "Uninstall:"
echo "  helm uninstall ama-metrics -n ${NAMESPACE}"
echo ""
