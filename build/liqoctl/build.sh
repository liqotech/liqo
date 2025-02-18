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

GO_VERSION="1.24"

CGO_ENABLED="${CGO_ENABLED:-0}"
GOOS="${GOOS:-linux}"
GOARCH="${GOARCH:-amd64}"
LIQOCTLVERSION="${LIQOCTLVERSION:-dev}"

docker run -v "$PWD:/liqo" -w /liqo -e="CGO_ENABLED=${CGO_ENABLED}" \
   -e "GOOS=${GOOS}" -e "GOARCH=${GOARCH}" \
   -e "LIQOCTLVERSION=${LIQOCTLVERSION}" \
   --rm "golang:${GO_VERSION}" \
   sh -c "go mod tidy && go build -o \"./liqoctl-${GOOS}-${GOARCH}\" \
   -ldflags=\"-s -w -X 'github.com/liqotech/liqo/pkg/liqoctl/version.LiqoctlVersion=${LIQOCTLVERSION}'\" \
   -buildvcs=false \
   ./cmd/liqoctl"
