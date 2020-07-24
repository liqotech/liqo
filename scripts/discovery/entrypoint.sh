#!/bin/bash

# update CA certificates
update-ca-certificates 2>/dev/null

# start component process
command=$1
shift
exec "$command" "$@"
