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

SCRIPT_DIR="$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"
# shellcheck disable=SC1091
# shellcheck source=./helm-utils.sh
source "${SCRIPT_DIR}/helm-utils.sh"

download_helm

for i in $(seq 1 "${CLUSTER_NUMBER}");
do
  export KUBECONFIG="${TMPDIR}/kubeconfigs/liqo_kubeconf_${i}"
  timeout 300 bash -c "${HELM} uninstall -n liqo liqo"
  timeout 300 kubectl delete -f ./deployments/liqo/crds
done;
