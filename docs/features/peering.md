# Peering

In Liqo, we define **peering** as a unidirectional resource and service consumption relationship between two Kubernetes clusters, with one cluster (i.e., the **consumer**) granted the capability to offload tasks (*pods*) and propagate resources (*volumes*, *secrets*, etc.) to a remote cluster (i.e., the **provider**), but not vice versa.

This configuration allows for maximum flexibility in asymmetric setups, while transparently supporting bidirectional peerings through their combination.
Additionally, the same cluster can play the role of provider and consumer in multiple peerings.

## Overview

Overall, the establishment of a peering relationship between two clusters involves four main tasks:

* **Network fabric setup**: the two clusters configure their **network fabric** and establish a secure cross-cluster VPN tunnel, according to the parameters negotiated (endpoints, security keys, address remappings, ...).
Essentially, this enables pods hosted by the local cluster to seamlessly communicate with the pods offloaded to a remote cluster and vice-versa, regardless of the underlying CNI plugin and configuration.
Additional details are presented in the [network fabric section](/features/network-fabric).
* **Authentication**: the consumer cluster, once properly authenticated with the provider (through shared endpoints and certificates), obtains a valid identity to interact with the provider cluster (i.e., its Kubernetes API server).
This identity, granted only limited permissions concerning Liqo-related resources, is then leveraged to negotiate the resources with the provider, as well as during the offloading process.
* **Resources negotiation**: the consumer cluster can now ask the provider cluster for a fixed amount of resources (CPU, memory, etc..).
If the provider approves the request, it gives the consumer a valid identity (in the form of a *kubeconfig*) to consume the requested resources.
* **Virtual node setup**: the consumer cluster uses the identity obtained in the previous step to create a new **virtual node** abstracting the resources shared by the provider cluster.
This transparently enables the task offloading process detailed in the [offloading section](/features/offloading), and it is completely compliant with standard Kubernetes practice (i.e., it requires no API modifications for application deployment and exposition).

(FeaturesPeeringArchitecture)=

## Architecture

This section describes the architecture and the approach used by Liqo to perform the peering process.
Additional in-depth details about the requirements are presented in the [installation requirements section](/installation/requirements), while the [usage section](/usage/peer) describes the operational commands to establish the peering.

To establish a successful peering, two kinds of traffic are required:

* Authentication and Offloading traffic: the consumer directly contacts the provider cluster API server, which needs to be exposed, or at least be reachable to the consumer (if using private networks).
* Network traffic: this is required for *pod-to-pod* and *pod-to-service* communication.
The network fabric is established by exposing a UDP endpoint (through a *Service*) on only one of the clusters (by default on the provider, but it can be configured).

The approach is schematized at a high level in the figure below.

![Peering architecture](/_static/images/features/peering/peering-arch.drawio.svg)

```{warning}
The standard *liqoctl* peer command requires the machine running it to have simultaneous access to both cluster API servers, through their *kubeconfigs*.
If this is not possible, refer to the advanced guide to learn how to perform the [peering manually](/advanced/manual-peering) without having contemporary access to both clusters.
```

```{admonition} Note
The user can adopt different peering approaches depending if it has contemporary access to both clusters or not, as described in the [dedicated page](/advanced/peering-strategies.md). 
```
