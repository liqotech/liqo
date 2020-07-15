# Route-Operator
## Overview
The Route-Operator runs as a DaemonSet on all Kubernetes nodes, and ensures route rules to allow all local pods/nodes
to communicate through the elected gateway node  with the pods of the peering cluster. It will ensure state and
react on tunnelEndpoint CR changes, which means that it is able to remove/add routes as new clusters decide to peer with
or unjoin the local cluster. It creates a VxLan overlay network to whom all the cluster nodes belong and uses it to
direct all the network traffic to the gateway node.

### Features
* All the network traffic for remote networks is routed through a Vxlan overlay network
* Single and Double NATting support
* Node to pod (remote) and pod to pod (remote) communication support.
* Tested with Flannel but should work with Calico, Cannal, etc..

### Limitations
* No support for dynamic gateway node (yet)
* New nodes added to the cluster are not dynamically added to the VxLan network

## Architecture and workflow

Will be included in the general overview of the network module.