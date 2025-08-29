#!/bin/bash

# Get current OpenTelemetry Collector version if it exists
CURRENT_VERSION=""
if [ -f "OPENTELEMETRY_VERSION" ]; then
    CURRENT_VERSION=$(cat OPENTELEMETRY_VERSION)
    echo "Current OpenTelemetry Collector version: $CURRENT_VERSION"
else
    echo "No existing version file found. Will create one."
fi

CURRENT_TA_VERSION=""
if [ -f "TARGETALLOCATOR_VERSION" ]; then
    CURRENT_TA_VERSION=$(cat TARGETALLOCATOR_VERSION)
    echo "Current OpenTelemetry TA version: $CURRENT_TA_VERSION"
else
    echo "No existing TA version file found. Will create one."
fi

# Script to check the latest released version of opentelemetry-collector-operator

# GitHub API URL for the repository releases
API_URL="https://api.github.com/repos/open-telemetry/opentelemetry-operator/releases/latest"

# Fetch the latest release information
echo "Fetching the latest release of opentelemetry-collector-operator..."
RESPONSE=$(curl -s $API_URL)

# Check if curl command was successful
if [ $? -ne 0 ]; then
    echo "Error: Failed to fetch release information"
    exit 1
fi

# Extract the version tag and name
VERSION_TAG=$(echo $RESPONSE | grep -o '"tag_name": "[^"]*' | cut -d'"' -f4)
RELEASE_NAME=$(echo $RESPONSE | grep -o '"name": "[^"]*' | head -1 | cut -d'"' -f4)
RELEASE_DATE=$(echo $RESPONSE | grep -o '"published_at": "[^"]*' | cut -d'"' -f4)

# Check if version information was extracted successfully
if [ -z "$VERSION_TAG" ]; then
    echo "Error: Could not extract version information"
    exit 1
fi

# Display the results
echo "Latest release: $RELEASE_NAME ($VERSION_TAG)"
echo "Released on: $RELEASE_DATE"
echo "GitHub URL: https://github.com/open-telemetry/opentelemetry-operator/releases/tag/$VERSION_TAG"

# Fetch release notes for opentelemetry-collector with the same version tag
echo -e "\nChecking for matching release in opentelemetry-collector repository..."

# GitHub API URL for opentelemetry-collector repository
COLLECTOR_API_URL="https://api.github.com/repos/open-telemetry/opentelemetry-collector/releases"

# Fetch releases for the collector repository
COLLECTOR_RESPONSE=$(curl -s "$COLLECTOR_API_URL")

# Check if curl command was successful
if [ $? -ne 0 ]; then
    echo "Error: Failed to fetch opentelemetry-collector release information"
    exit 1
fi
# Extract major.minor part of the version tag
MAJOR_MINOR=$(echo "$VERSION_TAG" | grep -o 'v[0-9]\+\.[0-9]\+')

if [ -z "$MAJOR_MINOR" ]; then
    echo "Error: Could not extract major.minor version from $VERSION_TAG"
    exit 1
fi

echo "Writing TARGETALLOCATOR_VERSION to TARGETALLOCATOR_VERSION file..."
echo "$VERSION_TAG" > TARGETALLOCATOR_VERSION
if [ $? -ne 0 ]; then
    echo "Error: Failed to write TARGETALLOCATOR_VERSION to file"
    exit 1
fi
TA_VERSION=$(cat TARGETALLOCATOR_VERSION)
echo "TARGETALLOCATOR_VERSION set to: $TA_VERSION"

# Find the latest patch version for the matching major.minor version
echo "Looking for the latest patch version matching major.minor version: $MAJOR_MINOR"

# Extract all releases that match the major.minor pattern and find the latest patch version
MATCHING_RELEASES=$(echo "$COLLECTOR_RESPONSE" | grep -o "\"tag_name\": \"$MAJOR_MINOR\.[0-9]\+\"" | grep -o "$MAJOR_MINOR\.[0-9]\+" | sort -V | tail -1)

if [ -z "$MATCHING_RELEASES" ]; then
    echo "No matching release found with major.minor version $MAJOR_MINOR in opentelemetry-collector repository"
    echo "Available releases in opentelemetry-collector:"
    echo "$COLLECTOR_RESPONSE" | grep -o '"tag_name": "[^"]*' | cut -d'"' -f4 | head -10
    exit 1
else
    LATEST_PATCH_VERSION="$MATCHING_RELEASES"
    echo "Found latest patch version: $LATEST_PATCH_VERSION"
    
    # Now find the complete release information for this specific version
    MATCHING_RELEASE=$(echo "$COLLECTOR_RESPONSE" | jq -r ".[] | select(.tag_name == \"$LATEST_PATCH_VERSION\")")
    
    if [ -z "$MATCHING_RELEASE" ] || [ "$MATCHING_RELEASE" = "null" ]; then
        # Fallback to grep-based extraction if jq is not available or fails
        echo "Using fallback method to extract release information..."
        COLLECTOR_RELEASE_NAME=$(echo "$COLLECTOR_RESPONSE" | grep -A 10 "\"tag_name\": \"$LATEST_PATCH_VERSION\"" | grep -o '"name": "[^"]*' | head -1 | cut -d'"' -f4)
        COLLECTOR_RELEASE_DATE=$(echo "$COLLECTOR_RESPONSE" | grep -A 10 "\"tag_name\": \"$LATEST_PATCH_VERSION\"" | grep -o '"published_at": "[^"]*' | head -1 | cut -d'"' -f4)
        COLLECTOR_RELEASE_URL="https://github.com/open-telemetry/opentelemetry-collector/releases/tag/$LATEST_PATCH_VERSION"
    else
        # Extract release information using jq
        COLLECTOR_RELEASE_NAME=$(echo "$MATCHING_RELEASE" | jq -r '.name')
        COLLECTOR_RELEASE_DATE=$(echo "$MATCHING_RELEASE" | jq -r '.published_at')
        COLLECTOR_RELEASE_URL=$(echo "$MATCHING_RELEASE" | jq -r '.html_url')
    fi
    
    echo "Found matching release: $COLLECTOR_RELEASE_NAME"
    echo "Released on: $COLLECTOR_RELEASE_DATE"
    echo "Release notes: $COLLECTOR_RELEASE_URL"
fi

# Parse the release name to extract stable and beta versions
# First, try to parse from the release name format (stable/beta)
if [[ "$COLLECTOR_RELEASE_NAME" =~ (v?[0-9]+\.[0-9]+\.[0-9]+)/(v?[0-9]+\.[0-9]+\.[0-9]+) ]]; then
    STABLE_VERSION="${BASH_REMATCH[1]}"
    BETA_VERSION="${BASH_REMATCH[2]}"
    
    echo -e "\nParsed versions from release name:"
    echo "STABLE_VERSION: $STABLE_VERSION"
    echo "BETA_VERSION: $BETA_VERSION"
else
    # If release name doesn't contain both versions, use the latest patch version we found
    echo -e "\nRelease name doesn't contain stable/beta format. Using latest patch version."
    BETA_VERSION="$LATEST_PATCH_VERSION"
    
    # For stable version, we need to determine it based on the version pattern
    # Typically, stable versions are v1.x.x and beta versions are v0.x.x
    if [[ "$BETA_VERSION" =~ ^v?1\. ]]; then
        STABLE_VERSION="$BETA_VERSION"
        # Find a corresponding v0.x.x version by looking for the previous major version
        BETA_SEARCH=$(echo "$BETA_VERSION" | sed 's/v1\./v0./')
        AVAILABLE_BETA=$(echo "$COLLECTOR_RESPONSE" | grep -o "\"tag_name\": \"$BETA_SEARCH[0-9]\+\"" | grep -o "$BETA_SEARCH[0-9]\+" | sort -V | tail -1)
        if [ ! -z "$AVAILABLE_BETA" ]; then
            BETA_VERSION="$AVAILABLE_BETA"
        fi
    else
        # If it's a v0.x.x version, look for corresponding v1.x.x stable version
        STABLE_SEARCH=$(echo "$BETA_VERSION" | sed 's/v0\./v1./')
        AVAILABLE_STABLE=$(echo "$COLLECTOR_RESPONSE" | grep -o "\"tag_name\": \"$STABLE_SEARCH[0-9]\+\"" | grep -o "$STABLE_SEARCH[0-9]\+" | sort -V | tail -1)
        if [ ! -z "$AVAILABLE_STABLE" ]; then
            STABLE_VERSION="$AVAILABLE_STABLE"
        else
            # If no v1.x.x version exists, use the beta version for both
            STABLE_VERSION="$BETA_VERSION"
        fi
    fi
    
    echo "Determined versions:"
    echo "STABLE_VERSION: $STABLE_VERSION"
    echo "BETA_VERSION: $BETA_VERSION"
fi

# Check if the BETA_VERSION is the same as the CURRENT_VERSION
if [ "$BETA_VERSION" = "$CURRENT_VERSION" ]; then
    echo "The latest version ($BETA_VERSION) is already in use. No update needed."
    exit 0
fi

# Write BETA_VERSION to file
echo "Writing BETA_VERSION to OPENTELEMETRY_VERSION file..."
echo "$BETA_VERSION" > OPENTELEMETRY_VERSION
if [ $? -eq 0 ]; then
    echo "Successfully wrote version $BETA_VERSION to OPENTELEMETRY_VERSION file"
else
    echo "Error: Failed to write to OPENTELEMETRY_VERSION file"
    exit 1
fi

# Run upgrade.sh with the parsed versions
echo -e "\nRunning upgrade.sh with the parsed versions..."
if [ -f "./internal/otel-upgrade-scripts/upgrade.sh" ]; then
    ./internal/otel-upgrade-scripts/upgrade.sh "$BETA_VERSION" "$STABLE_VERSION" "$TA_VERSION" "$CURRENT_VERSION" "$CURRENT_TA_VERSION"
else
    echo "Warning: upgrade.sh not found in the current directory"
    exit 1
fi

