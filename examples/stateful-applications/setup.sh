#!/bin/bash

set -e

here="$( cd "$( dirname "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"
# shellcheck source=/dev/null
source "$here/../common.sh"

CLUSTER_NAME_1=turin
CLUSTER_NAME_2=lyon

KUBECONFIG_1=liqo_kubeconf_turin
KUBECONFIG_2=liqo_kubeconf_lyon

LIQO_CLUSTER_CONFIG1_YAML="$here/manifests/cluster1.yaml"
LIQO_CLUSTER_CONFIG2_YAML="$here/manifests/cluster2.yaml"

check_requirements

delete_clusters "$CLUSTER_NAME_1" "$CLUSTER_NAME_2"

create_cluster "$CLUSTER_NAME_1" "$KUBECONFIG_1" "$LIQO_CLUSTER_CONFIG1_YAML"
create_cluster "$CLUSTER_NAME_2" "$KUBECONFIG_2" "$LIQO_CLUSTER_CONFIG2_YAML"

install_liqo "$CLUSTER_NAME_1" "$KUBECONFIG_1"
install_liqo "$CLUSTER_NAME_2" "$KUBECONFIG_2"
