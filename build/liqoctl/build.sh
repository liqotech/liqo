#!/bin/bash

set -e           # Fail in case of error
set -o nounset   # Fail if undefined variables are used
set -o pipefail  # Fail if one of the piped commands fails

error() {
   local sourcefile=$1
   local lineno=$2
   echo "An error occurred at $sourcefile:$lineno."
}
trap 'error "${BASH_SOURCE}" "${LINENO}"' ERR

GO_VERSION="1.21"

docker run -v "$PWD:/liqo" -w /liqo -e="CGO_ENABLED=${CGO_ENABLED}" --rm "golang:${GO_VERSION}" \
   sh -c "go mod tidy && go build -o \"./liqoctl-${GOOS}-${GOARCH}\" \
   -ldflags=\"-s -w -X 'github.com/liqotech/liqo/pkg/liqoctl/version.liqoctlVersion=${LIQOCTLVERSION}'\" \
   -buildvcs=false \
   ./cmd/liqoctl"
