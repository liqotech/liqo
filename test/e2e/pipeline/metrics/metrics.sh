#!/bin/bash
#shellcheck disable=SC1091

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

FILEPATH=$(realpath "$0")
WORKDIR=$(dirname "$FILEPATH")

# shellcheck disable=SC1091
# shellcheck source=../utils.sh
source "$WORKDIR/../utils.sh"

export KUBECONFIG="${TMPDIR}/kubeconfigs/liqo_kubeconf_1" # consumer cluster

for i in $(seq 2 "${CLUSTER_NUMBER}")
do
  CLUSTER_NAME=$(forge_clustername "${i}")
  if ! waitandretry 5s 12 "$KUBECTL top node ${CLUSTER_NAME}";
  then
      echo "Failed to get metrics from virtual node ${CLUSTER_NAME}"
      exit 1
  fi
done
