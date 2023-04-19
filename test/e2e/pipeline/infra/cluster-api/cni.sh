#!/usr/bin/env bash
#shellcheck disable=SC1091

FILEPATH=$(realpath "$0")
WORKDIR=$(dirname "$FILEPATH")

# shellcheck source=./pre-requirements.sh
source "$WORKDIR/pre-requirements.sh"

function install_calico() {
    local kubeconfig=$1
    curl https://raw.githubusercontent.com/projectcalico/calico/v3.24.4/manifests/calico.yaml |
        sed -E 's|^( +)# (- name: CALICO_IPV4POOL_CIDR)$|\1\2|g;'"\
s|^( +)# (  value: )\"192.168.0.0/16\"|\1\2\"$POD_CIDR\"|g;"'/- name: CLUSTER_TYPE/{ n; s/( +value: ").+/\1k8s"/g };''/- name: CALICO_IPV4POOL_IPIP/{ n; s/value: "Always"/value: "Never"/ };''/- name: CALICO_IPV4POOL_VXLAN/{ n; s/value: "Never"/value: "Always"/};''/# Set Felix endpoint to host default action to ACCEPT./a\            - name: FELIX_VXLANPORT\n              value: "6789"' |
        "${KUBECTL}" apply -f - --kubeconfig "$kubeconfig"
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
