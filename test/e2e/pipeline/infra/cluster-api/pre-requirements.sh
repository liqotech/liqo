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

  # list is available for k3d at https://github.com/k3d-io/k3d/releases
  # kubectl supported architecture list is a superset of the K3D one. No need to further compatibility check.
  local supported="darwin-amd64\n\nlinux-amd64\nlinux-arm64\nwindows-amd64"
  if ! echo "${supported}" | grep -q "${OS}-${ARCH}"; then
    echo "Error: No version of k3d for '${OS}-${ARCH}'"
    return 1
  fi

}

setup_arch_and_os

if ! command -v "${KUBECTL}" &> /dev/null
then
    echo "WARNING: kubectl could not be found. Downloading and installing it locally..."
    if ! curl --fail -Lo "${KUBECTL}" "https://storage.googleapis.com/kubernetes-release/release/$(curl -s https://storage.googleapis.com/kubernetes-release/release/stable.txt)/bin/${OS}/${ARCH}/kubectl"; then
        echo "Error: Unable to download kubectl for '${OS}-${ARCH}'"
        return 1
    fi
fi
chmod +x "${KUBECTL}"
echo "kubectl version:"
"${KUBECTL}" version --client

if ! command -v "${HELM}" &> /dev/null
then
    echo "WARNING: helm could not be found. Downloading and installing it locally..."
    if ! curl --fail -Lo "./helm-v3.12.3-${OS}-${ARCH}.tar.gz" "https://get.helm.sh/helm-v3.12.3-${OS}-${ARCH}.tar.gz"; then
        echo "Error: Unable to download helm for '${OS}-${ARCH}'"
        return 1
    fi
    tar -zxvf "helm-v3.12.3-${OS}-${ARCH}.tar.gz"
    mv "${OS}-${ARCH}/helm" "${HELM}"
    rm -rf "${OS}-${ARCH}"
fi

chmod +x "${HELM}"
echo "helm version:"
"${BINDIR}/helm" version