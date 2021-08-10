#!/bin/bash
for i in $(seq 1 "${CLUSTER_NUMBER}");
do
   export KUBECONFIG="${TMPDIR}/kubeconfigs/liqo_kubeconf_${i}"
   curl "https://raw.githubusercontent.com/LiqoTech/liqo/${LIQO_VERSION}/install.sh" | bash
done;

for i in $(seq 1 "${CLUSTER_NUMBER}");
do
   export KUBECONFIG="${TMPDIR}/kubeconfigs/liqo_kubeconf_${i}"
   kubectl wait pods --timeout=200s --namespace liqo --all --for=condition=Ready
done;