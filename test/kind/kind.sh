#!/bin/bash

localIP=10.0.4.6  # modify with the IP address of your local machine
clusterNum=2      # choose the number of kind clusters you want to create

# create kind cluster and its config file
configKindCluster(){
  i=$1
  port=$((30000+"$i"))
  cat << EOF > cluster"$i"-config.yaml
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
networking:
  apiServerAddress: "$localIP"
  apiServerPort: $port
EOF
  kind create cluster --name cluster"$i" --kubeconfig kubeconfig-cluster"$i" --config cluster"$i"-config.yaml --wait 1m
}


#delete all clusters
for ((i=1;i<=clusterNum;i++)); do
  kind delete cluster --name cluster"$i"
done

# create clusters
for ((i=1;i<=clusterNum;i++)); do
  configKindCluster $i &    # delete & to run sequentially
  pids[${i}]=$!
done

# wait for all pids
for pid in ${pids[*]}; do
    wait $pid
done

# configure clusters installing the advertisement crd and creating the configMaps containing the kubeconfig
for ((i=1;i<=clusterNum;i++)); do
  export KUBECONFIG=kubeconfig-cluster${i}
  kubectl apply -f adv-crd.yaml
  for ((j=1;j<=clusterNum;j++)); do
    if [ $i -eq $j ] ; then
      continue
    fi
    id=cluster${j}
    kubectl create configmap foreign-kubeconfig-${id} --from-file=remote=kubeconfig-${id}
  done
done

# create advertisement-operator deployment
for ((i=1;i<=clusterNum;i++)); do
  export KUBECONFIG=kubeconfig-cluster${i}
  sed -i -e "s/clusterX/cluster$i/g" adv-deploy.yaml
  kubectl apply -f adv-deploy.yaml
  sed -i -e "s/cluster$i/clusterX/g" adv-deploy.yaml
done

exit 0

