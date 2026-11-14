#!/bin/bash

TMPDIR="/opt"
cd $TMPDIR

if [ -z $1 ]; then
    ARCH="amd64"
else
    ARCH=$1
fi

# ca-certificates-microsoft and inotify-tools (configmap watching) are installed by the Dockerfile.
sudo update-ca-trust

#Need this for newer scripts
chmod 744 /usr/sbin/

# Setup hourly cron for logrotate
cp /etc/cron.daily/logrotate /etc/cron.hourly/
