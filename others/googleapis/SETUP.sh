#!/bin/sh

# Clone the Google APIs repository.
if [ ! -d "googleapis" ]
then
  git clone https://github.com/googleapis/googleapis
else
  echo "Using previous clone of googleapis."
fi

# Create the project if it doesn't already exist.
registry rpc admin update-project \
	--project.name projects/googleapis \
	--project.display_name "Public Google APIs" \
	--project.description "Google API descriptions from public Protocol Buffer descriptions and the API Discovery Service." \
	--allow_missing \
	--json
	>& /dev/null

# Configure the registry tool.
registry config set registry.project googleapis

# Get the commit hash of the checked-out OpenAPI directory
export COMMIT=`(cd googleapis; git rev-parse HEAD)`

# Upload all of the APIs in the googleapis directory.
registry upload protos googleapis \
	--base-uri https://github.com/googleapis/googleapis/blob/$COMMIT 

# Upload all of the APIs listed by Google's API Discovery Service.
registry upload discovery
