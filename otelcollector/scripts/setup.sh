#!/bin/bash

TMPDIR="/opt"
cd $TMPDIR

if [ -z $1 ]; then
    ARCH="amd64"
else
    ARCH=$1
fi

sudo tdnf install ca-certificates-microsoft -y
sudo update-ca-trust

#Need this for newer scripts
chmod 544 $TMPDIR/*.sh
chmod 544 $TMPDIR/microsoft/liveness/*.sh
chmod 544 $TMPDIR/microsoft/configmapparser/*.rb

chmod 744 /usr/sbin/

#download inotify tools for watching configmap changes
echo "Installing inotify..."
sudo tdnf check-update
sudo tdnf repolist --refresh
sudo tdnf install inotify-tools -y

echo "Installing mdsd..."
sudo tdnf install -y azure-mdsd-1.40.3

cp -f $TMPDIR/envmdsd /etc/mdsd.d
# Create the following directory for mdsd logs
mkdir /opt/microsoft/linuxmonagent

# Setup hourly cron for logrotate
cp /etc/cron.daily/logrotate /etc/cron.hourly/

# Install ME
echo "Installing Metrics Extension..."
sudo tdnf install -y metricsext2-2.2026.227.1509

sudo tdnf list installed | grep metricsext2 | awk '{print $2}' > metricsextversion.txt

# tdnf does not have an autoremove feature. Only necessary packages are copied over to distroless build. Below reduces the image size if using non-distroless
#sudo tdnf remove g++ binutils libgcc-atomic make patch bison diffutils docbook-dtd-xml gawk glibc-devel installkernel kernel-headers libgcc-devel libgomp-devel libmpc libstdc++-devel libtool libxml2-devel libxslt m4 mariner-rpm-macros mpfr python3-lxml python3-pygments dnf -y

# Remove any RPMs downloaded not from Mariner
rm -f $TMPDIR/metricsext2*.rpm
rm -f $TMPDIR/azure-mdsd*.rpm
