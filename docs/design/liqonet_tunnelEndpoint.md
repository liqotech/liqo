# TunnelEndpoint-Operator
## Overview
TunnelEndpoint is the component in charge of bringing up a VPN tunnel with the peering clusters.
It runs as deployment, and the node where it runs is called the Gateway Node. All the network
traffic for remote pods goes through the VPN tunnel. If NATing is not present then the traffic
leaves the local cluster as it is, otherwise for the out-going or incoming traffic NATing rules
have to be applied.

### Features
* GRE tunnel as VPN tunnel


### Limitations
* Does not support other VPN solutions(yet)
* Traffic flows as it is, does not add security 

## Architecture and workflow

Will be included in the general overview of the network module.