#!/bin/sh

registry rpc admin create-project --project_id menage \
	--project.display_name "Ménage à APIs" \
	--project.description "A curated collection of APIs." \
	>& /dev/null

registry config set registry.project menage
