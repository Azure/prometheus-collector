#!/usr/bin/python3
# Copyright (c) Microsoft Corporation.
# Licensed under the MIT License.

import argparse
import marinaracommon
import os
from pathlib import Path

class BuildData:
    def __init__(self,
               azureLinuxVersion,
               location,
               packagesToAdd,
               packagesToHoldback,
               addNonroot,
               user,
               userUid,
               userGid,
               originalManifestsDirectory,
               finalManifestsDirectory):
        self.azureLinuxVersion = azureLinuxVersion
        self.location = location
        self.packagesToAdd = packagesToAdd
        self.packagesToHoldback = packagesToHoldback
        self.addNonroot = addNonroot
        self.user = user
        self.userUid = userUid
        self.userGid = userGid
        self.originalManifestsDirectory = originalManifestsDirectory
        self.finalManifestsDirectory = finalManifestsDirectory

    def __repr__(self):
        return str(self.__dict__)

def readArgs():
    parser = argparse.ArgumentParser(
        description="Extend an Azure Linux Distroless Image.",
        formatter_class=argparse.ArgumentDefaultsHelpFormatter,
    )
    parser.add_argument(
        "--azure-linux-version", required=True, type=str, help="Azure Linux version (2.0, 3.0).",
    )
    parser.add_argument(
        "--location", required=True, type=str, help="Directory location to install the packages in.",
    )
    parser.add_argument(
        "--add-packages", required=True, type=str, help="Packages to install.",
    )
    parser.add_argument(
        "--packages-to-holdback", type=str, help="Packages to holdback from getting installed.",
    )
    parser.add_argument(
        "--existing-manifest-location", required=True, type=str, help="Existing manifest files location.",
    )
    parser.add_argument(
        "--new-manifest-location", required=True, type=str, help="New manifest files location.",
    )
    parser.add_argument(
        "--user", required=True, type=str, help="User to add as nonroot.",
    )
    parser.add_argument(
        "--user-uid", required=True, type=int, help="User's UID.",
    )
    parser.add_argument(
        "--user-gid", required=True, type=int, help="User's GID.",
    )

    return parser.parse_args()

def validateArgs(args):
    # Ensure manifest files exist
    manifestFile1 = "container-manifest-1"
    manifestFile2 = "container-manifest-2"
    existingManifestFile1Path = "{}/{}".format(args.existing_manifest_location, manifestFile1)
    existingManifestFile2Path = "{}/{}".format(args.existing_manifest_location, manifestFile2)
    existingManifestFile1 = Path(existingManifestFile1Path)
    existingManifestFile2 = Path(existingManifestFile2Path)

    if not (existingManifestFile1.exists() and existingManifestFile2.exists()):
        raise ValueError("Invalid value \"%s\" passed for argument %s." % (args.existing_manifest_location, "--existing-manifest-location"))

    # Validate packages
    if not args.add_packages.strip():
        raise ValueError("Invalid value \"%s\" passed for argument %s." % (args.add_packages, "--add-packages"))

    # Validate Azure Linux version
    if args.azure_linux_version != "2.0" and args.azure_linux_version != "3.0":
        raise ValueError("Invalid value \"%s\" passed for argument %s." % (args.azure_linux_version, "--azure-linux-version"))

    # Validate root/nonroot user
    if "root" == args.user:
        if args.user_uid != 0:
            raise ValueError("User UID cannot be non-zero for root user")
        if args.user_gid != 0:
            raise ValueError("User GID cannot be non-zero for root user")
    else:
        if not args.user.strip():
            raise ValueError("User name cannot be empty for nonroot user.")
        if args.user_uid == 0:
            raise ValueError("User UID cannot be 0 for nonroot user")
        if args.user_gid == 0:
            raise ValueError("User GID cannot be 0 for nonroot user")

def prepareBuild(args):
    addNonrootUser = False
    if "root" != args.user:
        addNonrootUser = True

    addPackages = args.add_packages.split()
    packagesToHoldback = args.packages_to_holdback.split()

    finalManifestsDirectory = args.location + args.new_manifest_location
    os.makedirs(finalManifestsDirectory)

    return BuildData(
        args.azure_linux_version,
        args.location,
        addPackages,
        packagesToHoldback,
        addNonrootUser,
        args.user,
        args.user_uid,
        args.user_gid,
        args.existing_manifest_location,
        finalManifestsDirectory
    )

def buildImage(buildData):
    marinaracommon.installAzureLinuxPackages(
        buildData.azureLinuxVersion,
        buildData.location,
        buildData.packagesToAdd,
        buildData.packagesToHoldback
    )

    marinaracommon.updateContainerManifestFiles(buildData.location, buildData.originalManifestsDirectory, buildData.finalManifestsDirectory)

    if buildData.addNonroot:
        marinaracommon.addNonrootUser(buildData.user, buildData.userUid, buildData.userGid, buildData.location)

    marinaracommon.cleanup(buildData.location)

args = readArgs()
validateArgs(args)
buildData = prepareBuild(args)
print(buildData)
buildImage(buildData)