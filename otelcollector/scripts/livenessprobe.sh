echo "test log"

if [ "${MAC}" == "true" ]; then
    # Checking if metricsextension folder exists, if it doesn't, it means that there is no DCR/DCE config for this resource and ME/MDSD will fail to start
    # To avoid the pods from going into crashloopbackoff, we are restarting the pod with this message every 15 minutes.
    if [ ! -d /etc/mdsd.d/config-cache/metricsextension ]; then
        epochTimeNow=`date +%s`
        duration=$((epochTimeNow - $AZMON_CONTAINER_START_TIME))
        durationInMinutes=$(($duration / 60))
        # Checking if 15 minutes have elapsed since container start, so that absence of configuration doesn't result in crashloopbackup which will flag the pods in AKS
        if $durationInMinutes > 5; then
            echo "No configuration present for the AKS resource" > /dev/termination-log
            exit 1
        fi
    else
        # Check if ME is not running, despite existing configuration 
        (ps -ef | grep MetricsExt | grep -v "grep")
        if [ $? -ne 0 ]
        then
            # If DCR/DCE config exists, ME is not running because of other issues, exiting
            echo "Metrics Extension is not running (configuration exists)" > /dev/termination-log
            exit 1
        fi

        # Check if MDSD is not running, despite existing configuration
        # Excluding MetricsExtenstion and inotifywait too since grep returns ME and inotify processes since mdsd is in the config file path
        (ps -ef | grep "mdsd" | grep -vE 'grep|MetricsExtension|inotifywait')
        if [ $? -ne 0 ]
        then
            echo "mdsd is not running (configuration exists)" > /dev/termination-log
            exit 1
        fi
    fi
    # Adding liveness probe check for AMCS config update by MDSD
    if [ -s "/opt/inotifyoutput-mdsd-config.txt" ]  #file exists and size > 0
    then
        echo "inotifyoutput-mdsd-config.txt has been updated - mdsd config changed" > /dev/termination-log
        exit 1
    fi
else
    # Non-MAC mode
    # Check if ME is not running
    (ps -ef | grep MetricsExt | grep -v "grep")
    if [ $? -ne 0 ]
    then
        echo "Metrics Extension is not running" > /dev/termination-log
        exit 1
    fi

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
fi

#test to exit non zero value if otelcollector is not running (applies for both MAC and non MAC mode)
(ps -ef | grep otelcollector | grep -v "grep")
if [ $? -ne 0 ]
then
    echo "OpenTelemetryCollector is not running" > /dev/termination-log
    exit 1
fi

#test to exit non zero value if config changed (applies for both MAC and non MAC mode)
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


# if [ "${MAC}" != "true" ]; then
#   # The mounted cert files are modified by the keyvault provider every time it probes for new certs
#   # even if the actual contents don't change. Need to check if actual contents changed.
#   if [ -d "/etc/config/settings/akv" ] && [ -d "/opt/akv-copy/akv" ]
#   then
#     diff -r -q /etc/config/settings/akv /opt/akv-copy/akv
#     if [ $? -ne 0 ]
#     then
#       echo "A Metrics Account certificate has changed" > /dev/termination-log
#       exit 1
#     fi
#   fi
# # else
# #   # MDSD is only running in MAC mode
# #   # Excluding MetricsExtenstion and inotifywait too since grep returns ME and inotify processes since mdsd is in the config file path
# #   (ps -ef | grep "mdsd" | grep -vE 'grep|MetricsExtension|inotifywait')
# #   if [ $? -ne 0 ]
# #   then
# #     echo "mdsd is not running" > /dev/termination-log
# #     exit 1
# #   fi
# fi

# Adding liveness probe check for AMCS config update by MDSD
# if [ -s "/opt/inotifyoutput-mdsd-config.txt" ]  #file exists and size > 0
# then
#   echo "inotifyoutput-mdsd-config.txt has been updated - mdsd config changed" > /dev/termination-log
#   exit 1
# fi


# if [ ! -s "/opt/inotifyoutput.txt" ] #file doesn't exists or size == 0
# then
#   exit 0
# else
#   if [ -s "/opt/inotifyoutput.txt" ]  #file exists and size > 0
#   then
#     echo "inotifyoutput.txt has been updated - config changed" > /dev/termination-log
#     exit 1
#   fi
# fi



