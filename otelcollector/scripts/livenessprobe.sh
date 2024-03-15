#test to exit non zero value if there is no configuration (MAC mode), otelcollector or ME is not running or config changed
if [ "${MAC}" == "true" ]; then
    # Checking if TokenConfig file exists, if it doesn't, it means that there is no DCR/DCE config for this resource and ME/MDSD will fail to start
    # To avoid the pods from going into crashloopbackoff, we are restarting the pod with this message every 15 minutes.
    # if [ ! -e /etc/mdsd.d/config-cache/metricsextension/TokenConfig.json ]; then
    #     if [ -e /opt/microsoft/liveness/azmon-container-start-time ]; then
    #         epochTimeNow=`date +%s`
    #         azmonContainerStartTime=`cat /opt/microsoft/liveness/azmon-container-start-time`
    #         duration=$((epochTimeNow - $azmonContainerStartTime))
    #         durationInMinutes=$(($duration / 60))
    #          # Logging this every 5 minutes so that it can picked up in traces for telemetry as well as sent to stdout of the container
    #         if (( $durationInMinutes % 5 == 0 )); then
    #             echo "`date "+%Y-%m-%dT%H:%M:%S"` No configuration present for the AKS resource" > /dev/write-to-traces
    #         fi
    #         # Checking if 15 minutes have elapsed since container start, so that absence of configuration doesn't result in crashloopbackup which will flag the pods in AKS
    #         if [ $durationInMinutes -gt 15 ]; then
    #             echo "No configuration present for the AKS resource" > /dev/termination-log
    #             exit 1
    #         fi
    #     fi
    # else
        # Check if ME is not running, despite existing configuration 
        (ps -ef | grep MetricsExt | grep -v "grep")
        if [ $? -ne 0 ]
        then
            # ME is not running , exiting
            echo "Metrics Extension is not running" > /dev/termination-log
            exit 1
        fi

        # Check if MDSD is not running, despite existing configuration
        # Excluding MetricsExtenstion and inotifywait too since grep returns ME and inotify processes since mdsd is in the config file path
        # (ps -ef | grep "mdsd" | grep -vE 'grep|MetricsExtension|inotifywait')
        # if [ $? -ne 0 ]
        # then
        #     echo "mdsd is not running (configuration exists)" > /dev/termination-log
        #     exit 1
        # fi
    # fi
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