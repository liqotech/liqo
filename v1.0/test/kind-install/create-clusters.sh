#!/bin/bash

# This script handles the creation of multiple clusters using kind

source "$(dirname "${BASH_SOURCE}")/util.sh"

NUM_CLUSTERS="${NUM_CLUSTERS:-2}"
KIND_IMAGE="${KIND_IMAGE:-}"
KIND_TAG="${KIND_TAG:-}"

cluster_name="clustertest"
cluster_conf="cluster.yaml"

function create-clusters() {
  local num_clusters=${1}

  local image_arg=""
  if [[ "${KIND_IMAGE}" ]]; then
    image_arg="--image=${KIND_IMAGE}"
  elif [[ "${KIND_TAG}" ]]; then
    image_arg="--image=kindest/node:${KIND_TAG}"
  fi
  
  for i in $(seq ${num_clusters}); do
    echo " --------------- Number $i --------------- "
    kind create cluster --name "${cluster_name}${i}" --config ${cluster_conf} ${image_arg}
    
    echo "Number $i Created. Fix it... "
    # remove once all workarounds are addressed.
    fixup-cluster ${i}

    echo
  done

  echo "Waiting for clusters to be ready"
  check-clusters-ready ${num_clusters}
}

function fixup-cluster() {
  local i=${1} # cluster num

  local kubeconfig_path="$(kind get kubeconfig-path --name ${cluster_name}${i})"
  export KUBECONFIG="${KUBECONFIG:-}:${kubeconfig_path}"

  if [ "$OS" != "Darwin" ];then
    # Set container IP address as kube API endpoint in order for clusters to reach kube API servers in other clusters.
    kind get kubeconfig --name "${cluster_name}${i}" --internal >${kubeconfig_path}
  fi

  # Simplify context name
  kubectl config rename-context "kubernetes-admin@${cluster_name}${i}" "${cluster_name}${i}"

  # Need to rename auth user name to avoid conflicts when using multiple cluster kubeconfigs.
  sed -i.bak "s/kubernetes-admin/kubernetes-${cluster_name}${i}-admin/" ${kubeconfig_path} && rm -rf ${kubeconfig_path}.bak
}

function check-clusters-ready() {
  for i in $(seq ${1}); do
    local kubeconfig_path="$(kind get kubeconfig-path --name ${cluster_name}${i})"
    util::wait-for-condition 'ok' "kubectl --kubeconfig ${kubeconfig_path} --context ${cluster_name}${i} get --raw=/healthz &> /dev/null" 120
  done
}

echo "Creating ${NUM_CLUSTERS} clusters"
create-clusters ${NUM_CLUSTERS}

echo "Complete"
