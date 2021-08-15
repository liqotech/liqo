#!/bin/bash

KIND_VERSION="v0.11.1"

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

if ! command -v kubectl &> /dev/null
then
    echo "WARNING: kubectl could not be found. Downloading and installing it locally..."
    if ! curl --fail -Lo "${BINDIR}"/kubectl "https://storage.googleapis.com/kubernetes-release/release/$(curl -s https://storage.googleapis.com/kubernetes-release/release/stable.txt)/bin/${OS}/${ARCH}/kubectl"; then
        echo "Error: Unable to download kubectl for '${OS}-${ARCH}'"
        return 1
    fi
    chmod +x "${BINDIR}"/kubectl
    export PATH=${PATH}:${BINDIR}
fi

if [[ ! -f "${BINDIR}/kind" ]]; then
    echo "kind could not be found. Downloading..."
	curl -Lo "${BINDIR}"/kind https://kind.sigs.k8s.io/dl/${KIND_VERSION}/kind-${OS}-${ARCH}
	chmod +x "${BINDIR}"/kind
fi

