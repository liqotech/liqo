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
# POD_CIDR_OVERLAPPING  -> the pod CIDR of the clusters is overlapping
# CLUSTER_TEMPLATE_FILE -> the file where the cluster template is stored

set -e           # Fail in case of error
set -o nounset   # Fail if undefined variables are used
set -o pipefail  # Fail if one of the piped commands fails

function check_no_resources() {
  local query="$1"
  local name="$2"

  nl=$(eval "${query}" | wc -l)
  if [ "${nl}" -ne 0 ]; then
    echo "Error: the peering is not correctly removed, resources ${name} found"
    eval "${query}"
    exit 1
  fi
}

error() {
   local sourcefile=$1
   local lineno=$2
   echo "An error occurred at $sourcefile:$lineno."
}
trap 'error "${BASH_SOURCE}" "${LINENO}"' ERR

SCRIPT_DIR="$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"
# shellcheck disable=SC1091
# shellcheck source=../../utils.sh
source "${SCRIPT_DIR}/../../utils.sh"

CONSUMER_KUBECONFIG="${TMPDIR}/kubeconfigs/liqo_kubeconf_1"
CLUSTER_ID=$(forge_clustername 1)
for i in $(seq 2 "${CLUSTER_NUMBER}");
do
  export KUBECONFIG="${CONSUMER_KUBECONFIG}"
  export PROVIDER_KUBECONFIG_ADMIN="${TMPDIR}/kubeconfigs/liqo_kubeconf_${i}"
  
  if [[ "${INFRA}" == "eks" ]]; then
    # Do not use peer-user on EKS since it is not supported
    PROVIDER_KUBECONFIG=$PROVIDER_KUBECONFIG_ADMIN
    "${LIQOCTL}" unpeer --kubeconfig "${KUBECONFIG}" --remote-kubeconfig "${PROVIDER_KUBECONFIG}" --skip-confirm
  else
    PROVIDER_KUBECONFIG="${TMPDIR}/kubeconfigs/generated/liqo_kubeconf_${i}"
    "${LIQOCTL}" unpeer --kubeconfig "${KUBECONFIG}" --remote-kubeconfig "${PROVIDER_KUBECONFIG}" --skip-confirm
    "${LIQOCTL}" delete peering-user --kubeconfig "${PROVIDER_KUBECONFIG_ADMIN}" --consumer-cluster-id "${CLUSTER_ID}"
  fi
done;

# check that the peering is correctly removed
for i in $(seq 1 "${CLUSTER_NUMBER}");
do
  export KUBECONFIG="${TMPDIR}/kubeconfigs/liqo_kubeconf_${i}"

  check_no_resources "${KUBECTL} get tenants.authentication.liqo.io" "Tenants"
  check_no_resources "${KUBECTL} get identities.authentication.liqo.io -A" "Identities"
  check_no_resources "${KUBECTL} get resourceslices.authentication.liqo.io -A" "ResourceSlices"

  check_no_resources "${KUBECTL} get gatewayclients.networking.liqo.io -A" "GatewayClients"
  check_no_resources "${KUBECTL} get gatewayservers.networking.liqo.io -A" "GatewayServers"
  check_no_resources "${KUBECTL} get publickeies.networking.liqo.io -A" "PublicKeys"
  check_no_resources "${KUBECTL} get wggatewayclients.networking.liqo.io -A" "WgGatewayClients"
  check_no_resources "${KUBECTL} get wggatewayservers.networking.liqo.io -A" "WgGatewayServers"
  check_no_resources "${KUBECTL} get connections.networking.liqo.io -A" "Connections"
  check_no_resources "${KUBECTL} get configurations.networking.liqo.io -A" "Configurations"

  check_no_resources "${KUBECTL} get namespacemaps.offloading.liqo.io -A" "NamespaceMaps"
  check_no_resources "${KUBECTL} get shadowendpointslices.offloading.liqo.io -A" "ShadowEndpointSlices"
  check_no_resources "${KUBECTL} get shadowpods.offloading.liqo.io -A" "ShadowPods"
  check_no_resources "${KUBECTL} get virtualnodes.offloading.liqo.io -A" "VirtualNodes"

  fc=$("${KUBECTL}" get foreignclusters.core.liqo.io -o json)
  auth_module=$(echo "${fc}" | jq -r '.items[0].status.modules.authentication.enabled')
  if [ "${auth_module}" == "true" ]; then
    echo "Error: the authentication module is still enabled"
    echo "${fc}"
    exit 1
  fi
  net_module=$(echo "${fc}" | jq -r '.items[0].status.modules.networking.enabled')
  if [ "${net_module}" == "true" ]; then
    echo "Error: the networking module is still enabled"
    echo "${fc}"
    exit 1
  fi
  off_module=$(echo "${fc}" | jq -r '.items[0].status.modules.offloading.enabled')
  if [ "${off_module}" == "true" ]; then
    echo "Error: the offloading module is still enabled"
    echo "${fc}"
    exit 1
  fi
done;
