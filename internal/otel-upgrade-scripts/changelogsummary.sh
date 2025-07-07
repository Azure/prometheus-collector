#!/bin/bash
# Extract and categorize changes from CHANGELOG.md for any component
# set -x # Enable debugging
# Features:
# - Extracts entries mentioning specific components between specified versions
# - Supports custom regex patterns for flexible filtering
# - Categorizes changes as Breaking, Feature, Bug Fix, or Other
# - Handles multi-line changelog entries that span across multiple lines
# - Provides a summary of changes by category
# - Can output results to a markdown file

# Usage information
function show_usage {
  echo "Usage: $0 [OPTIONS]"
  echo "Extract changes for a component from CHANGELOG.md"
  echo
  echo "OPTIONS:"
  echo "  -f, --from VERSION    Starting version (e.g., v0.120.0)"
  echo "  -t, --to VERSION      Ending version (e.g., v0.127.0)"
  echo "  -c, --changelog FILE  Path to CHANGELOG.md file (default: ./CHANGELOG.md)"
  echo "  -o, --output FILE     Output file to write results in markdown format (default: stdout)"
  echo "  -n, --name STRING     Component name to search for (default: prometheusreceiver)"
  echo "  -p, --pattern REGEX   Custom regex pattern to search for (overrides --name)"
  echo "  -h, --help            Show this help message"
  echo
  echo "Examples:"
  echo "  $0 --from v0.123.0 --to v0.127.0 --output prometheus_changes.md"
  echo "  $0 --from v0.120.0 --to v0.127.0 --name elasticsearchexporter"
  echo "  $0 --from v0.120.0 --to v0.127.0 --pattern \"kafka(exporter|receiver)\""
}

# Default values
CHANGELOG_FILE="./CHANGELOG.md"
FROM_VERSION=""
TO_VERSION=""
OUTPUT_FILE=""
COMPONENT_NAME="prometheusreceiver"
PATTERN=""

# Parse command line arguments
while [[ $# -gt 0 ]]; do
  case $1 in
    -f|--from)
      FROM_VERSION="$2"
      shift 2
      ;;
    -t|--to)
      TO_VERSION="$2"
      shift 2
      ;;
    -c|--changelog)
      CHANGELOG_FILE="$2"
      shift 2
      ;;
    -o|--output)
      OUTPUT_FILE="$2"
      shift 2
      ;;
    -n|--name)
      COMPONENT_NAME="$2"
      shift 2
      ;;
    -p|--pattern)
      PATTERN="$2"
      shift 2
      ;;
    -h|--help)
      show_usage
      exit 0
      ;;
    *)
      echo "Unknown option: $1"
      show_usage
      exit 1
      ;;
  esac
done

# Function to check if an entry matches a search pattern
function matches_search_pattern {
  local entry="$1"
  local pattern="$2"
  local component_name="$3"
  
  # Special handling for target-allocator
  if [[ "$component_name" == "target-allocator" ]]; then
    # Extended matching for various target-allocator formats
    if [[ "$entry" == *"target-allocator"* || 
          "$entry" == *"target allocator"* || 
          "$entry" == *"targetallocator"* || 
          "$entry" == *"\`target allocator\`"* || 
          "$entry" == *"\`target-allocator\`"* || 
          "$entry" == *"Target Allocator"* ]]; then
      echo "DEBUG: Found matching target-allocator entry: $entry" >&2
      return 0
    else
      return 1
    fi
  else
    # Regular pattern matching
    if [[ "$entry" =~ $pattern ]]; then
      return 0
    else
      return 1
    fi
  fi
}

# Check if required parameters are provided
if [[ -z "$FROM_VERSION" || -z "$TO_VERSION" ]]; then
  echo "Error: Both from and to versions are required"
  show_usage
  exit 1
fi

# Check if changelog file exists
if [[ ! -f "$CHANGELOG_FILE" ]]; then
  echo "Error: Changelog file '$CHANGELOG_FILE' not found"
  exit 1
fi

# Build the search pattern
if [[ -n "$PATTERN" ]]; then
  # Use the custom pattern provided by user
  SEARCH_PATTERN="$PATTERN"
else
  # Build pattern based on component name
  COMPONENT_NAME_LOWER=$(echo "$COMPONENT_NAME" | tr '[:upper:]' '[:lower:]')
  
  # Create pattern that matches various forms based on common component types
  if [[ "$COMPONENT_NAME_LOWER" == "target-allocator" ]]; then
    # Special case for target-allocator with extended patterns
    SEARCH_PATTERN="target-allocator|target allocator|targetallocator|\`target allocator\`|\`target-allocator\`|Target Allocator"
  elif [[ "$COMPONENT_NAME_LOWER" == "prometheusreceiver" ]]; then
    SEARCH_PATTERN="prometheusreceiver|receiver/prometheus|receiver/prometheusreceiver"
  else
    # Create pattern that matches various forms (componentname, component/name, component.name)
    SEARCH_PATTERN="$COMPONENT_NAME_LOWER|receiver/$COMPONENT_NAME_LOWER|receiver\.$COMPONENT_NAME_LOWER"
    
    # If the component name starts with a letter, also match the capitalized version
    if [[ "$COMPONENT_NAME_LOWER" =~ ^[a-z] ]]; then
      # Get first character and capitalize it
      FIRST_CHAR=$(echo "${COMPONENT_NAME_LOWER:0:1}" | tr '[:lower:]' '[:upper:]')
      # Rest of the component name
      REST_CHARS="${COMPONENT_NAME_LOWER:1}"
      # Add capitalized version to pattern
      SEARCH_PATTERN="$SEARCH_PATTERN|$FIRST_CHAR$REST_CHARS"
    fi
  fi
fi

# Function to convert version strings to comparable integers
function version_to_int {
  local version="${1#v}" # Remove 'v' prefix if present
  local -a parts
  IFS='.' read -ra parts <<< "$version"
  
  # Ensure we have 3 parts with proper padding
  local major="${parts[0]:-0}"
  local minor="${parts[1]:-0}"
  local patch="${parts[2]:-0}"
  
  # Calculate a single integer value (allowing for versions up to 999.999.999)
  echo "$((major * 1000000 + minor * 1000 + patch))"
}

# Function to handle output (to file or stdout)
function output_text {
  if [[ -n "$OUTPUT_FILE" ]]; then
    echo "$1" >> "$OUTPUT_FILE"
  else
    echo "$1"
  fi
}

# Initialize counters for summary
BREAKING_COUNT=0
FEATURE_COUNT=0
BUGFIX_COUNT=0
OTHER_COUNT=0

FROM_INT=$(version_to_int "$FROM_VERSION")
TO_INT=$(version_to_int "$TO_VERSION")

# Create or truncate the output file if one was specified
if [[ -n "$OUTPUT_FILE" ]]; then
  # Create directory for output file if it doesn't exist
  OUTPUT_DIR=$(dirname "$OUTPUT_FILE")
  if [[ ! -d "$OUTPUT_DIR" && "$OUTPUT_DIR" != "." ]]; then
    mkdir -p "$OUTPUT_DIR"
  fi
  
  # Create or truncate the output file
  : > "$OUTPUT_FILE"
  
  # Determine title based on whether a custom pattern or component name is used
  if [[ -n "$PATTERN" ]]; then
    TITLE="Changes Matching \"$PATTERN\""
  else
    # Format the component name with capitalization
    COMPONENT_DISPLAY=$(echo "$COMPONENT_NAME" | sed 's/\([a-z0-9]\)\([A-Z]\)/\1 \2/g' | sed 's/^\([a-z]\)/\u\1/g')
    TITLE="$COMPONENT_DISPLAY Changes"
  fi
  
  # Add a markdown title and timestamp
  output_text "# $TITLE"
  output_text "## ${FROM_VERSION} to ${TO_VERSION}"
  output_text ""
  output_text "Generated on: $(date '+%Y-%m-%d %H:%M:%S')"
  output_text ""
  output_text "---"
  output_text ""
else
  if [[ -n "$PATTERN" ]]; then
    output_text "Changes matching \"$PATTERN\" between $FROM_VERSION and $TO_VERSION:"
  else
    output_text "Changes to $COMPONENT_NAME between $FROM_VERSION and $TO_VERSION:"
  fi
  output_text "=================================================================="
  output_text ""
fi

IN_RANGE=0
VERSION_CHANGES_FOUND=0
TOTAL_CHANGES_FOUND=0
CURRENT_VERSION=""
IS_BREAKING_SECTION=0
IS_FEATURE_SECTION=0
IS_BUGFIX_SECTION=0

# Process the changelog content
CURRENT_ENTRY=""
IN_MULTI_LINE=0
ENTRY_CATEGORY=""

while IFS= read -r line; do
  # Check for version headers
  if [[ "$line" =~ ^##[[:space:]]+([vV]?[0-9]+\.[0-9]+\.[0-9]+) ]]; then
    # Process any pending multi-line entry before moving to a new version
    if [[ "$IN_MULTI_LINE" -eq 1 && -n "$CURRENT_ENTRY" && "$IN_RANGE" -eq 1 ]]; then
      # Only process if entry contains the search pattern
      if matches_search_pattern "$CURRENT_ENTRY" "$SEARCH_PATTERN" "$COMPONENT_NAME_LOWER"; then
        if [[ "$VERSION_CHANGES_FOUND" -eq 0 ]]; then
          output_text "### $CURRENT_VERSION"
          VERSION_CHANGES_FOUND=1
          TOTAL_CHANGES_FOUND=1
        fi
        
        # Output the entry with appropriate category
        output_text "- [$ENTRY_CATEGORY] ${CURRENT_ENTRY#- }"
        
        # Increment the appropriate counter
        case "$ENTRY_CATEGORY" in
          "BREAKING") ((BREAKING_COUNT++)) ;;
          "FEATURE") ((FEATURE_COUNT++)) ;;
          "BUG FIX") ((BUGFIX_COUNT++)) ;;
          "OTHER") ((OTHER_COUNT++)) ;;
        esac
      fi
      
      # Reset multi-line tracking
      CURRENT_ENTRY=""
      IN_MULTI_LINE=0
    fi
    
    VERSION="${BASH_REMATCH[1]}"
    VERSION_INT=$(version_to_int "$VERSION")
    
    # Reset changes found flag when moving to a new version
    VERSION_CHANGES_FOUND=0
    
    # Check if we're in the desired version range
    if (( VERSION_INT <= TO_INT && VERSION_INT > FROM_INT )); then
      IN_RANGE=1
      CURRENT_VERSION="$VERSION"
    else
      IN_RANGE=0
    fi
    continue
  fi
  
  # Skip if we're not in the desired version range
  if [[ "$IN_RANGE" -eq 0 ]]; then
    continue
  fi
  
  # Check if we've encountered a breaking changes section
  if [[ "$line" =~ "Breaking changes" || "$line" =~ "Breaking Changes" || "$line" =~ 🛑 || "$line" =~ ^###[[:space:]]+.*Breaking ]]; then
    IS_BREAKING_SECTION=1
    IS_FEATURE_SECTION=0
    IS_BUGFIX_SECTION=0
    continue
  fi
  
  # Check if we've encountered an enhancement/feature section
  if [[ "$line" =~ "Enhancements" || "$line" =~ "Features" || "$line" =~ "New features" || "$line" =~ 💡 || "$line" =~ ^###[[:space:]]+.*[Ee]nhancement || "$line" =~ ^###[[:space:]]+.*[Ff]eature ]]; then
    IS_BREAKING_SECTION=0
    IS_FEATURE_SECTION=1
    IS_BUGFIX_SECTION=0
    continue
  fi
  
  # Check if we've encountered a bug fixes section
  if [[ "$line" =~ "Bug fixes" || "$line" =~ "Bugfixes" || "$line" =~ "Bug Fixes" || "$line" =~ 🧰 || "$line" =~ ^###[[:space:]]+.*[Bb]ug[Ff]ix ]]; then
    IS_BREAKING_SECTION=0
    IS_FEATURE_SECTION=0
    IS_BUGFIX_SECTION=1
    continue
  fi

  # Check if we've moved to another section
  if [[ "$line" =~ ^###[[:space:]] ]]; then
    IS_BREAKING_SECTION=0
    IS_FEATURE_SECTION=0
    IS_BUGFIX_SECTION=0
  fi
  
  # Check if this is a new changelog entry (starts with dash)
  if [[ "$line" =~ ^[[:space:]]*-[[:space:]] ]]; then
    # Process any pending multi-line entry
    if [[ "$IN_MULTI_LINE" -eq 1 && -n "$CURRENT_ENTRY" ]]; then
      # Only process if entry contains the search pattern
      if matches_search_pattern "$CURRENT_ENTRY" "$SEARCH_PATTERN" "$COMPONENT_NAME_LOWER"; then
        if [[ "$VERSION_CHANGES_FOUND" -eq 0 ]]; then
          output_text "### $CURRENT_VERSION"
          VERSION_CHANGES_FOUND=1
          TOTAL_CHANGES_FOUND=1
        fi
        
        # Output the entry with appropriate category
        output_text "- [$ENTRY_CATEGORY] ${CURRENT_ENTRY#- }"
        
        # Increment the appropriate counter
        case "$ENTRY_CATEGORY" in
          "BREAKING") ((BREAKING_COUNT++)) ;;
          "FEATURE") ((FEATURE_COUNT++)) ;;
          "BUG FIX") ((BUGFIX_COUNT++)) ;;
          "OTHER") ((OTHER_COUNT++)) ;;
        esac
      fi
    fi
    
    # Start a new entry
    CLEAN_LINE=$(echo "$line" | sed 's/^[[:space:]]*-[[:space:]]*/- /')
    CURRENT_ENTRY="$CLEAN_LINE"
    IN_MULTI_LINE=1
    
    # Determine the category of this entry
    if [[ "$IS_BREAKING_SECTION" -eq 1 || "$line" =~ "Breaking" || "$line" =~ "breaking" || "$line" =~ 🛑 ]]; then
      ENTRY_CATEGORY="BREAKING"
    elif [[ "$IS_FEATURE_SECTION" -eq 1 || "$line" =~ "Enhance" || "$line" =~ "enhance" || "$line" =~ "Feature" || "$line" =~ "feature" || "$line" =~ 💡 ]]; then
      ENTRY_CATEGORY="FEATURE"
    elif [[ "$IS_BUGFIX_SECTION" -eq 1 || "$line" =~ "Fix" || "$line" =~ "fix" || "$line" =~ "Bug" || "$line" =~ "bug" || "$line" =~ 🧰 ]]; then
      ENTRY_CATEGORY="BUG FIX"
    else
      ENTRY_CATEGORY="OTHER"
    fi
  elif [[ "$IN_MULTI_LINE" -eq 1 && "$line" =~ ^[[:space:]]+ ]]; then
    # This is a continuation of a multi-line entry (indented line)
    # Append to current entry, preserving spaces after the dash in the first line
    CURRENT_ENTRY+=" $(echo "$line" | sed -e 's/^[[:space:]]*//')"
  elif [[ "$line" = "" && "$IN_MULTI_LINE" -eq 1 ]]; then
    # Empty line - ends the current multi-line entry
    # Process the entry if it matches the search pattern
    if matches_search_pattern "$CURRENT_ENTRY" "$SEARCH_PATTERN" "$COMPONENT_NAME_LOWER"; then
      if [[ "$VERSION_CHANGES_FOUND" -eq 0 ]]; then
        output_text "### $CURRENT_VERSION"
        VERSION_CHANGES_FOUND=1
        TOTAL_CHANGES_FOUND=1
      fi
      
      # Output the entry with appropriate category
      output_text "- [$ENTRY_CATEGORY] ${CURRENT_ENTRY#- }"
      
      # Increment the appropriate counter
      case "$ENTRY_CATEGORY" in
        "BREAKING") ((BREAKING_COUNT++)) ;;
        "FEATURE") ((FEATURE_COUNT++)) ;;
        "BUG FIX") ((BUGFIX_COUNT++)) ;;
        "OTHER") ((OTHER_COUNT++)) ;;
      esac
    fi
    
    # Reset multi-line tracking
    CURRENT_ENTRY=""
    IN_MULTI_LINE=0
  elif [[ "$line" =~ ^[^[:space:]] && "$IN_MULTI_LINE" -eq 1 ]]; then
    # Non-indented line that's not empty - ends the current multi-line entry
    # Process the entry if it matches the search pattern
    if matches_search_pattern "$CURRENT_ENTRY" "$SEARCH_PATTERN" "$COMPONENT_NAME_LOWER"; then
      if [[ "$VERSION_CHANGES_FOUND" -eq 0 ]]; then
        output_text "### $CURRENT_VERSION"
        VERSION_CHANGES_FOUND=1
        TOTAL_CHANGES_FOUND=1
      fi
      
      # Output the entry with appropriate category
      output_text "- [$ENTRY_CATEGORY] ${CURRENT_ENTRY#- }"
      
      # Increment the appropriate counter
      case "$ENTRY_CATEGORY" in
        "BREAKING") ((BREAKING_COUNT++)) ;;
        "FEATURE") ((FEATURE_COUNT++)) ;;
        "BUG FIX") ((BUGFIX_COUNT++)) ;;
        "OTHER") ((OTHER_COUNT++)) ;;
      esac
    fi
    
    # Reset multi-line tracking
    CURRENT_ENTRY=""
    IN_MULTI_LINE=0
  fi
  
done < "$CHANGELOG_FILE"

# Process any pending multi-line entry at the end of the file
if [[ "$IN_MULTI_LINE" -eq 1 && -n "$CURRENT_ENTRY" && "$IN_RANGE" -eq 1 ]]; then
  # Only process if entry matches the search pattern
  if matches_search_pattern "$CURRENT_ENTRY" "$SEARCH_PATTERN" "$COMPONENT_NAME_LOWER"; then
    if [[ "$VERSION_CHANGES_FOUND" -eq 0 ]]; then
      output_text "### $CURRENT_VERSION"
      VERSION_CHANGES_FOUND=1
      TOTAL_CHANGES_FOUND=1
    fi
    
    # Output the entry with appropriate category in format "- [CATEGORY] description"
    output_text "- [$ENTRY_CATEGORY] ${CURRENT_ENTRY#- }"
    
    # Increment the appropriate counter
    case "$ENTRY_CATEGORY" in
      "BREAKING") ((BREAKING_COUNT++)) ;;
      "FEATURE") ((FEATURE_COUNT++)) ;;
      "BUG FIX") ((BUGFIX_COUNT++)) ;;
      "OTHER") ((OTHER_COUNT++)) ;;
    esac
  fi
fi

if [[ "$TOTAL_CHANGES_FOUND" -eq 0 ]]; then
  if [[ -n "$PATTERN" ]]; then
    output_text "No changes found matching pattern \"$PATTERN\" between $FROM_VERSION and $TO_VERSION"
  else
    output_text "No changes found for $COMPONENT_NAME between $FROM_VERSION and $TO_VERSION"
  fi
else
  output_text ""
  output_text "## Summary"
  output_text ""
  output_text "| Category | Count |"
  output_text "|----------|-------|"
  output_text "| Breaking Changes | $BREAKING_COUNT |"
  output_text "| Features | $FEATURE_COUNT |"
  output_text "| Bug Fixes | $BUGFIX_COUNT |"
  output_text "| Other Changes | $OTHER_COUNT |"
  output_text "| **Total** | **$((BREAKING_COUNT + FEATURE_COUNT + BUGFIX_COUNT + OTHER_COUNT))** |"
fi

# Display success message if output was written to a file
if [[ -n "$OUTPUT_FILE" ]]; then
  echo "Results written to $OUTPUT_FILE"
fi
