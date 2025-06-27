#!/bin/bash
binaries=(
"otelcollector/opentelemetry-collector-builder/otelcollector"
"otelcollector/prom-config-validator-builder/promconfigvalidator"
"otelcollector/otel-allocator/targetallocator"
"otelcollector/configuration-reader-builder/configurationreader"
"otelcollector/prometheus-ui/prometheusui"
)

# Initialize arrays to track CVEs
declare -A current_cves
declare -A new_cves
declare -A components
declare -A current_severity
declare -A current_component

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
cp .trivyignore .trivyignore.bak
while IFS= read -r line; do
    # Check if line is a severity header and set current severity
    if [[ $line =~ ^#\ (CRITICAL|HIGH|MEDIUM|LOW)$ ]]; then
        current_severity=$(echo "$line" | sed 's/^# //')
        continue
    fi

    # Check if line is a component header (one of our binaries)
    for binary in "${binaries[@]}"; do
        name=$(basename "$binary")
        if [[ "$line" == "# $name" ]]; then
            current_component="$name"
            continue 2
        fi
    done

    # If line is a CVE, store it in the current_cves array
    if [[ $line =~ ^CVE-[0-9]+-[0-9]+ ]]; then
        cve=$(echo "$line" | awk '{print $1}')
        component=$current_component
        severity=$current_severity
        # Extract the package name from the comment if present
        pkg=""
        if [[ $line =~ \#[[:space:]]+(.*) ]]; then
            pkg="${BASH_REMATCH[1]}"
        fi
        current_cves["$cve,$component,$severity,$pkg"]=1
    fi
done < .trivyignore

# Create the vulns directory if it doesn't exist
mkdir -p vulns

# Get and display current directory
current_dir=$(pwd)

# Scan each binary with trivy
for binary in "${binaries[@]}"; do
    name=$(basename "$binary")
    binary_dir=$(dirname "$binary")
    echo "Scanning $name... for the directory $binary_dir"
    cd "$binary_dir" || { echo "Error: Directory $binary_dir does not exist"; continue; }
    trivy fs . -f json --scanners vuln > "$current_dir/vulns/${name}_vulnerabilities.json"
    cd $current_dir || { echo "Error: Could not return to directory $current_dir"; exit 1; }

    # Check if the file contains any vulnerabilities before processing
    if jq -e '.Results[] | select(.Vulnerabilities != null) | .Vulnerabilities | length > 0' "vulns/${name}_vulnerabilities.json" > /dev/null; then
        echo "Processing vulnerabilities for $name..."
        while IFS= read -r vuln; do
            cve=$(echo "$vuln" | awk '{print $1}')
            severity=$(echo "$vuln" | awk '{print $2}')
            pkg=$(echo "$vuln" | awk '{print $3}')
            new_cves["$cve,$name,$severity,$pkg"]=1
        done < <(jq -r '.Results[] | select(.Vulnerabilities != null) | .Vulnerabilities[] | "\(.VulnerabilityID) \(.Severity) \(.PkgName)"' "vulns/${name}_vulnerabilities.json" | sort | uniq)
    fi
    
    #rm "vuln/${name}_vulnerabilities.json"
done


temp_file=$(mktemp)
# Create the file with a header
echo "# This file contains CVEs to be ignored by Trivy" > "$temp_file"

# Add a note about auto-generation
echo "# Auto-generated on $(date)" >> "$temp_file"
echo "" >> "$temp_file"

# Process all severity levels in a loop
for severity in "CRITICAL" "HIGH" "MEDIUM" "LOW"; do
    echo "# $severity" >> "$temp_file"
    
    # First collect all CVEs by component for this severity
    declare -A component_has_cves
    for binary in "${binaries[@]}"; do
        component=$(basename "$binary")
        component_has_cves["$component"]=0
        
        # Check if this component has any CVEs with current severity
        for cve_comp_sev_pkg in "${!new_cves[@]}"; do
            IFS=',' read -r cve comp sev pkg <<< "$cve_comp_sev_pkg"
            if [[ "$comp" == "$component" && "$sev" == "$severity" ]]; then
                component_has_cves["$component"]=1
                break
            fi
        done
    done
    
    # Now print CVEs, with component headers only for those that have CVEs
    for binary in "${binaries[@]}"; do
        component=$(basename "$binary")
        
        # Only process components that have CVEs for this severity
        if [[ ${component_has_cves["$component"]} -eq 1 ]]; then
            echo "# $component" >> "$temp_file"
            
            # Add all CVEs for this component and severity
            for cve_comp_sev_pkg in "${!new_cves[@]}"; do
                IFS=',' read -r cve comp sev pkg <<< "$cve_comp_sev_pkg"
                if [[ "$comp" == "$component" && "$sev" == "$severity" ]]; then
                    echo "$cve # $pkg" >> "$temp_file"
                fi
            done
        fi
    done
    echo "" >> "$temp_file"
done
mv "$temp_file" .trivyignore

# Report changes
report_file="cve_changes_report.txt"
echo "=== CVE Changes Report ===" > "$report_file"
echo "Removed CVEs:" >> "$report_file"
for cve_comp_sev_pkg in "${!current_cves[@]}"; do
    if [[ -z "${new_cves[$cve_comp_sev_pkg]}" ]]; then
        cve=$(echo "$cve_comp_sev_pkg" | cut -d',' -f1)
        component=$(echo "$cve_comp_sev_pkg" | cut -d',' -f2)
        severity=$(echo "$cve_comp_sev_pkg" | cut -d',' -f3)
        pkg=$(echo "$cve_comp_sev_pkg" | cut -d',' -f4)
        echo "  - $cve from $component with severity $severity and package $pkg" >> "$report_file"
    fi
done

echo "Added CVEs:" >> "$report_file"
for cve_comp in "${!new_cves[@]}"; do
    if [[ -z "${current_cves[$cve_comp]}" ]]; then
        cve=$(echo "$cve_comp" | cut -d',' -f1)
        component=$(echo "$cve_comp" | cut -d',' -f2)
        severity=$(echo "$cve_comp" | cut -d',' -f3)
        pkg=$(echo "$cve_comp" | cut -d',' -f4)
        echo "  + $cve from $component with severity $severity and package $pkg" >> "$report_file"
    fi
done

echo "CVE changes have been written to $report_file"
echo "Updated .trivyignore file."