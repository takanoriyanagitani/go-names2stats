#!/bin/sh

export ENV_ROOT_DIR_NAME=.

ls \
	-f \
	. |
	fgrep -v .. |
	./names2stats2jsonl |
	jq -c
