# Custom peering

In the [peer two clusters](../usage/peer.md) section of this documentation, we used the `liqoctl peer` command, which automatically configures all the Liqo modules (namely _networking_, _authentication_, and _offloading_) to create a peering between two clusters. However, there are some cases in which the automatic process is not appropriate, and the user would like to customize advanced parameters, overcome specific network constraints, and more, hence a more granular tuning for the peering process is needed.
For instance, we list here some cases in which the automatic peering is not a suitable choice:

- you would like to configure Liqo the peerings via a [declarative approach](./peering/peering-via-cr.md), i.e., with Kubernetes CRs;
- you need to configure the WireGuard gateway server on the consumer cluster (e.g. the nodes of the provider cluster are [behind a NAT or a physical load balancer](./nat.md));
- the consumer needs to create multiple requests for resources (ResourceSlice) or you would like to customize how resources are distributed across virtual nodes.

To overcome the above necessities, Liqo allows to configure each module separately, with a more granual control.
This section shows how to interact with each individual Liqo module to better tune your peering process.

## Prerequirements

We suggest to read this section after you have completed the [quick-start](/examples/quick-start) guide.

## Modules

Check each module specific documentation to understand how to configure it:

- [Networking](/advanced/peering/inter-cluster-network)
- [Authentication](/advanced/peering/inter-cluster-authentication)
- [Offloading](/advanced/peering/offloading-in-depth)

Furthermore, an additional section describes how to interact with the above modules using [Kubernetes CRs](/advanced/peering/peering-via-cr), hence using a fully declarative approach.
