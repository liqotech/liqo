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
  "CN": "${POD_NAME}",
  "names": [
      {
	"C": "IT",
  "O": "system:nodes",
	"L": "Turin",
	"OU": "Virtual Kubelet",
  "ST": "Italy"
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
     "virtual-kubelet": "true"
spec:
  request: $(cat server.csr | base64 | tr -d '\n')
  usages:
  - digital signature
  - key encipherment
  - server auth
EOF
echo "Wait for CSR to be signed"
while true;
  do
     check=$(kubectl get certificatesigningrequests.certificates.k8s.io "${POD_NAME}" -o jsonpath='{.status.conditions[:1].type}')
     if [[ $check == "Approved" ]]; then
       echo "Approved!"
       kubectl get csr "${POD_NAME}" -o jsonpath='{.status.certificate}' | base64 -d > server.crt
       exit 0
     fi
     echo "Waiting for approval of CSR: ${POD_NAME}"
     sleep 3
done

