#!/bin/bash

TMPDIR="/opt"
cd $TMPDIR

chmod 744 /usr/sbin/

# One transaction: in the emulated arm64 build each tdnf call spends ~55s loading repo metadata.
# inotify-tools watches configmap changes.
echo "Installing inotify, mdsd and Metrics Extension..."
sudo tdnf install -y ca-certificates-microsoft inotify-tools azure-mdsd-1.40.2 metricsext2-2.2026.227.1509
sudo update-ca-trust
#Need this for newer scripts
chmod 544 $TMPDIR/*.sh
chmod 544 $TMPDIR/microsoft/liveness/*.sh

cp -f $TMPDIR/envmdsd /etc/mdsd.d
# Create the following directory for mdsd logs
mkdir /opt/microsoft/linuxmonagent

sudo tdnf list installed | grep metricsext2 | awk '{print $2}' > metricsextversion.txt

# Remove any RPMs downloaded not from Mariner
rm -f $TMPDIR/metricsext2*.rpm
rm -f $TMPDIR/azure-mdsd*.rpm
