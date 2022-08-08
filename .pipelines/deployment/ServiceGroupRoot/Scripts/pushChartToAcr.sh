#!/bin/bash
set -e

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

#Make sure that tag being pushed will not overwrite an existing tag in mcr
#echo "Checking if this tag already exists in prod MCR path"
#PROD_MCR_TAG_RESULT="`wget -qO- https://mcr.microsoft.com/v2$PROD_MCR_REPOSITORY/tags/list`"
#TAG_RESULT_EXIT_CODE=$?
#if [ $TAG_RESULT_EXIT_CODE -ne 0 ] && [ $TAG_RESULT_EXIT_CODE -ne 8 ]; then         
#   echo "-e error unable to get list of tags for $PROD_MCR_REPOSITORY"
#   exit 1
#fi

#if [ $PROD_MCR_TAG_RESULT ]; then 
#  echo "Checking tag list"
#  TAG_EXISTS=$(echo $PROD_MCR_TAG_RESULT | jq '.tags | contains(["'"$IMAGE_TAG"'"])')
#
#  if $TAG_EXISTS; then
#    echo "-e error ${IMAGE_TAG} already exists in Prod MCR. Make sure the image tag is unique"
#    exit 1
#  fi
#fi

ls

envsubst prometheus-collector/Chart-template.yaml > prometheus-collector/Chart.yaml && envsubst < prometheus-collector/values-template.yaml > prometheus-collector/values.yaml

helm version

# Wait for KSM and node-exporter charts to push
for i in 1 2 3 4 5 6 7 8 9 10; do
  sleep 30
  echo $(MCR_REGISTRY)$(PROD_MCR_KSM_REPOSITORY):$(KSM_CHART_TAG)
  echo $(MCR_REGISTRY)$(PROD_MCR_NE_REPOSITORY):$(NE_CHART_TAG)
  if docker manifest inspect $(MCR_REGISTRY)$(PROD_MCR_KSM_REPOSITORY):$(KSM_CHART_TAG) && docker manifest inspect $(MCR_REGISTRY)$(PROD_MCR_NE_REPOSITORY):$(NE_CHART_TAG); then
    echo "Dependent charts are published to mcr"
    break
  fi
done
echo "Dependent charts are not published to mcr within 5 minutes"
exit 1

cd prometheus-collector/
ls
helm dep update

cd ../
ls
helm package ./prometheus-collector/

#Login to az cli and authenticate to acr
echo "Login cli using managed identity"
az login --identity
if [ $? -eq 0 ]; then
  echo "Logged in successfully"
else
  echo "-e error failed to login to az with managed identity credentials"
  exit 1
fi

ACCESS_TOKEN=$(az acr login --name $(ACR_REGISTRY) --expose-token --output tsv --query accessToken)
if [ $? -ne 0 ]; then         
   echo "-e error az acr login failed. Please review the Ev2 pipeline logs for more details on the error."
   exit 1
fi

echo "login to acr:$(ACR_REGISTRY) using helm ..."
echo $ACCESS_TOKEN | helm registry login $(ACR_REGISTRY) -u 00000000-0000-0000-0000-000000000000 --password-stdin
if [ $? -eq 0 ]; then
  echo "login to acr:$(ACR_REGISTRY)} using helm completed successfully."
else
  echo "-e error login to acr:$(ACR_REGISTRY) using helm failed."
  exit 1
fi 

helm push $(HELM_CHART_NAME)-$(HELM_SEMVER).tgz oci://$(ACR_REGISTRY)$(PROD_ACR_REPOSITORY)
if [ $? -eq 0 ]; then            
  echo "pushing the chart to acr path: ${destAcrFullPath} completed successfully."
else     
  echo "-e error pushing the chart to acr path: ${destAcrFullPath} failed."
  exit 1
fi    
