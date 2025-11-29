# NodePort

The main challenge with NodePort traffic is the lack of a standardized way to identify the traffic’s origin based on the source IP address.
When traffic reaches a NodePort service, the behavior varies depending on the CNI (Container Network Interface). In some cases, the source IP is source NATed to the first IP of the pod CIDR (which is often reserved for NodePort traffic). In other cases, another IP address is used or source NAT is not applied at all.

Problems arise when this traffic traverses a WireGuard tunnel and needs to return to the cluster where it originated.
When packets from a Pod of Cluster A are directed toward a Pod in cluster B, return traffic can be easily handled by cluster B, as the destination IP address belongs to one of the CIDR assigned to cluster A, so traffic can be forwarded to the right gateway and finally forwarded to the appropriate WireGuard tunnel connecting to the Liqo Gateway in cluster A. However, in the case of NodePort traffic, the source IP address is infrastructure-dependent, making it difficult to determine which cluster originated the traffic.

To address this issue, before traffic leaves cluster A the Liqo’s gateway performs source NAT on traffic coming from unknown IP addresses, rewriting the source IP to the first IP of the tenant’s external CIDR. This ensures that, when the traffic reaches the destination cluster, its source IP is within a known CIDR associated with Cluster A. This allows the response traffic to be routed back through the correct WireGuard tunnel to the originating cluster.

![The NodePort issue solution](/_static/images/architecture/network/nodeport.excalidraw.png)

Another aspect to consider is that, once the return traffic reaches the originating cluster, it must exit from the same node where the original request was received. This ensures that traffic **follows the same path in both directions**, which is crucial because some CNIs may drop traffic if it follows a different return path.

To achieve this, a conntrack mark (ctmark) is applied. When traffic exits through a Geneve interface on the gateway (each connected to a specific node), a conntrack entry is created with a mark representing a unique identifier for the node where the traffic was initially received. As each Geneve interface is connected to one of the node, the mark is determined by checking from which Geneve interface the packets came out.

![conntrack_outbound](/_static/images/architecture/network/conntrack_outbound.excalidraw.png)

Conntrack stores the traffic quintuple (protocol, source and destination IP addresses, and ports) along with the mark. When return traffic passes through the gateway, the destination IP (which was previously set to the first IP of the external CIDR) is restored to the original source IP. The conntrack entry is matched, and the response packets are tagged with the same mark, identifying the node of origin.

![conntrack_inbound](/_static/images/architecture/network/conntrack_inbound.excalidraw.png)

A policy routing rule based on this mark ensures that traffic is forwarded to the correct Geneve tunnel, ultimately reaching the originating node, where packets are then delivered to their final destination.

## Resources involved

- **IPs**:
  - [\<tenant-name\>-unknown-source](ip.md#tenant-name-unknown-source): an IP address allocated to source NAT all traffic from an unknown IP address outgoing to the Gateway
- **Firewall Configurations**
  - [service-nodeport-routing](firewallconfiguration.md#service-nodeport-routing): containing the ctmark rule to create the contract with the mark corresponding to the node and to add the mark to the packet metadata once the return traffic traverse the gateway.
  - [\<tenant-name\>-masquerade-bypass](firewallconfiguration.md#tenant-name-masquerade-bypass-node): one of those rules applies source natting in the node where the gateway is running
- **Routing Configurations**:
  - [\<local-cluster-id\>-\<node-name\>-service-nodeport-routing](routeconfiguration.md#local-cluster-id-node-name-service-nodeport-routing-gateway): the routing rule allowing routing the return traffic to the specific node where the NodePort traffic has been received.
