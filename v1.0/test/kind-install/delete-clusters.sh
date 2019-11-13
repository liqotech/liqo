#!/bin/bash

# This script handles the deletion of multiple clusters using kind

NUM_CLUSTERS="${NUM_CLUSTERS:-2}"

cluster_name="clustertest"

function delete-clusters() {
  local num_clusters=${1}

  for i in $(seq ${num_clusters}); do
    kind delete cluster --name ${cluster_name}${i}
  done
}

echo "Deleting ${NUM_CLUSTERS} clusters"
delete-clusters ${NUM_CLUSTERS}

echo "Complete"