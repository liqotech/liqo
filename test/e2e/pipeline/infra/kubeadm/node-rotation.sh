#!/bin/bash
#shellcheck disable=SC1091

# This script expects the following variables to be set:
# CLUSTER_NUMBER -> the number of liqo clusters
# INFRA          -> the Kubernetes provider for the infrastructure
# TMPDIR         -> the directory where the test-related files are stored
# KUBECTL        -> the path where kubectl is stored

set -e          # Fail in case of error
set -o nounset  # Fail if undefined variables are used
set -o pipefail # Fail if one of the piped commands fails

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

if [[ "${INFRA}" != "kubeadm" ]]; then
  echo "Node rotation only supported for kubeadm infra, skipping"
  exit 0
fi

# Wait until the MachineDeployment's readyReplicas matches the expected count.
wait_machinedeployment_ready() {
  local namespace=$1
  local name=$2
  local expected=$3
  local deadline=$((SECONDS + 600))

  echo "Waiting for MachineDeployment ${name} to have ${expected} ready replicas"
  while [[ $SECONDS -lt $deadline ]]; do
    local ready
    ready=$(${KUBECTL} get machinedeployment -n "${namespace}" "${name}" \
      -o jsonpath='{.status.readyReplicas}' 2>/dev/null || echo "0")
    # readyReplicas is omitted when 0, treat empty as 0
    ready=${ready:-0}
    if [[ "${ready}" == "${expected}" ]]; then
      echo "MachineDeployment ${name} has ${expected} ready replicas"
      return 0
    fi
    echo "  readyReplicas=${ready}, expected=${expected}, retrying in 10s..."
    sleep 10
  done

  echo "Timeout waiting for MachineDeployment ${name} to reach ${expected} ready replicas"
  return 1
}

TARGET_NAMESPACE="liqo-ci"

rotate_cluster() {
  local i=$1
  local CAPI_CLUSTER_NAME
  CAPI_CLUSTER_NAME=$(forge_clustername "${i}")
  local KUBECONFIG="${TMPDIR}/kubeconfigs/liqo_kubeconf_${i}"

  echo "Rotating worker nodes in cluster ${CAPI_CLUSTER_NAME}"

  # Get the MachineDeployment name for this cluster
  local MD_NAME
  MD_NAME=$(${KUBECTL} get machinedeployment -n "${TARGET_NAMESPACE}" \
    -l "cluster.x-k8s.io/cluster-name=${CAPI_CLUSTER_NAME}" \
    --no-headers -o custom-columns=NAME:.metadata.name 2>/dev/null | head -1)

  if [[ -z "${MD_NAME}" ]]; then
    echo "No MachineDeployment found for cluster ${CAPI_CLUSTER_NAME}, skipping"
    return 0
  fi

  local SMALL_MD_NAME="${CAPI_CLUSTER_NAME}-md-small"
  local SMALL_MT_NAME="${CAPI_CLUSTER_NAME}-md-small"

  # Get the KubevirtMachineTemplate referenced by the existing MachineDeployment
  local EXISTING_MT_NAME
  EXISTING_MT_NAME=$(${KUBECTL} get machinedeployment -n "${TARGET_NAMESPACE}" "${MD_NAME}" \
    -o jsonpath='{.spec.template.spec.infrastructureRef.name}')

  # Create a smaller KubevirtMachineTemplate (1 core, 2Gi) by cloning the existing one
  echo "Creating small KubevirtMachineTemplate ${SMALL_MT_NAME} for cluster ${CAPI_CLUSTER_NAME}"
  ${KUBECTL} get kubevirtmachinetemplate -n "${TARGET_NAMESPACE}" "${EXISTING_MT_NAME}" -o json | \
    jq --arg name "${SMALL_MT_NAME}" \
      'del(.metadata.resourceVersion,.metadata.uid,.metadata.creationTimestamp,.metadata.generation,.status) |
       .metadata.name = $name |
       .spec.template.spec.virtualMachineTemplate.spec.template.spec.domain.cpu.cores = 1 |
       .spec.template.spec.virtualMachineTemplate.spec.template.spec.domain.memory.guest = "2Gi"' | \
    ${KUBECTL} apply -f -

  # Create a new MachineDeployment with 1 replica using the small template.
  # Update the deployment-name label so the MD selector stays unique within the cluster.
  echo "Creating small MachineDeployment ${SMALL_MD_NAME} for cluster ${CAPI_CLUSTER_NAME}"
  ${KUBECTL} get machinedeployment -n "${TARGET_NAMESPACE}" "${MD_NAME}" -o json | \
    jq --arg mdname "${SMALL_MD_NAME}" \
       --arg mtname "${SMALL_MT_NAME}" \
      'del(.metadata.resourceVersion,.metadata.uid,.metadata.creationTimestamp,.metadata.generation,.status) |
       .metadata.name = $mdname |
       .spec.replicas = 1 |
       .spec.selector.matchLabels["cluster.x-k8s.io/deployment-name"] = $mdname |
       .spec.template.metadata.labels["cluster.x-k8s.io/deployment-name"] = $mdname |
       .spec.template.spec.infrastructureRef.name = $mtname' | \
    ${KUBECTL} apply -f -

  wait_machinedeployment_ready "${TARGET_NAMESPACE}" "${SMALL_MD_NAME}" 1

  # Wait for the new node to join and be ready in the workload cluster
  echo "Waiting for all nodes to be ready in cluster ${CAPI_CLUSTER_NAME}"
  waitandretry 10s 30 \
    "${KUBECTL} wait nodes --all --for=condition=Ready=true --timeout=5m --kubeconfig ${KUBECONFIG}"

  echo "Node rotation completed for cluster ${CAPI_CLUSTER_NAME}"
}

PIDS=()
for i in $(seq 1 "${CLUSTER_NUMBER}"); do
  rotate_cluster "${i}" &
  PIDS+=($!)
done

FAILED=0
for pid in "${PIDS[@]}"; do
  if ! wait "${pid}"; then
    FAILED=1
  fi
done

if [[ "${FAILED}" -ne 0 ]]; then
  echo "One or more cluster node rotations failed"
  exit 1
fi
