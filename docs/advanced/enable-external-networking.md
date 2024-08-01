# Enable external networking

Similarly to the [**Use only offloading**](/advanced/use-only-offloading) feature, Liqo allows you to enable [**resource reflection**](/usage/reflection) and [**namespace offloading**](/usage/namespace-offloading) without the need to use the default Liqo networking module to establish pod-to-pod network connectivity between the clusters.

This is useful when the clusters and the pods running on them are **already connected to the same network**.
In this case, you may enable resource reflection and namespaces offloading without having to establish another network connection between the clusters.

In that case, the suggested configuration is to disable the Liqo networking module but leave the IP reflection enabled (i.e., `--set networking.enabled=false --set networking.reflectIPs=true`, see [here](AdvancedUseOnlyOffloadingDisableModule), or do not establish network connectivity between the clusters without adding the `--disable-ip-reflection` argument to the `VirtualNode` resource, see [here](AdvancedUseOnlyOffloadingSpecificCluster)).

This way, the pods will see the same IP in each cluster, and you will be able to connect to the remote pods using an external network tool.
