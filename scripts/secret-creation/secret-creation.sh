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
The following flags are optional.
       --service          Service name of webhook.
       --namespace        Namespace where webhook service and secret reside.
       --secret           Secret name for CA certificate and server certificate/key pair.
       --export-env-vars  this boolean flag tells to export the env vars for the following containers
       --output-dir       The output directory for the secrets
       --output-env-file   The output directory for env variables
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
        --output-dir)
            outputdir="$2"
            shift
            ;;
        --output-env-file)
            outputenvfile="$2"
            shift
            ;;
        --export-env-vars)
            exportenvvars="true"
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
[ -z ${outputdir} ] && outputdir=/etc/ssl/liqo
[ -z ${outputenvfile} ] && outputenvfile=/etc/environment/liqo
[ -z ${exportenvvars} ] && exportenvvars=false

if [ ! -x "$(command -v openssl)" ]; then
    echo "openssl not found"
    exit 1
fi

CSR_NAME=${service}.${namespace}

tmpdir="/tmp/liqo/ssl"
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

cp ${tmpdir}/server-cert.pem ${outputdir}/server-cert.pem
cp ${tmpdir}/server-key.pem ${outputdir}/server-key.pem

if [ "$exportenvvars" = true ]; then
  {
    echo "liqocert=${outputdir}/server-cert.pem"
    echo "liqokey=${outputdir}/server-key.pem"
    echo "liqonamespace=$namespace"
    echo "liqoservice=$service"
    echo "liqosecret=$secret"
  } >> $outputenvfile
fi

exit 0
