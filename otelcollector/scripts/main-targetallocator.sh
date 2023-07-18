#!/bin/bash

#Run inotify as a daemon to track changes to the mounted configmap.
touch /opt/inotifyoutput.txt
inotifywait /etc/config/settings --daemon --recursive --outfile "/opt/inotifyoutput.txt" --event create,delete --format '%e : %T' --timefmt '+%s'

# Run Targetallocator
/opt/targetallocator
if [ $? -ne 0 ] ; then
# Write to inotify and configure restart
fi

shutdown() {
	echo "shutting down"
}

trap "shutdown" SIGTERM

sleep inf & wait