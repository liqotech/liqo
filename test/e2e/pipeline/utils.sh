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

function forge_clustername() {
  local index=$1
  RUNNER_NAME=${RUNNER_NAME:-"test"}
  # Replace spaces and invalid characters with dashes, ensure it's valid for Kubernetes labels
  RUNNER_NAME=$(echo "${RUNNER_NAME}" | tr ' ' '-' | tr -c 'a-zA-Z0-9-_.' '-' | tr '[:upper:]' '[:lower:]' | tr -s '-')
  RUNNER_NAME=${RUNNER_NAME#liqo-runner-*-}
  local BASE_CLUSTER_NAME="cl-${RUNNER_NAME}-"
  echo "${BASE_CLUSTER_NAME}${index}"
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
    echo "Downloading https://dl.k8s.io/release/${version}/bin/${os}/${arch}/kubectl"
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
  "${HELM}" repo update
  "${HELM}" upgrade --install metrics-server metrics-server/metrics-server \
    --set 'args={"--kubelet-insecure-tls=true"}' \
    --namespace kube-system --kubeconfig "${kubeconfig}"
  "${KUBECTL}" -n kube-system rollout status deployment metrics-server --kubeconfig "${kubeconfig}"
}

function install_gcloud() {
  #Download and install gcloud
  cd "${BINDIR}"
  curl -O https://dl.google.com/dl/cloudsdk/channels/rapid/downloads/google-cloud-cli-linux-x86_64.tar.gz
  tar -xf google-cloud-cli-linux-x86_64.tar.gz
  ./google-cloud-sdk/install.sh --path-update true -q
  cd -

  #Login to gcloud
  echo "${GCLOUD_KEY}" | base64 -d > "${BINDIR}/gke_key_file.json"
  "${GCLOUD}" auth activate-service-account --key-file="${BINDIR}/gke_key_file.json"
  "${GCLOUD}" components install gke-gcloud-auth-plugin
}

function install_az() {
  local os=$1

  if ! command -v az &> /dev/null
  then
      echo "Azure CLI could not be found. Downloading and installing..."
      if [[ "${os}" == "linux" ]]
      then
          curl -sL https://aka.ms/InstallAzureCLIDeb | sudo bash
      elif [[ "${os}" == "darwin" ]]
      then
          brew update && brew install azure-cli
      else
          echo "Error: Azure CLI is not supported on ${os}"
          exit 1
      fi
  fi

  echo "Azure CLI version:"
  az --version
}

function login_az() {
  local username=$1
  local key=$2
  local tenant_id=$3

  az login --service-principal --username "${username}" --password "${key}" --tenant "${tenant_id}"
}

function install_kyverno() {
  local kubeconfig=$1

  "${HELM}" repo add kyverno https://kyverno.github.io/kyverno/
  "${HELM}" repo update
  "${HELM}" install kyverno kyverno/kyverno -n kyverno --create-namespace --kubeconfig "${kubeconfig}"
}

function wait_kyverno() {
  local kubeconfig=$1

  # Wait for the kyverno deployments to be ready
  if ! waitandretry 5s 2 "${KUBECTL} rollout status deployment -n kyverno --kubeconfig ${kubeconfig}"
  then
    echo "Failed to wait for kyverno deployments to be ready"
    exit 1
  fi
}

function install_clusterctl() {
  local os=$1
  local arch=$2

  curl -L "https://github.com/kubernetes-sigs/cluster-api/releases/download/v1.3.5/clusterctl-${os}-${arch}" -o clusterctl
  sudo install -o root -g root -m 0755 clusterctl /usr/local/bin/clusterctl
  clusterctl version
}
