# Manual peering

In the [peer two clusters](../usage/peer.md) section of this documentation, we used the `liqoctl peer`, which automatically configure each single module of Liqo to create a peering between two clusters. However, this command can be used when:

- the user is willing to use `liqoctl` to create the peerings between the clusters
- all Liqo modules need to be used and configured
- the cluster acting as provider can expose the service for the Wireguard gateway server that the client can reach
- a single ResourceSlice (request for resources) backed by a single VirtualNode is enough

Whenever you need a different configuration, like, for example:

- you want to configure Liqo peerings via a [declarative approach](./peering/peering-via-cr.md) via CRs.
- the networking module is not required, either because the inter-cluster networking is not needed or because networking is provided by a third party
- it is required to configure the WireGuard gateway server on the cluster consumer (e.g. the nodes of the cluster provider are [behind a NAT or a physical load balancer](./nat.md))
- The consumer needs to create multiple requests for resources (ResourceSlice) or you want to customize the way resources are distributed on virtual nodes

then, you will need to set up the peering with the cluster, by configuring each single module separatly.

In this section, you will discover how to interact with each Liqo module without using the automatic peering method.

## Prerequirements

We suggest to read this section after you have completed the [quick-start](/examples/quick-start) guide.

## Modules

Check each module specific documentation to understand how to configure it:

- [Networking](/advanced/peering/inter-cluster-network)
- [Authentication](/advanced/peering/inter-cluster-authentication)
- [Offloading](/advanced/peering/offloading-in-depth)
