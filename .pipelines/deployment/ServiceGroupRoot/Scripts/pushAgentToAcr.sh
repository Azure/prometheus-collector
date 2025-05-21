#!/bin/bash
set -e

if [ $STEP_NAME == "PushKSMChart" ] && [ $PUSH_NEW_KSM_CHART == "false" ]; then
  echo "Skipping pushing KSM Chart"
  exit 0
fi

if [ $STEP_NAME == "PushNEChart" ] && [ $PUSH_NEW_NE_CHART == "false" ]; then
  echo "Skipping pushing NE Chart"
  exit 0
fi

if [ -z $IMAGE_TAG ]; then
  echo "-e error value of IMAGE_TAG variable shouldnt be empty. Check release variables"
  exit 1
fi

if [ -z $MCR_REGISTRY ]; then
  echo "-e error MCR_REGISTRY shouldnt be empty. Check release variables"
  exit 1
fi

if [ -z $PROD_MCR_REPOSITORY ]; then
  echo "-e error PROD_MCR_REPOSITORY shouldnt be empty. Check release variables"
  exit 1
fi

if [ -z $DEV_MCR_REPOSITORY ]; then
  echo "-e error value of DEV_MCR_REPOSITORY shouldn't be empty. Check release variables"
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

PROD_ACR_REPOSITORY_WITHOUT_SLASH="${PROD_ACR_REPOSITORY:1}"
echo "Copy ${PROD_ACR_REPOSITORY_WITHOUT_SLASH}:${IMAGE_TAG} with artifacts to ${ACR_REGISTRY}"

# Login to ACR and get the access token
LOGIN_INFO=$(az acr login -n $ACR_REGISTRY --expose-token)
TOKEN=$(echo $LOGIN_INFO | jq -r '.accessToken')

# Define the source and destination image paths
SOURCE_IMAGE_FULL_PATH=${MCR_REGISTRY}${DEV_MCR_REPOSITORY}:${IMAGE_TAG}
AGENT_IMAGE_FULL_PATH=${PROD_ACR_REPOSITORY_WITHOUT_SLASH}:${IMAGE_TAG}

echo "Source image: $SOURCE_IMAGE_FULL_PATH"
echo "Destination image: $ACR_REGISTRY/$AGENT_IMAGE_FULL_PATH"

# Use oras to copy the image
echo "Checking oras version"
oras version
echo "Starting oras copy"
oras copy -r -v $SOURCE_IMAGE_FULL_PATH $ACR_REGISTRY/$AGENT_IMAGE_FULL_PATH --to-password $TOKEN
if [ $? -eq 0 ]; then
  echo "Retagged and pushed image successfully"
else
  echo "-e error failed to retag and push image to destination ACR"
  exit 1
fi
