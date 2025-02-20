#!/bin/bash

# This scripts expects the following variables to be set:
# CLUSTER_NUMBER        -> the number of liqo clusters
# K8S_VERSION           -> the Kubernetes version
# CNI                   -> the CNI plugin used
# TMPDIR                -> the directory where the test-related files are stored
# BINDIR                -> the directory where the test-related binaries are stored
# TEMPLATE_DIR          -> the directory where to read the cluster templates
# NAMESPACE             -> the namespace where liqo is running
# KUBECONFIGDIR         -> the directory where the kubeconfigs are stored
# LIQO_VERSION          -> the liqo version to test
# INFRA                 -> the Kubernetes provider for the infrastructure
# LIQOCTL               -> the path where liqoctl is stored
# KUBECTL               -> the path where kubectl is stored
# HELM                  -> the path where helm is stored
# POD_CIDR_OVERLAPPING  -> the pod CIDR of the clusters is overlapping
# CLUSTER_TEMPLATE_FILE -> the file where the cluster template is stored

set -e           # Fail in case of error
set -o nounset   # Fail if undefined variables are used
set -o pipefail  # Fail if one of the piped commands fails

error() {
   local sourcefile=$1
   local lineno=$2
   echo "An error occurred at $sourcefile:$lineno."
}
trap 'error "${BASH_SOURCE}" "${LINENO}"' ERR

# check that the liqo crds are removed
wait_for_crds() {
  cnt=0
  while [[ $cnt -lt 300 ]]; do
    if [[ $(${KUBECTL} get crds | grep -c liqo) -eq 0 ]]; then
      return
    fi
    sleep 1
    cnt=$((cnt+1))
  done
  echo "Liqo CRDs found after 300 seconds"
  exit 1
}

SCRIPT_DIR="$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"
# shellcheck disable=SC1091
# shellcheck source=./utils.sh
source "${SCRIPT_DIR}/../../utils.sh"

setup_arch_and_os
install_helm "${OS}" "${ARCH}"

# the unpeer command waits for the status in the local cluster, for the remote one
# it may take a bit longer. Sleep for a second to make sure that the uninstall command
# pre-checks will not fail
sleep 3

for i in $(seq 1 "${CLUSTER_NUMBER}");
do
  export KUBECONFIG="${TMPDIR}/kubeconfigs/liqo_kubeconf_${i}"
  "${LIQOCTL}" uninstall --purge --skip-confirm
  wait_for_crds
done;
