#!/bin/bash

# This scripts expects the following variables to be set:
# CLUSTER_NUMBER        -> the number of liqo clusters
# NAMESPACE             -> the namespace where liqo is running
# LIQO_VERSION          -> the liqo version to test
# K8S_VERSION           -> the Kubernetes version
# TMPDIR                -> the directory where kubeconfigs are stored

set -e           # Fail in case of error
set -o nounset   # Fail if undefined variables are used
set -o pipefail  # Fail if one of the piped commands fails

error() {
   local sourcefile=$1
   local lineno=$2
   echo "An error occurred at $sourcefile:$lineno."
}
trap 'error "${BASH_SOURCE}" "${LINENO}"' ERR

LIQO_VERSION="${LIQO_VERSION:-$(git rev-parse HEAD)}"

for i in $(seq 1 "${CLUSTER_NUMBER}")
do
  export KUBECONFIG="${TMPDIR}/kubeconfigs/liqo_kubeconf_${i}"
  go run ./cmd/telemetry/main.go --liqo-version "${LIQO_VERSION}" --kubernetes-version "${K8S_VERSION}" --dry-run
done
