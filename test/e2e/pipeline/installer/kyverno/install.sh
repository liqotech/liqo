#!/bin/bash
#shellcheck disable=SC1091

# This scripts expects the following variables to be set:
# CLUSTER_NUMBER        -> the number of liqo clusters
# TMPDIR                -> the directory where the test-related files are stored
# KUBECTL               -> the path where kubectl is stored
# HELM                  -> the path where helm is stored

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
# shellcheck source=../../utils.sh
source "$WORKDIR/../../utils.sh"

# Install needed utilities
PIDS=()

for i in $(seq 1 "${CLUSTER_NUMBER}"); do
	# Install kyverno for network tests
  	install_kyverno "${TMPDIR}/kubeconfigs/liqo_kubeconf_${i}" &
   PIDS+=($!)
done

for PID in "${PIDS[@]}"; do
    wait "${PID}"
done

for i in $(seq 1 "${CLUSTER_NUMBER}"); do
   # Wait for kyverno to be ready
   wait_kyverno "${TMPDIR}/kubeconfigs/liqo_kubeconf_${i}"
done
