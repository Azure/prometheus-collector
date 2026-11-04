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
                    [[ -f "$MOCK_DIR/reset" ]] || return 1
                fi
                cp "$3" "$MOCK_DIR/applied"
            fi
            ;;
        get)
            [[ "$MOCK_CASE" != list-failure ]] || return 1
            if [[ "$MOCK_CASE" != new-workflows ]]; then
                echo testworkflow.testworkflows.testkube.io/configprocessingcommon
            fi
            ;;
        annotate)
            [[ "$MOCK_CASE" != annotation-failure ]] || return 1
            [[ "$2" == --namespace && "$3" == testkube && "$5" == kubectl.kubernetes.io/last-applied-configuration- ]] || return 1
            touch "$MOCK_DIR/reset"
            ;;
        testkube)
            case "$2" in
                run)
                    [[ "$MOCK_CASE" != submit-failure ]] || return 1
                    if [[ "$MOCK_CASE" == missing-id ]]; then echo '{}'; else
                        printf '{"id":"%s-exact"}\n' "$4"
                    fi
                    ;;
                watch)
                    [[ "$4" == *-exact && "$MOCK_CASE" != watch-failure ]]
                    ;;
                get)
                    local id="$4" status=passed
                    [[ "$id" == *-exact ]] || return 1
                    case "$MOCK_CASE" in
                        running|queued|aborted|timeout|failed) status="$MOCK_CASE" ;;
                        missing-status) status="" ;;
                    esac
                    local workflow="${id%-exact}"
                    [[ "$MOCK_CASE" != mismatched-id ]] || id=some-other-execution
                    [[ "$MOCK_CASE" != mismatched-workflow ]] || workflow=some-other-workflow
                    printf '{"id":"%s","workflow":{"name":"%s"},"result":{"status":"%s"}}\n' "$id" "$workflow" "$status"
                    ;;
                download)
                    [[ "$MOCK_CASE" != download-failure && "$3" == artifacts && "$4" == *-exact ]] || return 1
                    local directory="$6"
                    mkdir -p "$directory"
                    local state=passed succeeded=true
                    [[ "$MOCK_CASE" != empty-report ]] || state=skipped
                    [[ "$MOCK_CASE" != failed-report ]] || { state=failed; succeeded=false; }
                    if [[ "$MOCK_CASE" != missing-json ]]; then
                        printf '[{"SuiteSucceeded":%s,"SpecReports":[{"State":"passed","LeafNodeType":"BeforeSuite"},{"State":"%s","LeafNodeType":"It"},{"State":"skipped","LeafNodeType":"It"}]}]\n' "$succeeded" "$state" > "$directory/ginkgo.json"
                    fi
                    if [[ "$MOCK_CASE" != missing-junit ]]; then
                        local failed=""
                        [[ "$MOCK_CASE" != failed-junit ]] || failed='<failure/>'
                        printf '<testsuites><testsuite><testcase name="[BeforeSuite]"/><testcase name="[It] runs">%s</testcase><testcase name="[It] skipped"><skipped/></testcase></testsuite></testsuites>\n' "$failed" > "$directory/junit.xml"
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

for scenario in existing-workflows new-workflows permissions-failure list-failure annotation-failure apply-failure \
    generation-failure submit-failure missing-id mismatched-id mismatched-workflow watch-failure \
    running queued aborted timeout failed missing-status download-failure missing-json missing-junit \
    empty-report failed-report failed-junit candidate-config candidate-health incomplete-candidate mutable-candidate unknown-workflow; do
    export MOCK_CASE="$scenario"
    export MOCK_DIR="$test_dir/$scenario"
    export BUILD_ARTIFACTSTAGINGDIRECTORY="$MOCK_DIR/results"
    unset CANDIDATE_TEST_SOURCE_SHA CANDIDATE_RUNNER_IMAGE SELECTED_WORKFLOW GINKGO_LABEL_FILTER
    mkdir -p "$MOCK_DIR/testkube/config-processing-test-crs"
    template=testkube-config-test-custom-configmap-error-crs.yaml
    cp "$script_dir/config-processing-test-crs/$template" "$MOCK_DIR/testkube/config-processing-test-crs/"
    cp "$script_dir/testkube-test-crs.yaml" "$MOCK_DIR/testkube/"
    [[ "$scenario" != generation-failure ]] || rm "$MOCK_DIR/testkube/config-processing-test-crs/$template"
    if [[ "$scenario" == candidate-* || "$scenario" == *-candidate || "$scenario" == unknown-workflow ]]; then
        export CANDIDATE_TEST_SOURCE_SHA=aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa
        export CANDIDATE_RUNNER_IMAGE="docker.io/cerebro31/ginkgo-test@sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
        export SELECTED_WORKFLOW=configprocessingcommon
        export GINKGO_LABEL_FILTER="config-processing-common || config-processing-custom-config-map-error"
        if [[ "$scenario" == candidate-health ]]; then
            template=testkube-test-crs.yaml
            export SELECTED_WORKFLOW=operator
            unset GINKGO_LABEL_FILTER
        fi
        [[ "$scenario" != incomplete-candidate ]] || unset CANDIDATE_RUNNER_IMAGE
        [[ "$scenario" != mutable-candidate ]] || export CANDIDATE_RUNNER_IMAGE=cerebro31/ginkgo-test:latest
        [[ "$scenario" != unknown-workflow ]] || export SELECTED_WORKFLOW=missing-workflow
    fi

    status=0
    (
        cd "$MOCK_DIR"
        bash "$workflow_script" https://example.invalid test-client "$template" generated.yaml false 0 ConfigTests
    ) > "$MOCK_DIR/output" 2>&1 || status=$?

    result="$MOCK_DIR/results/testkube-results-ConfigTests.json"
    if [[ "$scenario" == existing-workflows || "$scenario" == new-workflows || "$scenario" == candidate-* ]]; then
        if [[ "$status" -ne 0 ]] || ! jq -e '.status == "passed" and (.executions | length > 0) and all(.executions[]; .passed == 1 and .skipped == 1 and .failed == 0)' "$result" > /dev/null; then
            cat "$MOCK_DIR/output"; echo "FAIL: $scenario did not pass with evidence" >&2; exit 1
        fi
        if [[ "$scenario" == candidate-* ]]; then
            jq -e --arg sha "$CANDIDATE_TEST_SOURCE_SHA" --arg image "$CANDIDATE_RUNNER_IMAGE" --arg wf "$SELECTED_WORKFLOW" \
                '.test_source_sha == $sha and .runner_image == $image and (.executions | length == 1) and .executions[0].workflow == $wf' "$result" > /dev/null
            jq -e --arg sha "$CANDIDATE_TEST_SOURCE_SHA" --arg image "$CANDIDATE_RUNNER_IMAGE" \
                '.spec.content.git.revision == $sha and .spec.steps[0].container.image == $image' "$MOCK_DIR/applied" > /dev/null
            if grep -q 'apply -f ./test-cluster-yamls' "$MOCK_DIR/commands"; then
                echo "FAIL: candidate performed a second fixture lifecycle" >&2; exit 1
            fi
        else
            jq -e '.executions | length == 2' "$result" > /dev/null
        fi
        if [[ "$scenario" == new-workflows ]] && grep -q '^annotate ' "$MOCK_DIR/commands"; then
            echo "FAIL: annotated nonexistent workflows" >&2; exit 1
        fi
    else
        if [[ "$status" -eq 0 ]] || ! jq -e '.status == "failed"' "$result" > /dev/null; then
            cat "$MOCK_DIR/output"; echo "FAIL: $scenario was not reported failed" >&2; exit 1
        fi
        case "$scenario" in
            permissions-failure|list-failure|annotation-failure|apply-failure|generation-failure|incomplete-candidate|mutable-candidate|unknown-workflow)
                if [[ -f "$MOCK_DIR/commands" ]] && grep -q 'testkube run' "$MOCK_DIR/commands"; then
                    echo "FAIL: $scenario ran stale definitions" >&2; exit 1
                fi ;;
        esac
    fi
    echo "PASS: $scenario"
done
