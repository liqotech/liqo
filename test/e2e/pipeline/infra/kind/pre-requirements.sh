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

KIND_VERSION="v0.17.0"

function setup_arch_and_os(){
  ARCH=$(uname -m)
  case $ARCH in
    armv5*) ARCH="armv5";;
    armv6*) ARCH="armv6";;
    armv7*) ARCH="arm";;
    aarch64) ARCH="arm64";;
    x86) ARCH="386";;
    x86_64) ARCH="amd64";;
    i686) ARCH="386";;
    i386) ARCH="386";;
    *) echo "Error architecture '${ARCH}' unknown"; exit 1 ;;
  esac

  OS=$(uname |tr '[:upper:]' '[:lower:]')
  case "$OS" in
    # Minimalist GNU for Windows
    "mingw"*) OS='windows'; return ;;
  esac

  # list is available for kind at https://github.com/kubernetes-sigs/kind/releases
  # kubectl supported architecture list is a superset of the Kind one. No need to further compatibility check.
  local supported="darwin-amd64\n\nlinux-amd64\nlinux-arm64\nlinux-ppc64le\nwindows-amd64"
  if ! echo "${supported}" | grep -q "${OS}-${ARCH}"; then
    echo "Error: No version of kind for '${OS}-${ARCH}'"
    return 1
  fi

}

setup_arch_and_os



echo "Downloading Kind ${KIND_VERSION}"

if ! command -v docker &> /dev/null;
then
	echo "MISSING REQUIREMENT: docker engine could not be found on your system. Please install docker engine to continue: https://docs.docker.com/get-docker/"
	return 1
fi

if ! curl --fail -Lo "${KUBECTL}" "https://storage.googleapis.com/kubernetes-release/release/$(curl -s https://storage.googleapis.com/kubernetes-release/release/stable.txt)/bin/${OS}/${ARCH}/kubectl"; then
    echo "Error: Unable to download kubectl for '${OS}-${ARCH}'"
    return 1
fi
chmod +x "${KUBECTL}"
"${KUBECTL}" version --client

if [[ ! -f "${BINDIR}/kind" ]]; then
    echo "kind could not be found. Downloading https://kind.sigs.k8s.io/dl/${KIND_VERSION}/kind-${OS}-${ARCH} ..."
	curl -Lo "${BINDIR}"/kind "https://kind.sigs.k8s.io/dl/${KIND_VERSION}/kind-${OS}-${ARCH}"
	chmod +x "${BINDIR}"/kind
fi
