#!/bin/bash

TMPDIR="/opt"
cd $TMPDIR

chmod 744 /usr/sbin/

#download inotify tools for watching configmap changes
echo "Installing inotify..."
sudo tdnf check-update
sudo tdnf repolist --refresh
sudo tdnf install inotify-tools -y

echo "Installing mdsd..."
sudo tdnf install -y azure-mdsd-1.27.4

cp -f $TMPDIR/envmdsd /etc/mdsd.d
# Create the following directory for mdsd logs
mkdir /opt/microsoft/linuxmonagent

# Install ME
echo "Installing Metrics Extension..."
sudo tdnf install -y metricsext2-2.2023.928.2134
sudo tdnf list installed | grep metricsext2 | awk '{print $2}' > metricsextversion.txt

# Remove any RPMs downloaded not from Mariner
rm -f $TMPDIR/metricsext2*.rpm
rm -f $TMPDIR/azure-mdsd*.rpm
# Remove mdsd's telegraf
rm /usr/sbin/telegraf
