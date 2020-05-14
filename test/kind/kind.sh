#!/bin/bash

#modify with the IP address of your local machine, DO NOT use the localhost
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

# configure clusters installing the advertisement crd and creating the configMaps containing the kubeconfig
for ((i=1;i<=clusterNum;i++)); do
  export KUBECONFIG=kubeconfig-cluster${i}
  kubectl apply -f adv-crd.yaml
  for ((j=1;j<=clusterNum;j++)); do
    if [ $i -eq $j ] ; then
      continue
    fi
    id=cluster${j}
    #kubectl config view --flatten --minify >>kubeconfig-${id}
    kubectl create configmap foreign-kubeconfig-${id} --from-file=remote=kubeconfig-${id}
  done
done

# create advertisement-operator deployment
for ((i=1;i<=clusterNum;i++)); do
  export KUBECONFIG=kubeconfig-cluster${i}
  sed -i -e "s/clusterX/cluster$i/g" adv-operator_cm.yaml
  sed -i -e "s/0.0.0.0/172.17.0.$i/g" adv-operator_cm.yaml
  sed -i -e "s/1.2.3.4/192.168.0.$i/g" adv-operator_cm.yaml
  kubectl apply -f adv-operator_cm.yaml
  sed -i -e "s/cluster$i/clusterX/g" adv-operator_cm.yaml
  sed -i -e "s/172.17.0.$i/0.0.0.0/g" adv-operator_cm.yaml
  sed -i -e "s/192.168.0.$i/1.2.3.4/g" adv-operator_cm.yaml
  kubectl apply -f adv-deploy.yaml
  kubectl apply -f broadcaster-deploy.yaml
done

exit 0

