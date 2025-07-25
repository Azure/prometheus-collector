#!/bin/bash

# Script to send TestKube summary notification
# Parameters: 
# $1 - TEAMS_WEBHOOK_URL
# $2 - RESULTS_DIRECTORY (optional, defaults to ${BUILD_ARTIFACTSTAGINGDIRECTORY} or /tmp)

TEAMS_WEBHOOK_URL="$1"
RESULTS_DIRECTORY="${2:-${BUILD_ARTIFACTSTAGINGDIRECTORY:-/tmp}}"

if [ -z "$TEAMS_WEBHOOK_URL" ]; then
    echo "Teams webhook URL not provided, skipping notification"
    exit 0
fi

echo "Looking for result files in: $RESULTS_DIRECTORY"

# Find all testkube result files
result_files=$(find "$RESULTS_DIRECTORY" -name "testkube-results-*.json" 2>/dev/null || true)

if [ -z "$result_files" ]; then
    echo "No testkube result files found, skipping notification"
    exit 0
fi

echo "Found result files:"
echo "$result_files"

# Collect results from all environment files
total_count=0
passed_count=0
failed_count=0
earliest_start=""
latest_end=""
all_results=""

for file in $result_files; do
    echo "Processing: $file"
    if [ -f "$file" ]; then
        env=$(jq -r '.environment // "Unknown"' "$file" 2>/dev/null)
        status=$(jq -r '.status // "unknown"' "$file" 2>/dev/null)
        message=$(jq -r '.message // "No message"' "$file" 2>/dev/null)
        start_time=$(jq -r '.start_time // ""' "$file" 2>/dev/null)
        end_time=$(jq -r '.end_time // ""' "$file" 2>/dev/null)
        
        if [ "$status" != "unknown" ] && [ "$status" != "running" ]; then
            total_count=$((total_count + 1))
            
            if [ "$status" = "passed" ]; then
                passed_count=$((passed_count + 1))
            elif [ "$status" = "failed" ]; then
                failed_count=$((failed_count + 1))
            fi
            
            # Track earliest start time
            if [ -n "$start_time" ] && [ "$start_time" != "null" ]; then
                if [ -z "$earliest_start" ] || [ "$start_time" \< "$earliest_start" ]; then
                    earliest_start="$start_time"
                fi
            fi
            
            # Track latest end time
            if [ -n "$end_time" ] && [ "$end_time" != "null" ]; then
                if [ -z "$latest_end" ] || [ "$end_time" \> "$latest_end" ]; then
                    latest_end="$end_time"
                fi
            fi
            
            # Build results string
            if [ -n "$all_results" ]; then
                all_results="$all_results,"
            fi
            all_results="$all_results{\"environment\":\"$env\",\"status\":\"$status\",\"message\":\"$message\"}"
        fi
    fi
done

start_time="${earliest_start:-$(date -u '+%Y-%m-%d %H:%M:%S UTC')}"
end_time="${latest_end:-$(date -u '+%Y-%m-%d %H:%M:%S UTC')}"

echo "Summary stats: Total=$total_count, Passed=$passed_count, Failed=$failed_count"

if [ "$total_count" -eq 0 ]; then
    echo "No test results found, skipping notification"
    exit 0
elif [ "$failed_count" -eq 0 ]; then
    title="✅ All TestKube Tests Passed"
    color="00b294"
    summary_message="All $total_count test environments completed successfully!"
else
    title="❌ Some TestKube Tests Failed"
    color="d63333" 
    summary_message="$failed_count out of $total_count test environments failed."
fi

# Build detailed results showing each environment and failed tests
details=""
if [ "$total_count" -gt 0 ]; then
    # Create a temporary JSON structure to parse
    temp_json="{\"results\":[$all_results]}"
    
    while IFS= read -r result; do
        env=$(echo "$result" | jq -r '.environment')
        status=$(echo "$result" | jq -r '.status')
        message=$(echo "$result" | jq -r '.message')
        
        if [ "$status" = "passed" ]; then
            icon="✅"
            details="$details$icon **$env**: All tests passed\\n\\n"
        elif [ "$status" = "failed" ]; then
            icon="❌"
            # Extract failed test details from message if available
            if echo "$message" | grep -q "Tests failed:"; then
                failed_tests=$(echo "$message" | sed 's/.*Tests failed: \([^.]*\).*/\1/')
                details="$details$icon **$env**: Failed tests: $failed_tests\\n\\n"
            else
                details="$details$icon **$env**: $message\\n\\n"
            fi
        else
            icon="⚠️"
            details="$details$icon **$env**: $message\\n\\n"
        fi
    done <<< "$(echo "$temp_json" | jq -c '.results[]' 2>/dev/null)"
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
