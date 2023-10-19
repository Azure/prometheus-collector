#test to exit non zero value if config changed
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