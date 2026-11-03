#!/bin/bash
set -euo pipefail

script_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
template=${1:-"$script_dir/../azure-template-install-trivy.yml"}
test_dir=$(mktemp -d)
trap 'rm -rf "$test_dir"' EXIT
script=$(awk '
    { sub(/\r$/, "") }
    /^- bash: \|/ { capturing = 1; next }
    capturing && /^  displayName:/ { found = 1; exit }
    capturing { sub(/^    /, ""); print }
    END { if (!found) exit 1 }
' "$template")
export TRIVY_VERSION
TRIVY_VERSION=$(awk '/^  default:/ { gsub(/\047|\r/, "", $2); print $2 }' "$template")
export ARCHIVE="trivy_${TRIVY_VERSION}_Linux-64bit.tar.gz"

curl() {
    local output="" url="${!#}"
    while [[ $# -gt 0 ]]; do
        if [[ "$1" == --output ]]; then output="$2"; shift; fi
        shift
    done
    echo "$url" >> "$CASE_DIR/downloads"
    case "$url" in
        "https://github.com/aquasecurity/trivy/releases/download/v${TRIVY_VERSION}/$ARCHIVE")
            [[ "$SCENARIO" != archive-download-failure ]] || return 6
            cp "$CASE_DIR/$ARCHIVE" "$output"
            if [[ "$SCENARIO" == checksum-mismatch ]]; then echo tampered >> "$output"; fi
            ;;
        "https://github.com/aquasecurity/trivy/releases/download/v${TRIVY_VERSION}/trivy_${TRIVY_VERSION}_checksums.txt")
            [[ "$SCENARIO" != checksum-download-failure ]] || return 22
            cp "$CASE_DIR/checksums.txt" "$output"
            ;;
        *) echo "Unexpected download endpoint: $url" >&2; return 1 ;;
    esac
}

install() {
    [[ "$SCENARIO" != install-failure ]] || return 1
    command install "$@"
}
export -f curl install

for scenario in success archive-download-failure checksum-download-failure checksum-mismatch missing-checksum extraction-failure install-failure version-failure; do
    export SCENARIO="$scenario"
    export CASE_DIR="$test_dir/$scenario"
    mkdir -p "$CASE_DIR"
    printf '#!/bin/bash\necho "Version: %s"\nexit %s\n' "$TRIVY_VERSION" \
        "$([[ "$scenario" == version-failure ]] && echo 1 || echo 0)" > "$CASE_DIR/trivy"
    chmod +x "$CASE_DIR/trivy"
    tar -czf "$CASE_DIR/$ARCHIVE" -C "$CASE_DIR" trivy
    if [[ "$scenario" == extraction-failure ]]; then echo invalid-archive > "$CASE_DIR/$ARCHIVE"; fi
    (cd "$CASE_DIR" && sha256sum "$ARCHIVE") > "$CASE_DIR/checksums.txt"
    if [[ "$scenario" == missing-checksum ]]; then echo unrelated-file > "$CASE_DIR/checksums.txt"; fi
    status=0
    bash -c "${script//'$(Agent.TempDirectory)'/$CASE_DIR}" > "$CASE_DIR/output" 2>&1 || status=$?
    if [[ "$scenario" == success ]]; then
        if [[ "$status" -ne 0 ]]; then cat "$CASE_DIR/output"; exit 1; fi
        [[ $("$CASE_DIR/trivy-bin/trivy" --version) == "Version: $TRIVY_VERSION" ]]
        grep -Fxq "##vso[task.prependpath]$CASE_DIR/trivy-bin" "$CASE_DIR/output"
        [[ $(wc -l < "$CASE_DIR/downloads") -eq 2 ]]
    else
        if [[ "$status" -eq 0 ]] || grep -Fq '##vso[task.prependpath]' "$CASE_DIR/output"; then
            cat "$CASE_DIR/output"
            echo "FAIL: $scenario reported a ready installation" >&2
            exit 1
        fi
        if [[ "$scenario" != version-failure && -e "$CASE_DIR/trivy-bin/trivy" ]]; then
            echo "FAIL: $scenario installed an unverified binary" >&2
            exit 1
        fi
        if [[ "$scenario" == archive-download-failure ]]; then
            [[ $(wc -l < "$CASE_DIR/downloads") -eq 1 ]]
        fi
    fi
    echo "PASS: $scenario"
done
