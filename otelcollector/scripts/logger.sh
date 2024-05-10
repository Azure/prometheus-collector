#!/bin/bash

# Colors for Logging
Color_Off='\033[0m'
Red='\033[0;31m'
Green='\033[0;32m'
Yellow='\033[0;33m'
Cyan='\033[0;36m'

# Echo text in red
echo_error () {
  echo -e "${Red}$1${Color_Off}"
}

# Echo text in yellow
echo_warning () {
  echo -e "${Yellow}$1${Color_Off}"
}

# Echo variable name in Cyan and value in regular color
echo_var () {
  echo -e "${Cyan}$1${Color_Off}=$2"
}