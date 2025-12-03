n#!/bin/bash

# Uninstall ama-metrics from a given namespace
# Usage: ./uninstall-ama-metrics.sh [namespace]
# If no namespace is provided, uses ama-metrics-zane-test as default

set -e

NAMESPACE="${1:-ama-metrics-zane-test}"  # Use first argument or default

echo "=========================================="
echo "Uninstalling ama-metrics from ${NAMESPACE}"
echo "=========================================="
echo ""

# Check if namespace exists
if ! kubectl get namespace ${NAMESPACE} >/dev/null 2>&1; then
    echo "❌ ERROR: Namespace '${NAMESPACE}' does not exist"
    exit 1
fi

# Check if ama-metrics release exists in the namespace
if ! helm list -n ${NAMESPACE} 2>/dev/null | grep -q ama-metrics; then
    echo "ℹ️  No ama-metrics release found in namespace '${NAMESPACE}'"
    echo ""
    echo "Available helm releases in ${NAMESPACE}:"
    helm list -n ${NAMESPACE} || echo "  (none)"
    exit 0
fi

# Show what will be uninstalled
echo "Found ama-metrics release in namespace '${NAMESPACE}'"
echo ""
helm list -n ${NAMESPACE} | grep ama-metrics
echo ""

# Ask for confirmation
read -p "⚠️  Do you want to uninstall ama-metrics from '${NAMESPACE}'? (yes/no): " -r
echo ""
if [[ ! $REPLY =~ ^[Yy][Ee][Ss]$ ]]; then
    echo "Uninstall cancelled by user."
    exit 0
fi

# Uninstall the release
echo "Uninstalling ama-metrics..."
helm uninstall ama-metrics -n ${NAMESPACE} --wait

echo ""
echo "=========================================="
echo "✓ Uninstall complete!"
echo "=========================================="
echo ""
echo "Namespace '${NAMESPACE}' still exists. To delete it:"
echo "  kubectl delete namespace ${NAMESPACE}"
echo ""
