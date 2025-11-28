#!/bin/bash

# Deploy ama-metrics with Istio mTLS support
# This script sets up ama-metrics in a custom namespace with proper Istio configuration

set -e

# Configuration - UPDATE THESE VALUES
NAMESPACE="${NAMESPACE:-monitoring}"
CLUSTER_NAME="${CLUSTER_NAME:-your-cluster}"
RESOURCE_GROUP="${RESOURCE_GROUP:-your-rg}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo "=========================================="
echo "Deploying ama-metrics with Istio mTLS"
echo "=========================================="
echo "Namespace: ${NAMESPACE}"
echo "Cluster: ${CLUSTER_NAME}"
echo "Resource Group: ${RESOURCE_GROUP}"
echo ""

# Function to print colored output
print_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Check prerequisites
print_info "Checking prerequisites..."

if ! command -v kubectl &> /dev/null; then
    print_error "kubectl not found. Please install kubectl."
    exit 1
fi

if ! command -v helm &> /dev/null; then
    print_error "helm not found. Please install Helm 3."
    exit 1
fi

if ! command -v az &> /dev/null; then
    print_warning "az CLI not found. Some steps may fail."
fi

# Step 1: Create and label namespace for Istio
print_info "Step 1: Creating namespace and enabling Istio injection..."
kubectl create namespace ${NAMESPACE} --dry-run=client -o yaml | kubectl apply -f -
kubectl label namespace ${NAMESPACE} istio-injection=enabled --overwrite

print_info "Verifying namespace labels..."
kubectl get namespace ${NAMESPACE} --show-labels

# Step 2: Update namespace in YAML files
print_info "Step 2: Updating namespace in configuration files..."

# Update namespace in all YAML files
sed -i.bak "s/namespace: monitoring/namespace: ${NAMESPACE}/g" "${SCRIPT_DIR}/istio-peer-auth-ama-metrics.yaml"
sed -i.bak "s/namespace: monitoring/namespace: ${NAMESPACE}/g" "${SCRIPT_DIR}/istio-destinationrule-azure-monitor.yaml"
sed -i.bak "s/namespaces: \[\"monitoring\"\]/namespaces: [\"${NAMESPACE}\"]/g" "${SCRIPT_DIR}/istio-authz-ama-metrics.yaml"
sed -i.bak "s/ns\/monitoring/ns\/${NAMESPACE}/g" "${SCRIPT_DIR}/istio-authz-ama-metrics.yaml"
sed -i.bak "s/namespace: monitoring/namespace: ${NAMESPACE}/g" "${SCRIPT_DIR}/cross-namespace-secret-rbac.yaml"
sed -i.bak "s/namespace: \"monitoring\"/namespace: \"${NAMESPACE}\"/g" "${SCRIPT_DIR}/custom-istio-values.yaml"

print_info "Namespace updated in all configuration files"

# Step 3: Apply Istio configurations
print_info "Step 3: Applying Istio configurations..."

print_info "Applying PeerAuthentication..."
kubectl apply -f "${SCRIPT_DIR}/istio-peer-auth-ama-metrics.yaml"

print_info "Applying DestinationRule and ServiceEntry..."
kubectl apply -f "${SCRIPT_DIR}/istio-destinationrule-azure-monitor.yaml"

print_info "Applying AuthorizationPolicy..."
kubectl apply -f "${SCRIPT_DIR}/istio-authz-ama-metrics.yaml"

print_info "Applying cross-namespace RBAC..."
kubectl apply -f "${SCRIPT_DIR}/cross-namespace-secret-rbac.yaml"

# Step 4: Check if addon is enabled
print_info "Step 4: Checking AKS addon status..."
if command -v az &> /dev/null; then
    ADDON_ENABLED=$(az aks show -n ${CLUSTER_NAME} -g ${RESOURCE_GROUP} --query "azureMonitorProfile.metrics.enabled" -o tsv 2>/dev/null || echo "unknown")
    
    if [ "${ADDON_ENABLED}" != "true" ]; then
        print_warning "Azure Monitor metrics addon not enabled."
        print_warning "You need to enable it once to create the addon-token-adapter secret."
        read -p "Enable addon now? (y/n): " -n 1 -r
        echo
        if [[ $REPLY =~ ^[Yy]$ ]]; then
            print_info "Enabling addon..."
            az aks update --enable-azure-monitor-metrics -n ${CLUSTER_NAME} -g ${RESOURCE_GROUP}
            print_info "Waiting for secret to be created..."
            sleep 30
        fi
    fi
fi

# Step 5: Verify addon secret exists
print_info "Step 5: Verifying addon secret..."
if kubectl get secret aad-msi-auth-token -n kube-system &> /dev/null; then
    print_info "addon-token-adapter secret found in kube-system"
else
    print_error "addon-token-adapter secret not found in kube-system"
    print_error "Please enable the Azure Monitor metrics addon first"
    exit 1
fi

# Step 6: Optional - Disable addon
print_warning "Step 6: Disabling addon (optional)..."
print_warning "This will stop the kube-system deployment to avoid conflicts"
read -p "Disable the AKS addon? (y/n): " -n 1 -r
echo
if [[ $REPLY =~ ^[Yy]$ ]]; then
    if command -v az &> /dev/null; then
        print_info "Disabling addon..."
        az aks update --disable-azure-monitor-metrics -n ${CLUSTER_NAME} -g ${RESOURCE_GROUP}
    else
        print_warning "az CLI not available, skipping addon disable"
    fi
fi

# Step 7: Deploy ARM template
print_info "Step 7: ARM template deployment..."
print_warning "This creates DCR, DCE, and other Azure resources"
echo ""

# Additional variables needed for ARM template
AMW_RESOURCE_ID="${AMW_RESOURCE_ID:-}"
AMW_LOCATION="${AMW_LOCATION:-eastus}"

if command -v az &> /dev/null; then
    print_info "ARM template deployment options:"
    echo "  1. Deploy now with az CLI (recommended)"
    echo "  2. Skip - I'll deploy manually"
    echo ""
    read -p "Choose option (1 or 2): " -n 1 -r ARM_OPTION
    echo ""
    
    if [[ $ARM_OPTION == "1" ]]; then
        # Prompt for Azure Monitor Workspace if not set
        if [ -z "$AMW_RESOURCE_ID" ]; then
            echo ""
            print_warning "Azure Monitor Workspace Resource ID not set"
            echo "Example: /subscriptions/xxx/resourceGroups/xxx/providers/Microsoft.Monitor/accounts/xxx"
            read -p "Enter AMW Resource ID (or leave empty to skip): " AMW_RESOURCE_ID
        fi
        
        if [ -n "$AMW_RESOURCE_ID" ]; then
            ARM_TEMPLATE="../AddonArmTemplate/FullAzureMonitorMetricsProfile.json"
            
            if [ ! -f "$ARM_TEMPLATE" ]; then
                print_error "ARM template not found at $ARM_TEMPLATE"
                print_warning "Please deploy manually"
            else
                print_info "Deploying ARM template..."
                print_warning "NOTE: Make sure the addon enablement section (lines ~160-200) is commented out!"
                echo ""
                read -p "Have you commented out the addon section? (y/n): " -n 1 -r
                echo ""
                
                if [[ $REPLY =~ ^[Yy]$ ]]; then
                    # Get cluster resource ID
                    CLUSTER_ID=$(az aks show -n ${CLUSTER_NAME} -g ${RESOURCE_GROUP} --query id -o tsv)
                    CLUSTER_LOCATION=$(az aks show -n ${CLUSTER_NAME} -g ${RESOURCE_GROUP} --query location -o tsv)
                    
                    # Create parameters JSON
                    PARAMS_JSON=$(cat <<EOF
{
  "azureMonitorWorkspaceResourceId": {"value": "$AMW_RESOURCE_ID"},
  "azureMonitorWorkspaceLocation": {"value": "$AMW_LOCATION"},
  "clusterResourceId": {"value": "$CLUSTER_ID"},
  "clusterLocation": {"value": "$CLUSTER_LOCATION"}
}
EOF
)
                    
                    # Deploy ARM template
                    DEPLOYMENT_NAME="ama-metrics-istio-$(date +%s)"
                    
                    echo "$PARAMS_JSON" > /tmp/arm-params.json
                    
                    print_info "Starting ARM deployment: $DEPLOYMENT_NAME"
                    az deployment group create \
                        --name "$DEPLOYMENT_NAME" \
                        --resource-group ${RESOURCE_GROUP} \
                        --template-file "$ARM_TEMPLATE" \
                        --parameters @/tmp/arm-params.json
                    
                    rm -f /tmp/arm-params.json
                    
                    print_info "ARM template deployed successfully"
                else
                    print_error "Please comment out the addon section in the ARM template first"
                    print_error "File: ../AddonArmTemplate/FullAzureMonitorMetricsProfile.json"
                    print_error "Lines: ~160-200 (the azuremonitormetrics-profile section)"
                    exit 1
                fi
            fi
        else
            print_warning "Skipping ARM template deployment - no AMW Resource ID provided"
            print_warning "You'll need to deploy the ARM template manually"
        fi
    else
        print_warning "Skipping automated ARM deployment"
        print_warning "Deploy the ARM template manually before continuing"
        echo ""
        print_info "ARM template location: ../AddonArmTemplate/FullAzureMonitorMetricsProfile.json"
        print_warning "IMPORTANT: Comment out lines ~160-200 (addon enablement section)"
        echo ""
        read -p "Press enter after ARM template is deployed..."
    fi
else
    print_warning "az CLI not available - cannot deploy ARM template automatically"
    print_warning "Deploy the ARM template manually:"
    echo ""
    print_info "  1. Navigate to: ../AddonArmTemplate/"
    print_info "  2. Edit FullAzureMonitorMetricsProfile.json"
    print_info "  3. Comment out lines ~160-200 (addon enablement section)"
    print_info "  4. Deploy via Azure Portal or az CLI"
    echo ""
    read -p "Press enter after ARM template is deployed..."
fi

# Step 8: Install Helm chart
print_info "Step 8: Installing Helm chart..."

CHART_PATH="../otelcollector/deploy/addon-chart/azure-monitor-metrics-addon"

if [ ! -d "${CHART_PATH}" ]; then
    print_error "Helm chart not found at ${CHART_PATH}"
    print_error "Please run this script from the istio-configs directory"
    exit 1
fi

print_info "Installing ama-metrics Helm chart..."
helm upgrade --install ama-metrics ${CHART_PATH} \
  --namespace ${NAMESPACE} \
  --values "${SCRIPT_DIR}/custom-istio-values.yaml" \
  --create-namespace

# Step 9: Verify deployment
print_info "Step 9: Verifying deployment..."
echo ""

print_info "Waiting for pods to start..."
sleep 10

print_info "Checking pods (should show 2/2 containers - app + istio-proxy)..."
kubectl get pods -n ${NAMESPACE}

echo ""
print_info "Checking Istio configurations..."
kubectl get peerauthentication -n ${NAMESPACE}
kubectl get destinationrule -n ${NAMESPACE}
kubectl get authorizationpolicy -n istio-system | grep ama-metrics

echo ""
print_info "=========================================="
print_info "Deployment complete!"
print_info "=========================================="
echo ""
print_info "Next steps:"
echo "  1. Verify Istio sidecar injection:"
echo "     kubectl get pods -n ${NAMESPACE} -o jsonpath='{.items[*].spec.containers[*].name}'"
echo ""
echo "  2. Check ama-metrics logs:"
echo "     kubectl logs -n ${NAMESPACE} -l rsName=ama-metrics -c prometheus-collector"
echo ""
echo "  3. Check Istio proxy logs:"
echo "     kubectl logs -n ${NAMESPACE} -l rsName=ama-metrics -c istio-proxy"
echo ""
echo "  4. Verify mTLS certificates:"
echo "     kubectl exec -n ${NAMESPACE} deploy/ama-metrics -c istio-proxy -- openssl s_client -showcerts -connect localhost:15000 </dev/null 2>/dev/null | openssl x509 -noout -text | grep Subject"
echo ""
print_warning "Remember: This is a custom configuration. Monitor closely!"

# Cleanup backup files
rm -f "${SCRIPT_DIR}"/*.yaml.bak

exit 0
