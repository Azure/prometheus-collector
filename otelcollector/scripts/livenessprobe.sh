#test to exit non zero value if otelcollector or ME is not running or config changed

(ps -ef | grep otelcollector | grep -v "grep")
if [ $? -ne 0 ]
then
  echo "OpenTelemetryCollector is not running" > /dev/termination-log
  exit 1
fi

(ps -ef | grep MetricsExt | grep -v "grep")
if [ $? -ne 0 ]
then
  # Checking if metricsextension folder exists, if it doesn't, it means that there is no DCR/DCE config for this resource and ME will fail to start
  if [ ! -d /etc/mdsd.d/config-cache/metricsextension ]; then
    epochTimeNow=`date +%s`
    duration=$((epochTimeNow - $AZMON_CONTAINER_START_TIME))
    durationInMinutes=$(($duration / 60))
    # Checking if 15 minutes have elapsed between checks, so that no configuration doesn't result in crashloopbackup which will flag the pods in AKS
    if (( $durationInMinutes % 5 == 0 )); then
      echo "Metrics Extension is not running (no configuration)" > /dev/termination-log
      exit 1
    fi
  else
    # If DCR/DCE config exists, ME is not running because of other issues, exiting
    echo "Metrics Extension is not running (configuration exists)" > /dev/termination-log
    exit 1
  fi
fi

if [ "${MAC}" != "true" ]; then
  # The mounted cert files are modified by the keyvault provider every time it probes for new certs
  # even if the actual contents don't change. Need to check if actual contents changed.
  if [ -d "/etc/config/settings/akv" ] && [ -d "/opt/akv-copy/akv" ]
  then
    diff -r -q /etc/config/settings/akv /opt/akv-copy/akv
    if [ $? -ne 0 ]
    then
      echo "A Metrics Account certificate has changed" > /dev/termination-log
      exit 1
    fi
  fi
else
  # MDSD is only running in MAC mode
  # Excluding MetricsExtenstion and inotifywait too since grep returns ME and inotify processes since mdsd is in the config file path
  (ps -ef | grep "mdsd" | grep -vE 'grep|MetricsExtension|inotifywait')
  if [ $? -ne 0 ]
  then
    echo "mdsd is not running" > /dev/termination-log
    exit 1
  fi
fi

# Adding liveness probe check for AMCS config update by MDSD
if [ -s "/opt/inotifyoutput-mdsd-config.txt" ]  #file exists and size > 0
then
  echo "inotifyoutput-mdsd-config.txt has been updated - mdsd config changed" > /dev/termination-log
  exit 1
fi


if [ ! -s "/opt/inotifyoutput.txt" ] #file doesn't exists or size == 0
then
  exit 0
else
  if [ -s "/opt/inotifyoutput.txt" ]  #file exists and size > 0
  then
    echo "inotifyoutput.txt has been updated - config changed" > /dev/termination-log
    exit 1
  fi
fi



