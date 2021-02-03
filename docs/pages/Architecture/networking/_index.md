---
title: Networking
weight: 2
---


## Introduction

The Liqo networking module is in charge of connecting networks of different Kubernetes clusters. It aims at extending 
the Pod-to-Pod and Pod-to-Service communications to multiple clusters.

### Liqo Networking Model

In a single cluster scenario pods on a node can communicate with all pods on all nodes without NAT. The Liqo Networking extends this model to multiple clusters. More precisely, all the pods in a cluster should be able to communicate with all pods running on a peered cluster.

#### Conflicting Pod CIDRs

Two peered clusters may have 
The NAT is used when pod and service CIDRS of the two peering clusters have conflicts.
Also each node of a cluster can communicate with each pod scheduled on all nodes on a remote cluster with NAT. The NAT solution is always used in this case.
The module enables Kubernetes clusters to exchange only the POD traffic, which means that only the POD CIDR subnet of a remote cluster is reachable by a local cluster.

#### Multi-cluster Services

## Components

Based on peering information, Liqo can extend cluster network to remote clusters, dynamically negotiating network parameters.

Liqo defines a “gateway” pod (possibly replicated) that connects to the remote cluster
Possible overlapped IP addresses handled via double natting
Liqo can work with multiple and different underlying CNI
