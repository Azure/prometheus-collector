#!/bin/bash
set -e

if [ $ONLY_CCP_RELASE == "true" ] && [[ $IMAGE_TAG != *"ccp"* ]]; then
  echo "Skipping image push - not a CCP image"
  exit 0
fi

if [ -z $IMAGE_TAG ]; then
  echo "-e error value of IMAGE_TAG variable shouldnt be empty. Check release variables"
  exit 1
fi

if [ -z $IMAGE_TAG_WINDOWS ]; then
  echo "-e error value of IMAGE_TAG_WINDOWS variable shouldnt be empty. Check release variables"
  exit 1
fi

if [ -z $HELM_SEMVER ]; then
  echo "-e error value of HELM_SEMVER variable shouldnt be empty. Check release variables"
  exit 1
fi

if [ -z $MCR_REGISTRY ]; then
  echo "-e error MCR_REGISTRY shouldnt be empty. Check release variables"
  exit 1
fi

if [ -z $MCR_REPOSITORY ]; then
  echo "-e error PROD_MCR_REPOSITORY shouldnt be empty. Check release variables"
  exit 1
fi

if [ -z $MCR_REPOSITORY_HELM_DEPENDENCIES ]; then
  echo "-e error MCR_REPOSITORY_HELM_DEPENDENCIES shouldnt be empty. Check release variables"
  exit 1
fi

if [ -z $PROD_MCR_REPOSITORY ]; then
  echo "-e error PROD_MCR_REPOSITORY shouldnt be empty. Check release variables"
  exit 1
fi

if [ -z $ACR_REGISTRY ]; then
  echo "-e error value of ACR_REGISTRY shouldn't be empty. Check release variables"
  exit 1
fi

if [ -z $PROD_ACR_REPOSITORY ]; then
  echo "-e error value of PROD_ACR_REPOSITORY shouldn't be empty. Check release variables"
  exit 1
fi

echo "Done checking that all necessary variables exist."

# Wait for KSM and node-exporter charts to push
cd ${HELM_CHART_NAME}
for i in 1 2 3 4 5 6 7 8 9 10; do
  sleep 30
  helm dep update
  if [ $? -eq 0 ]; then
    echo "Dependent charts are published to mcr"
    DEPENDENT_CHARTS_PUBLISHED="true"
    break
  fi
done
if [ "$DEPENDENT_CHARTS_PUBLISHED" != "true" ]; then
  echo "Dependent charts are not published to mcr within 5 minutes"
  exit 1
fi

cd ../
helm package ./${HELM_CHART_NAME}

# Login to az cli and authenticate to acr
echo "Login cli using managed identity"

# Retries needed due to: https://stackoverflow.microsoft.com/questions/195032
n=0
signInExitCode=-1
until [ "$n" -ge 5 ]
do
   az login --identity && signInExitCode=0 && break
   n=$((n+1))
   sleep 15
done

if [ $signInExitCode -eq 0 ]; then
  echo "Logged in successfully"
else
  echo "-e error failed to login to az with managed identity credentials"
  exit 1
fi

ACCESS_TOKEN=$(az acr login --name ${ACR_REGISTRY} --expose-token --output tsv --query accessToken)
if [ $? -ne 0 ]; then         
   echo "-e error az acr login failed. Please review the Ev2 pipeline logs for more details on the error."
   exit 1
fi

echo "login to acr:${ACR_REGISTRY} using helm ..."
echo $ACCESS_TOKEN | helm registry login ${ACR_REGISTRY} -u 00000000-0000-0000-0000-000000000000 --password-stdin
if [ $? -eq 0 ]; then
  echo "login to acr:${ACR_REGISTRY} using helm completed successfully."
else
  echo "-e error login to acr:${ACR_REGISTRY} using helm failed."
  exit 1
fi 

helm push ${HELM_CHART_NAME}-${HELM_SEMVER}.tgz oci://${ACR_REGISTRY}${PROD_ACR_REPOSITORY}
if [ $? -eq 0 ]; then            
  echo "pushing the chart to acr path: ${ACR_REGISTRY}${PROD_ACR_REPOSITORY} completed successfully."
else     
  echo "-e error pushing the chart to acr path: ${ACR_REGISTRY}${PROD_ACR_REPOSITORY} failed."
  exit 1
fi    
