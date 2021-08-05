---
title: Concepts 
weight: 6
---

## Introduction

Liqo enables resource sharing across Kubernetes clusters.
This section presents the architectural documentation of Liqo, composed of four main sections:

* [Discovery](./discovery): how Liqo detects other clusters to peer with;
* [Peering](./peering): how Liqo exchanges information about the resources (and associated costs) available in the other (_foreign_) cluster, and eventually establishing a _peering_ relationship with it;
* [Networking](./networking): how the network interconnection between different clusters works;
* [Offloading](./offloading): how Kubernetes resources and services present in one cluster are propagated in the peered ones, leveraging the _big cluster_ / _big node_ model based on the Virtual Kubelet.


## Concepts

### Peering

Liqo relies on the concept of peering across different clusters. In Liqo, the *peering* is an administrative unidirectional interconnection in the purpose of exchanging resources and services. 

W.r.t. an home cluster, the peering may be:
* *Incoming*: the initiator is a foreign cluster that want to use resources and services from the local one.
* *Outgoing*: the initiator is the home cluster itself that want to use resources and services from a foreign cluster.

The presence of an established peering allows an home cluster to create namespaces and offload pods on a peered foreign cluster. 

Moreover, the peeering is unidirectional but can optionally be established biredictionally. When a peering is established:

* Both Liqo control planes negotiates and establish the network interconnection.
* The outgoing control plane forges a virtual node, used to offload pods on the remote cluster.

### Virtual Nodes

A virtual node is an opaque handler to the remote cluster resources. The size of the resource  by the  represents the available space on the remote cluster. A virtual node can be customized using specific variables that affinity/anti-affinity terms can use in scheduling and namespace offloading selection.


### The Namespace Model

Only namespaces selected are replicated on the remote clusters. Liqo provides a dedicated logic to limit on which remote clusters a certain namespace can be offloaded.

Pods offloaded on foreign namespaces are represented on the home cluster by a "shadow pod". All pods offloaded in remote clusters can be observed and manipulated from the home cluster which initiated the deployment.

Services are replicated and usable on remote clusters without modifying the applications.