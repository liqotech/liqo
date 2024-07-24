#!/usr/bin/env bash
#shellcheck disable=SC1091

# Define the retry function
waitandretry() {
  local waittime="$1"
  local retries="$2"
  local command="$3"
  local options="$-" # Get the current "set" options

  sleep "${waittime}"

  echo "Running command: ${command} (retries left: ${retries})"

  # Disable set -e
  if [[ $options == *e* ]]; then
    set +e
  fi

  # Run the command, and save the exit code
  $command
  local exit_code=$?

  # restore initial options
  if [[ $options == *e* ]]; then
    set -e
  fi

  # If the exit code is non-zero (i.e. command failed), and we have not
  # reached the maximum number of retries, run the command again
  if [[ $exit_code -ne 0 && $retries -gt 0 ]]; then
    waitandretry "$waittime" $((retries - 1)) "$command"
  else
    # Return the exit code from the command
    return $exit_code
  fi
}

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
}


function check_supported_arch_and_os(){
  local supported=$1
  local os=$2
  local arch=$3
  local resource=$4

  if ! echo "${supported}" | grep -q "${os}-${arch}"; then
    echo "Error: No version of ${resource} for '${os}-${arch}'"
    return 1
  fi
}

function install_kubectl() {
  local os=$1
  local arch=$2
  local version=$3

  if [ -z "${version}" ]; then
    version=$(curl -L -s https://dl.k8s.io/release/stable.txt)
  fi

  if ! command -v "${KUBECTL}" &> /dev/null
  then
      echo "WARNING: kubectl could not be found. Downloading and installing it locally..."
      if ! curl --fail -Lo "${KUBECTL}" "https://dl.k8s.io/release/${version}/bin/${os}/${arch}/kubectl"; then
          echo "Error: Unable to download kubectl for '${os}-${arch}'"
          return 1
      fi
  fi

  chmod +x "${KUBECTL}"
  echo "kubectl version:"
  "${KUBECTL}" version --client
}

function install_helm() {
  local os=$1
  local arch=$2

  # list of helm supported architectures
	local supported="darwin-amd64\ndarwin-arm64\nlinux-386\nlinux-amd64\nlinux-arm\nlinux-arm64\nlinux-ppc64le\nlinux-s390x\nwindows-amd64"
  check_supported_arch_and_os "${supported}" "${os}" "${arch}" helm

  HELM_VERSION="v3.15.3"

  if ! command -v "${HELM}" &> /dev/null
  then
    echo "WARNING: helm could not be found. Downloading and installing it locally..."
    if ! curl --fail -Lo "./helm-${HELM_VERSION}-${os}-${arch}.tar.gz" "https://get.helm.sh/helm-${HELM_VERSION}-${os}-${arch}.tar.gz"; then
        echo "Error: Unable to download helm for '${os}-${arch}'"
        return 1
    fi
    tar -zxvf "helm-${HELM_VERSION}-${os}-${arch}.tar.gz"
    mv "${os}-${arch}/helm" "${HELM}"
    rm -rf "${os}-${arch}"
    rm "helm-${HELM_VERSION}-${os}-${arch}.tar.gz"
  fi

  chmod +x "${HELM}"
  echo "helm version:"
  "${HELM}" version
}

function install_local_path_storage() {
  local kubeconfig=$1

  "${KUBECTL}" apply -f https://raw.githubusercontent.com/rancher/local-path-provisioner/v0.0.28/deploy/local-path-storage.yaml --kubeconfig "${kubeconfig}"
  "${KUBECTL}" annotate storageclass local-path storageclass.kubernetes.io/is-default-class=true --kubeconfig "${kubeconfig}"
}

function install_metrics_server() {
    local kubeconfig=$1

    "${HELM}" repo add metrics-server https://kubernetes-sigs.github.io/metrics-server/
    "${HELM}" upgrade --install metrics-server metrics-server/metrics-server \
        --set 'args={"--kubelet-insecure-tls=true"}' \
        --namespace kube-system --kubeconfig "${kubeconfig}"
    "${KUBECTL}" -n kube-system rollout status deployment metrics-server --kubeconfig "${kubeconfig}"
}
