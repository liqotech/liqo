#!/bin/bash

export GKE_REGIONS=(
    "europe-west1"
    "europe-west2"
    "europe-west3"
)
export GKE_ZONES=(
    "europe-west1-c"
    "europe-west2-b"
    "europe-west3-a"
)

# General
export GKE_NUM_NODES="2"
export GKE_MACHINE_TYPE="e2-standard-4" # "e2-micro", "e2-small", "e2-medium", "e2-standard-2", "e2-standard-4"
export GKE_DISK_TYPE="pd-ssd"
export GKE_DISK_SIZE="50"

#####################