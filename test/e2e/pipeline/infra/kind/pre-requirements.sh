#!/bin/bash
#shellcheck disable=SC1091

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
# HELM                  -> the path where helm is stored
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
# shellcheck source=../../utils.sh
source "$WORKDIR/../../utils.sh"

setup_arch_and_os

# list is available for kind at https://github.com/kubernetes-sigs/kind/releases
# kubectl supported architecture list is a superset of the Kind one. No need to further compatibility check.
SUPPORTED="darwin-amd64\ndarwin-arm64\nlinux-amd64\nlinux-arm64\nwindows-amd64"
check_supported_arch_and_os "${SUPPORTED}" "${OS}" "${ARCH}" kind

install_kubectl "${OS}" "${ARCH}" "${K8S_VERSION}"

install_helm "${OS}" "${ARCH}"

KIND_VERSION="v0.24.0"

echo "Downloading Kind ${KIND_VERSION}"

if ! command -v docker &> /dev/null;
then
	echo "MISSING REQUIREMENT: docker engine could not be found on your system. Please install docker engine to continue: https://docs.docker.com/get-docker/"
	return 1
fi

if [[ ! -f "${BINDIR}/kind" ]]; then
    echo "kind could not be found. Downloading https://kind.sigs.k8s.io/dl/${KIND_VERSION}/kind-${OS}-${ARCH} ..."
	curl -Lo "${BINDIR}"/kind "https://kind.sigs.k8s.io/dl/${KIND_VERSION}/kind-${OS}-${ARCH}"
	chmod +x "${BINDIR}"/kind
fi
