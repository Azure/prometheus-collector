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

# Step 1: Install TestKube CLI only if not ConfigTests
if [[ "$TARGET_ENV" != "ConfigTests" ]]; then
    echo "Step 1: Installing TestKube CLI..."
    wget -qO - https://repo.testkube.io/key.pub | sudo apt-key add -
    echo "deb https://repo.testkube.io/linux linux main" | sudo tee -a /etc/apt/sources.list
    sudo apt-get update
    sudo apt-get install -y testkube
    echo "✓ TestKube CLI installation completed"
fi

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
if [[ "$TARGET_ENV" == "ConfigTests" ]]; then
    envsubst < "./testkube/config-processing-test-crs/$SOURCE_TEMPLATE" > "./testkube/$TARGET_OUTPUT"
else
    envsubst < "./testkube/$SOURCE_TEMPLATE" > "./testkube/$TARGET_OUTPUT"
fi

# Apply the generated files
kubectl apply -f ./testkube/api-server-permissions.yaml
kubectl apply -f "./testkube/$TARGET_OUTPUT"

if [[ "$TARGET_ENV" != "ConfigTests" ]]; then
    # Apply common configmaps
    kubectl apply -f ./test-cluster-yamls/configmaps/ama-metrics-prometheus-config-configmap.yaml
    kubectl apply -f ./test-cluster-yamls/configmaps/ama-metrics-prometheus-config-node-configmap.yaml
    kubectl apply -f ./test-cluster-yamls/configmaps/ama-metrics-prometheus-config-node-windows-configmap.yaml
fi

# Apply settings configmap (unless explicitly disabled)
if [ "$APPLY_SETTINGS_CONFIGMAP" = "true" ]; then
    kubectl apply -f ./test-cluster-yamls/configmaps/ama-metrics-settings-configmap.yaml
    echo "✓ Applied settings configmap"
else
    echo "⚠ Skipped settings configmap (disabled)"
fi

if [[ "$TARGET_ENV" != "ConfigTests" ]]; then
    # Apply reference app
    kubectl apply -f ./test-cluster-yamls/customresources/prometheus-reference-app.yaml
    echo "✓ TestKube environment setup completed"
fi

# Step 3: Wait for cluster to be ready
echo "Step 3: Waiting for cluster to be ready for $SLEEP_DURATION seconds..."
sleep "$SLEEP_DURATION"
echo "✓ Cluster wait period completed"

# Step 4: Run TestKube tests
echo "Step 4: Starting TestKube workflow execution..."


echo "Run testkube testworkflows"
if [[ "$TARGET_ENV" == "ConfigTests" ]]; then
    echo "Running in ConfigTests environment"
    # Build workflow list for ConfigTests environment via template name
    case "$SOURCE_TEMPLATE" in
        testkube-config-test-all-targets-disabled-crs.yaml)
            workflows=(configprocessingcommon alltargetsdisabled)
            ;;
        testkube-config-test-all-targets-enabled-crs.yaml)
            workflows=(configprocessingcommon alltargetsenabled)
            ;;
        testkube-config-test-all-ds-targets-enabled-crs.yaml)
            workflows=(configprocessingcommon alldstargetsenabled)
            ;;
        testkube-config-test-all-rs-targets-enabled-crs.yaml)
            workflows=(configprocessingcommon allrstargetsenabled)
            ;;
        testkube-config-test-default-targets-on-crs.yaml)
            workflows=(configprocessingcommon defaultsettingsenabled)
            ;;
        testkube-config-test-default-targets-on-v2-crs.yaml)
            workflows=(configprocessingcommonv2 defaultsettingsenabledv2)
            ;;
        testkube-config-test-no-configmaps-crs.yaml)
            workflows=(configprocessingcommon noconfigmaps)
            ;;
        testkube-config-test-only-custom-configmap-crs.yaml)
            workflows=(configprocessingcommon customconfigmapallactions)
            ;;
        testkube-config-test-custom-configmap-error-crs.yaml)
            workflows=(configprocessingcommon customconfigmaperror)
            ;;
        testkube-config-test-custom-node-configmap-crs.yaml)
            workflows=(configprocessingcommon customnodeconfigmaps)
            ;;
        testkube-config-test-settings-error-crs.yaml)
            workflows=(configprocessingcommon settingserror)
            ;;
        testkube-config-test-global-ext-labels-error-crs.yaml)
            workflows=(configprocessingcommon extlabelserror)
            ;;
        testkube-config-test-global-settings-crs.yaml)
            workflows=(configprocessingcommon globalextlabels)
            ;;
        *)
            echo "Unknown ConfigTests source template: $SOURCE_TEMPLATE"
            exit 1
            ;;
    esac
else 
    # Build workflow list dynamically from cluster (exclude livenessprobe)
    mapfile -t workflows < <(kubectl testkube get testworkflows -o json | jq -r '.[].workflow.name')
    if [[ ${#workflows[@]} -gt 0 ]]; then
        # if [[ "$TARGET_ENV" != *Nightly* ]]; then
        # Filter out livenessprobe workflow if present
        workflows=("${workflows[@]/livenessprobe}")
        # else
        #     # Keep livenessprobe if present but ensure it runs last
        #     reordered=()
        #     lp=()
        #     for wf in "${workflows[@]}"; do
        #         if [[ "$wf" == "livenessprobe" ]]; then
        #             lp+=("$wf")
        #         else
        #             reordered+=("$wf")
        #         fi
        #     done
        #     workflows=("${reordered[@]}" "${lp[@]}")
        # fi
    fi
fi
if [[ ${#workflows[@]} -eq 0 ]]; then
    echo "No testworkflows found via kubectl testkube get testworkflows"
    exit 1
fi
failed_workflows=()
successful_workflows=()

for wf in "${workflows[@]}"; do
    echo "Running workflow: $wf"
    kubectl testkube run testworkflow "$wf"

    echo "Waiting for execution to be created..."
    sleep 5

    echo "Fetching testworkflow executions for $wf..."
    kubectl testkube get testworkflowexecution
    execution_id=$(kubectl testkube get testworkflowexecution | grep -i "$wf" | head -n 1 | awk '{print $1}')

    echo "Execution ID: $execution_id"

    # Check if execution_id is empty
    if [[ -z "$execution_id" ]]; then
        echo "Error: Could not find execution ID for $wf"
        exit 1
    fi

    # Watch until the testworkflow finishes
    kubectl testkube watch testworkflowexecution $execution_id

    # Get the results as a formatted json file
    kubectl testkube get testworkflowexecution $execution_id --output json > "testkube-results-${TARGET_ENV}-${wf}.json"

    # Verify the JSON is valid
    if ! jq empty "testkube-results-${TARGET_ENV}-${wf}.json" 2>/dev/null; then
        echo "Error: Failed to get valid JSON results from testkube for $wf"
        echo "Contents of testkube-results-${TARGET_ENV}-${wf}.json:"
        cat "testkube-results-${TARGET_ENV}-${wf}.json"
        exit 1
    fi

    # For any test that has failed, print out the logs
    if [[ $(jq -r '.result.status' "testkube-results-${TARGET_ENV}-${wf}.json") == "failed" ]]; then

        echo "$wf TestWorkflow failed. Execution ID: $execution_id"

        failed_workflows+=("${wf}")
    else
        successful_workflows+=("${wf}")
    fi
done

echo "\n========== TestWorkflow Summary =========="
if [[ ${#failed_workflows[@]} -gt 0 ]]; then
    echo "Failed workflows:"
    for wf in "${failed_workflows[@]}"; do
        echo "- $wf"
    done
    if [[ ${#successful_workflows[@]} -gt 0 ]]; then
        echo "Successful workflows:"
        for wf in "${successful_workflows[@]}"; do
            echo "- $wf"
        done
    fi
    pipeline_link=""
    if [ -n "$BUILD_BUILDID" ] && [ -n "$SYSTEM_JOBID" ] && [ -n "$SYSTEM_TASKINSTANCEID" ]; then
        pipeline_link="[View Pipeline Run](https://github-private.visualstudio.com/azure/_build/results?buildId=${BUILD_BUILDID}&view=logs&j=${SYSTEM_JOBID}&t=${SYSTEM_TASKINSTANCEID})"
    elif [ -n "$BUILD_BUILDID" ]; then
        pipeline_link="[View Pipeline Run](https://github-private.visualstudio.com/azure/_build/results?buildId=${BUILD_BUILDID})"
    fi
    
    echo "Failed test processing completed"
    add_result "$TARGET_ENV" "failed" "Tests failed: ${failed_workflows[*]}. Check pipeline logs for details." "${pipeline_link}"
    echo "========================================"
    exit 1
else
    echo "All workflows completed successfully."
    echo "Successful workflows:"
    for wf in "${successful_workflows[@]}"; do
        echo "- $wf"
    done
    add_result "$TARGET_ENV" "passed" "All tests passed" ""
    echo "========================================"
fi

echo "================================================="
echo "TestKube deployment and testing workflow completed"
echo "================================================="