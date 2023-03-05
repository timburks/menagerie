#!/bin/sh

registry rpc admin update-project --project.name projects/menagerie \
	--project.display_name "Menagerie of APIs" \
	--project.description "APIs collected from a variety of sources." \
	--allow_missing \
	>& /dev/null

registry config set registry.project menagerie
