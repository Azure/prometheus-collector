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

#used to setcaps for ruby process to read /proc/env
#echo "installing libcap2-bin"
#sudo apt-get install libcap2-bin -y

#install Metrics Extension
# Accept Microsoft public keys
#wget -qO - https://packages.microsoft.com/keys/microsoft.asc | sudo apt-key add -
#wget -qO - https://packages.microsoft.com/keys/msopentech.asc | sudo apt-key add -
# Determine OS distro and code name
#os_id=$(cat /etc/os-release | grep ^ID= | cut -d '=' -f2)
#os_code=$(cat /etc/os-release | grep VERSION_CODENAME | cut -d '=' -f2)
#Add Azure repos
#echo "deb [arch=amd64] https://packages.microsoft.com/repos/microsoft-${os_id}-${os_code}-prod ${os_code} main" | sudo tee /etc/apt/sources.list.d/azure.list
#echo "deb [arch=amd64] https://packages.microsoft.com/repos/azurecore ${os_code} main" | sudo tee -a /etc/apt/sources.list.d/azure.list
# Fetch the package index
#sudo apt-get update
##forceSilent='-o Dpkg::Options::="--force-confdef" -o Dpkg::Options::="--force-confold"'
#sudo apt-get install metricsext2=2.2021.302.1751-2918e9-~bionic -y

#Get collector
#wget https://github.com/open-telemetry/opentelemetry-collector/releases/download/v0.29.0/otelcol_linux_amd64
#mkdir --parents /opt/microsoft/otelcollector29
#mv ./otelcol_linux_amd64 /opt/microsoft/otelcollector29/otelcollector
#chmod 777 /opt/microsoft/otelcollector29/otelcollector

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
#sudo apt-get install -y apt-transport-https gnupg

# Accept Microsoft public keys
#wget -qO - https://packages.microsoft.com/keys/microsoft.asc | sudo apt-key add -
#wget -qO - https://packages.microsoft.com/keys/msopentech.asc | sudo apt-key add -

# Source information on OS distro and code name
#. /etc/os-release

#if [ "$ID" = ubuntu ]; then
#    REPO_NAME=azurecore
#elif [ "$ID" = debian ]; then
#    REPO_NAME=azurecore-debian
#else
#    echo "Unsupported distribution: $ID"
#    exit 1
#fi

# Add azurecore repo and update package list
#echo "deb [arch=amd64] https://packages.microsoft.com/repos/$REPO_NAME $VERSION_CODENAME main" | sudo tee -a /etc/apt/sources.list.d/azure.list
#sudo apt-get update

# Pinning to the latest stable version of ME
#sudo apt-get install -y metricsext2=2.2022.312.2300-d1b4f6-~focal

#wget https://rashmi.blob.core.windows.net/rashmi-mac-mdsd/metricsext2_2.2022.201.001-9e07c0-_focal_amd64.deb
#/usr/bin/dpkg -i $TMPDIR/metricsext2*.deb
#sudo apt --fix-broken install -y
#/usr/bin/dpkg -i $TMPDIR/metricsext2*.deb

wget https://github.com/microsoft/Docker-Provider/releases/download/04012021/metricsext2-2.2022.727.2052-1.cm2.x86_64.rpm
sudo tdnf install -y metricsext2-2.2022.727.2052-1.cm2.x86_64.rpm

# tdnf does not have an autoremove feature. Only necessary packages are copied over to distroless build.
sudo tdnf remove g++ binutils libgcc-atomic make patch bison diffutils docbook-dtd-xml gawk glibc-devel installkernel kernel-headers libgcc-devel libgomp-devel libmpc libstdc++-devel libtool libxml2-devel libxslt m4 mariner-rpm-macros mpfr python3-lxml python3-pygments dnf -y
#sudo dnf autoremove -y

rm -f $TMPDIR/metricsext2*.rpm