#!/bin/bash

set -e

here="$( cd "$( dirname "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"
# shellcheck source=/dev/null
source "$here/../common.sh"

CLUSTER_NAME_1=london
CLUSTER_NAME_2=newyork

KUBECONFIG_1=liqo_kubeconf_london
KUBECONFIG_2=liqo_kubeconf_newyork

LIQO_CLUSTER_CONFIG_YAML="$here/manifests/cluster.yaml"

check_requirements

delete_clusters "$CLUSTER_NAME_1" "$CLUSTER_NAME_2"

create_cluster "$CLUSTER_NAME_1" "$KUBECONFIG_1" "$LIQO_CLUSTER_CONFIG_YAML"
create_cluster "$CLUSTER_NAME_2" "$KUBECONFIG_2" "$LIQO_CLUSTER_CONFIG_YAML"

install_liqo "$CLUSTER_NAME_1" "$KUBECONFIG_1"
install_liqo "$CLUSTER_NAME_2" "$KUBECONFIG_2"
