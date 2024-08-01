# Requirements

This page presents an overview of the main requirements, both in terms of **resources** and **network connectivity**, to use Liqo and successfully establish peerings with remote clusters.

## Resources

Liqo requires very **limited resources** (i.e., CPU, RAM, network bandwidth), making it suitable for both traditional **K8s** clusters and **resource-constrained** clusters, e.g., the ones running K3s on a Raspberry Pi.

While the exact numbers depend on the **number of established peerings**, **number of offloaded pods**, and **size of the cluster**, as a ballpark figure the entire Liqo control plane, executed on a two-node KinD cluster, peered with one remote cluster, and while offloading 100 pods, requires less than:

* 0.5 CPU cores (only during transient periods, while CPU consumption is practically negligible in all the other instants).
* 200 MB of RAM (this metric increases the more pods are offloaded to remote clusters).
* 5 Mbps of cross-cluster control plane traffic (only during transient periods). Data plane traffic, instead, depends on the applications and their actual placements across the clusters.

However, to be on the safe side, we suggest installing Liqo on a cluster that has **at least 2 CPUs and 2 GB of RAM**, which takes into account also the resources used by standard Kubernetes components.

Liqo is guaranteed to be compatible with the **last 3 Kubernetes major releases**.
However, older versions may work as well, although they are not officially supported.

An accurate analysis of the Liqo performance compared to vanilla Kubernetes, including the characterization of the resources consumed by Liqo, is presented in a [dedicated blog post](https://medium.com/the-liqo-blog/benchmarking-liqo-kubernetes-multi-cluster-performance-d77942d7f67c).

## Kernel

Liqo requires a **Linux kernel version 5.10 or later** to run correctly, as it leverages some **nftables** features that are not available in older versions.

## Connectivity

Liqo requires a set of **connectivity requirements** to establish peerings with remote clusters.

Please note that the connectivity requirements depend on the **liqo modules** you choose to enable.
More details are available in the [peering section](/features/peering).

If you intend to enable all the Liqo features, you need to ensure that the following services on the **provider** cluster are reachable from the **consumer** cluster:

* **Kubernetes API server**: the standard Kubernetes API Server.
* **Gateway Server**: the Liqo gateway that acts as a server. Must be reachable from the gateways acting as a client.

```{admonition} Note
The position of **the gatway server** and **the gateway client** can be inverted, depending on the configuration of the peering. In this case, the gateway server on the **consumer** cluster must be reachable from the **provider** cluster.
```

```{Warning}
Any network device (**NAT**, **firewall**, etc.) sitting in the path between the two clusters must be configured to **enable direct connectivity** toward the above services, as presented in the [network firewalls](RequirementsConnectivityFirewall) section.
```

The tuple *<IP/port>* (used to expose the **gateway server**) depends on the Liqo configuration, chosen at peering time, which may depend on the physical setup of your cluster and the characteristics of your service.
The **gateway server** is the termination of a UDP-based network tunnel; hence only *LoadBalancer* and *NodePort* services are supported.

An overview of the overall connectivity requirements to establish connectivity in Liqo is shown in the figure below.
Dashed components do not need to be exposed, while solid line ones need to be exposed or at least reachable from the consumer.  

![Peering architecture](/_static/images/features/peering/peering-arch.drawio.svg)

### Additional considerations

The choice of the way you expose Liqo services to remote clusters may not be trivial in some cases.
Here, we list some additional notes you should consider in your choice:

* **NodePort service**: although a *NodePort* service can be used to expose the gateway server, often the IP addresses of the nodes are configured with private IP addresses, hence not being suitable for connections originating from the Internet.
This happens rather often in production clusters, and in public clusters as well.

(RequirementsConnectivityFirewall)=

### Network firewalls

In some cases, especially on production setups, additional network limitations are present, such as firewalls that may impair network connectivity, which must be considered in order to enable Liqo peerings.

Depending on your configuration and the selected peering approach, you may have to configure existing firewalls to enable remote clusters to contact the `gateway server` and the `Kubernetes API server` that need to be publicly accessible in the peering phase.

To know the network parameters (i.e., <IP/port>) used by the `gateway server`, you can use standard Kubernetes commands (e.g., `kubectl get services -n <liqo-tenant-ns>`), while the <IP/port> tuple used by your Kubernetes API server is the one written in the `kubeconfig` file.

Remember that the Kubernetes API server uses the HTTPS protocol (over TCP); vice versa, the network gateway uses the [WireGuard](https://www.wireguard.com/) protocol over UDP.
