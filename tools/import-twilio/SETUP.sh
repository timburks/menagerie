#!/bin/sh

if [ ! -d "twilio-oai" ]
then
  git clone git@github.com:twilio/twilio-oai
else
  echo "Using previous download."
fi

