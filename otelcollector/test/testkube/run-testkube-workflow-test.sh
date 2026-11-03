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
                run|watch) return 0 ;;
                get)
                    if [[ $# -eq 3 ]]; then
                        printf '%s\n' 'common-execution configprocessingcommon' 'error-execution customconfigmaperror'
                    else
                        echo '{"result":{"status":"passed"}}'
                    fi
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
