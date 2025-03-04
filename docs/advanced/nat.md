# Configure a Liqo gateway server behind a NAT

There might be some cases, especially when working in lab environments, where **it is not possible to configure the gateway server service as `LoadBalancer`**, requiring it to be exposed as a `NodePort` service instead.
However, the nodes of the cluster might not be directly reachable, e.g., they are behind a NAT or a physical load balancer.

This page describes how to configure Liqo in the above scenarios.

## Option A: Reverse Liqo network

![The provider is behind a NAT](../_static/images/advanced/nat/provider-nat.svg)

The `liqoctl peer` command by default configures the gateway server on the provider cluster.
However, there may be cases where the provider cluster's nodes are not directly reachable, such as when they are behind a NAT, while the consumer cluster is directly accessible.
For instance, in the image above, cluster 2 is behind a NAT and is therefore not directly reachable.

This problem can be solved by swapping the roles of the gateways, hence configuring the client on the cluster provider and the server on the consumer.
To do so, you have two options:

- run the `liqoctl peer` command with the `--gw-server-service-location=Consumer` flag
- perform a [advanced peering](./manual-peering.md), setting the inter-cluster network up separately

![The gateway server has been on the consumer side](../_static/images/advanced/nat/consumer-nat.svg)

In this case, once the gateway server has been moved to cluster 1, we are able to configure the Liqo networking without the need to configure port-mapping on the NAT, as the gateway server is directly reachable.

In the case you would like to keep the gateway server on the provider, you can follow the instructions on the next paragraph.

## Option B: configure the gateway server to be reachable through the NAT (or physical load balancer)

The solution presented in this section allows to reach the gateway server even when it is behind a NAT (or, for example, a physical load balancer) which prevents it from being directly reachable.

![The provider is behind a NAT](../_static/images/advanced/nat/port-address-override.svg)

In this case, **you must configure the appropriate ip/port-mapping on the NAT device** so that the traffic directed on a specific port of the NAT can be forwarded to one of nodes of the cluster.

**When network is configured via `liqoctl` commands**, if the service is of type `NodePort`, by default, the Liqo gateway client uses the IP address of the nodes of the cluster where the gateway server is running.
However, in the architecture described above, these IP addresses are not reachable from the gateway client.

You can force the Liqo gateway client to use the public IP or FQDN and the port of the NAT where we configured port mapping, by providing some options to `liqoctl peer` or `liqoctl network connect` commands when using `liqoctl`.
[With declarative peering](./peering/peering-via-cr.md#configuring-the-client-gateway-consumer-cluster) you can directly provide these values in the `GatewayClient` manifest.

For example, let's imagine we expose the gateway server with a `NodePort` on port `30742` and we configure port mapping on the NAT  (which has public IP address equal to `203.0.113.8`) so that port `40582` of the NAT redirects to port `30742` of a node of the cluster.

Of course, the client, running on cluster 1 cannot directly reach the gateway, but it needs to pass through the NAT.
To do so, you need to pass the IP address or FQDN of the NAT and port where configured port mapping (`203.0.113.8` and `40582` in this case):

```bash
liqoctl peer \
    --remote-kubeconfig $PATH_TO_CLUSTER2_KUBECONFIG \
    --gw-client-address $NAT_PUBBLIC_ADDR \
    --gw-client-port $NAT_MAPPING_PORT \
    --gw-server-service-nodeport $GATEWAY_SERVER_NODEPORT
```

Filling the placeholders, in our specific example, the command becomes:

```bash
liqoctl peer \
    --remote-kubeconfig $PATH_TO_CLUSTER2_KUBECONFIG \
    --gw-client-address 203.0.113.8 \
    --gw-client-port 40582 \
    --gw-server-service-nodeport 30742

```

The command above sets up a complete peering between cluster 1 and cluster 2.
**To configure only the network**, you can pass the same parameters to the `liqoctl network connect` command:

```bash
liqoctl network connect \
    --remote-kubeconfig $PATH_TO_CLUSTER2_KUBECONFIG \
    --gw-client-address $NAT_PUBBLIC_ADDR \
    --gw-client-port $NAT_MAPPING_PORT \
    --gw-server-service-nodeport $GATEWAY_SERVER_NODEPORT
```
