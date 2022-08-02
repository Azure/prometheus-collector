#!/bin/bash

TMPDIR="/opt"
cd $TMPDIR

sudo tdnf install ca-certificates-microsoft -y

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

echo "Installing MDSD dependencies"
sudo tdnf install -y which
echo "Downloading MDSD"
sudo tdnf --disablerepo="*" --enablerepo=packages-microsoft-com-azurecore install azure-mdsd-1.18.0 -y
cp -f $TMPDIR/envmdsd /etc/mdsd.d
# Create the following directory for logs
mkdir /opt/microsoft/linuxmonagent

# Install Telegraf
echo "Installing telegraf..."
sudo tdnf install telegraf-1.23.0 -y

# Install fluent-bit
echo "Installing fluent-bit..."
sudo tdnf install fluent-bit-1.8.12 -y

# setup hourly cron for logrotate
cp /etc/cron.daily/logrotate /etc/cron.hourly/

# Installing ME
echo "Installing Metrics Extension..."
wget https://github.com/microsoft/Docker-Provider/releases/download/04012021/metricsext2-2.2022.727.2052-1.cm2.x86_64.rpm
sudo tdnf install -y metricsext2-2.2022.727.2052-1.cm2.x86_64.rpm
sudo tdnf list installed | grep metricsext2 | awk '{print $2}' > metricsextversion.txt
rm -f $TMPDIR/metricsext2*.rpm

# tdnf does not have an autoremove feature. Only necessary packages are copied over to distroless build. Below reduces the image size if using non-distroless
#sudo tdnf remove g++ binutils libgcc-atomic make patch bison diffutils docbook-dtd-xml gawk glibc-devel installkernel kernel-headers libgcc-devel libgomp-devel libmpc libstdc++-devel libtool libxml2-devel libxslt m4 mariner-rpm-macros mpfr python3-lxml python3-pygments dnf -y
