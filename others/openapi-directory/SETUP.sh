#!/bin/sh

# Clone the OpenAPI Directory repository.
if [ ! -d "openapi-directory" ]
then
  git clone https://github.com/apis-guru/openapi-directory
else
  echo "Using previous clone of openapi-directory."
fi

# Create the project if it doesn't already exist.
registry rpc admin update-project \
	--project.name projects/openapi-directory \
	--project.display_name "OpenAPI Directory" \
	--project.description "OpenAPI descriptions collected by APIs.guru." \
	--allow_missing
	>& /dev/null

# Configure the registry tool.
registry config set registry.project openapi-directory

# Get the commit hash of the checked-out OpenAPI directory
export COMMIT=`(cd openapi-directory; git rev-parse HEAD)`

# Upload all of the APIs in the OpenAPI directory at once.
registry upload openapi \
	openapi-directory/APIs \
	--base-uri https://github.com/APIs-guru/openapi-directory/blob/$COMMIT/APIs 

# Apply common artifacts
registry apply -R -f artifacts

# Compute some metadata
registry-experimental extract apis/-/versions/-/specs
registry-experimental compute summary apis/-
registry-experimental compute summary projects/openapi-directory
registry-experimental count versions apis/-
