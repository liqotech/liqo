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
# AZ_SUBSCRIPTION_ID    -> the ID of the Azure subscription to use (only for AKS)

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
# shellcheck source=../../utils.sh
source "${SCRIPT_DIR}/../../utils.sh"

# shellcheck disable=SC1091
# shellcheck source=../../infra/gke/const.sh
source "${SCRIPT_DIR}/../../infra/gke/const.sh"

setup_arch_and_os
install_helm "${OS}" "${ARCH}"

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

export SERVICE_CIDR=10.100.0.0/16
export POD_CIDR=10.200.0.0/16
export POD_CIDR_OVERLAPPING=${POD_CIDR_OVERLAPPING:-"false"}
export HA_REPLICAS=2

for i in $(seq 1 "${CLUSTER_NUMBER}");
do
  export KUBECONFIG="${TMPDIR}/kubeconfigs/liqo_kubeconf_${i}"
  CLUSTER_LABELS="$(get_cluster_labels "${i}")"
  CLUSTER_NAME=$(forge_clustername "${i}")
  
  if [[ ${POD_CIDR_OVERLAPPING} != "true" ]]; then
		# this should avoid the ipam to reserve a pod CIDR of another cluster as local external CIDR causing remapping
		export POD_CIDR="10.$((i * 10)).0.0/16"
	fi

  COMMON_ARGS=(--cluster-id "${CLUSTER_NAME}" --local-chart-path ./deployments/liqo
    --version "${LIQO_VERSION}" --set metrics.enabled=true)
  if [[ "${CLUSTER_LABELS}" != "" ]]; then
    COMMON_ARGS=("${COMMON_ARGS[@]}" --cluster-labels "${CLUSTER_LABELS}")
  fi
  
  if [[ "${INFRA}" == "k3s" ]]; then
    COMMON_ARGS=("${COMMON_ARGS[@]}" --pod-cidr "${POD_CIDR}" --service-cidr "${SERVICE_CIDR}")
  fi

  if [[ "${INFRA}" == "eks" ]]; then
    COMMON_ARGS=("${COMMON_ARGS[@]}" --eks-cluster-region="eu-central-1" --eks-cluster-name="${CLUSTER_NAME}")
    # do not fail if variables are not set
    set +u
    if [[ "${AWS_LIQOCTL_USERNAME}" != "" ]]; then
      COMMON_ARGS=("${COMMON_ARGS[@]}" --user-name "${AWS_LIQOCTL_USERNAME}")
    fi
    if [[ "${AWS_LIQOCTL_POLICY_NAME}" != "" ]]; then
      COMMON_ARGS=("${COMMON_ARGS[@]}" --policy-name "${AWS_LIQOCTL_POLICY_NAME}")
    fi
    if [[ "${AWS_LIQOCTL_ACCESS_KEY_ID}" != "" ]]; then
      COMMON_ARGS=("${COMMON_ARGS[@]}" --access-key-id "${AWS_LIQOCTL_ACCESS_KEY_ID}")
    fi
    if [[ "${AWS_LIQOCTL_SECRET_ACCESS_KEY}" != "" ]]; then
      COMMON_ARGS=("${COMMON_ARGS[@]}" --secret-access-key "${AWS_LIQOCTL_SECRET_ACCESS_KEY}")
    fi
    set -u
  fi
  
  if [[ "${INFRA}" == "aks" ]]; then
    AKS_RESOURCE_GROUP="liqo${i}"
    COMMON_ARGS=("${COMMON_ARGS[@]}" --subscription-id "${AZ_SUBSCRIPTION_ID}" --resource-group-name "${AKS_RESOURCE_GROUP}" --resource-name "${CLUSTER_NAME}")
  fi

  if [[ "${INFRA}" == "gke" ]]; then
    COMMON_ARGS=("${COMMON_ARGS[@]}" --project-id "${GCLOUD_PROJECT_ID}" --zone "${GKE_ZONES[$i-1]}" --credentials-path "${BINDIR}/gke_key_file.json")
  fi

  if [[ "${INFRA}" == "kubeadm" ]]; then
    LIQO_PROVIDER="kubeadm"
    COMMON_ARGS=("${COMMON_ARGS[@]}" --set "networking.gatewayTemplates.replicas=$HA_REPLICAS" )
    COMMON_ARGS=("${COMMON_ARGS[@]}" --set "ipam.internal.replicas=$HA_REPLICAS" )
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
done;
