#!/bin/bash

TMPDIR="/opt"
cd $TMPDIR

chmod 744 /usr/sbin/

sudo tdnf install ca-certificates-microsoft -y
sudo update-ca-trust
#Need this for newer scripts
chmod 544 $TMPDIR/*.sh
chmod 544 $TMPDIR/microsoft/liveness/*.sh

echo "Installing packages for re2 gem install..."
sudo tdnf install -y build-essential re2-devel

echo "Installing tomlrb, deep_merge and re2 gems..."
gem install colorize
gem install tomlrb
gem install deep_merge
gem install re2

#download inotify tools for watching configmap changes
echo "Installing inotify..."
sudo tdnf check-update
sudo tdnf repolist --refresh
sudo tdnf install inotify-tools -y

echo "Installing mdsd..."
# sudo tdnf install -y azure-mdsd-1.30.3

cp -f $TMPDIR/envmdsd /etc/mdsd.d
# Create the following directory for mdsd logs
mkdir /opt/microsoft/linuxmonagent

# Install ME
echo "Installing Metrics Extension..."
# sudo tdnf install -y metricsext2-2.2024.419.1535
sudo tdnf list installed | grep metricsext2 | awk '{print $2}' > metricsextversion.txt

# Remove any RPMs downloaded not from Mariner
rm -f $TMPDIR/metricsext2*.rpm
rm -f $TMPDIR/azure-mdsd*.rpm
# Remove mdsd's telegraf
# rm /usr/sbin/telegraf
