# Network Fabric

The **network fabric** is the Liqo subsystem transparently extending the Kubernetes network model across multiple independent clusters, such that **offloaded pods can communicate with each other** as if they were all executed locally.

In detail, the network fabric ensures that **all pods in a given cluster can communicate with all pods on all remote peered clusters**, either with or without NAT translation.
The support for arbitrary clusters, with different parameters and components (e.g., CNI plugins), makes it impossible to guarantee **non-overlapping pod IP address ranges** (i.e., *PodCIDR*).
Hence, possibly requiring **address translation mechanisms**, provided that NAT-less communication is preferred whenever address ranges are disjointed.

The figure below represents at a high level the network fabric established between two clusters, with its main components detailed in the following.

![Network fabric representation](/_static/images/features/network-fabric/network-fabric.drawio.svg)

## Network manager

The **network manager** (not shown in figure) represents the **control plane** of the Liqo network fabric.
It is executed as a pod, and it is responsible for the **negotiation of the connection parameters** with each remote cluster during the peering process.

It features an **IP Address Management (IPAM) plugin**, which deals with possible **network conflicts** through the definition of high-level NAT rules (enforced by the data plane components).
Additionally, it exposes an interface consumed by the reflection logic to handle **IP addresses remapping**.
Specifically, this is leveraged to handle the [translation of pod IPs](usageReflectionPods) (i.e., during the synchronization process from the remote to the local cluster), as well as during [EndpointSlices reflection](UsageReflectionEndpointSlices) (i.e., propagated from the local to the remote cluster).

## Cross-cluster VPN tunnels

The interconnection between peered clusters is implemented through **secure VPN tunnels**, made with [WireGuard](https://www.wireguard.com/), which are dynamically established at the end of the peering process, based on the negotiated parameters.

Tunnels are set up by the **Liqo gateway**, a component of the network fabric that is executed as a *privileged* pod on one of the cluster nodes.
Additionally, it appropriately populates the **routing table**, and configures, by leveraging *iptables*, the **NAT rules** requested to comply with address conflicts.

Although this component is executed in the *host network*, it relies on a **separate network namespace** and **policy routing** to ensure isolation and prevent conflicts with the existing Kubernetes CNI plugin.
Moreover, **active/standby high-availability** is supported, to ensure minimum downtime in case the main replica is restarted.

## In-cluster overlay network

The **overlay network** is leveraged to **forward all traffic** originating from local pods/nodes, and directed to a remote cluster, **to the gateway**, where it will enter the VPN tunnel.
The same process occurs on the other side, with the traffic that exits from the VPN tunnel entering the overlay network to reach the node hosting the destination pod.

Liqo leverages a **VXLAN**-based setup, which is configured by a network fabric component executed on all physical nodes of the cluster (i.e., as a *DaemonSet*).
Additionally, it is also responsible for the population of the appropriate **routing entries** to ensure correct traffic forwarding.
