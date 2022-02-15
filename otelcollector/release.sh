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
echo "Replacing $previous_semver globally with $current_semver"
for file in $(find ../docs/ ! -name "PromMDMReleaseNotes.md" -name "*.md" -o -name "*.yaml" -o -name "*.yml" -type f); do echo -e "\n$file: "; sed -i "s/$previous_semver/$current_semver/g" $file; done

