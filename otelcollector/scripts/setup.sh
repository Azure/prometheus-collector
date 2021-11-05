#!/bin/bash

TMPDIR="/opt"
cd $TMPDIR

sed -i -e 's/# en_US.UTF-8 UTF-8/en_US.UTF-8 UTF-8/' /etc/locale.gen && \
    dpkg-reconfigure --frontend=noninteractive locales && \
    update-locale LANG=en_US.UTF-8

wget https://github.com/microsoft/Docker-Provider/releases/download/04012021/metricsext2_2.2021.901.1511-69f7bf-_focal_amd64.deb

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

# Build essential, libre2-dev and ruby-dev packages are required to install re2 gem
# sudo apt install -y build-essential
# sudo apt-get install -y libre2-dev
# sudo apt-get install -y ruby-dev

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

# Install Telegraf
wget https://dl.influxdata.com/telegraf/releases/telegraf-1.19.1_linux_amd64.tar.gz
tar -zxvf telegraf-1.19.1_linux_amd64.tar.gz
mv /opt/telegraf-1.19.1/usr/bin/telegraf /opt/telegraf/telegraf
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

# setup hourly cron for logrotate
cp /etc/cron.daily/logrotate /etc/cron.hourly/

#cleanup all install
rm -f $TMPDIR/metricsext2*.deb
rm -f $TMPDIR/prometheus-2.25.2.linux-amd64.tar.gz
rm -rf $TMPDIR/prometheus-2.25.2.linux-amd64
rm -f $TMPDIR/telegraf*.gz
rm -rf $TMPDIR/telegraf-1.19.1/
