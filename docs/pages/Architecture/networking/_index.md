---
title: Networking
weight: 2
---

The Liqo networking module is in charge of connecting networks of different Kubernetes clusters. It aims at extending 
the Pod-to-Pod and Pod-to-Service communications to multiple clusters.
In a single cluster scenario pods on a node can communicate with all pods on all nodes without NAT. The Liqo Networking 
extends this functionality: all pods in a cluster can communicate with all pods on all nodes on another cluster with or 
without NAT. The NAT is used when pod and service CIDRS of the two peering clusters have conflicts.
Also each node of a cluster can communicate with each pod scheduled on all nodes on a remote cluster with NAT. The NAT 
solution is always used in this case.
The module enables Kubernetes clusters to exchange only the POD traffic, which means that only the POD CIDR subnet of a 
remote cluster is reachable by a local cluster.
