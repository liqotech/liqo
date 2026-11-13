# IP

The IP resource serves two main purposes:

1. it allocates IPs to prevent other components from using them
2. it maps an IP (e.g. a local IP) to an external CIDR IP, allowing remote adjacent clusters to reach those IPs.

For example, if cluster A is peered with cluster B and an IP resource exists on cluster A with the "local IP" set to 10.111.129.179, its status will be updated with an IP from the local external CIDR. This external IP can then be used by adjacent clusters to reach 10.111.129.179.

This mechanism is useful for leaf-to-leaf communication because cluster A is unaware of the pod CIDR used by cluster C. Cluster B exposes cluster C's pods to A using the IP resource.

IP resources are managed by [a controller](https://github.com/liqotech/liqo/blob/978cc2ce96105507923dd167528946da4413804d/pkg/gateway/remapping/ip_controller.go) that creates these [firewall configurations](firewallconfiguration.md#name-remap-ipmapping-gw) (fwcfg) for the gateways.

## Masquerade

Please note that the firewall configurations created by the IP controller is applied by default inside the gateway pods. If you want to apply them on the nodes, you need to set the `spec.masquerade` field to `true` in the IP resource. This can be useful if you want to create an IP resource to expose a service outside the Cluster.

## \<tenant-name\>-unknown-source

This IP is the first address within the external CIDR. It is used in the **service-nodeport-routing** firewall configuration (see the relevant section). This IP is used when a packet originates from a NodePort on a leaf cluster and needs to reach a pod in another leaf cluster.

**The related _ipmapping-gw_ firewall configuration is not needed.**
