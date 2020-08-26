---
title: "Architecture"
weight: 2
---

## Introduction

Liqo enables resource sharing across Kubernetes clusters. To do so, it encapsulates (1) a logic to discover/advertise 
resources in a neighborhood (e.g. LAN) and (2) a protocol to negotiate resource exchange. In this document, we describe 
how the cluster peering logic works.


## Liqo operating workflow

Sharing resources with Liqo relies on three different phases:

1. **Discovery**: The cluster looks for available clusters where offload new resources (e.g. neighborhood, dns, manual 
insertion) and exchange credentials with each other to start communicate.
2. **Advertisement protocol**: Clusters shares updates about the resourcing they are willing to export and their
 capabilities (i.e. Advertisements)
3. **Resource Sharing**: When a cluster is interested in resources proposed by a certain advertisement, it accepts the
 advertisement. This triggers the establishment of network interconnections and the spawning of a new virtual-kubelet.

[Here](/images/complete-workflow.png) you can find a graphic description of the complete workflow.

## Discovery

This issue describes how two clusters discover each other and start sharing resources.
The **discovery** service exploits DNS ServiceDiscovery protocol, which works both on a LAN and WAN scenarios. In first 
case with mDNS, in second the one with standard DNS.
**Resource sharing** is based on periodic Advertisement exchanges, where each cluster exposes its capabilities, allowing
 others to use them to offload their jobs.

The output of the discovery phase is the exchange of advertisements with "foreign" clusters.

The discovery phase is presented in details [here](discovery-and-peering/).

## Advertisement management
The Advertisement operator can be split in two main components.

- **Broadcaster**: the module which creates and sends the Advertisement message
- **Advertisement Operator**: the module which is triggered when receiving an Advertisement and spawns a virtual node (using Virtual 
Kubelet)

 You can find more details about the [here](advertisement-management/).

## Resource Sharing

The resource sharing phase is in charge of the actual workload execution. From the computing perspective,
foreign cluster resources are seamlessly added to the cluster resources by the corresponding virtual kubelet instances. 
From the networking perspectives, the "acceptation" of an Advertisement triggers the networking logic which establishes
the tunnels and install the routes.

More details about [resource sharing](cluster-sharing/).


