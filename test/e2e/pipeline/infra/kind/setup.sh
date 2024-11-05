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

# shellcheck disable=SC1091
# shellcheck source=../cni.sh 
source "$WORKDIR/../cni.sh"

KIND="${BINDIR}/kind"

CLUSTER_NAME=cluster
export DISABLE_KINDNET=false

if [[ ${CNI} != "kindnet" ]]; then
	export DISABLE_KINDNET=true
fi

export SERVICE_CIDR=10.100.0.0/16
export POD_CIDR=10.200.0.0/16
export POD_CIDR_OVERLAPPING=${POD_CIDR_OVERLAPPING:-"false"}

CLUSTER_TEMPLATE_FILE=${CLUSTER_TEMPLATE_FILE:-cluster-templates.yaml.tmpl}

for i in $(seq 1 "${CLUSTER_NUMBER}");
do
	if [[ ${POD_CIDR_OVERLAPPING} != "true" ]]; then
		# this should avoid the ipam to reserve a pod CIDR of another cluster as local external CIDR causing remapping
		export POD_CIDR="10.$((i * 10)).0.0/16"
	fi
	envsubst < "${TEMPLATE_DIR}/templates/$CLUSTER_TEMPLATE_FILE" > "${TMPDIR}/liqo-cluster-${CLUSTER_NAME}${i}.yaml"
	echo "Creating cluster ${CLUSTER_NAME}${i}..."
	${KIND} create cluster --name "${CLUSTER_NAME}${i}" --kubeconfig "${TMPDIR}/kubeconfigs/liqo_kubeconf_${i}" --config "${TMPDIR}/liqo-cluster-${CLUSTER_NAME}${i}.yaml" --wait 2m

	# Install CNI if kindnet disabled
	if [[ ${DISABLE_KINDNET} == "true" ]]; then
		"install_${CNI}" "${TMPDIR}/kubeconfigs/liqo_kubeconf_${i}"
	fi

	# Install metrics-server
	install_metrics_server "${TMPDIR}/kubeconfigs/liqo_kubeconf_${i}"
done

