#!/bin/bash

# Script to send TestKube summary notification
# Parameters: 
# $1 - TEAMS_WEBHOOK_URL
# $2 - RESULTS_FILE (optional, defaults to /tmp/testkube-results-summary.json)

TEAMS_WEBHOOK_URL="$1"
RESULTS_FILE="${2:-/tmp/testkube-results-summary.json}"

if [ -z "$TEAMS_WEBHOOK_URL" ]; then
    echo "Teams webhook URL not provided, skipping notification"
    exit 0
fi

if [ ! -f "$RESULTS_FILE" ]; then
    echo "No results file found at: $RESULTS_FILE"
    echo "Creating default summary"
    # If no results file exists, check pipeline status from environment
    echo '{"results": [], "start_time": "'$(date -u '+%Y-%m-%d %H:%M:%S UTC')'"}' > "$RESULTS_FILE"
fi

echo "Using results file: $RESULTS_FILE"
echo "Results file contents:"
cat "$RESULTS_FILE"

total_count=$(jq '.results | length' "$RESULTS_FILE" 2>/dev/null || echo "0")
passed_count=$(jq '.results | map(select(.status == "passed")) | length' "$RESULTS_FILE" 2>/dev/null || echo "0")
failed_count=$(jq '.results | map(select(.status == "failed")) | length' "$RESULTS_FILE" 2>/dev/null || echo "0")
start_time=$(jq -r '.start_time' "$RESULTS_FILE" 2>/dev/null || date -u '+%Y-%m-%d %H:%M:%S UTC')
end_time="$(date -u '+%Y-%m-%d %H:%M:%S UTC')"

echo "Summary stats: Total=$total_count, Passed=$passed_count, Failed=$failed_count"

if [ "$total_count" -eq 0 ]; then
    title="ℹ️ TestKube Summary"
    color="0078d4"
    summary_message="TestKube workflow completed. No individual test results were collected."
    details=""
elif [ "$failed_count" -eq 0 ]; then
    title="✅ All TestKube Tests Passed"
    color="00b294"
    summary_message="All $total_count test environments completed successfully!"
else
    title="❌ Some TestKube Tests Failed"
    color="d63333" 
    summary_message="$failed_count out of $total_count test environments failed."
fi

# Build detailed results if we have any
details=""
if [ "$total_count" -gt 0 ]; then
    while IFS= read -r result; do
        env=$(echo "$result" | jq -r '.environment')
        status=$(echo "$result" | jq -r '.status')
        message=$(echo "$result" | jq -r '.message')
        icon="✅"
        if [ "$status" = "failed" ]; then
            icon="❌"
        fi
        details="$details$icon **$env**: $message\\n\\n"
    done <<< "$(jq -c '.results[]' "$RESULTS_FILE" 2>/dev/null)"
fi

payload=$(cat <<EOF
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

echo "Sending Teams notification..."
curl -H "Content-Type: application/json" -d "$payload" "$TEAMS_WEBHOOK_URL" || echo "Failed to send Teams notification"
echo "Summary notification sent."
