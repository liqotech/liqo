#!/bin/bash

set -e

usage() {
    cat <<EOF
Generate certificate suitable for use with an sidecar-injector webhook service.
This script uses k8s' CertificateSigningRequest API to a generate a
certificate signed by k8s CA suitable for use with sidecar-injector webhook
services. This requires permissions to create and approve CSR. See
https://kubernetes.io/docs/tasks/tls/managing-tls-in-a-cluster for
detailed explantion and additional instructions.
The server key/cert k8s CA cert are stored in a k8s secret.
usage: ${0} [OPTIONS]
The following flags are required.
       --service          Service name of webhook.
       --namespace        Namespace where webhook service and secret reside.
       --secret           Secret name for CA certificate and server certificate/key pair.
EOF
    exit 1
}

while [[ $# -gt 0 ]]; do
    case ${1} in
        --service)
            service="$2"
            shift
            ;;
        --secret)
            secret="$2"
            shift
            ;;
        --namespace)
            namespace="$2"
            shift
            ;;
        *)
            usage
            ;;
    esac
    shift
done

[ -z ${service} ] && service=mutatepodtoleration
[ -z ${secret} ] && secret=pod-mutator-secret
[ -z ${namespace} ] && namespace=default

if [ ! -x "$(command -v openssl)" ]; then
    echo "openssl not found"
    exit 1
fi

CSR_NAME=${service}.${namespace}

tmpdir="/etc/liqo/ssl"
mkdir -p $tmpdir

echo "creating certs in tmpdir ${tmpdir} "

cat <<EOF >> ${tmpdir}/csr.conf
[req]
req_extensions = v3_req
distinguished_name = req_distinguished_name
[req_distinguished_name]
[ v3_req ]
basicConstraints = CA:FALSE
keyUsage = nonRepudiation, digitalSignature, keyEncipherment
extendedKeyUsage = serverAuth
subjectAltName = @alt_names
[alt_names]
DNS.1 = ${service}
DNS.2 = ${service}.${namespace}
DNS.3 = ${service}.${namespace}.svc
EOF

openssl genrsa -out ${tmpdir}/server-key.pem 2048
openssl req -new -key ${tmpdir}/server-key.pem -subj "/CN=${service}.${namespace}.svc" -out ${tmpdir}/server.csr -config ${tmpdir}/csr.conf

# clean-up any previously created CSR for our service. Ignore errors if not present.
kubectl delete csr ${CSR_NAME} 2>/dev/null || true

# create  server cert/key CSR and  send to k8s API
cat << EOF | kubectl create -f -
apiVersion: certificates.k8s.io/v1beta1
kind: CertificateSigningRequest
metadata:
  name: ${CSR_NAME}
spec:
  groups:
  - system:authenticated
  request: $(cat ${tmpdir}/server.csr | base64 | tr -d '\n')
  usages:
  - digital signature
  - key encipherment
  - server auth
EOF

# verify CSR has been created
while true; do
    kubectl get csr ${CSR_NAME}
    if [ "$?" -eq 0 ]; then
        break
    fi
done

echo "Wait for CSR to be signed"
while true;
  do
     check=$(kubectl get certificatesigningrequests.certificates.k8s.io "${CSR_NAME}" -o jsonpath='{.status.conditions[:1].type}')
     if [[ $check == "Approved" ]]; then
       echo "Approved!"
       kubectl get csr "${CSR_NAME}" -o jsonpath='{.status.certificate}' | base64 -d > server.crt
       break
     fi
     echo "Waiting for approval of CSR: ${CSR_NAME}"
     sleep 3
done

serverCert=$(kubectl get csr ${CSR_NAME} -o jsonpath='{.status.certificate}')
echo ${serverCert} | openssl base64 -d -A -out ${tmpdir}/server-cert.pem

# create the secret with CA cert and server cert/key
kubectl create secret generic ${secret} \
        --from-file=key.pem=${tmpdir}/server-key.pem \
        --from-file=cert.pem=${tmpdir}/server-cert.pem \
        --dry-run -o yaml |
    kubectl -n ${namespace} apply -f -

CACRT=`cat /var/run/secrets/kubernetes.io/serviceaccount/ca.crt | base64 | sed ':a;N;$!ba;s/\n//g'`

cat <<EOF | kubectl apply -f -
apiVersion: admissionregistration.k8s.io/v1beta1
kind: MutatingWebhookConfiguration
metadata:
  name: mutatepodtoleration
  namespace: $namespace
  labels:
    app: mutatepodtoleration
webhooks:
  - name: mutatepodtoleration.$namespace.$service
    clientConfig:
      caBundle: $CACRT
      service:
        name: $service
        namespace: $namespace
        path: "/mutate"
        port: 443
    rules:
      - operations: ["CREATE"]
        apiGroups: [""]
        apiVersions: ["v1"]
        resources: ["pods"]
    sideEffects: None
    timeoutSeconds: 5
    reinvocationPolicy: Never
    failurePolicy: Ignore
    namespaceSelector:
      matchLabels:
        liqo.io/enabled: "true"
EOF

exit 0
