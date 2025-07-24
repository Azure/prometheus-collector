#!/bin/bash
set -e

# Comprehensive TestKube deployment and testing script
# Parameters:
# $1 - AMW_QUERY_ENDPOINT
# $2 - AZURE_CLIENT_ID  
# $3 - Source template file (e.g., testkube-test-crs-arc.yaml)
# $4 - Target output file (e.g., testkube-test-crs-ci-dev-arc-wcus.yaml)
# $5 - Apply settings configmap (optional, defaults to true)
# $6 - Sleep duration in seconds (optional, defaults to 360)
# $7 - Target environment name (e.g., ARC, AKS, OTel)
# $8 - Teams webhook URL (optional) - NOTE: Individual notifications disabled, results stored for summary

if [ $# -lt 4 ]; then
    echo "Usage: $0 <AMW_QUERY_ENDPOINT> <AZURE_CLIENT_ID> <SOURCE_TEMPLATE> <TARGET_OUTPUT> [APPLY_SETTINGS_CONFIGMAP] [SLEEP_DURATION] [TARGET_ENV] [TEAMS_WEBHOOK_URL]"
    exit 1
fi

AMW_QUERY_ENDPOINT="$1"
AZURE_CLIENT_ID="$2"
SOURCE_TEMPLATE="$3"
TARGET_OUTPUT="$4"
APPLY_SETTINGS_CONFIGMAP="${5:-true}"
SLEEP_DURATION="${6:-360}"
TARGET_ENV="${7:-Unknown}"
TEAMS_WEBHOOK_URL="$8"

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
    local timestamp="$(date -u '+%Y-%m-%d %H:%M:%S UTC')"
    
    # Create temporary file with updated results
    local temp_file=$(mktemp)
    jq --arg env "$env" --arg status "$status" --arg message "$message" --arg timestamp "$timestamp" \
       '.status = $status | .message = $message | .end_time = $timestamp' \
       "$RESULTS_FILE" > "$temp_file" && mv "$temp_file" "$RESULTS_FILE"
    
    echo "Result saved to: $RESULTS_FILE"
    cat "$RESULTS_FILE"
}

# Function to send Teams summary notification
send_teams_summary() {
    if [ -z "$TEAMS_WEBHOOK_URL" ]; then
        echo "Teams webhook URL not provided, skipping notification"
        return 0
    fi
    
    if [ ! -f "$RESULTS_FILE" ]; then
        echo "No results file found, skipping summary notification"
        return 0
    fi
    
    local total_count=$(jq '.results | length' "$RESULTS_FILE")
    local passed_count=$(jq '.results | map(select(.status == "passed")) | length' "$RESULTS_FILE")
    local failed_count=$(jq '.results | map(select(.status == "failed")) | length' "$RESULTS_FILE")
    local start_time=$(jq -r '.start_time' "$RESULTS_FILE")
    local end_time="$(date -u '+%Y-%m-%d %H:%M:%S UTC')"
    
    local title
    local color
    local summary_message
    
    if [ "$failed_count" -eq 0 ]; then
        title="✅ All TestKube Tests Passed"
        color="00b294"
        summary_message="All $total_count test environments completed successfully!"
    else
        title="❌ Some TestKube Tests Failed"
        color="d63333"
        summary_message="$failed_count out of $total_count test environments failed."
    fi
    
    # Build detailed results
    local details=""
    while IFS= read -r result; do
        local env=$(echo "$result" | jq -r '.environment')
        local status=$(echo "$result" | jq -r '.status')
        local message=$(echo "$result" | jq -r '.message')
        local icon="✅"
        if [ "$status" = "failed" ]; then
            icon="❌"
        fi
        details="$details$icon **$env**: $message\\n\\n"
    done <<< "$(jq -c '.results[]' "$RESULTS_FILE")"
    
    local payload=$(cat <<EOF
{
    "@type": "MessageCard",
    "@context": "http://schema.org/extensions",
    "themeColor": "$color",
    "summary": "$title",
    "sections": [{
        "activityTitle": "$title",
        "activitySubtitle": "TestKube Workflow Summary",
        "text": "$summary_message\\n\\n$details",
        "facts": [{
            "name": "Total Environments",
            "value": "$total_count"
        }, {
            "name": "Passed",
            "value": "$passed_count"
        }, {
            "name": "Failed", 
            "value": "$failed_count"
        }, {
            "name": "Started",
            "value": "$start_time"
        }, {
            "name": "Completed",
            "value": "$end_time"
        }]
    }]
}
EOF
    )
    
    curl -H "Content-Type: application/json" -d "$payload" "$TEAMS_WEBHOOK_URL" || echo "Failed to send Teams notification"
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

# Export environment variables for envsubst
export AMW_QUERY_ENDPOINT
export AZURE_CLIENT_ID

# Change to test directory
cd "$(dirname "$0")/../otelcollector/test/"

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

# Change to testkube directory
cd testkube

# Run the full test suite
kubectl testkube run testsuite e2e-tests-merge --verbose --job-template job-template.yaml

# Get the current id of the test suite now running
execution_id=$(kubectl testkube get testsuiteexecutions --test-suite e2e-tests-merge --limit 1 | grep e2e-tests | awk '{print $1}')
echo "Test suite execution ID: $execution_id"

# Watch until all the tests in the test suite finish
kubectl testkube watch testsuiteexecution "$execution_id"

# Get the results as a formatted json file
kubectl testkube get testsuiteexecution "$execution_id" --output json > testkube-results.json

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
    
    echo "Failed test processing completed"
    add_result "$TARGET_ENV" "failed" "Tests failed: $failed_tests_list. Check pipeline logs for details."
else
    echo "All tests passed successfully!"
    add_result "$TARGET_ENV" "passed" "All tests passed successfully"
fi

echo "================================================="
echo "TestKube deployment and testing workflow completed"
echo "================================================="
