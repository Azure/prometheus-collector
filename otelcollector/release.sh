#!/bin/bash

if [ -z "$previous_semver" ]
then
  echo "previous_semver is not set. Re-run as: previous_semver=prevsemver current_semver=currsemver ./release.sh"
  exit
fi

if [ -z "$current_semver" ]
then
  echo "current_semver is not set. Re-run as: previous_semver=prevsemver current_semver=currsemver ./release.sh"
  exit
fi

semver_regex="[0-9.]-main-\+"
echo "Replacing $previous_semver globally with $current_semver in docs folder"
for file in $(find ./docs/ ! -name "PromMDMReleaseNotes.md" -name "*.md" -o -name "*.yaml" -o -name "*.yml" -type f); do echo -e "\n$file: "; sed -i "s/$previous_semver/$current_semver/g" $file; done
# Replacing the image  in scan-released-image.yml
echo "Replacing $previous_semver globally with $current_semver in github workflow - scan-released-image.yml"
sed -i "s/$previous_semver/$current_semver/g" ../.github/workflows/scan-released-image.yml
