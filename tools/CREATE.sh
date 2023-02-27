#!/bin/sh

registry rpc admin create-project --project_id menagerie \
	--project.display_name "Menagerie of APIs" \
	--project.description "A curated collection of APIs." \
	>& /dev/null

registry config set registry.project menagerie
