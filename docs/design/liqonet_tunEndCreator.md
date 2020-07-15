# TunnelEndpointCreator-Operator
## Overview
The TunnelEndpointCreator is part of the network module and runs as a deployment. It reacts to **Advertisements Custom Resources (CR)**
and for each one it creates a **TunnelEndpoint CR**. The advertisement CR carries all the required data
in order to establish a point-to-point connection with the peering cluster. A simple IP manager
has been embedded inside the operator in order to resolve the possible conflicts rising between the network
address spaces used by the clusters. The peering clusters could have address spaces conflicts:
* pod network CIDR
* clusterIP network CIDR

### Features
If the pod network CIDR of a peering cluster does not have conflicts with the address spaces
used by the local network then the IPAM just marks this new network subnet as reserved.
In the other case the remote pod network CIDR is remapped in a new virtual address space.
In such way the remapping process happens if necessary. 


### Limitations
Currently the new virtual address spaces belong to the 10.0.0.0/8 CIDR Block hence:
* each new subnet is a 10.x.x.x/16
* max 256 new subnets available to remap the peering clusters

## Architecture and workflow

Will be included in the general overview of the network module.