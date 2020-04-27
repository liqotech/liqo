#!/bin/bash

#modify with the IP address of your local machine
localIP=10.0.4.6
clusterNum=2

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

exit 0

