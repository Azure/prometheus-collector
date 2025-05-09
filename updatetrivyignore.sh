#!/bin/bash
binaries=(
"otelcollector/opentelemetry-collector-builder/otelcollector"
"otelcollector/prom-config-validator-builder/promconfigvalidator"
"otelcollector/otel-allocator/targetallocator"
"otelcollector/configuration-reader-builder/configurationreader"
)

# Initialize arrays to track CVEs
declare -A current_cves
declare -A new_cves
declare -A components

# Test if trivy is available
if ! command -v trivy &> /dev/null; then
    echo "Error: trivy is not installed or not in PATH"
    exit 1
fi

# Check if Docker is available
if ! command -v docker &> /dev/null; then
    echo "Error: Docker is not installed or not in PATH"
    exit 1
fi

# Check if Docker daemon is running
if ! docker info &> /dev/null; then
    echo "Error: Docker daemon is not running"
    exit 1
fi

# Read current .trivyignore file if it exists
if [ -f .trivyignore.bak ]; then
    cp .trivyignore.bak .trivyignore
    while IFS= read -r line; do
        if [[ $line =~ ^CVE-[0-9]+-[0-9]+ ]]; then
            cve=$(echo "$line" | awk '{print $1}')
            component=$(grep -B 1 "$cve" .trivyignore | grep -v "^CVE" | grep -v "^--" | tail -n 1 | sed 's/^# //')
            current_cves["$cve,$component"]=1
        fi
    done < .trivyignore
else
    cp .trivyignore .trivyignore.bak
fi

# Scan each binary with trivy
for binary in "${binaries[@]}"; do
    name=$(basename "$binary")
    echo "Scanning $name..."
    trivy fs --security-checks vuln --severity CRITICAL,HIGH,MEDIUM,LOW --ignore-unfixed "$binary" -f json > "${name}_vulnerabilities.json"
    
    # Extract and categorize vulnerabilities
    jq -r '.Results[] | select(.Vulnerabilities != null) | .Vulnerabilities[] | "\(.VulnerabilityID) \(.Severity) \(.PkgName)"' "${name}_vulnerabilities.json" | sort | uniq | while read -r vuln; do
        cve=$(echo "$vuln" | awk '{print $1}')
        severity=$(echo "$vuln" | awk '{print $2}')
        pkg=$(echo "$vuln" | awk '{print $3}')
        new_cves["$cve,$name"]=1
        components["$name"]=1
        
        # Add to .trivyignore file with package information
        if ! grep -q "$cve.*$pkg" .trivyignore; then
            # Find the appropriate section or create it
            if ! grep -q "^# $name$" .trivyignore; then
                echo "# $name" >> .trivyignore
            fi
            echo "$cve # $pkg" >> .trivyignore
        fi
    done
    
    rm "${name}_vulnerabilities.json"
done

# Sort .trivyignore file by severity and components
temp_file=$(mktemp)
{
    echo "# CRITICAL"
    for component in "${!components[@]}"; do
        echo "# $component"
        grep -A 100 "^# $component$" .trivyignore | grep "^CVE" | grep -v -f /dev/null | sort
    done
    
    echo "# HIGH"
    for component in "${!components[@]}"; do
        echo "# $component"
        grep -A 100 "^# $component$" .trivyignore | grep "^CVE" | grep -v -f /dev/null | sort
    done
    
    echo "# MEDIUM"
    for component in "${!components[@]}"; do
        echo "# $component"
        grep -A 100 "^# $component$" .trivyignore | grep "^CVE" | grep -v -f /dev/null | sort
    done
    
    echo "# LOW"
    for component in "${!components[@]}"; do
        echo "# $component"
        grep -A 100 "^# $component$" .trivyignore | grep "^CVE" | grep -v -f /dev/null | sort
    done
} > "$temp_file"

mv "$temp_file" .trivyignore

# Report changes
report_file="cve_changes_report.txt"
echo "=== CVE Changes Report ===" > "$report_file"
echo "Removed CVEs:" >> "$report_file"
for cve_comp in "${!current_cves[@]}"; do
    if [[ -z "${new_cves[$cve_comp]}" ]]; then
        cve=$(echo "$cve_comp" | cut -d',' -f1)
        component=$(echo "$cve_comp" | cut -d',' -f2)
        echo "  - $cve from $component" >> "$report_file"
    fi
done

echo "Added CVEs:" >> "$report_file"
for cve_comp in "${!new_cves[@]}"; do
    if [[ -z "${current_cves[$cve_comp]}" ]]; then
        cve=$(echo "$cve_comp" | cut -d',' -f1)
        component=$(echo "$cve_comp" | cut -d',' -f2)
        echo "  + $cve to $component" >> "$report_file"
    fi
done

echo "CVE changes have been written to $report_file"
echo "Updated .trivyignore file."