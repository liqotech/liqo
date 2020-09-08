#!/bin/bash

echo "Cleaning: Deleting old clusters"
kind delete cluster --name liqo1
kind delete cluster --name liqo2

echo "Creating cluster Liqo1..."
kind create cluster --name liqo1 --kubeconfig liqo_kubeconf_1 --config liqo-cluster-config.yaml --wait 2m

echo "Create cluster Liqo2..."
kind create cluster --name liqo2 --kubeconfig liqo_kubeconf_2 --config liqo-cluster-config.yaml --wait 2m

echo ----------------
CURRENT_DIRECTORY=$(pwd)
echo "liqo1 KUBECONFIG=$CURRENT_DIRECTORY/liqo_kubeconf_1"
echo "liqo2 KUBECONFIG=$CURRENT_DIRECTORY/liqo_kubeconf_2"

exit 0
