#!/bin/bash

set -e

here="$( cd "$( dirname "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"
# shellcheck source=/dev/null
source "$here/../common.sh"

CLUSTER_NAME_DNS=edgedns
CLUSTER_NAME_1=gslb-eu
CLUSTER_NAME_2=gslb-us

LIQO_CLUSTER_CONFIG_DNS_YAML="$here/manifests/edge-dns.yaml"
LIQO_CLUSTER_CONFIG1_YAML="$here/manifests/gslb-eu.yaml"
LIQO_CLUSTER_CONFIG2_YAML="$here/manifests/gslb-us.yaml"

check_requirements "k3d" "helm"

delete_k3d_clusters "$CLUSTER_NAME_DNS" "$CLUSTER_NAME_1" "$CLUSTER_NAME_2"

create_k3d_cluster "$CLUSTER_NAME_DNS" "$LIQO_CLUSTER_CONFIG_DNS_YAML"
create_k3d_cluster "$CLUSTER_NAME_1" "$LIQO_CLUSTER_CONFIG1_YAML"
create_k3d_cluster "$CLUSTER_NAME_2" "$LIQO_CLUSTER_CONFIG2_YAML"

KUBECONFIG_EDGE_DNS=$(get_k3d_kubeconfig $CLUSTER_NAME_DNS)
KUBECONFIG_1=$(get_k3d_kubeconfig $CLUSTER_NAME_1)
KUBECONFIG_2=$(get_k3d_kubeconfig $CLUSTER_NAME_2)


# Ensure Helm Repos

helm repo add k8gb https://www.k8gb.io &> /dev/null
helm repo add nginx-stable https://kubernetes.github.io/ingress-nginx &> /dev/null
helm repo add podinfo https://stefanprodan.github.io/podinfo &> /dev/null

helm repo update &> /dev/null


# Deploy Bind Server

info "Deploying bind server..."

fail_on_error "kubectl apply -f $here/manifests/edge/ --kubeconfig=${KUBECONFIG_EDGE_DNS}" "Failed to deploy bind server"
DNS_IP=$(kubectl get nodes --selector=node-role.kubernetes.io/master -o jsonpath='{$.items[*].status.addresses[?(@.type=="InternalIP")].address}' --kubeconfig="${KUBECONFIG_EDGE_DNS}")

success_clear_line "Bind server has been deployed."

# Deploy on cluster gslb-eu

install_k8gb "$KUBECONFIG_1" "eu" "us" "$DNS_IP"
install_ingress_nginx "$KUBECONFIG_1" "k8gb" "$here/manifests/values/nginx-ingress.yaml" "4.0.15"
install_liqo_k3d "gslb-eu" "$KUBECONFIG_1" "10.40.0.0/16" "10.30.0.0/16"

install_k8gb "$KUBECONFIG_2" "us" "eu" "$DNS_IP"
install_ingress_nginx "$KUBECONFIG_2" "k8gb" "$here/manifests/values/nginx-ingress.yaml" "4.0.15"
install_liqo_k3d "gslb-us" "$KUBECONFIG_2" "10.40.0.0/16" "10.30.0.0/16"
