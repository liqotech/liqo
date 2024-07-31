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
    local machine_type=$5
    local image_type=$6
    local disk_type=$7
    local disk_size=$8
    local kubeconfig_name=$9

    local cluster_version="1.29.7-gke.1008000"

    "${GCLOUD}" container --project "${GKE_PROJECT_ID}" clusters create "${cluster_id}" --zone "${cluster_zone}" \
        --num-nodes "${num_nodes}" --machine-type "${machine_type}" --image-type "${image_type}" --disk-type "${disk_type}" --disk-size "${disk_size}" \
        --cluster-version "${cluster_version}" --no-enable-intra-node-visibility --enable-shielded-nodes --enable-ip-alias \
        --release-channel "regular" --no-enable-basic-auth --metadata disable-legacy-endpoints=true \
        --network "projects/${GKE_PROJECT_ID}/global/networks/default" --subnetwork "projects/${GKE_PROJECT_ID}/regions/${cluster_region}/subnetworks/default" \
        --default-max-pods-per-node "110" --security-posture=standard --workload-vulnerability-scanning=disabled --no-enable-master-authorized-networks \
        --enable-autorepair --max-surge-upgrade 1 --max-unavailable-upgrade 0 --binauthz-evaluation-mode=DISABLED \
        --addons HorizontalPodAutoscaling,HttpLoadBalancing,GcePersistentDiskCsiDriver \
        --no-enable-managed-prometheus 

    export KUBECONFIG="${kubeconfig_name}"
    "${GCLOUD}" container clusters get-credentials "${cluster_id}" --zone "${cluster_zone}" --project "${GKE_PROJECT_ID}"
    unset KUBECONFIG

    return 0
}

FILEPATH=$(realpath "$0")
WORKDIR=$(dirname "$FILEPATH")

# shellcheck source=../../utils.sh
source "$WORKDIR/../../utils.sh"

# shellcheck source=./const.sh
source "$WORKDIR/const.sh"

PIDS=()
for i in $(seq 1 "${CLUSTER_NUMBER}");
do
  GKE_CLUSTER_ID=""
  if [[ "${i}" == "1" ]]; then
    GKE_CLUSTER_ID="${GKE_CLUSTER_ID_CONS}"
  else 
    GKE_CLUSTER_ID="${GKE_CLUSTER_ID_PROV}${i}"
  fi

  GKE_CLUSTER_REGION=${regions[$i-1]}
  GKE_CLUSTER_ZONE=${zones[$i-1]}

	echo "Creating cluster ${GKE_CLUSTER_ID}"
  gke_create_cluster "${GKE_CLUSTER_ID}" "${GKE_CLUSTER_REGION}" "${GKE_CLUSTER_ZONE}" "${NUM_NODES}" "${MACHINE_TYPE}" "${IMAGE_TYPE}" "${DISK_TYPE}" "${DISK_SIZE}" "${TMPDIR}/kubeconfigs/liqo_kubeconf_${i}" &
  PIDS+=($!) 
done

# Create GKE clusters
for PID in "${PIDS[@]}"; do
    wait "$PID"
done

for i in $(seq 1 "${CLUSTER_NUMBER}");
do
  # install local-path storage class
  install_local_path_storage "${TMPDIR}/kubeconfigs/liqo_kubeconf_${i}"

  # Install kyverno
  install_kyverno "${TMPDIR}/kubeconfigs/liqo_kubeconf_${i}"
done
