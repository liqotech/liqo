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

FILEPATH=$(realpath "$0")
WORKDIR=$(dirname "$FILEPATH")

# shellcheck disable=SC1091
source "$HOME/.bashrc" || true

# shellcheck disable=SC1091
# shellcheck source=../../utils.sh
source "$WORKDIR/../../utils.sh"  

check_host_login() {
  local host=$1
  local user=$2
  local key=$3
  local timeout=${4:-"600"}

  s=$(date +%s)
  local start=${s}
  while true; do
    if ssh -i "${key}" -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o ConnectTimeout=5 "${user}@${host}" exit; then
      break
    fi
    if [[ $(( $(date +%s) - start )) -gt ${timeout} ]]; then
      echo "Timeout reached while waiting for the host to be reachable"
      exit 1
    fi
    sleep 5
  done

  sleep 5

  # check apt is able to take the lock
  start=$(date +%s)
  while true; do
    if ssh -i "${key}" -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o ConnectTimeout=5 "${user}@${host}" sudo apt update; then
      break
    fi
    if [[ $(( $(date +%s) - start )) -gt ${timeout} ]]; then
      echo "Timeout reached while waiting for apt to be available"
      exit 1
    fi
    sleep 5
  done
}

TARGET_NAMESPACE="liqo-ci"

BASE_DIR=$(dirname "$0")

export SERVICE_CIDR=10.100.0.0/16
export POD_CIDR=10.200.0.0/16
export POD_CIDR_OVERLAPPING=${POD_CIDR_OVERLAPPING:-"false"}

for i in $(seq 1 "${CLUSTER_NUMBER}");
do
  K3S_CLUSTER_NAME=$(forge_clustername "${i}")
	echo "Creating cluster ${K3S_CLUSTER_NAME}"
  CLUSTER_NAME="$K3S_CLUSTER_NAME" envsubst < "$BASE_DIR/vms.template.yaml" | "${KUBECTL}" apply -n "${TARGET_NAMESPACE}" -f -
done

# Wait for the clusters to be ready
for i in $(seq 1 "${CLUSTER_NUMBER}");
do
  K3S_CLUSTER_NAME=$(forge_clustername "${i}")
  "${KUBECTL}" wait --for=condition=Ready --timeout=20m vm "${K3S_CLUSTER_NAME}-control-plane" -n "${TARGET_NAMESPACE}"
  "${KUBECTL}" wait --for=condition=Ready --timeout=20m vm "${K3S_CLUSTER_NAME}-worker-1" -n "${TARGET_NAMESPACE}"
  "${KUBECTL}" wait --for=condition=Ready --timeout=20m vm "${K3S_CLUSTER_NAME}-worker-2" -n "${TARGET_NAMESPACE}"

  "${KUBECTL}" wait --for=condition=Ready --timeout=20m vmi "${K3S_CLUSTER_NAME}-control-plane" -n "${TARGET_NAMESPACE}"
  "${KUBECTL}" wait --for=condition=Ready --timeout=20m vmi "${K3S_CLUSTER_NAME}-worker-1" -n "${TARGET_NAMESPACE}"
  "${KUBECTL}" wait --for=condition=Ready --timeout=20m vmi "${K3S_CLUSTER_NAME}-worker-2" -n "${TARGET_NAMESPACE}"
done

SSH_KEY_FILE="${TMPDIR}/id_rsa"
echo "${SSH_KEY_PATH}" > "${SSH_KEY_FILE}"
chmod 600 "${SSH_KEY_FILE}"

rm -rf k3s-ansible || true
git clone https://github.com/k3s-io/k3s-ansible.git
cd k3s-ansible

for i in $(seq 1 "${CLUSTER_NUMBER}");
do
  K3S_CLUSTER_NAME=$(forge_clustername "${i}")

  if [[ ${POD_CIDR_OVERLAPPING} != "true" ]]; then
		# this should avoid the ipam to reserve a pod CIDR of another cluster as local external CIDR causing remapping
		export POD_CIDR="10.$((i * 10)).0.0/16"
	fi

  _CONTROL_PLANE_IP=$("${KUBECTL}" get vmi "${K3S_CLUSTER_NAME}-control-plane" -n "${TARGET_NAMESPACE}" -o jsonpath='{.status.interfaces[0].ipAddress}')
  _WORKER_1_IP=$("${KUBECTL}" get vmi "${K3S_CLUSTER_NAME}-worker-1" -n "${TARGET_NAMESPACE}" -o jsonpath='{.status.interfaces[0].ipAddress}')
  _WORKER_2_IP=$("${KUBECTL}" get vmi "${K3S_CLUSTER_NAME}-worker-2" -n "${TARGET_NAMESPACE}" -o jsonpath='{.status.interfaces[0].ipAddress}')
  export CONTROL_PLANE_IP="${_CONTROL_PLANE_IP}"
  export WORKER_1_IP="${_WORKER_1_IP}"
  export WORKER_2_IP="${_WORKER_2_IP}"

  check_host_login "${CONTROL_PLANE_IP}" "ubuntu" "${SSH_KEY_FILE}"
  check_host_login "${WORKER_1_IP}" "ubuntu" "${SSH_KEY_FILE}"
  check_host_login "${WORKER_2_IP}" "ubuntu" "${SSH_KEY_FILE}"

  # if running in GitHub Actions
  if [[ -n "${GITHUB_ACTIONS}" ]]; then
    sudo python3 "${BASE_DIR}/ansible-blocking-io.py"
  fi

  ansible-playbook --version
  envsubst < "$BASE_DIR/inventory.template.yml" > inventory.yml
  ansible-playbook playbooks/site.yml -i inventory.yml --key-file "${SSH_KEY_FILE}"

  mkdir -p "${TMPDIR}/kubeconfigs"
  scp -i "${SSH_KEY_FILE}" -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null ubuntu@"${CONTROL_PLANE_IP}":~/.kube/config "${TMPDIR}/kubeconfigs/liqo_kubeconf_${i}"
  sed -i "s/127.0.0.1/${CONTROL_PLANE_IP}/g" "${TMPDIR}/kubeconfigs/liqo_kubeconf_${i}"

  # add default namespace to kubeconfig
  KUBECONFIG="${TMPDIR}/kubeconfigs/liqo_kubeconf_${i}" "${KUBECTL}" config set-context --current --namespace=default
done

cd ..
