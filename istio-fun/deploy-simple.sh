#!/bin/bash

# Simple Helm deployment for ama-metrics
# Deploys to kube-system namespace (original templates are hardcoded)
# Run from anywhere - uses relative paths

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CHART_DIR="${SCRIPT_DIR}/../otelcollector/deploy/addon-chart/azure-monitor-metrics-addon"
NAMESPACE="${NAMESPACE:-ama-metrics-zane-test}"  # Can be overridden via environment variable
img="6.24.1-zane-istio-play-12-01-2025-5872518c" # for custom ns

#NAMESPACE="${NAMESPACE:-kube-system}"  # Can be overridden via environment variable
#img="6.24.1-zane-sequ-deploy-support-11-26-2025-d6f30328"

EXPECTED_CLUSTER="zane-metrics-custom-ns"  # Expected cluster name

echo "=========================================="
echo "Deploying ama-metrics Helm Chart"
echo "=========================================="

# Check current cluster context
CURRENT_CONTEXT=$(kubectl config current-context)
CURRENT_CLUSTER=$(kubectl config view -o jsonpath="{.contexts[?(@.name=='${CURRENT_CONTEXT}')].context.cluster}")

echo "Current Context: ${CURRENT_CONTEXT}"
echo "Current Cluster: ${CURRENT_CLUSTER}"
echo ""

# Verify we're on the correct cluster
if [[ ! "${CURRENT_CLUSTER}" =~ "${EXPECTED_CLUSTER}" ]]; then
    echo "❌ ERROR: You are not connected to the expected cluster!"
    echo "   Expected cluster name to contain: ${EXPECTED_CLUSTER}"
    echo "   Current cluster: ${CURRENT_CLUSTER}"
    echo ""
    echo "Please switch to the correct cluster using:"
    echo "  az aks get-credentials --resource-group zane-custom-ns --name ${EXPECTED_CLUSTER} --overwrite-existing"
    echo ""
    echo "Or set EXPECTED_CLUSTER environment variable to override:"
    echo "  EXPECTED_CLUSTER=your-cluster-name ./deploy-simple.sh"
    exit 1
fi

echo "✓ Cluster verification passed"
echo "Namespace: ${NAMESPACE}"
echo ""

# Ask user to confirm before proceeding
read -p "⚠️  Do you want to proceed with deployment to cluster '${CURRENT_CLUSTER}' in namespace '${NAMESPACE}'? (yes/no): " -r
echo ""
if [[ ! $REPLY =~ ^[Yy][Ee][Ss]$ ]]; then
    echo "Deployment cancelled by user."
    exit 0
fi

echo "✓ User confirmation received"
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
        -e 's|${MCR_REPOSITORY}|/azuremonitor/containerinsights/cidev/prometheus-collector/images|g' \
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
    
    # Only delete namespace if it's not kube-system (protected namespace)
    if [ "${NAMESPACE}" != "kube-system" ]; then
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
    else
        echo "ℹ️  Skipping namespace deletion (kube-system is a protected namespace)"
        echo "Cleaning up existing ama-metrics release if present..."
        
        # Just uninstall the helm release, don't delete the namespace
        if helm list -n ${NAMESPACE} 2>/dev/null | grep -q ama-metrics; then
            helm uninstall ama-metrics -n ${NAMESPACE} --wait || true
            echo "✓ Uninstalled existing ama-metrics release"
        else
            echo "ℹ️  No existing ama-metrics release found"
        fi
    fi
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
