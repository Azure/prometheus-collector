#!/bin/bash

# Change to the utils directory
cd utils || exit
# Update Go packages and tidy up
go get -u && go mod tidy
# Return to the parent directory
cd ..

# List all directories
for dir in */; do
    # Check if it is a directory
    if [ -d "$dir" ]; then
        # Change to the directory
        cd "$dir" || exit
        # Update Go packages and tidy up
        go get -u && go mod tidy
        # Return to the parent directory
        cd ..
    fi
done