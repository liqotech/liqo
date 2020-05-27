#!/bin/bash

set -e

usage() {
    cat <<EOF
usage: ${0} [OPTIONS]
The following flags are required.
       --kubeconfig          path to kubeconfig file.
EOF
    exit 1
}

while [[ $# -gt 0 ]]; do
    case ${1} in
        --kubeconfig)
            kubeconfig="$2"
            shift
            ;;
        *)
            usage
            ;;
    esac
    shift
done

kubectl create secret generic foreign-config --from-file=kubeconfig=$kubeconfig
kubectl apply -f foreign-cluster.yaml
