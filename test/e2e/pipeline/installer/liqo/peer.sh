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

set_certificate_renewal_policy() {
  local POLICY_MANIFEST
  POLICY_MANIFEST=$(cat <<'EOF'
apiVersion: kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: patch-csr-expiration
  annotations:
    policies.kyverno.io/title: Patch CSR Expiration Time
    policies.kyverno.io/category: Security
    policies.kyverno.io/severity: medium
    policies.kyverno.io/subject: CertificateSigningRequest
    policies.kyverno.io/description: >-
      This policy patches CertificateSigningRequest resources with names
      starting with 'liqo-identity' to set the expiration duration to 600 seconds.
spec:
  rules:
    - name: set-csr-expiration
      match:
        any:
          - resources:
              kinds:
                - CertificateSigningRequest
              names:
                - "liqo-identity*"
      mutate:
        patchStrategicMerge:
          spec:
            expirationSeconds: 600
EOF
)

  for i in $(seq 1 "${CLUSTER_NUMBER}")
  do
    export KUBECONFIG="${TMPDIR}/kubeconfigs/liqo_kubeconf_${i}"
    echo "Applying Kyverno ClusterPolicy on cluster ${i} to set the CSR expiration time to 600 seconds"
    echo "${POLICY_MANIFEST}" | "${KUBECTL}" apply -f -
    echo "Waiting for Kyverno ClusterPolicy to become ready on cluster ${i}"
    "${KUBECTL}" wait --for=condition=Ready clusterpolicy/patch-csr-expiration --timeout=120s
  done
}

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

echo "Enabling certificate renewal test"
set_certificate_renewal_policy

mkdir -p "${TMPDIR}/kubeconfigs/generated"
CLUSTER_ID=$(forge_clustername 1)
for i in $(seq 2 "${CLUSTER_NUMBER}")
do
  export KUBECONFIG="${TMPDIR}/kubeconfigs/liqo_kubeconf_1"
  export PROVIDER_KUBECONFIG_ADMIN="${TMPDIR}/kubeconfigs/liqo_kubeconf_${i}"

  if [[ "${INFRA}" == "eks" ]]; then
    # Do not generate peer-user on EKS since it is not supported
    PROVIDER_KUBECONFIG=$PROVIDER_KUBECONFIG_ADMIN
  else
    echo "Generating kubeconfig for consumer cluster 1 on provider cluster ${i}"
    "${LIQOCTL}" generate peering-user --kubeconfig "${PROVIDER_KUBECONFIG_ADMIN}" --consumer-cluster-id "${CLUSTER_ID}" > "${TMPDIR}/kubeconfigs/generated/liqo_kubeconf_${i}"
    PROVIDER_KUBECONFIG="${TMPDIR}/kubeconfigs/generated/liqo_kubeconf_${i}"
  fi

  ARGS=(--kubeconfig "${KUBECONFIG}" --remote-kubeconfig "${PROVIDER_KUBECONFIG}")

  if [[ "${INFRA}" == "kubeadm" ]]; then
    ARGS=("${ARGS[@]}" --gw-server-service-type NodePort)
  elif [[ "${INFRA}" == "kind" ]]; then
    ARGS=("${ARGS[@]}" --gw-server-service-type NodePort)
  elif [[ "${INFRA}" == "k3s" ]]; then
    ARGS=("${ARGS[@]}" --gw-server-service-type NodePort)
  fi

  echo "Environment variables:"
  env

  echo "Kubeconfig consumer:"
  cat "${KUBECONFIG}"

  echo "Kubeconfig provider:"
  cat "${PROVIDER_KUBECONFIG}"

  ARGS=("${ARGS[@]}")
  "${LIQOCTL}" peer "${ARGS[@]}"

  # Sleep a bit, to avoid generating a race condition with the
  # authentication process triggered by the incoming peering.
  sleep 3
done
