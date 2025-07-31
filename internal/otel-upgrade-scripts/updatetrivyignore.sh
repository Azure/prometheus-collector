#!/bin/bash
# This script updates the .trivyignore file by:
# 1. Scanning specified binaries with trivy to find current vulnerabilities
# 2. Preserving CVEs for components not in the binaries array (e.g., kube-state-metrics)
# 3. Updating only the CVEs for the scanned binaries
# 4. Generating a report of changes made

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
if [[ ! -f .trivyignore ]]; then
    echo "Error: .trivyignore file not found"
    exit 1
fi

cp .trivyignore .trivyignore.bak
declare -A preserved_cves  # CVEs for components not in binaries array
while IFS= read -r line; do
    # Check if line is a severity header and set current severity
    if [[ $line =~ ^#\ (CRITICAL|HIGH|MEDIUM|LOW)$ ]]; then
        current_severity=$(echo "$line" | sed 's/^# //')
        continue
    fi

    # Check if line is a component header (one of our binaries)
    is_scanned_binary=false
    for binary in "${binaries[@]}"; do
        name=$(basename "$binary")
        if [[ "$line" == "# $name" ]]; then
            current_component="$name"
            is_scanned_binary=true
            continue 2
        fi
    done
    
    # Check if line is a component header for non-scanned components
    if [[ $line =~ ^#\ [a-zA-Z0-9_-]+$ ]] && [[ "$is_scanned_binary" == false ]]; then
        current_component=$(echo "$line" | sed 's/^# //')
        continue
    fi

    # If line is a CVE, store it in the appropriate array
    if [[ $line =~ ^CVE-[0-9]+-[0-9]+ ]] || [[ $line =~ ^GHSA- ]]; then
        cve=$(echo "$line" | awk '{print $1}')
        component=$current_component
        severity=$current_severity
        # Extract the package name from the comment if present
        pkg=""
        if [[ $line =~ \#[[:space:]]+(.*) ]]; then
            pkg="${BASH_REMATCH[1]}"
        fi
        
        # Check if this component is in our binaries array
        is_component_scanned=false
        for binary in "${binaries[@]}"; do
            name=$(basename "$binary")
            if [[ "$component" == "$name" ]]; then
                is_component_scanned=true
                break
            fi
        done
        
        if [[ "$is_component_scanned" == true ]]; then
            current_cves["$cve,$component,$severity,$pkg"]=1
        else
            preserved_cves["$cve,$component,$severity,$pkg"]=1
        fi
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
    
    # First collect all CVEs by component for this severity (both scanned and preserved)
    declare -A component_has_cves
    
    # Check scanned components
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
    
    # Check preserved components
    for cve_comp_sev_pkg in "${!preserved_cves[@]}"; do
        IFS=',' read -r cve comp sev pkg <<< "$cve_comp_sev_pkg"
        if [[ "$sev" == "$severity" ]]; then
            component_has_cves["$comp"]=1
        fi
    done
    
    # Collect all components that have CVEs for this severity and sort them
    components_with_cves=()
    for component in "${!component_has_cves[@]}"; do
        if [[ ${component_has_cves["$component"]} -eq 1 ]]; then
            components_with_cves+=("$component")
        fi
    done
    
    # Sort components alphabetically
    IFS=$'\n' sorted_components=($(sort <<<"${components_with_cves[*]}"))
    unset IFS
    
    # Now print CVEs for each component
    for component in "${sorted_components[@]}"; do
        echo "# $component" >> "$temp_file"
        
        # Collect all CVEs for this component and severity
        component_cves=()
        
        # Add preserved CVEs for this component and severity
        for cve_comp_sev_pkg in "${!preserved_cves[@]}"; do
            IFS=',' read -r cve comp sev pkg <<< "$cve_comp_sev_pkg"
            if [[ "$comp" == "$component" && "$sev" == "$severity" ]]; then
                component_cves+=("$cve # $pkg")
            fi
        done
        
        # Add new CVEs for this component and severity
        for cve_comp_sev_pkg in "${!new_cves[@]}"; do
            IFS=',' read -r cve comp sev pkg <<< "$cve_comp_sev_pkg"
            if [[ "$comp" == "$component" && "$sev" == "$severity" ]]; then
                component_cves+=("$cve # $pkg")
            fi
        done
        
        # Sort and output CVEs
        IFS=$'\n' sorted_cves=($(sort <<<"${component_cves[*]}"))
        unset IFS
        for cve_line in "${sorted_cves[@]}"; do
            echo "$cve_line" >> "$temp_file"
        done
    done
    echo "" >> "$temp_file"
done
mv "$temp_file" .trivyignore

# Validate the new file has content
if [[ ! -s .trivyignore ]]; then
    echo "Error: Generated .trivyignore file is empty, restoring backup"
    cp .trivyignore.bak .trivyignore
    exit 1
fi

# Report changes
report_file="cve_changes_report.txt"
echo "=== CVE Changes Report ===" > "$report_file"
echo "Removed CVEs:" >> "$report_file"

# Check for removed CVEs from scanned components
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

# Check for added CVEs from scanned components
for cve_comp_sev_pkg in "${!new_cves[@]}"; do
    if [[ -z "${current_cves[$cve_comp_sev_pkg]}" ]]; then
        cve=$(echo "$cve_comp_sev_pkg" | cut -d',' -f1)
        component=$(echo "$cve_comp_sev_pkg" | cut -d',' -f2)
        severity=$(echo "$cve_comp_sev_pkg" | cut -d',' -f3)
        pkg=$(echo "$cve_comp_sev_pkg" | cut -d',' -f4)
        echo "  + $cve from $component with severity $severity and package $pkg" >> "$report_file"
    fi
done

echo "" >> "$report_file"
echo "Preserved CVEs (not scanned):" >> "$report_file"
for cve_comp_sev_pkg in "${!preserved_cves[@]}"; do
    cve=$(echo "$cve_comp_sev_pkg" | cut -d',' -f1)
    component=$(echo "$cve_comp_sev_pkg" | cut -d',' -f2)
    severity=$(echo "$cve_comp_sev_pkg" | cut -d',' -f3)
    pkg=$(echo "$cve_comp_sev_pkg" | cut -d',' -f4)
    echo "  = $cve from $component with severity $severity and package $pkg" >> "$report_file"
done

echo "CVE changes have been written to $report_file"
echo "Updated .trivyignore file."
echo "Backup saved as .trivyignore.bak"