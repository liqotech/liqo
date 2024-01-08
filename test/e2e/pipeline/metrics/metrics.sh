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

# shellcheck source=../utils.sh
source "$WORKDIR/../utils.sh"

for i in $(seq 1 "${CLUSTER_NUMBER}")
do
  for j in $(seq 1 "${CLUSTER_NUMBER}")
  do
    if [ "$i" -ne "$j" ]
    then
      export KUBECONFIG="${TMPDIR}/kubeconfigs/liqo_kubeconf_${i}"
      if ! waitandretry 5s 12 "$KUBECTL top node liqo-cluster-${j}";
      then
          echo "Failed to get metrics from liqo-cluster-${j} in cluster liqo-cluster-${i}"
          exit 1
      fi
    fi
  done
done
