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
# CNI                   -> the CNI plugin used

set -e           # Fail in case of error
set -o nounset   # Fail if undefined variables are used
set -o pipefail  # Fail if one of the piped commands fails

error() {
   local sourcefile=$1
   local lineno=$2
   echo "An error occurred at $sourcefile:$lineno."
}
trap 'error "${BASH_SOURCE}" "${LINENO}"' ERR

function gke_create_cluster() {
    local cluster_id=$1
    local cluster_region=$2
    local cluster_zone=$3
    local num_nodes=$4
    local index=$5

    local cluster_version="${K8S_VERSION#v}"
    cluster_version=$(echo "${cluster_version}" | awk -F. '{print $1"."$2}')

    local arg_dataplane=""
    if [[ $CNI == "v2" ]]; then
        arg_dataplane="--enable-dataplane-v2"
    fi

    pod_cidr=10.${index}.0.0/16
    if [[ ${POD_CIDR_OVERLAPPING} == "true" ]]; then
        pod_cidr=10.200.0.0/16
    fi
        
    "${GCLOUD}" container --project "${GCLOUD_PROJECT_ID}" clusters create "${cluster_id}" --zone "${cluster_zone}" \
        --num-nodes "${num_nodes}" --machine-type "${GKE_MACHINE_TYPE}" --image-type "${OS_IMAGE}" --disk-type "${GKE_DISK_TYPE}" --disk-size "${GKE_DISK_SIZE}" \
        --cluster-version "${cluster_version}" --no-enable-intra-node-visibility --enable-shielded-nodes --enable-ip-alias \
        --release-channel "regular" --no-enable-basic-auth --metadata disable-legacy-endpoints=true \
        --network "projects/${GCLOUD_PROJECT_ID}/global/networks/liqo-${index}" --subnetwork "projects/${GCLOUD_PROJECT_ID}/regions/${cluster_region}/subnetworks/liqo-nodes" $arg_dataplane --cluster-ipv4-cidr="${pod_cidr}" \
        --default-max-pods-per-node "110" --security-posture=standard --workload-vulnerability-scanning=disabled --no-enable-master-authorized-networks \
        --enable-autorepair --max-surge-upgrade 1 --max-unavailable-upgrade 0 --binauthz-evaluation-mode=DISABLED \
        --addons HorizontalPodAutoscaling,HttpLoadBalancing,GcePersistentDiskCsiDriver \
        --no-enable-managed-prometheus 
    return 0
}

function gke_generate_kubeconfig() {
    local cluster_id=$1
    local cluster_zone=$2
    local kubeconfig_file=$3

    export KUBECONFIG="${kubeconfig_file}"
    "${GCLOUD}" container clusters get-credentials "${cluster_id}" --zone "${cluster_zone}" --project "${GCLOUD_PROJECT_ID}"
    unset KUBECONFIG
}

FILEPATH=$(realpath "$0")
WORKDIR=$(dirname "$FILEPATH")

# shellcheck disable=SC1091
# shellcheck source=../../utils.sh
source "$WORKDIR/../../utils.sh"

# shellcheck disable=SC1091
# shellcheck source=./const.sh
source "$WORKDIR/const.sh"

if [[ "${CLUSTER_NUMBER}" -gt 3 ]]; then
    echo "Error: CLUSTER_NUMBER cannot be greater than 3."
    exit 1
fi

PIDS=()
for i in $(seq 1 "${CLUSTER_NUMBER}");
do
  GKE_CLUSTER_ID=$(forge_clustername "${i}")
  GKE_CLUSTER_REGION=${GKE_REGIONS[$i-1]}
  GKE_CLUSTER_ZONE=${GKE_ZONES[$i-1]}

  gke_create_cluster "${GKE_CLUSTER_ID}" "${GKE_CLUSTER_REGION}" "${GKE_CLUSTER_ZONE}" "${GKE_NUM_NODES}" "${i}" &
  PIDS+=($!)
done

# Create GKE clusters
for PID in "${PIDS[@]}"; do
    wait "$PID"
done

for i in $(seq 1 "${CLUSTER_NUMBER}");
do
  GKE_CLUSTER_ID=$(forge_clustername "${i}")
  GKE_CLUSTER_ZONE=${GKE_ZONES[$i-1]}

  gke_generate_kubeconfig "${GKE_CLUSTER_ID}" "${GKE_CLUSTER_ZONE}" "${TMPDIR}/kubeconfigs/liqo_kubeconf_${i}"
done

