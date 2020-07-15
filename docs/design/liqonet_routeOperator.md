# Route-Operator
## Overview
The Route-Operator runs as a DaemonSet on all Kubernetes nodes and coordinates the setup of all the routes that allow each local pod/node to communicate with the pods of the peering cluster, through the elected gateway node.
It will ensure state and react on tunnelEndpoint CR changes, which means that it is able to add/remove routes as soon as a new cluster peers/de-peers with the local cluster.
It creates a VxLan overlay network to whom all the cluster nodes belong and uses it to direct all the network traffic to the gateway node.

### Features
* Traffic toward remote networks routed through a Vxlan overlay (set-up dynamically)
* Suppor for Single and Double NATting
* Support for Node-to-(Remote)Pod and Pod-to-(Remote)Pod communication patterns
* Tested with Flannel but should work with any other CNI plugin (Calico, Cannal, etc.).

### Limitations
* No support for dynamic gateway node (yet)
* New nodes added to the cluster are not dynamically added to the VxLan network

## Architecture and workflow
Will be included in the general overview of the network module.
