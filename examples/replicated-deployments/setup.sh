#!/usr/bin/env bash

set -e

here="$( cd "$( dirname "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"
# shellcheck source=/dev/null
source "$here/../common.sh"

CLUSTER_NAME_ORIGIN=europe-cloud
CLUSTER_NAME_DESTINATION1=europe-rome-edge
CLUSTER_NAME_DESTINATION2=europe-milan-edge

CLUSTER_LABEL_ORIGIN="topology.liqo.io/type=origin"
CLUSTER_LABEL_DESTINATION="topology.liqo.io/type=destination"

KUBECONFIG_ORIGIN=liqo_kubeconf_europe-cloud
KUBECONFIG_DESTINATION1=liqo_kubeconf_europe-rome-edge
KUBECONFIG_DESTINATION2=liqo_kubeconf_europe-milan-edge

LIQO_CLUSTER_CONFIG_YAML="$here/manifests/cluster.yaml"

check_requirements

delete_clusters "$CLUSTER_NAME_ORIGIN" "$CLUSTER_NAME_DESTINATION1" "$CLUSTER_NAME_DESTINATION2"

create_cluster "$CLUSTER_NAME_ORIGIN" "$KUBECONFIG_ORIGIN" "$LIQO_CLUSTER_CONFIG_YAML"
create_cluster "$CLUSTER_NAME_DESTINATION1" "$KUBECONFIG_DESTINATION1" "$LIQO_CLUSTER_CONFIG_YAML"
create_cluster "$CLUSTER_NAME_DESTINATION2" "$KUBECONFIG_DESTINATION2" "$LIQO_CLUSTER_CONFIG_YAML"

install_liqo "$CLUSTER_NAME_ORIGIN" "$KUBECONFIG_ORIGIN" "$CLUSTER_LABEL_ORIGIN"
install_liqo "$CLUSTER_NAME_DESTINATION1" "$KUBECONFIG_DESTINATION1" "$CLUSTER_LABEL_DESTINATION"
install_liqo "$CLUSTER_NAME_DESTINATION2" "$KUBECONFIG_DESTINATION2" "$CLUSTER_LABEL_DESTINATION"
