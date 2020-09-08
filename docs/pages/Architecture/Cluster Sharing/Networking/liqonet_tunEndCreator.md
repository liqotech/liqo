---
title: "TunnelEndpointCreator Operator"

---

## Overview
The TunnelEndpointCreator is part of the network module and runs as a deployment. It reacts to **Advertisements Custom Resources (CR)**
and for each one it creates a **TunnelEndpoint CR**. The **advertisement CR** carries all the required data
in order to establish a point-to-point connection with the peering cluster. A simple IP manager
has been embedded inside the operator in order to resolve the possible conflicts rising between the network
address spaces used by the clusters. The peering clusters could have address spaces conflicts:
* pod network CIDR:
* clusterIP network CIDR:
* subnets used inside the data center.

If the pod network CIDR of a peering cluster does not have conflicts with the address spaces
used by the local network then the IPAM just marks this new network subnet as reserved.
In the other case the remote pod network CIDR is remapped in a new virtual address space.

Currently, the new virtual address spaces belong to the 10.0.0.0/8 CIDR Block. It is divided in 256 subnets each of them
being an a.b.c.d/16 CIDR block.  


### Features
* Detects and resolves possible address spaces conflicts.
* the NAT solution is used only in presence of overlapping subnets.


### Limitations
* NAT is supported only on peering clusters that have a pod CIDR with subnet mask 255.255.0.0.
* The maximum number of peering clusters using the NAT service is 256.

