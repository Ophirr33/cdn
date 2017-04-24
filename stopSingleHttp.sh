#!/bin/bash

DOMAIN=""
USER=""
IDENTITY=""

while getopts "u:i:d:" opt; do
  case $opt in
    d)
      DOMAIN=$OPTARG
      ;;
    u)
      USER=$OPTARG
      ;;
    i)
      IDENTITY=$OPTARG
      ;;
    \?)
      echo "INVALID ARGUMENT"
      exit 1
      ;;
  esac
done
# ========== HTTP SERVERS ==========
ssh $USER@$DOMAIN -i $IDENTITY -o StrictHostKeyChecking=no "cd gilpin-project5 && pkill -2 -f HTTP_SERVER"

