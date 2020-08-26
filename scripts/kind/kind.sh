#!/bin/bash

#!/bin/bash

kind delete cluster --name liqo1
kind delete cluster --name liqo2
echo create cluster liqo1...
kind create cluster --name liqo1 --kubeconfig liqo_kubeconf_1 --config liqo-cluster-config.yaml --wait 2m
export KUBECONFIG=$(pwd)/liqo_kubeconf_1
kubectl label no liqo1-worker1 liqonet.liqo.io/gateway=true
echo liqo1 installed and configured
echo installing liqo...
curl https://raw.githubusercontent.com/LiqoTech/liqo/master/install.sh | bash
echo COMPLETED
echo ----------------
echo create cluster liqo2...
kind create cluster --name liqo2 --kubeconfig liqo_kubeconf_2 --config liqo-cluster-config.yaml --wait 2m
export KUBECONFIG=$(pwd)/liqo_kubeconf_2
kubectl label no liqo2-worker2 liqonet.liqo.io/gateway=true
echo liqo2 installed and configured
echo installing liqo...
curl https://raw.githubusercontent.com/LiqoTech/liqo/master/install.sh | bash
echo COMPLETED
echo ----------------
echo "liqo1 KUBECONFIG=$(pwd)/liqo_kubeconf_1"
echo "liqo2 KUBECONFIG=$(pwd)/liqo_kubeconf_2"

exit 0



