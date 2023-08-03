#!/usr/bin/env bash
#shellcheck disable=SC1091

FILEPATH=$(realpath "$0")
WORKDIR=$(dirname "$FILEPATH")

# shellcheck source=./pre-requirements.sh
source "$WORKDIR/pre-requirements.sh"

DOCKER_PROXY="${DOCKER_PROXY:-docker.io}"

function install_calico() {
    local kubeconfig=$1
    "${KUBECTL}" create -f https://raw.githubusercontent.com/projectcalico/calico/v3.26.1/manifests/tigera-operator.yaml --kubeconfig "$kubeconfig"

    # append a slash to DOCKER_PROXY if not present
    if [[ "${DOCKER_PROXY}" != */ ]]; then
        registry="${DOCKER_PROXY}/"
    else
        registry="${DOCKER_PROXY}"
    fi

    cat <<EOF > custom-resources.yaml
# This section includes base Calico installation configuration.
# For more information, see: https://projectcalico.docs.tigera.io/master/reference/installation/api#operator.tigera.io/v1.Installation
apiVersion: operator.tigera.io/v1
kind: Installation
metadata:
  name: default
spec:
  registry: $registry
  # Configures Calico networking.
  calicoNetwork:
    # Note: The ipPools section cannot be modified post-install.
    ipPools:
    - blockSize: 26
      cidr: $POD_CIDR
      encapsulation: VXLAN
      natOutgoing: Enabled
      nodeSelector: all()
    nodeAddressAutodetectionV4:
      skipInterface: liqo.*

---

# This section configures the Calico API server.
# For more information, see: https://projectcalico.docs.tigera.io/master/reference/installation/api#operator.tigera.io/v1.APIServer
apiVersion: operator.tigera.io/v1
kind: APIServer
metadata:
  name: default
spec: {}
EOF
    "${KUBECTL}" apply -f custom-resources.yaml --kubeconfig "$kubeconfig"
}

function wait_calico() {
    local kubeconfig=$1
    sleep 5
    "${KUBECTL}" wait --for condition=Ready=true -n calico-system pod --all --kubeconfig "$kubeconfig" --timeout=-1s
    sleep 10
    # set felix to use different port for VXLAN
    "${KUBECTL}" patch felixconfiguration default --type='merge' -p '{"spec":{"vxlanPort": 6789}}' --kubeconfig "$kubeconfig"
}

function install_cilium() {
    local kubeconfig=$1

    if [ ! -f "${BINDIR/cilium/}" ]; then
        setup_arch_and_os
        local CILIUM_CLI_VERSION
        CILIUM_CLI_VERSION="v0.14.0"

        echo "Downloading Cilium CLI ${CILIUM_CLI_VERSION} for ${OS}-${ARCH}"
        curl -L --remote-name-all "https://github.com/cilium/cilium-cli/releases/download/${CILIUM_CLI_VERSION}/cilium-${OS}-${ARCH}.tar.gz{,.sha256sum}"
        sha256sum --check "cilium-${OS}-${ARCH}.tar.gz.sha256sum"
        sudo tar -C "${BINDIR}" -xzvf "cilium-${OS}-${ARCH}.tar.gz"
        rm "cilium-${OS}-${ARCH}.tar.gz"
        rm "cilium-${OS}-${ARCH}.tar.gz.sha256sum"
    fi

    export KUBECONFIG="$kubeconfig"
    "${BINDIR}/cilium" install --helm-set ipam.operator.clusterPoolIPv4PodCIDRList="${POD_CIDR}"
    "${BINDIR}/cilium" status --wait
    unset KUBECONFIG
}

function wait_cilium() {
    # empty, cilium status --wait already waits for cilium pods to be ready
    true
}
