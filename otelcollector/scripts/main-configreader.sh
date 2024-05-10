#!/bin/bash

# Run logging utility
source /opt/logger.sh

#Run inotify as a daemon to track changes to the mounted configmap.
touch /opt/inotifyoutput.txt
inotifywait /etc/config/settings --daemon --recursive --outfile "/opt/inotifyoutput.txt" --event create,delete --format '%e : %T' --timefmt '+%s'

# Run ARC EULA utility
source /opt/arc-eula.sh

echo_var "MODE" "$MODE"
echo_var "CONTAINER_TYPE" "$CONTAINER_TYPE"
echo_var "CLUSTER" "$CLUSTER"

# Run configmap parser utility
source /opt/configmap-parser.sh

#start cron daemon for logrotate
/usr/sbin/crond -n -s &


# Run configreader to update the configmap for TargetAllocator
if [ "$AZMON_USE_DEFAULT_PROMETHEUS_CONFIG" = "true" ] && [ -e "/opt/microsoft/otelcollector/collector-config-default.yml" ] ; then
      echo_warning "Running config reader with only default scrape configs enabled"
      /opt/configurationreader --config /opt/microsoft/otelcollector/collector-config-default.yml
elif [ -e "/opt/microsoft/otelcollector/collector-config.yml" ]; then
      echo_warning "Running config reader with merged default and custom scrape config via configmap"
      /opt/configurationreader --config /opt/microsoft/otelcollector/collector-config.yml
else
      echo_warning "No configs found via configmap, not running config reader"
fi

# Get ruby version
RUBY_VERSION=`ruby --version`
echo_var "RUBY_VERSION" "$RUBY_VERSION"

# Get golang version
GOLANG_VERSION=`cat /opt/goversion.txt`
echo_var "GOLANG_VERSION" "$GOLANG_VERSION"

shutdown() {
	echo "shutting down"
}

trap "shutdown" SIGTERM

sleep inf & wait