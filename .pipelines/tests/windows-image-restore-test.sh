#!/bin/bash
set -euo pipefail

script_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
pipeline=${1:-"$script_dir/../azure-pipeline-build.yml"}
test_dir=$(mktemp -d)
trap 'rm -rf "$test_dir"' EXIT

script=$(awk '
    { sub(/\r$/, "") }
    /^        - bash: \|/ { script = ""; capturing = 1; next }
    capturing && /^          displayName: "Build: ensure images are present in ACR"/ {
        printf "%s", script
        found = 1
        exit
    }
    capturing { sub(/^            /, ""); script = script $0 "\n" }
    END { if (!found) exit 1 }
' "$pipeline")
export ACR_USERNAME=test-user
export ACR_PASSWORD='test-password'
script=${script//'$(WINDOWS_FULL_IMAGE_NAME)'/containerinsightsprod.azurecr.io/test/images:test-win}
script=${script//'$(WINDOWS_IMAGE_TAG)'/test-win}
script=${script//'$(WINDOWS_2019_BASE_IMAGE_VERSION)'/ltsc2019}
script=${script//'$(WINDOWS_2022_BASE_IMAGE_VERSION)'/ltsc2022}
script=${script//'$(MCR_REGISTRY)'/mcr.microsoft.com}
script=${script//'$(MCR_REPOSITORY)'//test/images}
if [[ "$script" == *'$('* ]]; then
    echo "FAIL: recovery script contains unresolved pipeline variables or command substitutions" >&2
    exit 1
fi

docker() {
    echo "$*" >> "$CASE_DIR/commands"
    case "$1" in
        login)
            cat > /dev/null
            [[ "$SCENARIO" != login-failure ]]
            ;;
        pull)
            case "$2" in
                containerinsightsprod.azurecr.io/test/images:test-win-ltsc*)
                    case "$SCENARIO" in
                        existing|login-failure) return 0 ;;
                        restore-2019|tag-failure|push-failure) [[ "$2" != *-ltsc2019 ]] ;;
                        restore-2022|delayed|unavailable) [[ "$2" != *-ltsc2022 ]] ;;
                        restore-both) return 1 ;;
                        *) return 1 ;;
                    esac
                    ;;
                mcr.microsoft.com/test/images:test-win-ltsc*)
                    local count=0
                    local count_file="$CASE_DIR/${2##*-}-pulls"
                    if [[ -f "$count_file" ]]; then read -r count < "$count_file"; fi
                    count=$((count + 1))
                    echo "$count" > "$count_file"
                    [[ "$SCENARIO" != unavailable ]] || return 1
                    [[ "$SCENARIO" != delayed || "$count" -ge 3 ]]
                    ;;
                *) return 1 ;;
            esac
            ;;
        tag) [[ "$SCENARIO" != tag-failure ]] ;;
        push) [[ "$SCENARIO" != push-failure ]] ;;
        *) echo "Unexpected docker command: $*" >&2; return 1 ;;
    esac
}

sleep() { echo "sleep $*" >> "$CASE_DIR/commands"; }
export -f docker sleep

for scenario in existing restore-2019 restore-2022 restore-both delayed unavailable login-failure tag-failure push-failure; do
    export SCENARIO="$scenario"
    export CASE_DIR="$test_dir/$scenario"
    mkdir -p "$CASE_DIR"
    status=0
    bash -c "$script" > "$CASE_DIR/output" 2>&1 || status=$?
    case "$scenario" in
        unavailable|login-failure|tag-failure|push-failure)
            if [[ "$status" -eq 0 ]]; then
                echo "FAIL: $scenario reported success" >&2
                exit 1
            fi
            ;;
        *)
            if [[ "$status" -ne 0 ]]; then
                cat "$CASE_DIR/output"
                echo "FAIL: $scenario failed" >&2
                exit 1
            fi
            for version in ltsc2019 ltsc2022; do
                if [[ "$scenario" == restore-both ||
                      ("$scenario" == restore-2019 && "$version" == ltsc2019) ||
                      ("$scenario" == restore-2022 && "$version" == ltsc2022) ||
                      ("$scenario" == delayed && "$version" == ltsc2022) ]]; then
                    grep -Fxq "tag mcr.microsoft.com/test/images:test-win-$version containerinsightsprod.azurecr.io/test/images:test-win-$version" "$CASE_DIR/commands"
                    grep -Fxq "push containerinsightsprod.azurecr.io/test/images:test-win-$version" "$CASE_DIR/commands"
                else
                    if grep -Fq "mcr.microsoft.com/test/images:test-win-$version" "$CASE_DIR/commands"; then
                        echo "FAIL: $scenario unnecessarily restored $version" >&2
                        exit 1
                    fi
                fi
            done
            ;;
    esac
    if [[ "$scenario" == unavailable ]]; then
        [[ $(cat "$CASE_DIR/ltsc2022-pulls") -eq 20 ]]
        [[ $(grep -c '^sleep ' "$CASE_DIR/commands") -eq 19 ]]
        if grep -Eq '^(tag|push) ' "$CASE_DIR/commands"; then
            echo "FAIL: unpublished image was tagged or pushed" >&2
            exit 1
        fi
    fi
    if [[ "$scenario" == delayed ]]; then
        [[ $(cat "$CASE_DIR/ltsc2022-pulls") -eq 3 ]]
    fi
    if [[ "$scenario" == tag-failure ]] && grep -q '^push ' "$CASE_DIR/commands"; then
        echo "FAIL: image was pushed after tagging failed" >&2
        exit 1
    fi
    echo "PASS: $scenario"
done
