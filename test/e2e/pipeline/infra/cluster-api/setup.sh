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

FILEPATH=$(realpath "$0")
WORKDIR=$(dirname "$FILEPATH")

# shellcheck disable=SC1091
# shellcheck source=../../utils.sh
source "$WORKDIR/../../utils.sh"

# shellcheck disable=SC1091
# shellcheck source=../cni.sh 
source "$WORKDIR/../cni.sh"

export K8S_VERSION=${K8S_VERSION:-"1.29.7"}
K8S_VERSION=$(echo -n "$K8S_VERSION" | sed 's/v//g') # remove the leading v

OS_IMAGE=${OS_IMAGE:-"ubuntu-2204"}

export CRI_PATH="/var/run/containerd/containerd.sock"
export NODE_VM_IMAGE_TEMPLATE="harbor.crownlabs.polito.it/capk/${OS_IMAGE}-container-disk:v${K8S_VERSION}"
export IMAGE_REPO=k8s.gcr.io

export SERVICE_CIDR=10.100.0.0/16
export POD_CIDR=10.200.0.0/16
export POD_CIDR_OVERLAPPING=${POD_CIDR_OVERLAPPING:-"false"}

TARGET_NAMESPACE="liqo-ci"

for i in $(seq 1 "${CLUSTER_NUMBER}");
do
  CAPI_CLUSTER_NAME=$(forge_clustername "${i}")
	if [[ ${POD_CIDR_OVERLAPPING} != "true" ]]; then
		# this should avoid the ipam to reserve a pod CIDR of another cluster as local external CIDR causing remapping
		export POD_CIDR="10.$((i * 10)).0.0/16"
	fi
	echo "Creating cluster ${CAPI_CLUSTER_NAME}"
  POD_CIDR_ESC_1=$(echo $POD_CIDR | cut -d'/' -f1)
  POD_CIDR_ESC_2=$(echo $POD_CIDR | cut -d'/' -f2)
  POD_CIDR_ESC="${POD_CIDR_ESC_1}\/${POD_CIDR_ESC_2}"
  clusterctl generate cluster "${CAPI_CLUSTER_NAME}" \
    --kubernetes-version "$K8S_VERSION" \
    --control-plane-machine-count 1 \
    --worker-machine-count 2 \
    --target-namespace "$TARGET_NAMESPACE" \
    --infrastructure kubevirt | sed "s/10.243.0.0\/16/$POD_CIDR_ESC/g" | ${KUBECTL} apply -f -
done

for i in $(seq 1 "${CLUSTER_NUMBER}");
do
  CAPI_CLUSTER_NAME=$(forge_clustername "${i}")
  if [[ ${POD_CIDR_OVERLAPPING} != "true" ]]; then
		# this should avoid the ipam to reserve a pod CIDR of another cluster as local external CIDR causing remapping
		export POD_CIDR="10.$((i * 10)).0.0/16"
	fi
  echo "Waiting for cluster ${CAPI_CLUSTER_NAME} to be ready"
  "${KUBECTL}" wait --for condition=Ready=true -n "$TARGET_NAMESPACE" "clusters.cluster.x-k8s.io/${CAPI_CLUSTER_NAME}" --timeout=-1s

  echo "Getting kubeconfig for cluster ${CAPI_CLUSTER_NAME}"
  mkdir -p "${TMPDIR}/kubeconfigs"
  clusterctl get kubeconfig -n "$TARGET_NAMESPACE" "${CAPI_CLUSTER_NAME}" > "${TMPDIR}/kubeconfigs/liqo_kubeconf_${i}"

  CURRENT_CONTEXT=$("${KUBECTL}" config current-context --kubeconfig "${TMPDIR}/kubeconfigs/liqo_kubeconf_${i}")
  "${KUBECTL}" config set contexts."${CURRENT_CONTEXT}".namespace default --kubeconfig "${TMPDIR}/kubeconfigs/liqo_kubeconf_${i}"

  echo "Installing ${CNI} for cluster ${CAPI_CLUSTER_NAME}"
  "install_${CNI}" "${TMPDIR}/kubeconfigs/liqo_kubeconf_${i}"

  # install local-path storage class
  install_local_path_storage "${TMPDIR}/kubeconfigs/liqo_kubeconf_${i}"

  # Install metrics-server
  install_metrics_server "${TMPDIR}/kubeconfigs/liqo_kubeconf_${i}"
done

for i in $(seq 1 "${CLUSTER_NUMBER}");
do
  echo "Waiting for cluster ${CAPI_CLUSTER_NAME} CNI to be ready"
  "wait_${CNI}" "${TMPDIR}/kubeconfigs/liqo_kubeconf_${i}"
done
