#!/bin/bash

TMPDIR="/opt"
cd $TMPDIR

if [ -z $1 ]; then
    ARCH="amd64"
else
    ARCH=$1
fi

# One transaction: in the emulated arm64 build each tdnf call spends ~55s loading repo metadata.
# inotify-tools watches configmap changes.
echo "Installing inotify..."
sudo tdnf install -y ca-certificates-microsoft inotify-tools
sudo update-ca-trust

#Need this for newer scripts
chmod 744 /usr/sbin/

# Setup hourly cron for logrotate
cp /etc/cron.daily/logrotate /etc/cron.hourly/
