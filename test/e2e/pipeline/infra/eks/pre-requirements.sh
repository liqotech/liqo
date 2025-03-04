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
# EKSCTL                -> the path where eksctl is stored
# AWS_CLI               -> the path where aws-cli is stored
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

install_kubectl "${OS}" "${ARCH}" "${K8S_VERSION}"

install_helm "${OS}" "${ARCH}"

if ! command -v "${EKSCTL}" &> /dev/null
then
    ARCH=amd64
    PLATFORM=$(uname -s)_$ARCH
    echo "WARNING: eksctl could not be found. Downloading and installing it locally..."
    if ! curl --fail -sLO "https://github.com/eksctl-io/eksctl/releases/latest/download/eksctl_$PLATFORM.tar.gz"; then
        echo "Error: Unable to download eksctl for '${OS}-${ARCH}'"
        return 1
    fi
    tar -xzf "eksctl_$PLATFORM.tar.gz" && rm "eksctl_$PLATFORM.tar.gz"
    chmod +x eksctl
    mv eksctl "${EKSCTL}"
fi
echo "eksctl version:"
"${EKSCTL}" version

if ! command -v "${AWS_CLI}" &> /dev/null
then
    case $ARCH in
      arm64) AWS_ARCH="aarch64";;
      arm) AWS_ARCH="aarch64";;
      armv5) AWS_ARCH="aaarch64";;
      armv6) AWS_ARCH="aarch64";;
      amd64) AWS_ARCH="x86_64";;
      386) AWS_ARCH="x86_64";;
      *) echo "Error architecture '${ARCH}' unknown"; exit 1 ;;
    esac
    echo "WARNING: aws-cli could not be found. Downloading and installing it locally..."
    if ! curl --fail -sLO "https://awscli.amazonaws.com/awscli-exe-linux-$AWS_ARCH.zip"; then
        echo "Error: Unable to download aws-cli for '${OS}-${ARCH}'"
        return 1
    fi
    unzip awscli-exe-linux-${AWS_ARCH}.zip
    ./aws/install -i "${BINDIR}/aws-tmp" -b "${BINDIR}"
    rm -rf aws awscli-exe-linux-${AWS_ARCH}.zip
    chmod +x "${AWS_CLI}"
fi
echo "aws-cli version:"
"${AWS_CLI}" --version
