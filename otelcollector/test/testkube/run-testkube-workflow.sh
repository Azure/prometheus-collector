#!/bin/bash

# Comprehensive TestKube deployment and testing script
# Parameters:
# $1 - AMW_QUERY_ENDPOINT
# $2 - AZURE_CLIENT_ID  
# $3 - Source template file (e.g., testkube-test-crs-arc.yaml)
# $4 - Target output file (e.g., testkube-test-crs-ci-dev-arc-wcus.yaml)
# $5 - Apply settings configmap (optional, defaults to true)
# $6 - Sleep duration in seconds (optional, defaults to 360)
# $7 - Target environment name (e.g., ARC, AKS, OTel)

if [ $# -lt 4 ]; then
    echo "Usage: $0 <AMW_QUERY_ENDPOINT> <AZURE_CLIENT_ID> <SOURCE_TEMPLATE> <TARGET_OUTPUT> [APPLY_SETTINGS_CONFIGMAP] [SLEEP_DURATION] [TARGET_ENV]"
    exit 0
fi

AMW_QUERY_ENDPOINT="$1"
AZURE_CLIENT_ID="$2"
SOURCE_TEMPLATE="$3"
TARGET_OUTPUT="$4"
APPLY_SETTINGS_CONFIGMAP="${5:-true}"
SLEEP_DURATION="${6:-360}"
TARGET_ENV="${7:-Unknown}"
BRANCH_NAME="${8:-main}"

# Define shared results file  
RESULTS_FILE="${BUILD_ARTIFACTSTAGINGDIRECTORY}/testkube-results-${TARGET_ENV}.json"

# Function to initialize or update results file
init_results_file() {
    # Create the artifact staging directory if it doesn't exist
    mkdir -p "$(dirname "$RESULTS_FILE")"
    
    if [ ! -f "$RESULTS_FILE" ]; then
        echo '{"environment": "'$TARGET_ENV'", "start_time": "'$(date -u '+%Y-%m-%d %H:%M:%S UTC')'", "status": "running"}' > "$RESULTS_FILE"
    fi
}

# Function to add result to file
add_result() {
    local env="$1"
    local status="$2" 
    local message="$3"
    local link="$4"
    
    # Create temporary file with updated results
    local temp_file=$(mktemp)
    jq --arg env "$env" --arg status "$status" --arg message "$message" --arg link "$link" \
       '.status = $status | .message = $message | .link = $link' \
       "$RESULTS_FILE" > "$temp_file" && mv "$temp_file" "$RESULTS_FILE"
    
    echo "Result saved to: $RESULTS_FILE"
    cat "$RESULTS_FILE"
}

echo "================================================="
echo "Starting TestKube deployment and testing workflow"
echo "================================================="

# Initialize results file
init_results_file

# Step 1: Install TestKube CLI
echo "Step 1: Installing TestKube CLI..."
wget -qO - https://repo.testkube.io/key.pub | sudo apt-key add -
echo "deb https://repo.testkube.io/linux linux main" | sudo tee -a /etc/apt/sources.list
sudo apt-get update
sudo apt-get install -y testkube
echo "✓ TestKube CLI installation completed"

# Step 2: Set up TestKube environment
echo "Step 2: Setting up TestKube environment..."
echo "AMW_QUERY_ENDPOINT: $AMW_QUERY_ENDPOINT"
echo "AZURE_CLIENT_ID: $AZURE_CLIENT_ID"
echo "Source template: $SOURCE_TEMPLATE"
echo "Target output: $TARGET_OUTPUT"
echo "BRANCH_NAME: $BRANCH_NAME"

# Export environment variables for envsubst
export AMW_QUERY_ENDPOINT
export AZURE_CLIENT_ID
export BRANCH_NAME

# Generate the test CRs from template
envsubst < "./testkube/$SOURCE_TEMPLATE" > "./testkube/$TARGET_OUTPUT"

# Apply the generated files
kubectl apply -f ./testkube/api-server-permissions.yaml
kubectl apply -f "./testkube/$TARGET_OUTPUT"

# Apply common configmaps
kubectl apply -f ./test-cluster-yamls/configmaps/ama-metrics-prometheus-config-configmap.yaml
kubectl apply -f ./test-cluster-yamls/configmaps/ama-metrics-prometheus-config-node-configmap.yaml
kubectl apply -f ./test-cluster-yamls/configmaps/ama-metrics-prometheus-config-node-windows-configmap.yaml

# Apply settings configmap (unless explicitly disabled)
if [ "$APPLY_SETTINGS_CONFIGMAP" = "true" ]; then
    kubectl apply -f ./test-cluster-yamls/configmaps/ama-metrics-settings-configmap.yaml
    echo "✓ Applied settings configmap"
else
    echo "⚠ Skipped settings configmap (disabled)"
fi

# Apply reference app
kubectl apply -f ./test-cluster-yamls/customresources/prometheus-reference-app.yaml
echo "✓ TestKube environment setup completed"

# Step 3: Wait for cluster to be ready
echo "Step 3: Waiting for cluster to be ready for $SLEEP_DURATION seconds..."
sleep "$SLEEP_DURATION"
echo "✓ Cluster wait period completed"

# Step 4: Run TestKube tests
echo "Step 4: Starting TestKube test suite execution..."

# Run the full test suite
kubectl testkube run testsuite e2e-tests-merge --verbose --job-template testkube/job-template.yaml

# Get the current id of the test suite now running with retry logic
max_get_id_retries=5
get_id_retry_count=0
execution_id=""

while [ $get_id_retry_count -lt $max_get_id_retries ] && [ -z "$execution_id" ]; do
    execution_id=$(kubectl testkube get testsuiteexecutions --test-suite e2e-tests-merge --limit 1 | grep e2e-tests | awk '{print $1}')
    
    if [ -n "$execution_id" ]; then
        echo "Test suite execution ID: $execution_id"
    else
        get_id_retry_count=$((get_id_retry_count+1))
        echo "Failed to get test suite execution ID (attempt $get_id_retry_count/$max_get_id_retries). Retrying in 5 seconds..."
        sleep 5
    fi
done

if [ -z "$execution_id" ]; then
    echo "Error: Failed to get test suite execution ID after $max_get_id_retries attempts."
    add_result "$TARGET_ENV" "error" "Failed to retrieve test suite execution ID" ""
    exit 0
fi

# Watch until all the tests in the test suite finish with retry logic
max_retries=3
retry_count=0
watch_success=false

echo "Monitoring test suite execution progress..."

while [ $retry_count -lt $max_retries ] && [ "$watch_success" != "true" ]; do
    # Suppress the verbose output but still monitor completion
    if kubectl testkube watch testsuiteexecution "$execution_id" >/dev/null 2>&1; then
        watch_success=true
        echo "✓ Test suite execution completed"
    else
        retry_count=$((retry_count+1))
        echo "Watching test suite execution failed (attempt $retry_count/$max_retries). Retrying in 10 seconds..."
        sleep 10
    fi
done

if [ "$watch_success" != "true" ]; then
    echo "Warning: Failed to watch test suite execution after $max_retries attempts. Continuing with result collection..."
fi

# Get the results as a formatted json file with retry logic
max_retries=3
retry_count=0
get_results_success=false

while [ $retry_count -lt $max_retries ] && [ "$get_results_success" != "true" ]; do
    if kubectl testkube get testsuiteexecution "$execution_id" --output json > testkube-results.json; then
        get_results_success=true
    else
        retry_count=$((retry_count+1))
        echo "Getting test results failed (attempt $retry_count/$max_retries). Retrying in 10 seconds..."
        sleep 10
    fi
done

if [ "$get_results_success" != "true" ]; then
    echo "Error: Failed to get test results after $max_retries attempts."
    add_result "$TARGET_ENV" "error" "Failed to retrieve test results after multiple attempts" ""
    exit 0
fi

# Check if any tests failed and process results
if [[ $(jq -r '.status' testkube-results.json) == "failed" ]]; then
    echo "Some tests failed. Processing failed test details..."
    
    # Collect failed test names in a variable
    failed_tests=""
    
    # Get each test name and id that failed
    jq -r '.executeStepResults[].execute[] | select(.execution.executionResult.status=="failed") | "\(.execution.testName) \(.execution.id)"' testkube-results.json | while read line; do
        testName=$(echo $line | cut -d ' ' -f 1)
        id=$(echo $line | cut -d ' ' -f 2)
        echo "Test $testName failed. Test ID: $id"
        failed_tests+="$testName, "
        
        # Get the Ginkgo logs of the test
        kubectl testkube get execution "$id" > out 2>error.log
        
        # Remove superfluous logs of everything before the last occurrence of 'go downloading'.
        # The actual errors can be viewed from the ADO run, instead of needing to view the testkube dashboard.
        cat error.log | tac | awk '/go: downloading/ {exit} 1' | tac
    done
    
    # Get complete list of failed tests for the result message
    failed_tests_list=$(jq -r '.executeStepResults[].execute[] | select(.execution.executionResult.status=="failed") | .execution.testName' testkube-results.json | paste -sd ", " -)
    
    # Build pipeline link if environment variables are available
    pipeline_link=""
    if [ -n "$BUILD_BUILDID" ] && [ -n "$SYSTEM_JOBID" ] && [ -n "$SYSTEM_TASKINSTANCEID" ]; then
        pipeline_link="[View Pipeline Run](https://github-private.visualstudio.com/azure/_build/results?buildId=${BUILD_BUILDID}&view=logs&j=${SYSTEM_JOBID}&t=${SYSTEM_TASKINSTANCEID})"
    elif [ -n "$BUILD_BUILDID" ]; then
        pipeline_link="[View Pipeline Run](https://github-private.visualstudio.com/azure/_build/results?buildId=${BUILD_BUILDID})"
    fi
    
    echo "Failed test processing completed"
    add_result "$TARGET_ENV" "failed" "Tests failed: $failed_tests_list. Check pipeline logs for details." "${pipeline_link}"
else
    echo "All tests passed successfully!"
    add_result "$TARGET_ENV" "passed" "All tests passed successfully" ""
fi

echo "================================================="
echo "TestKube deployment and testing workflow completed"
echo "================================================="
