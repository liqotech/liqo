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

for i in $(seq 2 "${CLUSTER_NUMBER}");
do
  export KUBECONFIG="${TMPDIR}/kubeconfigs/liqo_kubeconf_${i}"
  ADD_COMMAND=$(${LIQOCTL} generate-add-command --only-command)

  export KUBECONFIG="${TMPDIR}/kubeconfigs/liqo_kubeconf_1"
  eval "${ADD_COMMAND}"
done;
