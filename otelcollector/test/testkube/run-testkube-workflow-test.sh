#!/bin/bash
set -euo pipefail

script_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
workflow_script=${1:-"$script_dir/run-testkube-workflow.sh"}
test_dir=$(mktemp -d)
trap 'rm -rf "$test_dir"' EXIT

kubectl() {
    echo "$*" >> "$MOCK_DIR/commands"
    case "$1" in
        apply)
            if [[ "$3" == *api-server-permissions.yaml ]]; then
                [[ "$MOCK_CASE" != permissions-failure ]]
            else
                [[ "$MOCK_CASE" != apply-failure ]] || return 1
                if [[ "$MOCK_CASE" != new-workflows ]]; then
                    [[ -f "$MOCK_DIR/configprocessingcommon-reset" && -f "$MOCK_DIR/customconfigmaperror-reset" ]] || {
                        echo "metadata.resourceVersion: Invalid value: 0: must be specified for an update" >&2
                        return 1
                    }
                fi
                grep -q 'config-processing-common-with-error-config-maps' "$3"
            fi
            ;;
        get)
            [[ "$MOCK_CASE" != list-failure ]] || return 1
            if [[ "$MOCK_CASE" != new-workflows ]]; then
                printf '%s\n' testworkflow.testworkflows.testkube.io/configprocessingcommon testworkflow.testworkflows.testkube.io/customconfigmaperror
            fi
            ;;
        annotate)
            [[ "$MOCK_CASE" != annotation-failure ]] || return 1
            [[ "$2" == --namespace && "$3" == testkube && "$5" == kubectl.kubernetes.io/last-applied-configuration- ]] || return 1
            touch "$MOCK_DIR/${4##*/}-reset"
            ;;
        testkube)
            case "$2" in
                run)
                    [[ "$3" == testworkflow ]] || return 1
                    if [[ "$5" == --output && "$6" == json ]]; then
                        printf '{"id":"execution-%s"}\n' "$4"
                    else
                        [[ $# -eq 4 ]]
                    fi
                    ;;
                watch)
                    [[ "$MOCK_CASE" != watch-failure ]]
                    ;;
                get)
                    [[ "$3" == testworkflowexecution ]] || return 1
                    if [[ $# -eq 3 ]]; then
                        printf '%s\n' \
                            'execution-configprocessingcommon TestWorkflow configprocessingcommon' \
                            'execution-customconfigmaperror TestWorkflow customconfigmaperror'
                        return 0
                    fi
                    [[ "$5" == --output && "$6" == json ]] || return 1
                    status=passed
                    [[ "$MOCK_CASE" != execution-failure ]] || status=failed
                    if [[ "$MOCK_CASE" == running-then-passed ]]; then
                        count=0
                        [[ ! -f "$MOCK_DIR/get-count" ]] || count=$(cat "$MOCK_DIR/get-count")
                        count=$((count + 1))
                        printf '%s\n' "$count" > "$MOCK_DIR/get-count"
                        [[ "$count" -ne 1 ]] || status=running
                    fi
                    printf '{"id":"%s","workflow":{"name":"%s"},"result":{"status":"%s"}}\n' \
                        "$4" "${4#execution-}" "$status"
                    ;;
                download)
                    [[ "$3" == artifacts && "$5" == --download-dir && "$7" == --format && "$8" == folder ]] || return 1
                    [[ "$MOCK_CASE" != artifact-failure ]] || return 1
                    mkdir -p "$6"
                    touch "$6/artifact"
                    ;;
                *) return 1 ;;
            esac
            ;;
        *) echo "Unexpected kubectl command: $*" >&2; return 1 ;;
    esac
}

sleep() { :; }
export -f kubectl sleep

for scenario in existing-workflows new-workflows permissions-failure list-failure annotation-failure apply-failure generation-failure; do
    export MOCK_CASE="$scenario"
    export MOCK_DIR="$test_dir/$scenario"
    export BUILD_ARTIFACTSTAGINGDIRECTORY="$MOCK_DIR/results"
    mkdir -p "$MOCK_DIR/testkube/config-processing-test-crs"
    cp "$script_dir/config-processing-test-crs/testkube-config-test-custom-configmap-error-crs.yaml" "$MOCK_DIR/testkube/config-processing-test-crs/"
    if [[ "$scenario" == generation-failure ]]; then
        rm "$MOCK_DIR/testkube/config-processing-test-crs/testkube-config-test-custom-configmap-error-crs.yaml"
    fi

    status=0
    (
        cd "$MOCK_DIR"
        bash "$workflow_script" https://example.invalid test-client \
            testkube-config-test-custom-configmap-error-crs.yaml generated.yaml false 0 ConfigTests
    ) > "$MOCK_DIR/output" 2>&1 || status=$?

    if [[ "$scenario" == existing-workflows || "$scenario" == new-workflows ]]; then
        if [[ "$status" -ne 0 ]] || ! jq -e '.status == "passed"' "$MOCK_DIR/results/testkube-results-ConfigTests.json" > /dev/null; then
            cat "$MOCK_DIR/output"
            echo "FAIL: $scenario did not complete successfully" >&2
            exit 1
        fi
        grep -q 'testkube run testworkflow configprocessingcommon' "$MOCK_DIR/commands"
        grep -q 'testkube run testworkflow customconfigmaperror' "$MOCK_DIR/commands"
        if [[ "$scenario" == existing-workflows && (! -f "$MOCK_DIR/configprocessingcommon-reset" || ! -f "$MOCK_DIR/customconfigmaperror-reset") ]]; then
            echo "FAIL: polluted last-applied annotations were not reset" >&2
            exit 1
        fi
        if [[ "$scenario" == new-workflows ]] && grep -q '^annotate ' "$MOCK_DIR/commands"; then
            echo "FAIL: tried to annotate workflows that do not exist" >&2
            exit 1
        fi
    else
        if [[ "$status" -eq 0 ]] || ! jq -e '.status == "failed"' "$MOCK_DIR/results/testkube-results-ConfigTests.json" > /dev/null; then
            cat "$MOCK_DIR/output"
            echo "FAIL: $scenario was not reported as failed" >&2
            exit 1
        fi
        if [[ -f "$MOCK_DIR/commands" ]] && grep -q 'testkube run' "$MOCK_DIR/commands"; then
            echo "FAIL: $scenario executed a stale workflow" >&2
            exit 1
        fi
    fi
    echo "PASS: $scenario"
done

export MOCK_CASE=explicit-workflow
export MOCK_DIR="$test_dir/explicit-workflow"
execution_directory="$MOCK_DIR/executions"
mkdir -p "$MOCK_DIR"
if ! bash "$workflow_script" --run-existing customconfigmaperror "$execution_directory" \
    > "$MOCK_DIR/output" 2>&1; then
    cat "$MOCK_DIR/output"
    echo "FAIL: selected workflow execution did not complete successfully" >&2
    exit 1
fi

submission="$execution_directory/execution-customconfigmaperror/submission.json"
execution="$execution_directory/execution-customconfigmaperror/execution.json"
if ! jq -e '.id == "execution-customconfigmaperror"' "$submission" > /dev/null \
    || ! jq -e '.id == "execution-customconfigmaperror" and .workflow.name == "customconfigmaperror" and .result.status == "passed"' "$execution" > /dev/null \
    || [[ ! -f "$execution_directory/execution-customconfigmaperror/artifacts/artifact" ]]; then
    cat "$MOCK_DIR/output"
    echo "FAIL: selected workflow execution evidence was incomplete or mismatched" >&2
    exit 1
fi
[[ $(grep -c '^testkube run ' "$MOCK_DIR/commands") -eq 1 ]] || {
    echo "FAIL: selected-workflow mode ran more than one workflow" >&2
    exit 1
}
grep -q 'testkube watch testworkflowexecution execution-customconfigmaperror' "$MOCK_DIR/commands"
grep -q 'testkube get testworkflowexecution execution-customconfigmaperror --output json' "$MOCK_DIR/commands"
grep -q 'testkube download artifacts execution-customconfigmaperror' "$MOCK_DIR/commands"
echo "PASS: explicit-workflow"

export MOCK_CASE=running-then-passed
export MOCK_DIR="$test_dir/running-then-passed"
mkdir -p "$MOCK_DIR"
if ! bash "$workflow_script" --run-existing customconfigmaperror "$MOCK_DIR/executions" \
    > "$MOCK_DIR/output" 2>&1; then
    cat "$MOCK_DIR/output"
    echo "FAIL: selected workflow did not wait for a terminal execution status" >&2
    exit 1
fi
[[ $(cat "$MOCK_DIR/get-count") -eq 2 ]] \
    && jq -e '.result.status == "passed"' "$MOCK_DIR/executions/execution-customconfigmaperror/execution.json" > /dev/null \
    || {
    cat "$MOCK_DIR/output"
    echo "FAIL: selected workflow returned before the execution reached terminal status" >&2
    exit 1
}
echo "PASS: running-then-passed"

export MOCK_CASE=execution-failure
export MOCK_DIR="$test_dir/execution-failure"
mkdir -p "$MOCK_DIR"
if ! bash "$workflow_script" --run-existing customconfigmaperror "$MOCK_DIR/executions" \
    > "$MOCK_DIR/output" 2>&1; then
    cat "$MOCK_DIR/output"
    echo "FAIL: terminal workflow failure was not returned as execution evidence" >&2
    exit 1
fi
jq -e '.result.status == "failed"' \
    "$MOCK_DIR/executions/execution-customconfigmaperror/execution.json" > /dev/null
echo "PASS: terminal-workflow-failure"

export MOCK_CASE=watch-failure
export MOCK_DIR="$test_dir/watch-failure"
mkdir -p "$MOCK_DIR"
if ! bash "$workflow_script" --run-existing customconfigmaperror "$MOCK_DIR/executions" \
    > "$MOCK_DIR/output" 2>&1; then
    cat "$MOCK_DIR/output"
    echo "FAIL: watch failure prevented exact execution evidence from being returned" >&2
    exit 1
fi
[[ ! -f "$MOCK_DIR/executions/execution-customconfigmaperror/watch-failed" ]] \
    && jq -e '.result.status == "passed"' "$MOCK_DIR/executions/execution-customconfigmaperror/execution.json" > /dev/null || {
    echo "FAIL: a successful terminal execution was marked failed because watch errored" >&2
    exit 1
}
echo "PASS: watch-failure"

export MOCK_CASE=artifact-failure
export MOCK_DIR="$test_dir/artifact-failure"
mkdir -p "$MOCK_DIR"
if bash "$workflow_script" --run-existing customconfigmaperror "$MOCK_DIR/executions" \
    > "$MOCK_DIR/output" 2>&1; then
    cat "$MOCK_DIR/output"
    echo "FAIL: artifact download failure was reported as success" >&2
    exit 1
fi
[[ -f "$MOCK_DIR/executions/execution-customconfigmaperror/execution.json" ]] || {
    echo "FAIL: terminal execution evidence was not retained after artifact failure" >&2
    exit 1
}
echo "PASS: artifact-failure"
