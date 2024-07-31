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
# EKSCTL                -> the path where eksctl is stored
# AWS_CLI               -> the path where aws-cli is stored
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

# shellcheck source=../../utils.sh
source "$WORKDIR/../../utils.sh"

CLUSTER_NAME=cluster

export POD_CIDR=10.200.0.0/16
export POD_CIDR_OVERLAPPING=${POD_CIDR_OVERLAPPING:-"false"}

RUNNER_NAME=${RUNNER_NAME:-"test"}
CLUSTER_NAME="${RUNNER_NAME}-${CLUSTER_NAME}"

PIDS=()

for i in $(seq 1 "${CLUSTER_NUMBER}");
do
	if [[ ${POD_CIDR_OVERLAPPING} != "true" ]]; then
		export POD_CIDR="10.$((i * 10)).0.0/16"
	fi
	echo "Creating cluster ${CLUSTER_NAME}${i}"
    "${EKSCTL}" create cluster \
        --name "${CLUSTER_NAME}${i}" \
        --region "eu-central-1" \
        --instance-types c4.large,c5.large \
        --nodes 2 \
        --managed \
        --alb-ingress-access \
        --node-ami-family "AmazonLinux2" \
        --vpc-cidr "$POD_CIDR" \
        --kubeconfig "${TMPDIR}/kubeconfigs/liqo_kubeconf_${i}" &
    PIDS+=($!)
done

for PID in "${PIDS[@]}"; do
    wait "${PID}"
done

for i in $(seq 1 "${CLUSTER_NUMBER}");
do
  CURRENT_CONTEXT=$("${KUBECTL}" config current-context --kubeconfig "${TMPDIR}/kubeconfigs/liqo_kubeconf_${i}")
  "${KUBECTL}" config set contexts."${CURRENT_CONTEXT}".namespace default --kubeconfig "${TMPDIR}/kubeconfigs/liqo_kubeconf_${i}"

  # install local-path storage class
  install_local_path_storage "${TMPDIR}/kubeconfigs/liqo_kubeconf_${i}"

  # Install metrics-server
  install_metrics_server "${TMPDIR}/kubeconfigs/liqo_kubeconf_${i}"

  # Install kyverno for network tests
  install_kyverno "${TMPDIR}/kubeconfigs/liqo_kubeconf_${i}"

  # Install AWS Load Balancer Controller
  "${HELM}" repo add eks https://aws.github.io/eks-charts
  "${HELM}" repo update
  "${HELM}" install aws-load-balancer-controller eks/aws-load-balancer-controller \
    -n kube-system \
    --set clusterName="${CLUSTER_NAME}${i}" \
    --kubeconfig "${TMPDIR}/kubeconfigs/liqo_kubeconf_${i}"
done
