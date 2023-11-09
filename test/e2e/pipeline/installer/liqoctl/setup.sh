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
# SECURITY_MODE         -> the security mode to use
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

SCRIPT_DIR="$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"
# shellcheck disable=SC1091
# shellcheck source=./helm-utils.sh
source "${SCRIPT_DIR}/helm-utils.sh"

download_helm

function get_cluster_labels() {
  case $1 in
  1)
  echo "provider=Azure,region=A"
  ;;
  2)
  echo "provider=AWS,region=B"
  ;;
  3)
  echo "provider=GKE,region=C"
  ;;
  4)
  echo "provider=GKE,region=D"
  ;;
  esac
}

LIQO_VERSION="${LIQO_VERSION:-$(git rev-parse HEAD)}"
SECURITY_MODE="${SECURITY_MODE:-"FullPodToPod"}"

export SERVICE_CIDR=10.100.0.0/16
export POD_CIDR=10.200.0.0/16
export POD_CIDR_OVERLAPPING=${POD_CIDR_OVERLAPPING:-"false"}

for i in $(seq 1 "${CLUSTER_NUMBER}");
do
  export KUBECONFIG="${TMPDIR}/kubeconfigs/liqo_kubeconf_${i}"
  CLUSTER_LABELS="$(get_cluster_labels "${i}")"
  if [[ ${POD_CIDR_OVERLAPPING} != "true" ]]; then
		# this should avoid the ipam to reserve a pod CIDR of another cluster as local external CIDR causing remapping
		export POD_CIDR="10.$((i * 10)).0.0/16"
	fi
  COMMON_ARGS=(--cluster-name "cluster-${i}" --local-chart-path ./deployments/liqo
    --version "${LIQO_VERSION}" --set controllerManager.config.enableResourceEnforcement=true --set "networking.securityMode=${SECURITY_MODE}")
  if [[ "${CLUSTER_LABELS}" != "" ]]; then
    COMMON_ARGS=("${COMMON_ARGS[@]}" --cluster-labels "${CLUSTER_LABELS}")
  fi
  if [[ "${INFRA}" == "k3s" ]]; then
    COMMON_ARGS=("${COMMON_ARGS[@]}" --pod-cidr "${POD_CIDR}" --service-cidr "${SERVICE_CIDR}")
  fi
  if [[ "${INFRA}" == "cluster-api" ]]; then
    LIQO_PROVIDER="kubeadm"
    COMMON_ARGS=("${COMMON_ARGS[@]}" --set auth.service.type=NodePort --set gateway.service.type=NodePort)
  else
    LIQO_PROVIDER="${INFRA}"
  fi

  if [ "${i}" == "1" ]; then
    # Install Liqo with Helm, to check that values generation works correctly.
    "${LIQOCTL}" install "${LIQO_PROVIDER}" "${COMMON_ARGS[@]}" --only-output-values --dump-values-path "${TMPDIR}/values.yaml"
    "${HELM}" install -n "${NAMESPACE}" --create-namespace liqo ./deployments/liqo -f "${TMPDIR}/values.yaml"
  else
    "${LIQOCTL}" install "${LIQO_PROVIDER}" "${COMMON_ARGS[@]}"
  fi
done;

for i in $(seq 1 "${CLUSTER_NUMBER}");
do
  export KUBECONFIG="${TMPDIR}/kubeconfigs/liqo_kubeconf_${i}"
  "${KUBECTL}" wait --for=condition=Ready pods --all -n liqo
  "${LIQOCTL}" status --verbose
done;
