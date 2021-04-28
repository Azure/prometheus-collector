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
  echo "Metrics Extension is not running" > /dev/termination-log
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

