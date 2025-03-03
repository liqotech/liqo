# Manual peering

In the [peer two clusters](../usage/peer.md) section of this documentation, we used the `liqoctl peer`, which automatically configure each single module of Liqo to create a peering between two clusters. However, in some cases where:

- you want to configure Liqo peerings via a [declarative approach](./peering/peering-via-cr.md) via CRs.
- The consumer needs to create multiple requests for resources (ResourceSlice) or you want to customize the way resources are distributed on virtual nodes

you might need to configure each single module separatly, or to interact with a specific module to obtain the desired result.

In this section, you will discover how to interact with each Liqo module without using the automatic peering method.

## Prerequirements

We suggest to read this section after you have completed the [quick-start](/examples/quick-start) guide.

## Modules

Check each module specific documentation to understand how to configure it:

- [Networking](/advanced/peering/inter-cluster-network)
- [Authentication](/advanced/peering/inter-cluster-authentication)
- [Offloading](/advanced/peering/offloading-in-depth)
