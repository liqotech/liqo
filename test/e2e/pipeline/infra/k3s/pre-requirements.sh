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

# list is available for k3d at https://github.com/k3d-io/k3d/releases
# kubectl supported architecture list is a superset of the K3D one. No need to further compatibility check.
SUPPORTED="darwin-amd64\ndarwin-arm64\nlinux-386\nlinux-amd64\nlinux-arm\nlinux-arm64\nwindows-amd64"
check_supported_arch_and_os "${SUPPORTED}" "${OS}" "${ARCH}" k3d

# shellcheck disable=SC2153
install_kubectl "${OS}" "${ARCH}" "${K8S_VERSION}"

install_helm "${OS}" "${ARCH}"

# install ansible

# ensure pipx is installed
if ! command -v pipx &> /dev/null; then
   python3 -m pip install --user pipx
   python3 -m pipx ensurepath --force
   source "$HOME/.bashrc" || true

   sudo apt update
   sudo apt install -y python3-venv
fi

# ensure envsubst is installed
if ! command -v envsubst &> /dev/null; then
   sudo apt update
   sudo apt install -y gettext
fi

# ensure ansible is installed
if ! command -v ansible &> /dev/null; then
   pipx install --include-deps ansible
   ansible-playbook --version
fi
