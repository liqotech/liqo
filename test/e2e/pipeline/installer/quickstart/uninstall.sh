for i in $(seq 1 "${CLUSTER_NUMBER}");
do
   export KUBECONFIG="${TMPDIR}/kubeconfigs/liqo_kubeconf_${i}"
   timeout 300 bash -c "curl https://raw.githubusercontent.com/LiqoTech/liqo/${LIQO_VERSION}/install.sh | \
   bash -s -- --uninstall --purge"
done;