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
            if [[ "$3" == "-" ]]; then
                cat > "$MOCK_DIR/shared-template.yaml"
                [[ "$MOCK_CASE" != template-failure ]] || return 1
                grep -q 'kind: TestWorkflowTemplate' "$MOCK_DIR/shared-template.yaml" &&
                    grep -q 'name: configprocessingginkgo' "$MOCK_DIR/shared-template.yaml"
            elif [[ "$3" == *api-server-permissions.yaml ]]; then
                [[ "$MOCK_CASE" != permissions-failure ]]
            else
                [[ "$MOCK_CASE" != apply-failure ]] || return 1
                if [[ "$MOCK_CASE" == customer-settings-v1 || "$MOCK_CASE" == customer-settings-v2 || "$MOCK_CASE" == all-targets-enabled || "$MOCK_CASE" == default-targets-on || "$MOCK_CASE" == template-failure ]]; then
                    [[ -f "$MOCK_DIR/shared-template.yaml" ]] || return 1
                fi
                if [[ "$MOCK_CASE" != new-workflows ]]; then
                    case "$MOCK_CASE" in
                        customer-settings-v1) reset_files=(customersettingsv1-reset) ;;
                        customer-settings-v2) reset_files=(customersettingsv2-reset) ;;
                        all-targets-enabled) reset_files=(configprocessingcommon-reset alltargetsenabled-reset) ;;
                        default-targets-on) reset_files=(configprocessingcommon-reset defaultsettingsenabled-reset) ;;
                        *) reset_files=(configprocessingcommon-reset customconfigmaperror-reset) ;;
                    esac
                    for reset_file in "${reset_files[@]}"; do
                        [[ -f "$MOCK_DIR/$reset_file" ]] || {
                            echo "metadata.resourceVersion: Invalid value: 0: must be specified for an update" >&2
                            return 1
                        }
                    done
                fi
                case "$MOCK_CASE" in
                    customer-settings-v1) grep -q 'labelFilter: config-processing-customer-settings-v1' "$3" ;;
                    customer-settings-v2) grep -q 'labelFilter: config-processing-customer-settings-v2' "$3" ;;
                    all-targets-enabled) grep -q 'config-processing-all-targets-enabled' "$3" ;;
                    default-targets-on) grep -q 'config-processing-default-targets-enabled' "$3" ;;
                    *) grep -q 'config-processing-common-with-error-config-maps' "$3" ;;
                esac
            fi
            ;;
        get)
            [[ "$MOCK_CASE" != list-failure ]] || return 1
            if [[ "$MOCK_CASE" == all-targets-enabled ]]; then
                printf '%s\n' testworkflow.testworkflows.testkube.io/configprocessingcommon testworkflow.testworkflows.testkube.io/alltargetsenabled
            elif [[ "$MOCK_CASE" == default-targets-on ]]; then
                printf '%s\n' testworkflow.testworkflows.testkube.io/configprocessingcommon testworkflow.testworkflows.testkube.io/defaultsettingsenabled
            elif [[ "$MOCK_CASE" == customer-settings-v1 ]]; then
                printf '%s\n' testworkflow.testworkflows.testkube.io/customersettingsv1
            elif [[ "$MOCK_CASE" == customer-settings-v2 ]]; then
                printf '%s\n' testworkflow.testworkflows.testkube.io/customersettingsv2
            elif [[ "$MOCK_CASE" != new-workflows ]]; then
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
                    printf '{"id":"execution-%s"}\n' "$4"
                    ;;
                watch) return 0 ;;
                get)
                    if [[ $# -eq 3 ]]; then
                        for workflow in configprocessingcommon customconfigmaperror customersettingsv1 customersettingsv2 alltargetsenabled defaultsettingsenabled; do
                            printf 'execution-%s %s\n' "$workflow" "$workflow"
                        done
                    else
                        printf '{"id":"%s","result":{"status":"passed"}}\n' "$4"
                    fi
                    ;;
                download)
                    mkdir -p "$6"
                    printf '[{"SuiteSucceeded":true,"SpecReports":[{"State":"passed"}]}]\n' > "$6/ginkgo.json"
                    printf '<testsuite><testcase name="passed" /></testsuite>\n' > "$6/junit.xml"
                    ;;
                *) return 1 ;;
            esac
            ;;
        *) echo "Unexpected kubectl command: $*" >&2; return 1 ;;
    esac
}

sleep() { :; }
python3() {
    [[ "$1" == "-" && -f "$2" ]] && grep -q '<testcase name="passed"' "$2"
}
export -f kubectl sleep python3

for scenario in existing-workflows new-workflows permissions-failure list-failure annotation-failure apply-failure generation-failure customer-settings-v1 customer-settings-v2 all-targets-enabled default-targets-on template-failure missing-template; do
    export MOCK_CASE="$scenario"
    export MOCK_DIR="$test_dir/$scenario"
    export BUILD_ARTIFACTSTAGINGDIRECTORY="$MOCK_DIR/results"
    mkdir -p "$MOCK_DIR/testkube/config-processing-test-crs"
    cp "$script_dir/config-processing-test-crs/testkube-config-test-custom-configmap-error-crs.yaml" "$MOCK_DIR/testkube/config-processing-test-crs/"
    cp "$script_dir/config-processing-test-crs/testkube-config-test-ginkgo-template-crs.yaml" "$MOCK_DIR/testkube/config-processing-test-crs/"
    cp "$script_dir/config-processing-test-crs/testkube-config-test-customer-settings-v1-crs.yaml" "$MOCK_DIR/testkube/config-processing-test-crs/"
    cp "$script_dir/config-processing-test-crs/testkube-config-test-customer-settings-v2-crs.yaml" "$MOCK_DIR/testkube/config-processing-test-crs/"
    cp "$script_dir/config-processing-test-crs/testkube-config-test-all-targets-enabled-crs.yaml" "$MOCK_DIR/testkube/config-processing-test-crs/"
    cp "$script_dir/config-processing-test-crs/testkube-config-test-default-targets-on-crs.yaml" "$MOCK_DIR/testkube/config-processing-test-crs/"
    if [[ "$scenario" == generation-failure ]]; then
        rm "$MOCK_DIR/testkube/config-processing-test-crs/testkube-config-test-custom-configmap-error-crs.yaml"
    elif [[ "$scenario" == missing-template ]]; then
        rm "$MOCK_DIR/testkube/config-processing-test-crs/testkube-config-test-ginkgo-template-crs.yaml"
    fi

    source_template=testkube-config-test-custom-configmap-error-crs.yaml
    case "$scenario" in
        customer-settings-v1|template-failure|missing-template) source_template=testkube-config-test-customer-settings-v1-crs.yaml ;;
        customer-settings-v2) source_template=testkube-config-test-customer-settings-v2-crs.yaml ;;
        all-targets-enabled) source_template=testkube-config-test-all-targets-enabled-crs.yaml ;;
        default-targets-on) source_template=testkube-config-test-default-targets-on-crs.yaml ;;
    esac
    status=0
    (
        cd "$MOCK_DIR"
        bash "$workflow_script" https://example.invalid test-client \
            "$source_template" generated.yaml false 0 ConfigTests
    ) > "$MOCK_DIR/output" 2>&1 || status=$?

    if [[ "$scenario" == existing-workflows || "$scenario" == new-workflows || "$scenario" == customer-settings-v1 || "$scenario" == customer-settings-v2 || "$scenario" == all-targets-enabled || "$scenario" == default-targets-on ]]; then
        if [[ "$status" -ne 0 ]] || ! jq -e '.status == "passed"' "$MOCK_DIR/results/testkube-results-ConfigTests.json" > /dev/null; then
            cat "$MOCK_DIR/output"
            echo "FAIL: $scenario did not complete successfully" >&2
            exit 1
        fi
        case "$scenario" in
            customer-settings-v1|customer-settings-v2)
                workflow=customersettingsv1
                [[ "$scenario" != customer-settings-v2 ]] || workflow=customersettingsv2
                grep -q "testkube run testworkflow $workflow" "$MOCK_DIR/commands"
                grep -q 'revision: main' "$MOCK_DIR/shared-template.yaml"
                ;;
            all-targets-enabled)
                grep -q 'testkube run testworkflow configprocessingcommon' "$MOCK_DIR/commands"
                grep -q 'testkube run testworkflow alltargetsenabled' "$MOCK_DIR/commands"
                grep -q 'AMW_QUERY_ENDPOINT' "$MOCK_DIR/testkube/generated.yaml"
                grep -q 'AZURE_CLIENT_ID' "$MOCK_DIR/testkube/generated.yaml"
                grep -q 'value: "https://example.invalid"' "$MOCK_DIR/testkube/generated.yaml"
                grep -q 'value: "test-client"' "$MOCK_DIR/testkube/generated.yaml"
                ;;
            default-targets-on)
                grep -q 'testkube run testworkflow configprocessingcommon' "$MOCK_DIR/commands"
                grep -q 'testkube run testworkflow defaultsettingsenabled' "$MOCK_DIR/commands"
                grep -q 'AMW_QUERY_ENDPOINT' "$MOCK_DIR/testkube/generated.yaml"
                grep -q 'AZURE_CLIENT_ID' "$MOCK_DIR/testkube/generated.yaml"
                grep -q 'value: "https://example.invalid"' "$MOCK_DIR/testkube/generated.yaml"
                grep -q 'value: "test-client"' "$MOCK_DIR/testkube/generated.yaml"
                ;;
            *)
                grep -q 'testkube run testworkflow configprocessingcommon' "$MOCK_DIR/commands"
                grep -q 'testkube run testworkflow customconfigmaperror' "$MOCK_DIR/commands"
                if [[ "$scenario" == existing-workflows && (! -f "$MOCK_DIR/configprocessingcommon-reset" || ! -f "$MOCK_DIR/customconfigmaperror-reset") ]]; then
                    echo "FAIL: polluted last-applied annotations were not reset" >&2
                    exit 1
                fi
                ;;
        esac
        if [[ "$scenario" == customer-settings-v1 || "$scenario" == customer-settings-v2 || "$scenario" == all-targets-enabled || "$scenario" == default-targets-on ]]; then
            template_apply=$(grep -n '^apply -f -$' "$MOCK_DIR/commands" | cut -d: -f1)
            workflow_apply=$(grep -n '^apply -f ./testkube/generated.yaml$' "$MOCK_DIR/commands" | cut -d: -f1)
            [[ -n "$template_apply" && -n "$workflow_apply" && "$template_apply" -lt "$workflow_apply" ]] || {
                echo "FAIL: shared template was not applied before its workflows" >&2
                exit 1
            }
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
