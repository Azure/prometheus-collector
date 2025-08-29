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
all_results=""

# Get PR information from environment variables or git commit
pr_title="${SYSTEM_PULLREQUEST_PULLREQUESTTITLE:-}"
pr_number="${SYSTEM_PULLREQUEST_PULLREQUESTNUMBER:-}"
build_reason="${BUILD_REASON:-}"
source_version_message="${BUILD_SOURCEVERSIONMESSAGE:-}"
source_branch_name="${BUILD_SOURCEBRANCHNAME:-}"
build_number="${BUILD_BUILDNUMBER:-}"

pr_info=""
repo_url="https://github.com/Azure/prometheus-collector"

if [ -n "$pr_title" ] && [ "$pr_title" != "null" ] && [ -n "$pr_number" ] && [ "$pr_number" != "null" ]; then
    # Running in PR context
    pr_info="[PR #$pr_number]($repo_url/pull/$pr_number): $pr_title"
elif [ "$build_reason" = "IndividualCI" ] || [ "$build_reason" = "BatchedCI" ]; then
    # Running on main branch after PR merge - try multiple approaches
    
    # First try Build.SourceVersionMessage (commit message)
    if [ -n "$source_version_message" ] && [ "$source_version_message" != "null" ]; then
        # Try to extract PR number from commit message
        if [[ "$source_version_message" =~ Merged\ PR\ ([0-9]+):\ (.+) ]]; then
            pr_number="${BASH_REMATCH[1]}"
            pr_title="${BASH_REMATCH[2]}"
            pr_info="Merged [PR #$pr_number]($repo_url/pull/$pr_number): $pr_title"
        elif [[ "$source_version_message" =~ \(#([0-9]+)\) ]]; then
            # Alternative format: "Title (#1234)"
            pr_number="${BASH_REMATCH[1]}"
            pr_title=$(echo "$source_version_message" | sed 's/ (#[0-9]\+)$//')
            pr_info="Merged [PR #$pr_number]($repo_url/pull/$pr_number): $pr_title"
        else
            pr_info="Latest commit: $source_version_message"
        fi
    else
        # Fallback to git command
        latest_commit_msg=$(git log -1 --pretty=format:"%s" 2>/dev/null || echo "")
        
        if [[ "$latest_commit_msg" =~ Merged\ PR\ ([0-9]+):\ (.+) ]]; then
            pr_number="${BASH_REMATCH[1]}"
            pr_title="${BASH_REMATCH[2]}"
            pr_info="Merged [PR #$pr_number]($repo_url/pull/$pr_number): $pr_title"
        elif [[ "$latest_commit_msg" =~ \(#([0-9]+)\) ]]; then
            pr_number="${BASH_REMATCH[1]}"
            pr_title=$(echo "$latest_commit_msg" | sed 's/ (#[0-9]\+)$//')
            pr_info="Merged [PR #$pr_number]($repo_url/pull/$pr_number): $pr_title"
        elif [ -n "$latest_commit_msg" ]; then
            pr_info="Latest commit: $latest_commit_msg"
        else
            pr_info="Main branch build #$build_number"
        fi
    fi
elif [ "$build_reason" = "Manual" ]; then
    pr_info="Manual build"
else
    pr_info="Build reason: $build_reason"
fi

for file in $result_files; do
    echo "Processing: $file"
    if [ -f "$file" ]; then
        env=$(jq -r '.environment // "Unknown"' "$file" 2>/dev/null)
        status=$(jq -r '.status // "unknown"' "$file" 2>/dev/null)
        message=$(jq -r '.message // "No message"' "$file" 2>/dev/null)
        link=$(jq -r '.link // ""' "$file" 2>/dev/null)

        if [ "$status" != "unknown" ] && [ "$status" != "running" ]; then
            total_count=$((total_count + 1))
            
            if [ "$status" = "passed" ]; then
                passed_count=$((passed_count + 1))
            elif [ "$status" = "failed" ]; then
                failed_count=$((failed_count + 1))
            fi
            
            # Build results string
            if [ -n "$all_results" ]; then
                all_results="$all_results,"
            fi
            all_results="$all_results{\"environment\":\"$env\",\"status\":\"$status\",\"message\":\"$message\",\"link\":\"$link\"}"
        fi
    fi
done

echo "Summary stats: Total=$total_count, Passed=$passed_count, Failed=$failed_count"

if [ "$total_count" -eq 0 ]; then
    echo "No test results found, skipping notification"
    exit 0
elif [ "$failed_count" -eq 0 ]; then
    title="✅ All TestKube Tests Passed"
    color="00b294"
    if [ -n "$pr_info" ]; then
        summary_message="All $total_count test environments completed successfully for: $pr_info"
    else
        summary_message="All $total_count test environments completed successfully!"
    fi
else
    title="❌ Some TestKube Tests Failed"
    color="d63333" 
    if [ -n "$pr_info" ]; then
        summary_message="$failed_count out of $total_count test environments failed for: $pr_info"
    else
        summary_message="$failed_count out of $total_count test environments failed."
    fi
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
        link=$(echo "$result" | jq -r '.link')
        
        if [ "$status" = "passed" ]; then
            icon="✅"
            details="$details$icon **$env**: All tests passed\\n\\n"
        elif [ "$status" = "failed" ]; then
            icon="❌"
            # Extract failed test details from message if available
            if echo "$message" | grep -q "Tests failed:"; then
                failed_tests=$(echo "$message" | sed 's/.*Tests failed: \([^.]*\).*/\1/')
                details="$details$icon **$env**: Failed tests: $failed_tests. $link\\n\\n"
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
        "facts": []
    }]
}
EOF
)

echo "Sending Teams notification..."
curl -H "Content-Type: application/json" -d "$payload" "$TEAMS_WEBHOOK_URL" || echo "Failed to send Teams notification"
echo "Summary notification sent."
