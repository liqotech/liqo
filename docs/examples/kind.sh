#!/bin/bash
CLUSTER_NAME=cluster
CLUSTER_NAME_1=${CLUSTER_NAME}1
CLUSTER_NAME_2=${CLUSTER_NAME}2
KIND_VERSION="v0.9.0"
echo "Downloading Kind ${KIND_VERSION}"
TMPDIR=$(mktemp -d -t liqo-install.XXXXXXXXXX)
BINDIR="${TMPDIR}/bin"
mkdir --parent "${BINDIR}"
curl -Lo "${BINDIR}"/kind https://kind.sigs.k8s.io/dl/${KIND_VERSION}/kind-linux-amd64
chmod +x "${BINDIR}"/kind
KIND="${BINDIR}/kind"

echo "Cleaning: Deleting old clusters"
${KIND} delete cluster --name $CLUSTER_NAME_1
${KIND} delete cluster --name $CLUSTER_NAME_2

cat << EOF > liqo-cluster-config.yaml
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
networking:
  serviceSubnet: "10.90.0.0/12"
  podSubnet: "10.200.0.0/16"
nodes:
  - role: control-plane
    image: kindest/node:v1.19.1
  - role: worker
    image: kindest/node:v1.19.1
EOF

echo "Creating cluster $CLUSTER_NAME_1..."
${KIND} create cluster --name $CLUSTER_NAME_1 --kubeconfig liqo_kubeconf_1 --config liqo-cluster-config.yaml --wait 2m

echo "Create cluster $CLUSTER_NAME_2..."
${KIND} create cluster --name $CLUSTER_NAME_2 --kubeconfig liqo_kubeconf_2 --config liqo-cluster-config.yaml --wait 2m

echo ----------------
#Environment variables for E2E testing
CURRENT_DIRECTORY=$(pwd)
#Environment variables for E2E testing
export KUBECONFIG_1=${CURRENT_DIRECTORY}/liqo_kubeconf_1
export KUBECONFIG_2=${CURRENT_DIRECTORY}/liqo_kubeconf_2
export NAMESPACE=liqo
echo "Exported Variables:"
echo "- KUBECONFIG_1=${KUBECONFIG_1}"
echo "- KUBECONFIG_2=${KUBECONFIG_2}"
echo "- NAMESPACE=liqo"
# shellcheck disable=SC2016
echo "If you want to select $CLUSTER_NAME_1, you should simply type:" 'export KUBECONFIG=$KUBECONFIG_1'
# shellcheck disable=SC2016
echo "If you want to select $CLUSTER_NAME_2, you should simply type:" 'export KUBECONFIG=$KUBECONFIG_2'
