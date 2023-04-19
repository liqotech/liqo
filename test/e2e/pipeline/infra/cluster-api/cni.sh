#!/usr/bin/env bash

function install_calico () {
    local kubeconfig=$1
    curl https://raw.githubusercontent.com/projectcalico/calico/v3.24.4/manifests/calico.yaml \
        | sed -E 's|^( +)# (- name: CALICO_IPV4POOL_CIDR)$|\1\2|g;'\
"s|^( +)# (  value: )\"192.168.0.0/16\"|\1\2\"$POD_CIDR\"|g;"\
'/- name: CLUSTER_TYPE/{ n; s/( +value: ").+/\1k8s"/g };'\
'/- name: CALICO_IPV4POOL_IPIP/{ n; s/value: "Always"/value: "Never"/ };'\
'/- name: CALICO_IPV4POOL_VXLAN/{ n; s/value: "Never"/value: "Always"/};'\
'/# Set Felix endpoint to host default action to ACCEPT./a\            - name: FELIX_VXLANPORT\n              value: "6789"' \
        | "${KUBECTL}" apply -f - --kubeconfig "$kubeconfig"
}
