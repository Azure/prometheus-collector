#!/usr/bin/python3
# Copyright (c) Microsoft Corporation.
# Licensed under the MIT License.

import os
import re
import shutil
import subprocess

manifestFile1 = "container-manifest-1"
manifestFile2 = "container-manifest-2"
rpmQueryFormatShort = r'%{NAME}.%{VERSION}-%{RELEASE}.%{ARCH}\n'
rpmQueryFormatLong = r'%{NAME}\t%{VERSION}-%{RELEASE}\t%{INSTALLTIME}\t%{BUILDTIME}\t%{VENDOR}\t%{EPOCH}\t%{SIZE}\t%{ARCH}\t%{EPOCHNUM}\t%{SOURCERPM}\n'

def cleanup(rootDirectory):
    directoriesToDelete = ["/run/lock", "/run/media", "/var/cache/tdnf", "/var/cache/dnf",
                           "/var/lib/dnf", "/var/lib/rpm", "/usr/share/doc", "/usr/local/share/doc",
                           "/usr/share/man", "/usr/local/share/man", "/var/log"]

    for directoryToDelete in directoriesToDelete:
        try:
            shutil.rmtree(rootDirectory + directoryToDelete)
        except OSError as error:
            print("Error: %s - %s." % (error.filename, error.strerror))

def executeBashCommand(command):
    process = subprocess.Popen(command.split(), stdout=subprocess.PIPE, shell = False, encoding='utf-8')
    return process.communicate()

def updateUserConfigFiles(filePath, location, user):
    fileContent = []
    try:
        with open(filePath) as fp:
            for line in fp:
                if "root" in line or user in line:
                    fileContent.append(line)
        with open("{}{}".format(location, filePath), "w") as file:
            file.writelines(fileContent)
    except OSError:
        print("No such file: {}.".format(filePath))

def addNonrootUser(user, userUid, userGid, location):
    print("Adding %s user to image with UID %s and GUI %s." % (user, userUid, userGid))

    tdnfGroupAddCommand = "groupadd --gid {} {}".format(userGid, user)
    executeBashCommand(tdnfGroupAddCommand)
    tdnfUserAddCommand = "useradd --gid {} -g {} {} -u {}".format(userGid, user, user, userUid)
    executeBashCommand(tdnfUserAddCommand)
    tdnfInstallHomeDirCommand = "install -d -m 0755 -o {} -g {} {}/home/{}".format(userUid, userGid, location, user)
    executeBashCommand(tdnfInstallHomeDirCommand)
    updateUserConfigFiles("/etc/passwd", location, user)
    updateUserConfigFiles("/etc/group", location, user)

def getRpmQueryResult(rpmQueryFormat, installLocation):
    rpmCommand = 'rpm --query --all --queryformat {} --root {}'.format(rpmQueryFormat, installLocation)
    rpmQueryResultRaw = list(str(executeBashCommand(rpmCommand)[0]).split('\n'))
    rpmQueryResultCleaned = []

    for rpmQueryElement in rpmQueryResultRaw:
        if len(rpmQueryElement.strip()) > 0 and not rpmQueryElement.startswith("gpg-pubkey"):
            rpmQueryResultCleaned.append(rpmQueryElement)

    return rpmQueryResultCleaned

def writeRpmQueryResultToManifestFile(manifestsDirectory, rpmQueryResult, fileName):
    containerManifestFilePath = "{}/{}".format(manifestsDirectory, fileName)

    with open(containerManifestFilePath, "w") as containerManifestFile:
        for line in rpmQueryResult:
            if len(line.strip()) > 0:
                containerManifestFile.write("{}\n".format(line.strip()))

def getPackageNameFromRpmFileName(rpmFileName):
    return re.split("[-][0-9]", rpmFileName)[0]

def installAzureLinuxPackages(azureLinuxVersion, installLocation, packagesToInstall, packagesToHoldback):
    useRpmNoDeps = len(packagesToHoldback) > 0
    if useRpmNoDeps:
        # Install the distroless-packages-* first using tdnf
        # and then install the remaining packages using rpm --install --nodeps
        distrolessBasePackages = []
        reducedListOfPackagesToInstall = []
        for package in packagesToInstall:
            if package.startswith("distroless-packages-"):
                distrolessBasePackages.append(package)
            else:
                reducedListOfPackagesToInstall.append(package)

        distrolessBasePackagesString = ' '.join(map(str, distrolessBasePackages))
        print(distrolessBasePackages)
        tdnfInstallCommand = "tdnf install -y --releasever={} --installroot={} {}".format(azureLinuxVersion, installLocation, distrolessBasePackagesString)
        _ = executeBashCommand(tdnfInstallCommand)

        rpmsDownloadDirPath = os.path.join(installLocation, "rpms")

        os.makedirs(rpmsDownloadDirPath)
        packagesToDownloadString = ' '.join(map(str, reducedListOfPackagesToInstall))
        tdnfDownloadCommand = "tdnf install -y --downloadonly --downloaddir {} --releasever={} --installroot={} {}".format(rpmsDownloadDirPath, azureLinuxVersion, installLocation, packagesToDownloadString)
        _ = executeBashCommand(tdnfDownloadCommand)

        print("Packages To Holdback: {}".format(packagesToHoldback))

        for file in os.scandir(rpmsDownloadDirPath):
            if file.is_file():
                packageName = getPackageNameFromRpmFileName(file.name)
                if packageName not in packagesToHoldback:
                    rpmInstallCommand = "rpm --install --nodeps --root={} {}".format(installLocation, file.path)
                    _ = executeBashCommand(rpmInstallCommand)
                else:
                    print("Skipping package {} as it is in holdback packages list".format(packageName))

        shutil.rmtree(rpmsDownloadDirPath)
    else:
        packagesToInstallString = ' '.join(map(str, packagesToInstall))
        tdnfInstallCommand = "tdnf install -y --releasever={} --installroot={} {}".format(azureLinuxVersion, installLocation, packagesToInstallString)
        _ = executeBashCommand(tdnfInstallCommand)
    
    tdnfCleanupCommand = "tdnf clean all --releasever={} --installroot={}".format(azureLinuxVersion, installLocation)
    _ = executeBashCommand(tdnfCleanupCommand)

def createContainerManifestFiles(installLocation, manifestsDirectory):
    # Use rpm to get the installed packages list
    rpmQueryResultShort = getRpmQueryResult(rpmQueryFormatShort, installLocation)
    rpmQueryResultLong = getRpmQueryResult(rpmQueryFormatLong, installLocation)

    # Sort rpmQueryResults
    rpmQueryResultShort.sort()
    rpmQueryResultLong.sort()

    # Create new container manifest files with the packages list
    writeRpmQueryResultToManifestFile(manifestsDirectory, rpmQueryResultShort, manifestFile1)
    writeRpmQueryResultToManifestFile(manifestsDirectory, rpmQueryResultLong, manifestFile2)

def readManifestFilesIntoList(filePath, list):
    try:
        with open(filePath) as fp:
            for line in fp:
                list.append(line)
    except OSError:
        print("No such file: {}.".format(filePath))

def updateContainerManifestFiles(installLocation, originalManifestsDirectory, finalManifestsDirectory):
    # Use rpm to get the installed packages list
    rpmQueryResultShort = getRpmQueryResult(rpmQueryFormatShort, installLocation)
    rpmQueryResultLong = getRpmQueryResult(rpmQueryFormatLong, installLocation)

    # Read original container manifest files
    originalRpmQueryResultShort = []
    originalRpmQueryResultLong = []

    originalRpmQueryResultShortFilePath = "{}/{}".format(originalManifestsDirectory, manifestFile1)
    originalRpmQueryResultLongFilePath = "{}/{}".format(originalManifestsDirectory, manifestFile2)

    readManifestFilesIntoList(originalRpmQueryResultShortFilePath, originalRpmQueryResultShort)
    readManifestFilesIntoList(originalRpmQueryResultLongFilePath, originalRpmQueryResultLong)

    # Add existing packages to the current rpmQueryList
    for rpm in originalRpmQueryResultShort:
        if rpm not in rpmQueryResultShort:
            rpmQueryResultShort.append(rpm)

    for rpm in originalRpmQueryResultLong:
        if rpm not in rpmQueryResultLong:
            rpmQueryResultLong.append(rpm)

    # Sort rpmQueryResults
    rpmQueryResultShort.sort()
    rpmQueryResultLong.sort()

    # Create new container manifest files with the packages list
    writeRpmQueryResultToManifestFile(finalManifestsDirectory, rpmQueryResultShort, manifestFile1)
    writeRpmQueryResultToManifestFile(finalManifestsDirectory, rpmQueryResultLong, manifestFile2)