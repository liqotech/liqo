#!/usr/bin/env bash
cd "$1" || exit 1
pwd
cleanup ()
{
kill -s SIGTERM $$
exit 0
}

trap cleanup SIGINT SIGTERM

cat <<EOF | cfssl genkey - | cfssljson -bare server
{
  "hosts": [
    "${POD_IP}"
  ],
  "CN": "system:node:${NODE_NAME}",
  "names": [
      {
  "O": "system:nodes"
      }
    ],
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
  name: ${POD_NAME}
  labels:
     "liqo.io/csr": "true"
spec:
  request: $(< server.csr base64 | tr -d '\n')
  signerName: kubernetes.io/kubelet-serving
  usages:
  - digital signature
  - key encipherment
  - server auth
EOF

echo "Wait for CSR to be signed"
while true;
do
   cert=$(kubectl get csr "${POD_NAME}" -o jsonpath='{.status.certificate}')
   if [[ -n $cert ]]; then
     echo "certificate signed!"
     echo "$cert" | base64 -d > server.crt
     exit 0
   fi
   echo "Waiting for signing of CSR: ${POD_NAME}"
   sleep 3
done

