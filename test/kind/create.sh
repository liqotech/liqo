#!/bin/bash

#modify with the IP address of your local machine
localIP=10.0.4.6
clusterNum=2

configKindCluster(){
  i=$1
  port=$((30000+"$i"))
  cat << EOF > cluster"$i"-config.yaml
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
networking:
  apiServerAddress: "$localIP"
  apiServerPort: $port
  podSubnet: "10.${i}00.0.0/16"
  serviceSubnet: "10.9$i.0.0/12"
EOF
  kind create cluster --name cluster"$i" --kubeconfig kubeconfig-cluster"$i" --config cluster"$i"-config.yaml --wait 2m
}

#delete all clusters
for ((i=1;i<=clusterNum;i++)); do
  kind delete cluster --name cluster"$i"
done

# create clusters
for ((i=1;i<=clusterNum;i++)); do
  configKindCluster $i
  pids[${i}]=$!
done

# wait for all pids
for pid in ${pids[*]}; do
    wait $pid
done

exit 0

