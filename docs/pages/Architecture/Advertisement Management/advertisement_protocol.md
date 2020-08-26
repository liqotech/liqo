---
title: "Advertisement Protocol"
weight: 1
---

## Overview
The advertisement protocol is the logic that enables resource sharing between different clusters.
It allows a cluster to make its resources available to others, in order to host their jobs, and, on the other hand, 
to send and execute its applications in another cluster.

The general idea behind this protocol is to exploit periodic _Advertisement_ messages in which a cluster exposes its capabilities.
These messages are then used to build a local virtual-node where jobs can be scheduled: if a job is assigned to a virtual-node, it will be
actually sent to the respective foreign cluster. 

## Architecture and workflow

![](/images/advertisement-protocol/architecture.png)

### Components
* The [broadcaster](/architecture/advertisement-management/broadcaster) is in charge of sending to other clusters the Advertisement message, containing the
  resources made available for sharing and (optionally) their prices
* The [advertisement operator](/architecture/advertisement-management/controller) (briefly called **controller**) is the module that receives Advertisement 
  messages and creates the virtual nodes with the announced resources.
  
### Workflow

#### Outgoing chain
1. A foreign cluster is provided (manually by the admin or through the [discovery protocol](/architecture/discovery-and-peering/)).
2. When the foreign cluster requests some resources, the discovery logic creates the broadcaster
3. The broadcaster retrieves the available cluster resources and, after applying some policies, creates an Advertisement.
4. The Advertisement is pushed to the foreign cluster.
  
#### Ingoing chain
1. An Advertisement is received from the foreign cluster
2. The Advertisement is checked by a policy block: if it is accepted, it is further processed by the controller
3. The controller creates a virtual node with the information taken by the Advertisement
4. The virtual node will masquerade the foreign cluster and the resources created on it will be reflected according to the [sharing process](/architecture/cluster-sharing/)