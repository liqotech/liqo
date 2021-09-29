#!/bin/bash

# This scripts expects the following variables to be set:
# CLUSTER_NUMBER -> the number of liqo clusters
# K8S_VERSION    -> the Kubernetes version
# CNI            -> the CNI plugin used
# TMPDIR         -> the directory where the test-related files are stored
# BINDIR         -> the directory where the test-related binaries are stored
# TEMPLATE_DIR   -> the directory where to read the cluster templates
# NAMESPACE      -> the namespace where liqo is running
# KUBECONFIGDIR  -> the directory where the kubeconfigs are stored
# LIQO_VERSION   -> the liqo version to test
# INFRA          -> the Kubernetes provider for the infrastructure
# LIQOCTL        -> the path where liqoctl is stored

set -e           # Fail in case of error
set -o nounset   # Fail if undefined variables are used
set -o pipefail  # Fail if one of the piped commands fails

CLUSTER_NAME=cluster
KIND="${BINDIR}/kind"

export DISABLE_KINDNET=false

if [[ ${CNI} != "kindnet" ]]; then
	export DISABLE_KINDNET=true
fi

export SERVICE_CIDR=10.100.0.0/16
export POD_CIDR=10.200.0.0/16
export POD_CIDR_OVERLAPPING=${POD_CIDR_OVERLAPPING:-"true"}

for i in $(seq 1 "${CLUSTER_NUMBER}");
do
	if [[ ${POD_CIDR_OVERLAPPING} != "true" ]]; then
		export POD_CIDR=10.${i}.0.0/16
	fi
	envsubst < "${TEMPLATE_DIR}/templates/cluster-templates.yaml.tmpl" > "${TMPDIR}/liqo-cluster-${CLUSTER_NAME}${i}.yaml"
	echo "Creating cluster ${CLUSTER_NAME}${i}..."
	${KIND} create cluster --name "${CLUSTER_NAME}${i}" --kubeconfig "${TMPDIR}/kubeconfigs/liqo_kubeconf_${i}" --config "${TMPDIR}/liqo-cluster-${CLUSTER_NAME}${i}.yaml" --wait 2m
done
