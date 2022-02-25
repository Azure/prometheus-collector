#!/bin/bash

TMPDIR="/opt"
cd $TMPDIR

sed -i -e 's/# en_US.UTF-8 UTF-8/en_US.UTF-8 UTF-8/' /etc/locale.gen && \
    dpkg-reconfigure --frontend=noninteractive locales && \
    update-locale LANG=en_US.UTF-8

#Need this for newer scripts
chmod 775 $TMPDIR/*.sh
chmod 775 $TMPDIR/microsoft/liveness/*.sh
chmod 775 $TMPDIR/microsoft/configmapparser/*.rb

chmod 777 /usr/sbin/

#download inotify tools for watching configmap changes
echo "Installing inotify..."
sudo apt-get update
sudo apt-get install inotify-tools -y

echo "Installing packages for re2 gem install..."
sudo apt-get install -y build-essential libre2-dev ruby-dev

echo "Installing tomlrb, deep_merge and re2 gems..."
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

echo "Downloading MDSD"
wget https://rashmi.blob.core.windows.net/rashmi-mac-mdsd/azure-mdsd_1.16.0-build.develop.2626_x86_64.deb
/usr/bin/dpkg -i $TMPDIR/azure-mdsd*.deb
# cp -f $TMPDIR/mdsd.xml /etc/mdsd.d
cp -f $TMPDIR/envmdsd /etc/mdsd.d

# Install Telegraf
echo "Installing telegraf..."
wget https://dl.influxdata.com/telegraf/releases/telegraf-1.18.0_linux_amd64.tar.gz
tar -zxvf telegraf-1.18.0_linux_amd64.tar.gz
mv /opt/telegraf-1.18.0/usr/bin/telegraf /opt/telegraf/telegraf
chmod 777 /opt/telegraf/telegraf

# Install fluent-bit
echo "Installing fluent-bit..."
wget -qO - https://packages.fluentbit.io/fluentbit.key | sudo apt-key add -
sudo echo "deb https://packages.fluentbit.io/ubuntu/xenial xenial main" >> /etc/apt/sources.list
sudo echo "deb http://security.ubuntu.com/ubuntu bionic-security main" >> /etc/apt/sources.list.d/bionic.list
sudo apt-get update


# Some dependencies were fixed with sudo apt --fix-broken, try installing td-agent-bit again
# This is because we are keeping the same fluentbit version but have upgraded ubuntu
sudo apt-get install td-agent-bit=1.7.8 -y

# setup hourly cron for logrotate
cp /etc/cron.daily/logrotate /etc/cron.hourly/

# Moving ME installation to the end until we fix the broken dependencies issue
# wget https://github.com/microsoft/Docker-Provider/releases/download/04012021/metricsext2_2.2021.901.1511-69f7bf-_focal_amd64.deb

# # Install ME
# /usr/bin/dpkg -i $TMPDIR/metricsext2*.deb

# # Fixing broken installations in order to get a clean ME install
# sudo apt --fix-broken install -y

# # Installing ME again after fixing broken dependencies
# /usr/bin/dpkg -i $TMPDIR/metricsext2*.deb

# Installing ME
echo "Installing Metrics Extension..."
sudo apt-get install -y apt-transport-https gnupg

# Accept Microsoft public keys
wget -qO - https://packages.microsoft.com/keys/microsoft.asc | sudo apt-key add -
wget -qO - https://packages.microsoft.com/keys/msopentech.asc | sudo apt-key add -

# Source information on OS distro and code name
. /etc/os-release

if [ "$ID" = ubuntu ]; then
    REPO_NAME=azurecore
elif [ "$ID" = debian ]; then
    REPO_NAME=azurecore-debian
else
    echo "Unsupported distribution: $ID"
    exit 1
fi

# Add azurecore repo and update package list
echo "deb [arch=amd64] https://packages.microsoft.com/repos/$REPO_NAME $VERSION_CODENAME main" | sudo tee -a /etc/apt/sources.list.d/azure.list
sudo apt-get update


wget https://rashmi.blob.core.windows.net/rashmi-mac-mdsd/metricsext2_2.2022.201.001-9e07c0-_focal_amd64.deb
/usr/bin/dpkg -i $TMPDIR/metricsext2*.deb

sudo apt --fix-broken install -y

/usr/bin/dpkg -i $TMPDIR/metricsext2*.deb


# Pinning to the latest stable version of ME
#sudo apt-get install -y metricsext2=2.2021.924.1646-2df972-~focal

# sudo apt-get install -y metricsext2=2.2022.201.001-9e07c0-~focal

# Cleaning up unused packages
echo "Cleaning up packages used for re2 gem install..."

#Uninstalling packages after gem install re2
sudo apt-get remove build-essential -y
sudo apt-get remove ruby-dev -y

echo "auto removing unused packages..."
sudo apt-get autoremove -y

#cleanup all install
echo "cleaning up all install.."
#rm -f $TMPDIR/metricsext2*.deb
rm -f $TMPDIR/prometheus-2.25.2.linux-amd64.tar.gz
rm -rf $TMPDIR/prometheus-2.25.2.linux-amd64
rm -f $TMPDIR/telegraf*.gz
rm -rf $TMPDIR/telegraf-1.18.0/
