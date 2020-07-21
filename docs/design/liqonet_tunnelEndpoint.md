# TunnelEndpoint-Operator
## Overview
This component is in charge of establishing a VPN tunnel with the peering clusters, which is created between the (local) Gateway Node and the Gateway Node of the remote cluster.
All the traffic between the local and remote cluster is first delivered to the Gateway Node, then tunneled towards the remote gateway and finally delivered to the desired destination.
The traffic can leave local cluster as it is in case the home and remote addressing spaces do not overlap; otherwise, the traffic crosses a properly configurated NAT in order to avoid overlapped spaces.

The TunnelEndpoint-Operator runs as deployment only on the local Gateway Node.

### Features
* GRE tunnel as VPN tunnel

### Limitations
* Does not support other VPN solutions
* Traffic flows as it is, does not add security 

## Architecture and workflow
Will be included in the general overview of the network module.
