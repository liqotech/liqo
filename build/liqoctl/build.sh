#!/bin/bash

set -e          # Fail in case of error
set -o nounset  # Fail if undefined variables are used
set -o pipefail # Fail if one of the piped commands fails

error() {
   local sourcefile=$1
   local lineno=$2
   echo "An error occurred at $sourcefile:$lineno."
}
trap 'error "${BASH_SOURCE}" "${LINENO}"' ERR

CGO_ENABLED="${CGO_ENABLED:-0}"
GOOS="${GOOS:-linux}"
GOARCH="${GOARCH:-amd64}"
LIQOCTLVERSION="${LIQOCTLVERSION:-dev}"

echo "Downloading Go modules..."
start_time=$(date +%s)
go mod download
end_time=$(date +%s)
elapsed=$((end_time - start_time))
echo "Go modules downloaded in ${elapsed} seconds."

CGO_ENABLED=0 go build -o "./liqoctl-${GOOS}-${GOARCH}" \
   -ldflags="-s -w -X 'github.com/liqotech/liqo/pkg/liqoctl/version.LiqoctlVersion=${LIQOCTLVERSION}'" \
   -buildvcs=false \
   ./cmd/liqoctl
