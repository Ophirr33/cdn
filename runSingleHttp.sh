#!/bin/bash

PORT=""
ORIGIN=""
DOMAIN=""
USER=""
IDENTITY=""

while getopts "p:o:u:i:d:" opt; do
  case $opt in
    p)
      PORT=$OPTARG
      ;;
    o)
      ORIGIN=$OPTARG
      ;;
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
# ssh $USER@$DOMAIN -n -i $IDENTITY -o StrictHostKeyChecking=no "cd gilpin-project5 && nohup sh -c \"( ( exec -a HTTP_SERVER ./httpserver -p $PORT -o $ORIGIN &> /dev/null 2>&1 ) & )\""
ssh $USER@$DOMAIN -n -i $IDENTITY -o StrictHostKeyChecking=no "cd gilpin-project5 && exec -a HTTP_SERVER ./httpserver -p $PORT -o $ORIGIN > /dev/null 2>&1 &"

