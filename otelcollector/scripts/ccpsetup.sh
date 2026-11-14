#!/bin/bash

TMPDIR="/opt"
cd $TMPDIR

chmod 744 /usr/sbin/

# ca-certificates-microsoft, inotify-tools, mdsd and Metrics Extension are installed by the Dockerfile.
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
