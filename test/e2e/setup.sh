#!/usr/bin/env bash

set +e
kind delete cluster --name remote
kind delete cluster --name origin
set -e
kind create cluster --name remote --kubeconfig remote
kind create cluster --name origin --kubeconfig origin
export KUBECONFIG=origin
cat <<EOF | cfssl genkey - | cfssljson -bare server
{
  "hosts": [
    "my-svc.my-namespace.svc.cluster.local",
    "10.0.34.2"
  ],
  "CN": "remote",
  "names": [
      {
	"C": "IT",
        "O": "system:nodes",
	"L": "Portland",
	"OU": "Virtual Kubelet 4 Ever",
        "ST": "Oregon"
      }
    ]
  "key": {
    "algo": "ecdsa",
    "size": 256
  }
}
EOF
cat <<EOF | kubectl apply -f -
apiVersion: certificates.k8s.io/v1beta1
kind: CertificateSigningRequest
metadata:
  name: my-svc.my-namespace
spec:
  request: $(cat server.csr | base64 | tr -d '\n')
  usages:
  - digital signature
  - key encipherment
  - server auth
EOF
kubectl describe csr my-svc.my-namespace
kubectl get csr
kubectl get csr my-svc.my-namespace -o jsonpath='{.status.certificate}' \
    | base64 --decode > server.crt
