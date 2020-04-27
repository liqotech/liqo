#!/bin/bash

#modify with the IP address of your local machine
localIP=10.0.4.6
clusterNum=2

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
done

exit 0

