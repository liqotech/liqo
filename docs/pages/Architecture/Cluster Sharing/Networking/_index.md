---
title: Networking 
---


The Liqo networking module is in charge of connecting networks of different Kubernetes clusters. It aims to extend the Pod-to-Pod communications to multiple clusters.
In a single cluster scenario pods on a node can communicate with all pods on all nodes without NAT. The Liqo Networking extends this functionality: all pods in a cluster can 
communicate with all pods on all nodes on another cluster with or without NAT. The NAT is used when pod and service CIDRS of the two peering clusters have conflicts.
Also all the nodes of a cluster can communicate with all the pods on all nodes on a remote cluster with NAT. The NAT solution is always used in this case.
The module enables Kubernetes clusters to exchange only the POD traffic, which means that only the POD CIDR subnet of a remote cluster is reachable by a local cluster.
Thanks to the [resource reflection](/architecture/cluster-sharing/computing/) pods and nodes of a local cluster can reach also the pods running behind a service in a remote cluster.

## Architecture
The following image shows the basic architecture of liqonet.

  ![](/images/liqonet/liqonet_architecture.png)

More information on the operators can be found on their relative section.

### Network Path
The way how the traffic flows from one cluster two another varies depending on the origin/destination.
All the traffic leaving a cluster or reaching it has to pass through the endpoint/gateway node.
Having traffic originated on the node of a local cluster it is routed through the overlay network toward the gateway node;
After that it is sent through the VPN tunnel and reaches the remote cluster gateway node where it is handled by the CNI of the cluster in order to reach the destination pod. 

### Workflow

![](/images/liqonet/liqonet_workflow.png)

The initialization network connection between two cluster goes through the following steps:
1. a `sharing.liqo.io` custom resource called **Advertisement** is created in the local cluster by peering cluster;
2. the tunnelEndpointCreator reacts and from this **Advertisement** derives a **TunnelEndpoint** custom resource of type `net.liqo.io`;
3. the IPAM embedded in the tunnelEndpointCreator resolves possible conflicts between the subnet used in the local cluster and the pod CIDR used by the peering cluster;
4. the Remote Watcher on the peering cluster checks the **TunnelEndpoint CR** on the local cluster if the NAT is enabled and updates the **CR** on the peering cluster accordingly;
5. the local Remote Watcher does the same thing with the **TunnelEndpoint CR** on the peering cluster to check if the NAT has been enabled by the peering cluster and updates the **CR** on the local cluster;
6. the TunnelOperator reacts to the update on the **CR** and creates a VPN tunnel with the gateway on the peering cluster;
7. the TunnelOperator updates the **CR** with information about the VPN Tunnel
8. the RouteOperator has now all the required info to insert the routing and iptables rules in order to reach the peering cluster's pods.

### More Information
Network operators:
* [tunnelEndpointCreator-operator](liqonet_tunendcreator);
* [route-operator](liqonet_routeoperator);
* [tunnelEndpoint-operator](liqonet_tunnelendpoint).