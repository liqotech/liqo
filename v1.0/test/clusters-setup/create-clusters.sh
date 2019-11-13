#!/bin/bash

# This script handles the creation of multiple clusters using kind

source "$(dirname "${BASH_SOURCE}")/util.sh"

NUM_CLUSTERS="${NUM_CLUSTERS:-2}"
KIND_IMAGE="${KIND_IMAGE:-}"
KIND_TAG="${KIND_TAG:-}"

cluster_name="cluster"
cluster_conf="cluster.yaml"

_username="drone"
_password="drone"
_exchange_name="drone-exchange"
_pattern=".*drone.*"
_set_name="drone"
_policy_name="federate-drone"
_namespace="drone"
federation_upstream_set=""

declare -a clusters_name
declare -a clusters_ip

function create-clusters() {
  local num_clusters=${1}

  local image_arg=""
  if [[ "${KIND_IMAGE}" ]]; then
    image_arg="--image=${KIND_IMAGE}"
  elif [[ "${KIND_TAG}" ]]; then
    image_arg="--image=kindest/node:${KIND_TAG}"
  fi
  
  for i in $(seq ${num_clusters}); do
    echo " --------------- Number $i --------------- "
    kind create cluster --name "${cluster_name}${i}" --config ${cluster_conf} ${image_arg}
    
    echo "Number $i Created. Fix it... "
    # remove once all workarounds are addressed.
    fixup-cluster ${i}

    echo "Deploy rabbitMQ... "
    deploy-rabbitMQ ${i}

    echo
  done

  echo "Waiting for clusters to be ready"
  check-clusters-ready ${num_clusters}
}

function fixup-cluster() {
  local i=${1} # cluster num

  local kubeconfig_path="$(kind get kubeconfig-path --name ${cluster_name}${i})"
  # export KUBECONFIG="${KUBECONFIG:-}:${kubeconfig_path}"
  export KUBECONFIG="$(kind get kubeconfig-path --name="${cluster_name}${i}")"

  if [ "$OS" != "Darwin" ];then
    # Set container IP address as kube API endpoint in order for clusters to reach kube API servers in other clusters.
    kind get kubeconfig --name "${cluster_name}${i}" --internal >${kubeconfig_path}
  fi

  # Simplify context name
  kubectl config rename-context "kubernetes-admin@${cluster_name}${i}" "${cluster_name}${i}"

  # Need to rename auth user name to avoid conflicts when using multiple cluster kubeconfigs.
  sed -i.bak "s/kubernetes-admin/kubernetes-${cluster_name}${i}-admin/" ${kubeconfig_path} && rm -rf ${kubeconfig_path}.bak
}

function check-clusters-ready() {
  for i in $(seq ${1}); do
    local kubeconfig_path="$(kind get kubeconfig-path --name ${cluster_name}${i})"
    util::wait-for-condition 'ok' "kubectl --kubeconfig ${kubeconfig_path} --context ${cluster_name}${i} get --raw=/healthz &> /dev/null" 120 ${cluster_name}${i}
  done
}

function deploy-rabbitMQ() {
  local i=${1} # cluster num

  export KUBECONFIG="$(kind get kubeconfig-path --name="${cluster_name}${i}")"

  clusters_name[$i]="${cluster_name}${i}"

  # Find Docker IP
  clusters_ip[${i}]=$(docker inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' "${clusters_name[${i}]}-control-plane")
  echo "${clusters_name[${i}]}-control-plane -> ${clusters_ip[${i}]}"

  # Deploy rabbitmq
  kubectl create namespace ${_namespace}
  kubectl create -f ../../rabbitMq/rabbitDeploy/deployment.yaml --namespace ${_namespace}
  kubectl create -f ../../rabbitMq/rabbitDeploy/clusterip.yaml --namespace ${_namespace}
  kubectl create -f ../../rabbitMq/rabbitDeploy/nodeport.yaml --namespace ${_namespace}
}

function check-rabbitmq-ready() {
  for i in $(seq ${1}); do
    local kubeconfig_path="$(kind get kubeconfig-path --name ${cluster_name}${i})"

    export KUBECONFIG="$(kind get kubeconfig-path --name="${cluster_name}${i}")"

    # Get name of rabbit pod
    pod_name=$(kubectl get pod -n ${_namespace} --selector=app=rabbitmq -o jsonpath="{.items[0].metadata.name}")
    # command="kubectl --kubeconfig ${kubeconfig_path} --context ${cluster_name}${i} get pod/$pod_name -n ${_namespace} -o yaml | awk '/phase:/ {print $2}'"
    # command="kubectl --kubeconfig ${kubeconfig_path} --context ${cluster_name}${i} get pod/$pod_name -n ${_namespace} -o yaml | grep 'phase:' | cut -c 10-"
    # echo $pod_name
    # echo $(kubectl --kubeconfig ${kubeconfig_path} --context ${cluster_name}${i} get pod/$pod_name -n ${_namespace} -o yaml | grep 'phase:' | cut -c 10-)

    #util::wait-for-condition 'Running' "kubectl --kubeconfig ${kubeconfig_path} --context ${cluster_name}${i} get pod/$pod_name -n ${_namespace} -o yaml | awk '/phase:/ {print $2}' &> /dev/null" 180 ${cluster_name}${i}
    while true; do
      state=$(echo $(kubectl --kubeconfig ${kubeconfig_path} --context ${cluster_name}${i} get pod/$pod_name -n ${_namespace} -o yaml | grep 'phase:' | cut -c 10-))
      if [ $state == "Running" ]; then
        echo " -> Rabbit running"
        break
      else
        echo -n '.'
      fi
      sleep 1
    done
  done
}

function federate-rabbitMQ() {
  local j=${1} # cluster num

  echo "Federate ${clusters_name[${j}]}"

  export KUBECONFIG="$(kind get kubeconfig-path --name=${clusters_name[${j}]})"

  pod_name=$(kubectl get pod -n ${_namespace} -o jsonpath="{.items[0].metadata.name}")

  kubectl exec -it $pod_name --namespace ${_namespace} -- bash -c "rabbitmq-plugins enable rabbitmq_federation rabbitmq_federation_management"
  kubectl exec -it $pod_name --namespace ${_namespace} rabbitmqctl add_user ${_username} ${_password}
  kubectl exec -it $pod_name --namespace ${_namespace} rabbitmqctl set_user_tags ${_username} administrator
  kubectl exec -it $pod_name --namespace ${_namespace} -- bash -c "rabbitmqctl set_permissions -p / ${_username} \".*\" \".*\" \".*\""

  for x in $(seq ${NUM_CLUSTERS})
  do
    if [ "${clusters_name[$x]}" != "${clusters_name[${j}]}" ]
      then
        kubectl exec -it $pod_name --namespace ${_namespace} -- bash -c "rabbitmqctl set_parameter federation-upstream ${clusters_name[$x]} '{\"uri\":\"amqp://${_username}:${_password}@${clusters_ip[$x]}:31001\"}'"
    fi
  done

  federation_upstream_set="rabbitmqctl set_parameter federation-upstream-set ${_set_name} '["
  for x in $(seq ${NUM_CLUSTERS})
  do
    if [ "${clusters_name[$x]}" != "${clusters_name[${j}]}" ]
      then
        federation_upstream_set="${federation_upstream_set} {\"upstream\":\"${clusters_name[$x]}\"},"
    fi
  done
  federation_upstream_set="${federation_upstream_set}]'"

  kubectl exec -it $pod_name --namespace ${_namespace} -- bash -c "${federation_upstream_set}"

  kubectl exec -it $pod_name --namespace ${_namespace} -- bash -c "rabbitmqctl set_policy --apply-to exchanges ${_policy_name} \"${_pattern}\" '{\"federation-upstream-set\":\"${_set_name}\"}'"

}


echo "Creating ${NUM_CLUSTERS} clusters"
create-clusters ${NUM_CLUSTERS}

echo "Waiting for Rabbit pod running"
check-rabbitmq-ready ${NUM_CLUSTERS}
# watch kubectl get deployments,pods,services --namespace ${_namespace}

for j in $(seq ${NUM_CLUSTERS}); do
  echo "Federate all RabbitMq deployment"
  federate-rabbitMQ $j

  # Install metrics-server
  # echo "Install metrics-server"
  # kubectl create -f metric/metrics-server/deploy/1.8+/

  if [ $j == 1 ]; then
    cd ..
  fi

  # Load and Deploy drone-agent
  cd ../drone
  CLUSTER_NAME=${cluster_name}${j} ./buildDockerLoadOnKindAndApply.sh

  # Create docker image, Load and Deploy drone-daemon-resources
  cd ../drone-daemon-resources
  CLUSTER_NAME=${cluster_name}${j} ./buildAppAndBuildDocker.sh 

  # Operator deploy
  cd ../drone-operator
  CLUSTER_NAME=${cluster_name}${j} ./buildLoadAndDeploy.sh
done

echo "Complete"
