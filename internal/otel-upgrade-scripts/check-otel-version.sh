#!/bin/bash

# Get current OpenTelemetry Collector version if it exists
CURRENT_VERSION=""
if [ -f "OPENTELEMETRY_VERSION" ]; then
    CURRENT_VERSION=$(cat OPENTELEMETRY_VERSION)
    echo "Current OpenTelemetry Collector version: $CURRENT_VERSION"
else
    echo "No existing version file found. Will create one."
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

# Look for the matching version tag
MATCHING_RELEASE=$(echo "$COLLECTOR_RESPONSE" | grep -A 100 "\"tag_name\": \"$VERSION_TAG\"" | grep -m 1 -B 100 -A 100 "}")

if [ -z "$MATCHING_RELEASE" ]; then
    echo "No matching release found with tag $VERSION_TAG in opentelemetry-collector repository"
else
    # Extract release information
    COLLECTOR_RELEASE_NAME=$(echo "$MATCHING_RELEASE" | grep -o '"name": "[^"]*' | head -1 | cut -d'"' -f4)
    COLLECTOR_RELEASE_DATE=$(echo "$MATCHING_RELEASE" | grep -o '"published_at": "[^"]*' | cut -d'"' -f4)
    COLLECTOR_RELEASE_URL=$(echo "$MATCHING_RELEASE" | grep -o '"html_url": "[^"]*' | head -1 | cut -d'"' -f4)
    
    echo "Found matching release: $COLLECTOR_RELEASE_NAME"
    echo "Released on: $COLLECTOR_RELEASE_DATE"
    echo "Release notes: $COLLECTOR_RELEASE_URL"
fi

# Parse the release name to extract stable and beta versions
if [[ "$COLLECTOR_RELEASE_NAME" =~ (v?[0-9]+\.[0-9]+\.[0-9]+)/(v?[0-9]+\.[0-9]+\.[0-9]+) ]]; then
    STABLE_VERSION="${BASH_REMATCH[1]}"
    BETA_VERSION="${BASH_REMATCH[2]}"
    
    echo -e "\nParsed versions:"
    echo "STABLE_VERSION: $STABLE_VERSION"
    echo "BETA_VERSION: $BETA_VERSION"

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
    if [ -f "upgrade.sh" ]; then
        ./upgrade.sh "$BETA_VERSION" "$STABLE_VERSION"
    else
        echo "Warning: script.sh not found in the current directory"
        exit 1
    fi
else
    echo -e "\nError: Could not parse stable and beta versions from release name: $COLLECTOR_RELEASE_NAME"
    echo "Expected format: {STABLE_VERSION}/{BETA_VERSION}"
    exit 1
fi

