#!/bin/bash

set -e

here="$( cd "$( dirname "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"
# shellcheck source=/dev/null
source "$here/../common.sh"

CLUSTER_NAME_1=venice
CLUSTER_NAME_2=florence
CLUSTER_NAME_3=naples

KUBECONFIG_1=liqo_kubeconf_venice
KUBECONFIG_2=liqo_kubeconf_florence
KUBECONFIG_3=liqo_kubeconf_naples

LIQO_CLUSTER_CONFIG_YAML="$here/manifests/cluster.yaml"

check_requirements

delete_clusters "$CLUSTER_NAME_1" "$CLUSTER_NAME_2" "$CLUSTER_NAME_3"

create_cluster "$CLUSTER_NAME_1" "$KUBECONFIG_1" "$LIQO_CLUSTER_CONFIG_YAML"
create_cluster "$CLUSTER_NAME_2" "$KUBECONFIG_2" "$LIQO_CLUSTER_CONFIG_YAML"
create_cluster "$CLUSTER_NAME_3" "$KUBECONFIG_3" "$LIQO_CLUSTER_CONFIG_YAML"

install_liqo "$CLUSTER_NAME_1" "$KUBECONFIG_1" "topology.liqo.io/region=north"
install_liqo "$CLUSTER_NAME_2" "$KUBECONFIG_2" "topology.liqo.io/region=center"
install_liqo "$CLUSTER_NAME_3" "$KUBECONFIG_3" "topology.liqo.io/region=south"
