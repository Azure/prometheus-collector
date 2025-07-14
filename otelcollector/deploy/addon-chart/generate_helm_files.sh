#!/bin/bash

# This script generates Chart.yaml and values.yaml from template files
# by replacing placeholders with actual values

# Define variables - UPDATE THESE VALUES FOR YOUR ENVIRONMENT
IMAGE_TAG="6.18.0-rakechill-node-auto-provisioning-metrics-07-02-2025-b261d8b2-ccp"
AKS_REGION="eastus"  # Update with your cluster region
AKS_RESOURCE_ID="/subscriptions/YOUR_SUBSCRIPTION/resourceGroups/YOUR_RG/providers/Microsoft.ContainerService/managedClusters/YOUR_CLUSTER"  # Update with your cluster resource ID
MCR_REPOSITORY="/azuremonitor/containerinsights/cidev/prometheus-collector/images"
HELM_CHART_NAME="ama-metrics"

echo "Generating Chart.yaml and values.yaml from templates..."

# Process azure-monitor-metrics-addon
if [ -d "azure-monitor-metrics-addon" ]; then
    cd azure-monitor-metrics-addon
    
    # Generate Chart.yaml from Chart-template.yaml
    if [ -f "Chart-template.yaml" ]; then
        echo "Processing Chart-template.yaml..."
        sed -e "s/\${HELM_CHART_NAME}/${HELM_CHART_NAME}/g" \
            -e "s/\${IMAGE_TAG}/${IMAGE_TAG}/g" \
            Chart-template.yaml > Chart.yaml
        
        # Remove dependencies section (for non-Arc extension usage)
        sed -i '/^dependencies:/,/^[[:space:]]*$/d' Chart.yaml
        
        echo "Generated Chart.yaml"
    fi
    
    # Generate values.yaml from values-template.yaml
    if [ -f "values-template.yaml" ]; then
        echo "Processing values-template.yaml..."
        sed -e "s|\${IMAGE_TAG}|${IMAGE_TAG}|g" \
            -e "s|\${MCR_REPOSITORY}|${MCR_REPOSITORY}|g" \
            -e "s/\${ARC_EXTENSION}/false/g" \
            -e "s/\${AKS_REGION}/${AKS_REGION}/g" \
            -e "s|\${AKS_RESOURCE_ID}|${AKS_RESOURCE_ID}|g" \
            values-template.yaml > values.yaml
        
        echo "Generated values.yaml"
    fi
    
    cd ..
fi

# Process ccp-metrics-plugin
if [ -d "ccp-metrics-plugin" ]; then
    cd ccp-metrics-plugin
    
    # Generate Chart.yaml from Chart-template.yaml
    if [ -f "Chart-template.yaml" ]; then
        echo "Processing ccp-metrics-plugin Chart-template.yaml..."
        # For ccp-metrics-plugin, the Chart-template.yaml seems to already have actual values
        # Just copy it as Chart.yaml if it doesn't exist
        if [ ! -f "Chart.yaml" ]; then
            cp Chart-template.yaml Chart.yaml
            echo "Generated ccp-metrics-plugin Chart.yaml"
        fi
    fi
    
    # Generate values.yaml from values-template.yaml
    if [ -f "values-template.yaml" ]; then
        echo "Processing ccp-metrics-plugin values-template.yaml..."
        sed -e "s|\${IMAGE_TAG}|${IMAGE_TAG}|g" \
            -e "s|\${MCR_REPOSITORY}|${MCR_REPOSITORY}|g" \
            -e "s/\${ARC_EXTENSION}/false/g" \
            -e "s/\${AKS_REGION}/${AKS_REGION}/g" \
            -e "s|\${AKS_RESOURCE_ID}|${AKS_RESOURCE_ID}|g" \
            values-template.yaml > values.yaml
        
        echo "Generated ccp-metrics-plugin values.yaml"
    fi
    
    cd ..
fi

echo "Done! Generated non-templated Chart.yaml and values.yaml files."
echo ""
echo "Please update the following variables in this script for your environment:"
echo "- AKS_REGION: ${AKS_REGION}"
echo "- AKS_RESOURCE_ID: ${AKS_RESOURCE_ID}"
