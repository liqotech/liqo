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

for i in $(seq 1 "${CLUSTER_NUMBER}");
do
  export KUBECONFIG="${TMPDIR}/kubeconfigs/liqo_kubeconf_${i}"
  CLUSTER_LABELS="$(get_cluster_labels "${i}")"
  COMMON_ARGS=(--cluster-name "liqo-${i}" --chart-path ./deployments/liqo --version "${LIQO_VERSION}")
  if [[ "${CLUSTER_LABELS}" != "" ]]; then
    COMMON_ARGS=("${COMMON_ARGS[@]}" --cluster-labels "${CLUSTER_LABELS}")
  fi

  if [ "${i}" == "1" ]; then
    "${LIQOCTL}" install kind "${COMMON_ARGS[@]}" --only-output-values --dump-values-path "${TMPDIR}/values.yaml"

    # update the discovery settings, this cluster will not discover the other clusters, but the other clusters will discover it
    sed -i 's/enableDiscovery: true/enableDiscovery: false/' "${TMPDIR}/values.yaml"
    "${HELM}" install -n "${NAMESPACE}" --create-namespace liqo ./deployments/liqo -f "${TMPDIR}/values.yaml" --dependency-update
  else
    "${LIQOCTL}" install kind "${COMMON_ARGS[@]}"
  fi
done;
