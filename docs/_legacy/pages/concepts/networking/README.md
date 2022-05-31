# Networking

The networking module is in charge of connecting networks of different Kubernetes clusters. It aims at extending the Pod-to-Pod communications to multiple clusters, by flattening the networks between the connected clusters. The interconnection between clusters is done in a dynamic and secure way on top of the existing network configuration of the clusters. Liqo's network isolates its configuration as much as possible using overlay networks, custom network namespaces, custom routing tables, and policy routing rules in order to avoid changing the existing network configuration. At the same time, when connecting to remote clusters no input is required from the user in order to configure the interconnection with remote clusters other than the ones required at [install time](/installation/connect-requirements).

Liqo network consists of several components that enable workloads connection across multiple clusters:

* [Liqo-Network-Manager](./components/network-manager): manages the exchange of network configuration with remote clusters;
* [Liqo-Gateway](./components/gateway): manages life-cycle of secure tunnels to remote clusters;
* [Liqo-Route](./components/route): configures routes for cross-cluster traffic from the nodes to the active Liqo-Gateway.

The diagram below illustrates the basic architecture of Liqo networking:

![Liqo Network Architecture](../../../images/liqonet/network-architecture.png)
