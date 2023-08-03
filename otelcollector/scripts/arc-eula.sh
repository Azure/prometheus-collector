#!/bin/bash

export IS_ARC_CLUSTER="false"
CLUSTER_nocase=$(echo $CLUSTER | tr "[:upper:]" "[:lower:]")
if [[ $CLUSTER_nocase =~ "connectedclusters" ]]; then
  export IS_ARC_CLUSTER="true"
fi
echo "export IS_ARC_CLUSTER=$IS_ARC_CLUSTER" >> ~/.bashrc

# EULA statement is required for Arc extension
if [ "$IS_ARC_CLUSTER" == "true" ]; then
  echo "MICROSOFT SOFTWARE LICENSE TERMS\n\nMICROSOFT Azure Arc-enabled Kubernetes\n\nThis software is licensed to you as part of your or your company's subscription license for Microsoft Azure Services. You may only use the software with Microsoft Azure Services and subject to the terms and conditions of the agreement under which you obtained Microsoft Azure Services. If you do not have an active subscription license for Microsoft Azure Services, you may not use the software. Microsoft Azure Legal Information: https://azure.microsoft.com/en-us/support/legal/"
fi
