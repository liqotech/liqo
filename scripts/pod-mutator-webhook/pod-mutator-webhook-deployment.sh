#!/bin/bash

set -e

usage() {
    cat <<EOF
Create the mutating webhook deputed to add pod tolerations to
virtual node taints
The following flags are optional.
       --input-env-file   The output directory for env variables
EOF
    exit 1
}

while [[ $# -gt 0 ]]; do
    case ${1} in
        --input-env-file)
            inputenvfile="$2"
            shift
            ;;
        *)
            usage
            ;;
    esac
    shift
done

[ -z ${inputenvfile} ] && inputenvfile=/etc/environment/liqo/env

# shellcheck source=/dev/null
source ${inputenvfile}

CACRT=$(cat /var/run/secrets/kubernetes.io/serviceaccount/ca.crt | base64 | sed ':a;N;$!ba;s/\n//g')

# shellcheck disable=SC2154
cat <<EOF | kubectl apply -f -
apiVersion: admissionregistration.k8s.io/v1beta1
kind: MutatingWebhookConfiguration
metadata:
  name: mutatepodtoleration
  namespace: $liqonamespace
  labels:
    app: mutatepodtoleration
webhooks:
  - name: mutatepodtoleration.$liqonamespace.$liqoservice
    clientConfig:
      caBundle: $CACRT
      service:
        name: $liqoservice
        namespace: $liqonamespace
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