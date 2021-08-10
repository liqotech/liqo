for i in $(seq 1 ${CLUSTER_NUMBER});
do
   echo "OUTPUT CLUSTER ${i}"
   export KUBECONFIG=${TMPDIR}/kubeconfigs/liqo_kubeconf_${i}
   echo "Pods created in the cluster"
   echo "|------------------------------------------------------------|"
   kubectl get po -A -o wide
   echo "Core resources in Liqo namespace"
   echo "|------------------------------------------------------------|"
   kubectl get all -n liqo -o wide
   echo "Installed CRDs"
   echo "|------------------------------------------------------------|"
   kubectl get crd -A
   echo "Available Nodes"
   echo "|------------------------------------------------------------|"
   kubectl get no -o wide --show-labels
   echo "|------------------------------------------------------------|"
done;