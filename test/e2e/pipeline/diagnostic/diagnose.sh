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

for i in $(seq 1 "${CLUSTER_NUMBER}");
do
   echo "OUTPUT CLUSTER ${i}"
   export KUBECONFIG=${TMPDIR}/kubeconfigs/liqo_kubeconf_${i}
   echo "Pods created in the cluster"
   echo "|------------------------------------------------------------|"
   ${KUBECTL} get po -A -o wide
   echo "Core resources in Liqo namespace"
   echo "|------------------------------------------------------------|"
   ${KUBECTL} get all -n liqo -o wide
   echo "Installed CRDs"
   echo "|------------------------------------------------------------|"
   ${KUBECTL} get crd -A
   echo "Available Nodes"
   echo "|------------------------------------------------------------|"
   ${KUBECTL} get no -o wide --show-labels
   echo "Gateway API resources"
   echo "|------------------------------------------------------------|"
   ${KUBECTL} get gatewayclasses,gateways,httproutes,grpcroutes,tlsroutes,referencegrants -A -o wide || true
   ${KUBECTL} get shadowgatewaystatuses,shadowroutestatuses -A -o yaml || true
   ${KUBECTL} get pods,svc -n envoy-gateway-system -o wide || true
   ${KUBECTL} get gateways -A -o yaml || true
   echo "Envoy Gateway pods details and logs"
   echo "|------------------------------------------------------------|"
   ${KUBECTL} describe pods -n envoy-gateway-system || true
   for pod in $(${KUBECTL} get pods -n envoy-gateway-system -o name 2>/dev/null); do
      echo "Logs of ${pod}"
      ${KUBECTL} logs -n envoy-gateway-system "${pod}" --all-containers --prefix --tail=300 || true
      ${KUBECTL} logs -n envoy-gateway-system "${pod}" --all-containers --prefix --tail=300 --previous || true
   done
   echo "Liqo controller manager logs"
   echo "|------------------------------------------------------------|"
   for pod in $(${KUBECTL} get pods -n liqo -l app.kubernetes.io/name=controller-manager -o name 2>/dev/null); do
      ${KUBECTL} logs -n liqo "${pod}" --tail=300 || true
      ${KUBECTL} logs -n liqo "${pod}" --tail=300 --previous || true
   done
   echo "Liqo local status"
   echo "|------------------------------------------------------------|"
   ${LIQOCTL} info --verbose
   echo "Liqo peerings statuses"
   echo "|------------------------------------------------------------|"
   ${LIQOCTL} info peer --verbose
done;
