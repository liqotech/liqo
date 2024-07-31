#!/bin/bash

# Consumer
export GKE_CLUSTER_ID_CONS="consumer"
export GKE_CLUSTER_ID_PROV="provider"

export regions=(
    "europe-west1"
    "europe-west2"
    "europe-west3"
)
export zones=(
    "europe-west1-c"
    "europe-west2-b"
    "europe-west3-a"
)

# General
export NUM_NODES="2"
export MACHINE_TYPE="e2-standard-2" # "e2-micro", "e2-small", "e2-medium", "e2-standard-2", "e2-standard-4"
export IMAGE_TYPE="UBUNTU_CONTAINERD" # "COS_CONTAINERD", "UBUNTU_CONTAINERD"
export DISK_TYPE="pd-balanced"
export DISK_SIZE="10"

#####################