#!/bin/bash

USER=""
IDENTITY=""
DOMAIN=""
CDN="cs5700cdnproject.ccs.neu.edu"

while getopts "u:i:d:" opt; do
  case $opt in
    u)
      USER=$OPTARG
      ;;
    i)
      IDENTITY=$OPTARG
      ;;
    d)
      DOMAIN=$OPTARG
      ;;
    \?)
      echo "INVALID ARGUMENT"
      exit 1
      ;;
  esac
done

# CLEAN UP
ssh $USER@$DOMAIN -i $IDENTITY -o StrictHostKeyChecking=no 'rm -rf gilpin-project5 && mkdir gilpin-project5' &&

# scp source files
  scp -i $IDENTITY src/cdn/httpserver/* popular_to_text.py Makefile-HTTP $USER@$DOMAIN:gilpin-project5 &&

# scp popular_raw.html into HTTP Server
  scp -3i $IDENTITY $USER@$CDN:/course/cs5700sp17/popular_raw.html $USER@$DOMAIN:gilpin-project5 &&

# Make HTTP Server binary and clean up
  ssh $USER@$DOMAIN -i $IDENTITY 'cd gilpin-project5 && mv Makefile-HTTP Makefile && python popular_to_text.py && rm *.html *.py && make && rm *.go Makefile'
