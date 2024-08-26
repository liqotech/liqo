#!/bin/bash
#shellcheck disable=SC1091

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

FILEPATH=$(realpath "$0")
WORKDIR=$(dirname "$FILEPATH")

source "$WORKDIR/../../utils.sh"

NUM_NODES="2"
VM_TYPE="Standard_B2s"
REGIONS=("italynorth" "francecentral" "germanywestcentral" "switzerlandnorth")

POD_CIDR_OVERLAPPING=${POD_CIDR_OVERLAPPING:-"true"}
CNI=${CNI:-"azure"} # "azure", "kubenet", "none"

if [[ "${CNI}" == "azure" ]]; then
    POD_CIDR_OVERLAPPING="true"
fi

function create_resource_group() {
    local aks_resource_group=$1
    local region=$2

    rg_exists=$(az group exists --name "$aks_resource_group")
    if  [[ $rg_exists == "false" ]]; then
        echo "Creating resource group $aks_resource_group in region $region"
        az group create \
            --name "$aks_resource_group" \
            --location "$region"
    fi
}

function aks_create_cluster() {
    local aks_resource_group=$1
    local aks_resource_name=$2
    local kubeconfig=$3
    local pod_cidr=$4

    args=()
    args+=("--resource-group $aks_resource_group")
    args+=("--name $aks_resource_name")
    args+=("--node-count $NUM_NODES")
    args+=("--node-vm-size $VM_TYPE")
    args+=("--kubernetes-version $K8S_VERSION")
    args+=("--tier free")
    args+=("--generate-ssh-keys")

    args+=("--network-plugin $CNI")
    if [[ "${CNI}" == "kubenet" ]]; then
        args+=("--pod-cidr $pod_cidr")
    fi
    
    ARGS="${args[*]}"
    eval "az aks create $ARGS"

    az aks get-credentials \
        --resource-group "$aks_resource_group" \
        --name "$aks_resource_name" \
        --file "$kubeconfig"
}

# Create the clusters
PIDS=()

for i in $(seq 1 "${CLUSTER_NUMBER}");
do
    AKS_RESOURCE_GROUP="liqo${i}"
    RUNNER_NAME=${RUNNER_NAME:-"test"}
    AKS_CLUSTER_NAME="${RUNNER_NAME}-cluster${i}"
    REGION=${REGIONS[$i-1]}
    KUBECONFIG="${TMPDIR}/kubeconfigs/liqo_kubeconf_${i}"
    
    create_resource_group "${AKS_RESOURCE_GROUP}" "${REGION}"

    # The PodCIDR can be set only for kubenet. On AzureCNI it is fixed, so only pod cidr overlapping is possible.
    POD_CIDR=""
    if [[ ${CNI} == "kubenet" ]]; then
        if [[ ${POD_CIDR_OVERLAPPING} == "true" ]]; then
            POD_CIDR="10.50.0.0/16"
        else
            POD_CIDR="10.$((i * 10)).0.0/16"
        fi
    fi

	aks_create_cluster "${AKS_RESOURCE_GROUP}" "${AKS_CLUSTER_NAME}" "${KUBECONFIG}" "${POD_CIDR}" &
    PIDS+=($!)
done

for PID in "${PIDS[@]}"; do
    wait "${PID}"
done
