# Network Fabric

The **network fabric** is the Liqo subsystem transparently extending the Kubernetes network model across multiple independent clusters, such that **offloaded pods can communicate with each other** as if they were all executed locally.

In detail, the network fabric ensures that **all pods in a given cluster can communicate with all pods on all remote peered clusters**, either with or without NAT translation.
The support for arbitrary clusters, with different parameters and components (e.g., CNI plugins), makes it impossible to guarantee **non-overlapping pod IP address ranges** (i.e., *PodCIDR*).
Hence, possibly requiring **address translation mechanisms**, provided that NAT-less communication is preferred whenever address ranges are disjointed.

The figure below represents at a high level the network fabric established between two clusters, with its main components detailed in the following.

![Network fabric representation](/_static/images/features/network-fabric/network-fabric.drawio.svg)

## Network manager

The **controller-manager** (not shown in the figure) contains the **control plane** of the Liqo network fabric.
It runs as a pod (**liqo-controller-manager**) and is responsible for **setting up the network CRDs** during the connection process to a remote cluster.
This includes the management of potential **network conflicts** through the definition of high-level NAT rules (enforced by the data plane components).
Specifically, network CRDs are used to handle the [Translation of Pod IPs](usageReflectionPods) (i.e. during the synchronisation process from the remote to the local cluster), as well as during the [EndpointSlices reflection](usageReflectionEndpointSlices) (i.e. propagation from the local to the remote cluster).

An **IP Address Management (IPAM) plugin** is included in another pod (**liqo-ipam**).
It exposes an interface that is consumed by the **controller-manager** to handle **IPs acquisitions**.

## Cross-cluster VPN tunnels

The interconnection between peered clusters is implemented through **secure VPN tunnels**, made with [WireGuard](https://www.wireguard.com/), which are dynamically established at the end of the peering process, based on the negotiated parameters.

Tunnels are set up by **Liqo Gateways**, components of the network fabric that run as pods. Consider that each remote cluster has its own **Liqo Gateway** pod on each side of the tunnel.
It also populates the **routing table** accordingly with the Liqo CRs, and configures the **NAT rules** required to deal with address conflicts, using *nftables*.

## In-cluster overlay network

The **overlay network** is leveraged to **forward all traffic** originating from local pods/nodes, and directed to a remote cluster, **to the gateway**, where it will enter the VPN tunnel.
The same process occurs on the other side, with the traffic that exits from the VPN tunnel entering the overlay network to reach the node hosting the destination pod.

Liqo uses a **Geneve** based setup, configured by a network fabric component running on all physical nodes of the cluster (i.e. as a *DaemonSet*), which creates a tunnel from all **nodes** to all **gateways**.
Note that the endpoints of these tunnels are node and pod IPs. This allows liqo to use the **CNI** to establish a connection between **nodes and gateways**, and to take advantage of the **features offered by the CNI** (i.e. **encryption**).
It is also responsible for creating the appropriate **routing entries** on the node to ensure the correct routing of traffic.
