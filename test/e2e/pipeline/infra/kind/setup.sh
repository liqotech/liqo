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
# DOCKER_USERNAME       -> the Dockerhub username
# DOCKER_PASSWORD       -> the Dockerhub password

set -e           # Fail in case of error
set -o nounset   # Fail if undefined variables are used
set -o pipefail  # Fail if one of the piped commands fails

error() {
   local sourcefile=$1
   local lineno=$2
   echo "An error occurred at $sourcefile:$lineno."
}
trap 'error "${BASH_SOURCE}" "${LINENO}"' ERR

CLUSTER_NAME=cluster
KIND="${BINDIR}/kind"
# Container's name for registry
REGISTRY_NAME="registry"
# A port to be used in repositories, e.g. "localhost:5000"
REGISTRY_PORT="5000"
# A path where blobs will be located.
REGISTRY_STORAGE_PATH=${TMPDIR}/registry

export DISABLE_KINDNET=false

echo Check local registry ...
regRunning="$(docker inspect -f '{{.State.Running}}' "${REGISTRY_NAME}" 2>/dev/null || true)"
REG_STARTED=0
if [ "${regRunning}" == 'true' ]; then
  echo -e " Registry is running 🎁"
else
  echo -e " No registry running, start ..."
  mkdir -p "${REGISTRY_STORAGE_PATH}"
  docker run \
    --detach \
    --restart always \
    --publish "${REGISTRY_PORT}:5000" \
    --name "${REGISTRY_NAME}" \
    --volume "${REGISTRY_STORAGE_PATH}":/var/lib/registry \
    --env REGISTRY_STORAGE_DELETE_ENABLED=true \
    --env REGISTRY_PROXY_USERNAME="${DOCKER_USERNAME}" \
    --env REGISTRY_PROXY_PASSWORD="${DOCKER_PASSWORD}" \
    registry:2
  echo -e " Registry started 🎁"
  # Set flag to connect the registry container to a "kind" network later.
  REG_STARTED=1
fi

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
done

if [ $REG_STARTED == 1 ]; then
    echo " 🔗 Connect registry container to docker network kind"
    docker network connect "kind" "${REGISTRY_NAME}"
fi
