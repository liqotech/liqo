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


# Cleaning all remaining clusters

PIDS=()
for i in $(seq 1 "${CLUSTER_NUMBER}")
do
    AKS_RESOURCE_GROUP="liqo${i}"
    CLUSTER_NAME=$(forge_clustername "${i}")

    # if the cluster exists, delete it
    if az aks show --resource-group "${AKS_RESOURCE_GROUP}" --name "${CLUSTER_NAME}" &> /dev/null; then
        echo "Deleting cluster ${CLUSTER_NAME}"
        az aks delete --resource-group "${AKS_RESOURCE_GROUP}" --name "${CLUSTER_NAME}" --yes &
        PIDS+=($!)
    else
        echo "Cluster ${CLUSTER_NAME} does not exist"
    fi
done

for PID in "${PIDS[@]}"; do
    wait "${PID}"
done

# Cleaning the resource group

PIDS=()
for i in $(seq 1 "${CLUSTER_NUMBER}")
do
    AKS_RESOURCE_GROUP="liqo${i}"
    rg_exists=$(az group exists --name "${AKS_RESOURCE_GROUP}")
    if [[ $rg_exists == "true" ]]; then
        echo "Deleting resource group ${AKS_RESOURCE_GROUP}"
        az group delete --name "${AKS_RESOURCE_GROUP}" --yes &
        PIDS+=($!)
    else
        echo "Resource group ${AKS_RESOURCE_GROUP} does not exist. Nothing to delete"
    fi
done

for PID in "${PIDS[@]}"; do
    wait "${PID}"
done
