#!/bin/bash

TMPDIR="/opt"
cd $TMPDIR

sed -i -e 's/# en_US.UTF-8 UTF-8/en_US.UTF-8 UTF-8/' /etc/locale.gen && \
    dpkg-reconfigure --frontend=noninteractive locales && \
    update-locale LANG=en_US.UTF-8

wget https://github.com/microsoft/Docker-Provider/releases/download/04012021/metricsext2_2.2021.423.1034-da440c-_focal_amd64.deb

#Need this for newer scripts
chmod 775 $TMPDIR/*.sh
chmod 775 $TMPDIR/microsoft/liveness/*.sh
chmod 775 $TMPDIR/microsoft/configmapparser/*.rb

chmod 777 /usr/sbin/

#Install ME
/usr/bin/dpkg -i $TMPDIR/metricsext2*.deb

#download inotify tools for watching configmap changes
sudo apt-get update
sudo apt-get install inotify-tools -y

gem install tomlrb

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

# Install promtools for prometheus config validation
wget https://github.com/prometheus/prometheus/releases/download/v2.25.2/prometheus-2.25.2.linux-amd64.tar.gz
tar -xf $TMPDIR/prometheus-2.25.2.linux-amd64.tar.gz
cp -f $TMPDIR/prometheus-2.25.2.linux-amd64/promtool /opt/promtool
chmod 777 /opt/promtool

# Install Telegraf
wget https://dl.influxdata.com/telegraf/releases/telegraf-1.18.0_linux_amd64.tar.gz
tar -zxvf telegraf-1.18.0_linux_amd64.tar.gz
mv /opt/telegraf-1.18.0/usr/bin/telegraf /opt/telegraf/telegraf
chmod 777 /opt/telegraf/telegraf

# Install fluent-bit
wget -qO - https://packages.fluentbit.io/fluentbit.key | sudo apt-key add -
sudo echo "deb https://packages.fluentbit.io/ubuntu/xenial xenial main" >> /etc/apt/sources.list
sudo echo "deb http://security.ubuntu.com/ubuntu bionic-security main" >> /etc/apt/sources.list.d/bionic.list
sudo apt-get update
#sudo apt-get install td-agent-bit=1.6.8 -y


sudo apt --fix-broken install -y
sudo apt-get install inotify-tools -y


# Some dependencies were fixed with sudo apt --fix-broken, try installing td-agent-bit again
# This is because we are keeping the same fluentbit version but have upgraded ubuntu
sudo apt-get install td-agent-bit=1.6.8 -y

#cleanup all install
rm -f $TMPDIR/metricsext2*.deb
rm -f $TMPDIR/prometheus-2.25.2.linux-amd64.tar.gz
rm -rf $TMPDIR/prometheus-2.25.2.linux-amd64
rm -f $TMPDIR/telegraf*.gz
rm -rf $TMPDIR/telegraf-1.18.0/
