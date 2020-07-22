# Route-Operator
## Overview
The Route-Operator runs as a DaemonSet on all Kubernetes nodes and coordinates the setup of all the routes that allow each local pod/node to communicate with the pods of the peering cluster, through the elected gateway node.
It will ensure state and react on tunnelEndpoint CR changes, which means that it is able to add/remove routes as soon as a new cluster peers/de-peers with the local cluster.

It creates a VxLan overlay network to whom all the cluster nodes belong and uses it to direct all the network traffic to the [gateway node](liqonet_tunnelEndpoint.md).

The operator manages a set of iptables rules in order to achieve the communication between two peering clusters so that:
* the NAT service is enabled for a peering cluster having [overlapping address spaces](liqonet_tunEndCreator.md);
* each pod communicates with the other pods using its IP address, or the NATed one;
* each local node communicates with the remote pods using the private IP given to the [VPN interface](liqonet_tunnelEndpoint.md).


### Features
* Traffic toward remote networks routed through a Vxlan overlay (set-up dynamically).
* Support for Single and Double NATting.
* Support for Node-to-(Remote)Pod and Pod-to-(Remote)Pod communication patterns.
* Tested with Flannel but should work with any other CNI plugin (Calico, Cannal, etc.).

### Limitations
* No support for dynamic gateway node.
* New nodes added to the local cluster are not dynamically added by the operator to the VxLan network.

## Architecture and workflow
Will be included in the general overview of the network module.
