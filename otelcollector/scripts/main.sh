#!/bin/bash

# Run logging utility
source /opt/logger.sh

#Run inotify as a daemon to track changes to the mounted configmap.
touch /opt/inotifyoutput.txt
inotifywait /etc/config/settings /etc/prometheus/certs --daemon --recursive --outfile "/opt/inotifyoutput.txt" --event create,delete --format '%e : %T' --timefmt '+%s'

# Run ARC EULA utility
source /opt/arc-eula.sh

if [ -z $MODE ]; then
  MODE="simple"
fi
echo_var "MODE" "$MODE"
echo_var "CONTROLLER_TYPE" "$CONTROLLER_TYPE"
echo_var "CLUSTER" "$CLUSTER"

customEnvironment_lower=$(echo "$customEnvironment" | tr '[:upper:]' '[:lower:]')
if [ "$customEnvironment_lower" == "azurepubliccloud" ]; then
  encodedaikey="MWNkYTMxMTItYWY1Ni00ZmNiLWI4MDQtZjg5NDVhYTFjYjMy"
elif [ "$customEnvironment_lower" == "azureusgovernment" ]; then
  # encodedaikey="ZmRjMTE0MmUtY2U0YS1mNTFmLWE4M2EtODBjM2ZjNDYwNGE5"
  # aiendpoint="https://dc.applicationinsights.us/v2/track"
  encodedaikey="OWNmYzNmZDEtMzFiZS1mOWE4LTgzMmYtMjNiYzIzNmQ0MWIy"
  aiendpoint="https://usgovvirginia-1.in.applicationinsights.azure.us/"
elif [ "$customEnvironment_lower" == "azurechinacloud" ]; then
  encodedaikey="ZTcyY2ZjOTYtNjY3Zi1jZGYwLTkwOWMtNzhiZjAwZjQ0NDg4"
  aiendpoint="https://dc.applicationinsights.azure.cn/v2/track"
# elif [ "$customEnvironment_lower" == "usnat" ]; then
#   encodedaikey="usnat key"
# elif [ "$customEnvironment_lower" == "ussec" ]; then
#   encodedaikey="ussec key"
else
    echo "Unknown customEnvironment: $customEnvironment_lower, setting telemetry output to the default azurepubliccloud instance"
    encodedaikey="MWNkYTMxMTItYWY1Ni00ZmNiLWI4MDQtZjg5NDVhYTFjYjMy"
fi

export APPLICATIONINSIGHTS_AUTH=$encodedaikey
echo "export APPLICATIONINSIGHTS_AUTH=$encodedaikey" >> ~/.bashrc
if [ -n "$aiendpoint" ]; then
    export APPLICATIONINSIGHTS_ENDPOINT="$aiendpoint"
    echo "export APPLICATIONINSIGHTS_ENDPOINT=\"$aiendpoint\"" >> ~/.bashrc
fi
source ~/.bashrc

#get controller kind in lowercase, trimmed
controllerType=$(echo $CONTROLLER_TYPE | tr "[:upper:]" "[:lower:]" | xargs)

# If using a trusted CA for HTTP Proxy, copy this over from the node and install
cp /anchors/ubuntu/* /etc/pki/ca-trust/source/anchors 2>/dev/null
cp /anchors/mariner/* /etc/pki/ca-trust/source/anchors 2>/dev/null
cp /anchors/proxy/* /etc/pki/ca-trust/source/anchors 2>/dev/null
update-ca-trust

# These env variables are populated by AKS in every kube-system pod
# Remove ending '/' character since mdsd doesn't recognize this as a valid url
if [ "$http_proxy" != "" ] && [ "${http_proxy: -1}" == "/" ]; then
  export http_proxy=${http_proxy::-1}
fi
if [ "$HTTP_PROXY" != "" ] && [ "${HTTP_PROXY: -1}" == "/" ]; then
 export HTTP_PROXY=${HTTP_PROXY::-1}
fi
if [ "$https_proxy" != "" ] && [ "${https_proxy: -1}" == "/" ]; then
  export https_proxy=${https_proxy::-1}
fi
if [ "$HTTPS_PROXY" != "" ] && [ "${HTTPS_PROXY: -1}" == "/" ]; then
  export HTTPS_PROXY=${HTTPS_PROXY::-1}
fi

# Add our target-allocator service to the no_proxy env variable
export NO_PROXY=$NO_PROXY,ama-metrics-operator-targets.kube-system.svc.cluster.local
echo "export NO_PROXY=$NO_PROXY" >> ~/.bashrc
export no_proxy=$no_proxy,ama-metrics-operator-targets.kube-system.svc.cluster.local
echo "export no_proxy=$no_proxy" >> ~/.bashrc

# If HTTP Proxy is enabled, HTTP_PROXY will always have a value.
# HTTPS_PROXY will be set to same value as HTTP_PROXY if not specified.
export HTTP_PROXY_ENABLED="false"
if [ "$HTTP_PROXY" != "" ]; then
  export HTTP_PROXY_ENABLED="true"
fi
echo "export HTTP_PROXY_ENABLED=$HTTP_PROXY_ENABLED" >> ~/.bashrc

if [ $IS_ARC_CLUSTER == "true" ] && [ $HTTP_PROXY_ENABLED == "true" ]; then
  proxyprotocol="$(echo $HTTPS_PROXY | grep :// | sed -e's,^\(.*://\).*,\1,g')"
  proxyprotocol=$(echo $proxyprotocol | tr "[:upper:]" "[:lower:]")
  if [ "$proxyprotocol" != "http://" -a "$proxyprotocol" != "https://" ]; then
    echo_error "HTTP Proxy specified does not include http:// or https://"
  fi

  url="$(echo ${HTTPS_PROXY/$proxyprotocol/})"
  creds="$(echo $url | grep @ | cut -d@ -f1)"
  user="$(echo $creds | cut -d':' -f1)"
  password="$(echo $creds | cut -d':' -f2)"
  hostport="$(echo ${url/$creds@/} | cut -d/ -f1)"
  host="$(echo $hostport | sed -e 's,:.*,,g')"
  if [ -z "$host" ]; then
    echo_error "HTTP Proxy specified does not include a host"
  fi
  echo $password | base64 > /opt/microsoft/proxy_password
  export MDSD_PROXY_MODE=application
  echo "export MDSD_PROXY_MODE=$MDSD_PROXY_MODE" >> ~/.bashrc
  export MDSD_PROXY_ADDRESS=$proxyprotocol$hostport
  echo "export MDSD_PROXY_ADDRESS=$MDSD_PROXY_ADDRESS" >> ~/.bashrc
  if [ ! -z "$user" -a ! -z "$password" ]; then
    export MDSD_PROXY_USERNAME=$user
    echo "export MDSD_PROXY_USERNAME=$MDSD_PROXY_USERNAME" >> ~/.bashrc
    export MDSD_PROXY_PASSWORD_FILE=/opt/microsoft/proxy_password
    echo "export MDSD_PROXY_PASSWORD_FILE=$MDSD_PROXY_PASSWORD_FILE" >> ~/.bashrc
  fi
fi

source /opt/configmap-parser.sh

#start cron daemon for logrotate
/usr/sbin/crond -n -s &


if [ $controllerType = "replicaset" ]; then
   fluentBitConfigFile="/opt/fluent-bit/fluent-bit.conf"
   if [ "$CLUSTER_OVERRIDE" = "true" ]; then
      meConfigFile="/usr/sbin/me_internal.config"
   else
      meConfigFile="/usr/sbin/me.config"
   fi
else
   fluentBitConfigFile="/opt/fluent-bit/fluent-bit-daemonset.conf"
   if [ "$CLUSTER_OVERRIDE" = "true" ]; then
      meConfigFile="/usr/sbin/me_ds_internal.config"
   else
      meConfigFile="/usr/sbin/me_ds.config"
   fi
fi

if [ "${MAC}" == "true" ]; then
      #wait for addon-token-adapter to be healthy
      tokenAdapterWaitsecs=60
      waitedSecsSoFar=1
      while true; do
            if [ $waitedSecsSoFar -gt $tokenAdapterWaitsecs ]; then
                  wget -T 2 -S http://localhost:9999/healthz -Y off 2>&1
                  echo "giving up waiting for token adapter to become healthy after $waitedSecsSoFar secs"
                  # log telemetry about failure after waiting for waitedSecsSoFar and break
                  echo "export tokenadapterUnhealthyAfterSecs=$waitedSecsSoFar" >>~/.bashrc
                  break
            else
                  echo "checking health of token adapter after $waitedSecsSoFar secs"
                  tokenAdapterResult=$(wget -T 2 -S http://localhost:9999/healthz -Y off 2>&1| grep HTTP/|awk '{print $2}'| grep 200)
            fi
            if [ ! -z $tokenAdapterResult ]; then
                        echo "found token adapter to be healthy after $waitedSecsSoFar secs"
                        # log telemetry about success after waiting for waitedSecsSoFar and break
                        echo "export tokenadapterHealthyAfterSecs=$waitedSecsSoFar" >>~/.bashrc
                        break
            fi
            sleep 1
            waitedSecsSoFar=$(($waitedSecsSoFar + 1))
      done
      source ~/.bashrc
      #end wait for addon-token-adapter to be healthy
fi

export ME_CONFIG_FILE=$meConfigFile
export FLUENT_BIT_CONFIG_FILE=$fluentBitConfigFile
echo "export ME_CONFIG_FILE=$meConfigFile" >> ~/.bashrc
echo "export FLUENT_BIT_CONFIG_FILE=$fluentBitConfigFile" >> ~/.bashrc
source ~/.bashrc
echo_var "ME_CONFIG_FILE" "$ME_CONFIG_FILE"

if [ "${MAC}" != "true" ]; then
      if [ -z $CLUSTER ]; then
            echo "CLUSTER is empty or not set. Using $NODE_NAME as CLUSTER"
            export customResourceId=$NODE_NAME
            echo "export customResourceId=$NODE_NAME" >> ~/.bashrc
            source ~/.bashrc
      else
            export customResourceId=$CLUSTER
            echo "export customResourceId=$CLUSTER" >> ~/.bashrc
            source ~/.bashrc
      fi

      # Make a copy of the mounted akv directory to see if it changes
      mkdir -p /opt/akv-copy
      cp -r /etc/config/settings/akv /opt/akv-copy

      decodeLocation="/opt/akv/decoded"
      # secrets can only be alpha numeric chars and dashes
      ENCODEDFILES=/etc/config/settings/akv/*
      mkdir -p $decodeLocation
      for ef in $ENCODEDFILES
      do
            name="$(basename -- $ef)"
            base64 -d $ef > $decodeLocation/$name
      done

      DECODEDFILES=$decodeLocation/*
      decodedFiles=""
      for df in $DECODEDFILES
      do
            if [ ${#decodedFiles} -ge 1 ]; then
                  decodedFiles=$decodedFiles:$df
            else
                  decodedFiles=$df
            fi
      done

      export AZMON_METRIC_ACCOUNTS_AKV_FILES=$(echo $decodedFiles)
      echo "export AZMON_METRIC_ACCOUNTS_AKV_FILES=$decodedFiles" >> ~/.bashrc
      source ~/.bashrc

      echo_var "AKV_FILES" "$AZMON_METRIC_ACCOUNTS_AKV_FILES"

      echo "Starting metricsextension"
      # will need to rotate the entire log location
      # will need to remove accountname fetching from env
      # Logs at level 'Info' to get metrics processed count. Fluentbit and out_appinsights filter the logs to only send errors and the metrics processed count to the telemetry
      /usr/sbin/MetricsExtension -Logger File -LogLevel Info -DataDirectory /opt/MetricsExtensionData -Input otlp_grpc_prom -PfxFile $AZMON_METRIC_ACCOUNTS_AKV_FILES -MonitoringAccount $AZMON_DEFAULT_METRIC_ACCOUNT_NAME -ConfigOverridesFilePath $ME_CONFIG_FILE $ME_ADDITIONAL_FLAGS > /dev/null &
else
      echo_var "customResourceId" "$CLUSTER"
      export customResourceId=$CLUSTER
      echo "export customResourceId=$CLUSTER" >> ~/.bashrc
      source ~/.bashrc

      trimmedRegion=$(echo $AKSREGION | sed 's/ //g' | awk '{print tolower($0)}')
      export customRegion=$trimmedRegion
      echo "export customRegion=$trimmedRegion" >> ~/.bashrc
      source ~/.bashrc
      echo_var "customRegion" "$trimmedRegion"

      echo "Waiting for 10s for token adapter sidecar to be up and running so that it can start serving IMDS requests"
      # sleep for 10 seconds
      sleep 10

      echo "Setting env variables from envmdsd file for MDSD"
      cat /etc/mdsd.d/envmdsd | while read line; do
            echo $line >> ~/.bashrc
      done
      source /etc/mdsd.d/envmdsd
      echo "Starting MDSD"
      # Use options -T 0x1 or -T 0xFFFF for debug logging
      mdsd -a -A -e ${MDSD_LOG}/mdsd.err -w ${MDSD_LOG}/mdsd.warn -o ${MDSD_LOG}/mdsd.info -q ${MDSD_LOG}/mdsd.qos 2>> /dev/null &

      # Running mdsd --version can't be captured into a variable unlike telegraf and otelcollector, have to run after printing the string
      echo -n -e "${Cyan}MDSD_VERSION${Color_Off}="; mdsd --version

      echo "Waiting for 30s for MDSD to get the config and put them in place for ME"
      # sleep for 30 seconds
      sleep 30

      echo "Reading me config file as a string for configOverrides paramater"
      export meConfigString=`cat $ME_CONFIG_FILE | tr '\r' ' ' |  tr '\n' ' ' | sed 's/\"/\\"/g' | sed 's/ //g'`
      echo "Starting metricsextension"
      /usr/sbin/MetricsExtension -Logger File -LogLevel Info -LocalControlChannel -TokenSource AMCS -DataDirectory /etc/mdsd.d/config-cache/metricsextension -Input otlp_grpc_prom -ConfigOverrides $meConfigString > /dev/null &
fi

# Get ME version
ME_VERSION=`cat /opt/metricsextversion.txt`
echo_var "ME_VERSION" "$ME_VERSION"

# Get ruby version
RUBY_VERSION=`ruby --version`
echo_var "RUBY_VERSION" "$RUBY_VERSION"

# Get golang version
GOLANG_VERSION=`cat /opt/goversion.txt`
echo_var "GOLANG_VERSION" "$GOLANG_VERSION"

# Start otelcollector
if [ $controllerType = "replicaset" ] && [ "${AZMON_OPERATOR_ENABLED}" == "true" ]; then
      echo_warning "Starting otelcollector in replicaset with Target allocator settings"
      /opt/microsoft/otelcollector/otelcollector --config /opt/microsoft/otelcollector/collector-config-replicaset.yml &> /opt/microsoft/otelcollector/collector-log.txt &
elif [ "$AZMON_USE_DEFAULT_PROMETHEUS_CONFIG" = "true" ]; then
      # Commenting this out since config can be applied via CRD
      # echo_warning "Starting otelcollector with only default scrape configs enabled"
      /opt/microsoft/otelcollector/otelcollector --config /opt/microsoft/otelcollector/collector-config-default.yml &> /opt/microsoft/otelcollector/collector-log.txt &
else
      echo "Starting otelcollector"
      /opt/microsoft/otelcollector/otelcollector --config /opt/microsoft/otelcollector/collector-config.yml &> /opt/microsoft/otelcollector/collector-log.txt &
fi
OTELCOLLECTOR_VERSION=`/opt/microsoft/otelcollector/otelcollector --version`
echo_var "OTELCOLLECTOR_VERSION" "$OTELCOLLECTOR_VERSION"
PROMETHEUS_VERSION=`cat /opt/microsoft/otelcollector/PROMETHEUS_VERSION`
echo_var "PROMETHEUS_VERSION" "$PROMETHEUS_VERSION"

echo "starting telegraf"
if [ "$TELEMETRY_DISABLED" != "true" ]; then
  if [ "$CONTROLLER_TYPE" == "ReplicaSet" ]  && [ "${AZMON_OPERATOR_ENABLED}" == "true" ]; then
    /usr/bin/telegraf --config /opt/telegraf/telegraf-prometheus-collector-ta-enabled.conf &
  elif [ "$CONTROLLER_TYPE" == "ReplicaSet" ]; then
    /usr/bin/telegraf --config /opt/telegraf/telegraf-prometheus-collector.conf &
  else
    /usr/bin/telegraf --config /opt/telegraf/telegraf-prometheus-collector-ds.conf &
  fi
  TELEGRAF_VERSION=`cat /opt/telegrafversion.txt`
  echo_var "TELEGRAF_VERSION" "$TELEGRAF_VERSION"
fi

echo "starting fluent-bit"
mkdir /opt/microsoft/fluent-bit
touch /opt/microsoft/fluent-bit/fluent-bit-out-appinsights-runtime.log
fluent-bit -c $FLUENT_BIT_CONFIG_FILE -e /opt/fluent-bit/bin/out_appinsights.so &
FLUENT_BIT_VERSION=`fluent-bit --version`
echo_var "FLUENT_BIT_VERSION" "$FLUENT_BIT_VERSION"
echo_var "FLUENT_BIT_CONFIG_FILE" "$FLUENT_BIT_CONFIG_FILE"

if [ "${MAC}" == "true" ]; then
  # Run inotify as a daemon to track changes to the dcr/dce config folder and restart container on changes, so that ME can pick them up.
  echo "starting inotify for watching mdsd config update"
  touch /opt/inotifyoutput-mdsd-config.txt
  inotifywait /etc/mdsd.d/config-cache/metricsextension/TokenConfig.json --daemon --outfile "/opt/inotifyoutput-mdsd-config.txt" --event ATTRIB --format '%e : %T' --timefmt '+%s'
fi

# Setting time at which the container started running, so that it can be used for empty configuration checks in livenessprobe
epochTimeNow=`date +%s`
echo $epochTimeNow > /opt/microsoft/liveness/azmon-container-start-time
echo_var "AZMON_CONTAINER_START_TIME" "$epochTimeNow"
epochTimeNowReadable=`date --date @$epochTimeNow`
echo_var "AZMON_CONTAINER_START_TIME_READABLE" "$epochTimeNowReadable"

shutdown() {
	echo "shutting down"
}

trap "shutdown" SIGTERM

sleep inf & wait
