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

echo "Installing packages for re2 gem install..."
sudo tdnf install -y build-essential re2-devel

echo "Installing tomlrb, deep_merge and re2 gems..."
gem install colorize
gem install tomlrb
gem install deep_merge
gem install re2

echo "Installing mdsd..."
if [ "${ARCH}" != "amd64" ]; then
  wget https://github.com/microsoft/Docker-Provider/releases/download/mdsd-mac-official-06-13/azure-mdsd_1.19.3-build.master.428_aarch64.rpm
  sudo tdnf install -y azure-mdsd_1.19.3-build.master.428_aarch64.rpm
else
  wget https://github.com/microsoft/Docker-Provider/releases/download/mdsd-mac-official-06-13/azure-mdsd_1.19.3-build.master.428_x86_64.rpm
  sudo tdnf install -y azure-mdsd_1.19.3-build.master.428_x86_64.rpm
fi
# Install this way once moving to the Mariner published RPMs:
# sudo tdnf install -y azure-mdsd

cp -f $TMPDIR/envmdsd /etc/mdsd.d
# Create the following directory for mdsd logs
mkdir /opt/microsoft/linuxmonagent

# Install telegraf
echo "Installing telegraf..."
sudo tdnf install telegraf-1.23.0 -y
sudo tdnf list installed | grep telegraf | awk '{print $2}' > telegrafversion.txt

# Install fluent-bit
echo "Installing fluent-bit..."
sudo tdnf install fluent-bit-1.9.6 -y

# Setup hourly cron for logrotate
cp /etc/cron.daily/logrotate /etc/cron.hourly/

# Install ME
echo "Installing Metrics Extension..."
sudo tdnf install -y metricsext2-2.2022.1021.1309
sudo tdnf list installed | grep metricsext2 | awk '{print $2}' > metricsextversion.txt

# tdnf does not have an autoremove feature. Only necessary packages are copied over to distroless build. Below reduces the image size if using non-distroless
#sudo tdnf remove g++ binutils libgcc-atomic make patch bison diffutils docbook-dtd-xml gawk glibc-devel installkernel kernel-headers libgcc-devel libgomp-devel libmpc libstdc++-devel libtool libxml2-devel libxslt m4 mariner-rpm-macros mpfr python3-lxml python3-pygments dnf -y

# Remove any RPMs downloaded not from Mariner
rm -f $TMPDIR/metricsext2*.rpm
rm -f $TMPDIR/azure-mdsd*.rpm
# Remove mdsd's telegraf
rm /usr/sbin/telegraf
