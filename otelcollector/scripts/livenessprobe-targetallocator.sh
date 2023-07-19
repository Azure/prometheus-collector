#test to exit non zero value if config changed
if [ -s "/opt/inotifyoutput.txt" ]  #file exists and size > 0
then
    echo "inotifyoutput.txt has been updated - config changed" > /dev/termination-log
    exit 1
fi

# Making a call to the localhost to check for metrics and trigger restart for a non 200 response code
RET=`curl --max-time 10 -s -o /dev/null -w "%{http_code}" http://localhost:8080/metrics`
if [ $RET -ge 200 ]; then
    exit 0
else
    echo "curl request to http://localhost:8080/metrics returned a non 200 reponse code" > /dev/termination-log
    exit 1
fi