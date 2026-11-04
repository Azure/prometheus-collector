#!/bin/bash
set -eo pipefail

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
    exit 2
fi

AMW_QUERY_ENDPOINT="$1"
AZURE_CLIENT_ID="$2"
SOURCE_TEMPLATE="$3"
TARGET_OUTPUT="$4"
APPLY_SETTINGS_CONFIGMAP="${5:-true}"
SLEEP_DURATION="${6:-360}"
TARGET_ENV="${7:-Unknown}"
BRANCH_NAME="${8:-main}"
[[ "$TARGET_ENV" =~ ^[A-Za-z0-9_-]+$ ]] || { echo "Unsafe environment name" >&2; exit 2; }
SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
PYTHON="${PYTHON:-python3}"
CANDIDATE_MODE=false
if [[ -n "${CANDIDATE_TEST_SOURCE_SHA:-}${CANDIDATE_RUNNER_IMAGE:-}${SELECTED_WORKFLOW:-}${GINKGO_LABEL_FILTER:-}" ]]; then
    CANDIDATE_MODE=true
fi
: "${BUILD_ARTIFACTSTAGINGDIRECTORY:?Set an artifact staging directory}"
mkdir -p "$BUILD_ARTIFACTSTAGINGDIRECTORY"
BUILD_ARTIFACTSTAGINGDIRECTORY=$(cd "$BUILD_ARTIFACTSTAGINGDIRECTORY" && pwd -P)

# Define shared results file  
RESULTS_FILE="${BUILD_ARTIFACTSTAGINGDIRECTORY}/testkube-results-${TARGET_ENV}.json"

# Function to initialize or update results file
init_results_file() {
    # Create the artifact staging directory if it doesn't exist
    mkdir -p "$(dirname "$RESULTS_FILE")"
    
    jq -n --arg env "$TARGET_ENV" --arg start "$(date -u '+%Y-%m-%d %H:%M:%S UTC')" \
        --arg sha "${CANDIDATE_TEST_SOURCE_SHA:-}" --arg image "${CANDIDATE_RUNNER_IMAGE:-}" \
        '{environment:$env, start_time:$start, status:"running", test_source_sha:$sha,
          runner_image:$image, executions:[]}' > "$RESULTS_FILE"
}

# Function to add result to file
add_result() {
    local env="$1"
    local status="$2" 
    local message="$3"
    local link="$4"
    
    # Create temporary file with updated results
    local temp_file="${RESULTS_FILE}.tmp.$$"
    jq --arg env "$env" --arg status "$status" --arg message "$message" --arg link "$link" \
       '.status = $status | .message = $message | .link = $link' \
       "$RESULTS_FILE" > "$temp_file" || { echo "Failed to write TestKube result" >&2; exit 1; }
    mv "$temp_file" "$RESULTS_FILE" || { echo "Failed to retain TestKube result" >&2; exit 1; }
    
    echo "Result saved to: $RESULTS_FILE"
    cat "$RESULTS_FILE"
}

fail_setup() {
    echo "Error: $1" >&2
    add_result "$TARGET_ENV" "failed" "$1" ""
    exit 1
}

echo "================================================="
echo "Starting TestKube deployment and testing workflow"
echo "================================================="

# Initialize results file
init_results_file
trap 'fail_setup "TestKube runner command failed at line $LINENO"' ERR

[[ "$SOURCE_TEMPLATE" =~ ^[A-Za-z0-9_-]+\.yaml$ ]] || fail_setup "Use an existing template filename"
[[ "$TARGET_OUTPUT" =~ ^[A-Za-z0-9_-]+\.yaml$ ]] || fail_setup "Use a generated output filename"
if [[ "$CANDIDATE_MODE" == true ]]; then
    [[ -n "${CANDIDATE_TEST_SOURCE_SHA:-}" && -n "${CANDIDATE_RUNNER_IMAGE:-}" && -n "${SELECTED_WORKFLOW:-}" ]] \
        || fail_setup "Candidate mode requires source SHA, runner digest and selected workflow together"
    [[ "$APPLY_SETTINGS_CONFIGMAP" == false && "$SLEEP_DURATION" == 0 ]] \
        || fail_setup "Candidate fixture lifecycle and readiness belong to the caller; use false 0"
fi

# Step 1: Install TestKube CLI only if not ConfigTests
if [[ "$TARGET_ENV" != "ConfigTests" && "$CANDIDATE_MODE" != true ]]; then
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
if [[ "$SOURCE_TEMPLATE" == testkube-config-test-*-crs.yaml ]]; then
    template="./testkube/config-processing-test-crs/$SOURCE_TEMPLATE"
else
    template="./testkube/$SOURCE_TEMPLATE"
fi
if [[ "$CANDIDATE_MODE" == true && "$template" == "./testkube/$TARGET_OUTPUT" ]]; then
    fail_setup "Candidate output must not overwrite its source template"
fi
rendered=$(mktemp)
envsubst '${AMW_QUERY_ENDPOINT} ${AZURE_CLIENT_ID} ${BRANCH_NAME}' < "$template" > "$rendered" \
    || { rm -f "$rendered"; fail_setup "Failed to generate TestKube workflows"; }
if [[ "$CANDIDATE_MODE" == true ]]; then
    label_args=()
    if [[ -v GINKGO_LABEL_FILTER ]]; then
        label_args=(--label-filter "$GINKGO_LABEL_FILTER")
    fi
    for expectation in EXPECTED_CLUSTER EXPECT_OPERATOR_HTTPS EXPECT_WINDOWS_EXPORTER; do
        if [[ -v "$expectation" ]]; then
            option=${expectation,,}
            label_args+=("--${option//_/-}" "${!expectation}")
        fi
    done
    "$PYTHON" "$SCRIPT_DIR/workflow_evidence.py" render "$rendered" \
        "$SELECTED_WORKFLOW" "$CANDIDATE_TEST_SOURCE_SHA" "$CANDIDATE_RUNNER_IMAGE" \
        "${label_args[@]}" > "./testkube/$TARGET_OUTPUT" \
        || { rm -f "$rendered"; fail_setup "Candidate workflow validation failed"; }
else
    cp "$rendered" "./testkube/$TARGET_OUTPUT"
fi
rm -f "$rendered"

# Apply the generated files
kubectl apply -f ./testkube/api-server-permissions.yaml \
    || fail_setup "Failed to apply TestKube API permissions"

if [[ "$TARGET_ENV" == "ConfigTests" || "$CANDIDATE_MODE" == true ]]; then
    existing_workflows=$(kubectl get -f "./testkube/$TARGET_OUTPUT" --ignore-not-found -o name) \
        || fail_setup "Failed to list existing ConfigTests workflows"
    # Exported last-applied metadata can make apply try to delete resourceVersion.
    # Reset only the workflows in this scenario, without deleting their history.
    while IFS= read -r workflow; do
        [[ -z "$workflow" ]] && continue
        kubectl annotate --namespace testkube "$workflow" kubectl.kubernetes.io/last-applied-configuration- \
            || fail_setup "Failed to reset last-applied metadata for $workflow"
    done <<< "$existing_workflows"
fi

kubectl apply -f "./testkube/$TARGET_OUTPUT" \
    || fail_setup "Failed to apply TestKube workflows; refusing to run stale definitions"

if [[ "$TARGET_ENV" != "ConfigTests" && "$CANDIDATE_MODE" != true ]]; then
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

if [[ "$TARGET_ENV" != "ConfigTests" && "$CANDIDATE_MODE" != true ]]; then
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
if [[ "$CANDIDATE_MODE" == true ]]; then
    workflows=("$SELECTED_WORKFLOW")
elif [[ "$TARGET_ENV" == "ConfigTests" ]]; then
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
        testkube-config-test-controlplane-istio-crs.yaml)
            workflows=(configprocessingcommon controlplaneistioenabled)
            ;;
        testkube-config-test-mip-false-crs.yaml)
            workflows=(configprocessingmipfalse mipfalse)
            ;;
        testkube-config-test-partial-no-pcs-v1-crs.yaml)
            workflows=(partialnopcsv1)
            ;;
        testkube-config-test-partial-no-pcs-v2-crs.yaml)
            workflows=(partialnopcsv2)
            ;;
        testkube-config-test-basic-auth-smon-crs.yaml)
            workflows=(basicauthsmon)
            ;;
        testkube-config-test-basic-auth-secret-update-crs.yaml)
            workflows=(basicauthsecretupdate)
            ;;
        testkube-config-test-basic-auth-smon-v2-crs.yaml)
            workflows=(basicauthsmonv2)
            ;;
        testkube-config-test-basic-auth-secret-update-v2-crs.yaml)
            workflows=(basicauthsecretupdatev2)
            ;;
        testkube-config-test-deny-fs-access-crs.yaml)
            workflows=(denyfsaccess)
            ;;
        testkube-config-test-customer-settings-v1-crs.yaml)
            workflows=(customersettingsv1)
            ;;
        testkube-config-test-customer-settings-v2-crs.yaml)
            workflows=(customersettingsv2)
            ;;
        testkube-config-test-dcgm-crs.yaml)
            workflows=(dcgmexporter)
            ;;
        *)
            echo "Unknown ConfigTests source template: $SOURCE_TEMPLATE"
            exit 1
            ;;
    esac
else 
    # Build workflow list dynamically from cluster (exclude livenessprobe)
    workflow_json=$(kubectl testkube get testworkflows -o json) \
        || fail_setup "Failed to list TestKube workflows"
    if ! jq -e 'type == "array" and all(.[]; (.workflow.name | type == "string" and length > 0))' \
        <<< "$workflow_json" >/dev/null; then
        fail_setup "TestKube workflow list is invalid or contains empty names"
    fi
    mapfile -t workflows < <(jq -r '.[].workflow.name' <<< "$workflow_json")
    if [[ ${#workflows[@]} -gt 0 ]]; then
        if [[ "$TARGET_ENV" != *Nightly* ]]; then
            # Filter out livenessprobe workflow without leaving empty entries
            filtered=()
            for wf in "${workflows[@]}"; do
                [[ "$wf" == "livenessprobe" ]] && continue
                filtered+=("$wf")
            done
            workflows=("${filtered[@]}")
        else
            # Keep livenessprobe if present but ensure it runs last
            reordered=()
            lp=()
            for wf in "${workflows[@]}"; do
                if [[ "$wf" == "livenessprobe" ]]; then
                    lp+=("$wf")
                else
                    reordered+=("$wf")
                fi
            done
            workflows=("${reordered[@]}" "${lp[@]}")
        fi
    fi
fi
if [[ ${#workflows[@]} -eq 0 ]]; then
    fail_setup "No TestWorkflows selected"
fi
failed_workflows=()
successful_workflows=()

for wf in "${workflows[@]}"; do
    [[ "$wf" =~ ^[a-z0-9][a-z0-9-]*$ ]] || fail_setup "Unsafe TestWorkflow name"
    echo "Running workflow: $wf"
    submission_output=$(kubectl testkube run testworkflow "$wf" --output json) \
        || fail_setup "Failed to submit TestKube workflow $wf"
    execution_id=$(jq -er 'if type == "object" and (.id | type == "string" and length > 0) then .id else empty end' \
        <<< "$submission_output" 2>/dev/null) \
        || fail_setup "TestKube submission for $wf returned no execution ID"
    [[ "$execution_id" =~ ^[A-Za-z0-9][A-Za-z0-9_-]*$ ]] || fail_setup "Unsafe execution ID"

    echo "Execution ID: $execution_id"
    case_dir="${BUILD_ARTIFACTSTAGINGDIRECTORY}/testkube-executions/${TARGET_ENV}/${wf}/${execution_id}"
    mkdir -p "$case_dir" || fail_setup "Failed to create evidence directory for $wf"
    printf '%s\n' "$submission_output" > "$case_dir/submission.json"

    # Watch the exact execution returned by the submission command.
    watch_status=0
    kubectl testkube watch testworkflowexecution "$execution_id" || watch_status=$?

    # Keep the exact execution result alongside its submission metadata.
    execution_file="$case_dir/execution.json"
    kubectl testkube get testworkflowexecution "$execution_id" --output json > "$execution_file" \
        || fail_setup "Failed to fetch execution $execution_id for $wf"

    # Verify the JSON is valid
    if ! jq -e --arg id "$execution_id" --arg workflow "$wf" \
        'type == "object" and .id == $id and .workflow.name == $workflow' "$execution_file" >/dev/null 2>&1; then
        fail_setup "TestKube returned invalid or mismatched execution evidence for $wf ($execution_id)"
    fi

    execution_status=$(jq -er '.result.status | strings | select(length > 0)' "$execution_file" 2>/dev/null) \
        || execution_status=""
    case_dir_record="$case_dir/record.json"
    jq -n --arg workflow "$wf" --arg execution_id "$execution_id" --arg status "$execution_status" \
        '{workflow: $workflow, execution_id: $execution_id, status: $status}' > "$case_dir_record"

    evidence_passed=true
    if [[ "$TARGET_ENV" == "ConfigTests" || "$CANDIDATE_MODE" == true ]]; then
        artifact_dir="$case_dir/artifacts"
        mkdir -p "$artifact_dir" || fail_setup "Failed to create artifact directory for $wf"
        kubectl testkube download artifacts "$execution_id" --download-dir "$artifact_dir" --format folder \
            || fail_setup "Failed to download artifacts for $wf ($execution_id)"

        if ! counts=$("$PYTHON" "$SCRIPT_DIR/workflow_evidence.py" reports "$artifact_dir"); then
            echo "$wf has missing, empty, failed or mismatched reports for $execution_id" >&2
            evidence_passed=false
        else
            artifact_path=$("$PYTHON" -c 'from pathlib import Path; import sys; print(Path(sys.argv[1]).resolve())' "$artifact_dir")
            jq --argjson counts "$counts" --arg dir "$artifact_path" \
                '. + $counts + {artifact_dir:$dir}' "$case_dir_record" > "$case_dir_record.tmp"
            mv "$case_dir_record.tmp" "$case_dir_record"
        fi
    fi

    if [[ "$execution_status" != "passed" || "$watch_status" -ne 0 || "$evidence_passed" != true ]]; then
        echo "$wf TestWorkflow did not pass (status: ${execution_status:-missing}, execution ID: $execution_id)"
        failed_workflows+=("${wf}")
        jq '.status="failed"' "$case_dir_record" > "$case_dir_record.tmp"
        mv "$case_dir_record.tmp" "$case_dir_record"
    else
        successful_workflows+=("${wf}")
    fi
    jq --slurpfile record "$case_dir_record" '.executions += $record' "$RESULTS_FILE" > "$RESULTS_FILE.tmp"
    mv "$RESULTS_FILE.tmp" "$RESULTS_FILE"
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