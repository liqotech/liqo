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

for i in $(seq 1 "${CLUSTER_NUMBER}")
do
  export KUBECONFIG="${TMPDIR}/kubeconfigs/liqo_kubeconf_${i}"
  ADD_COMMAND=$(${LIQOCTL} generate peer-command --only-command)

  for j in $(seq 1 "${CLUSTER_NUMBER}");
  do
    if [[ $i -ne $j ]]
    then
      export KUBECONFIG="${TMPDIR}/kubeconfigs/liqo_kubeconf_${j}"
      set +e
      retries=0
      until eval "${ADD_COMMAND}" || [ $retries -eq 5 ]; do
        echo "Failed to add cluster ${j} to cluster ${i}. Retrying in 5 seconds..."
        sleep 5
        retries=$((retries + 1))
      done
      set -e
      "${LIQOCTL}" status peer "cluster-${i}" --verbose
    fi
  done

  # Sleep a bit, to avoid generating a race condition with the
  # authentication process triggered by the incoming peering.
  sleep 1
done
