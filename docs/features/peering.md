# Peering

In Liqo, we define **peering** a unidirectional resource and service consumption relationship between two Kubernetes clusters, with one cluster (i.e., the consumer) granted the capability to offload tasks to a remote cluster (i.e., the provider), but not vice versa.
In this case, we say that the consumer establishes an **outgoing peering** towards the provider, which in turn is subjected to an **incoming peering** from the consumer.

This configuration allows for maximum flexibility in asymmetric setups, while transparently supporting bidirectional peerings through their combination.
Additionally, the same cluster can play the role of provider and consumer in multiple peerings.

## Overview

Overall, the establishment of a peering relationship between two clusters involves four main tasks:

* **Authentication**: each cluster, once properly authenticated through pre-shared tokens, obtains a valid identity to interact with the other cluster (i.e., its Kubernetes API server).
This identity, granted only limited permissions concerning Liqo-related resources, is then leveraged to negotiate the necessary parameters, as well as during the offloading process.
* **Parameters negotiation**: the two clusters exchange the set of parameters required to complete the peering establishment, including the amount of resources shared with the consumer cluster, the information concerning the setup of the network VPN tunnel, and more.
The process is completely automatic and requires no user intervention.
* **Virtual node setup**: the consumer cluster creates a new **virtual node** abstracting the resources shared by the provider cluster.
This transparently enables the task offloading process detailed in the [offloading section](/features/offloading), and it is completely compliant with standard Kubernetes practice (i.e., it requires no API modifications for application deployment and exposition).
* **Network fabric setup**: the two clusters configure their **network fabric** and establish a secure cross-cluster VPN tunnel, according to the parameters negotiated in the previous phase (endpoints, security keys, address remappings, ...).
Essentially, this enables pods hosted by the local cluster to seamlessly communicate with the pods offloaded to a remote cluster, regardless of the underlying CNI plugin and configuration.
Additional details are presented in the [network fabric section](/features/network-fabric).

(FeaturesPeeringApproaches)=

## Approaches

Liqo supports two non-mutually exclusive peering approaches (i.e., the same cluster can leverage a different approach for different remote clusters), respectively referred to as **out-of-band control plane** and **in-band control plane**.
The following sections briefly overview the differences among them, outlining the respective trade-offs.
Additional in-depth details about the networking requirements are presented in the [installation requirements section](/installation/requirements), while the [usage section](/usage/peer) describes the operational commands to establish both types of peering.

(FeaturesPeeringOutOfBandControlPlane)=

### Out-of-band control plane

The standard peering approach is referred to as **out-of-band control plane**, since the **Liqo control plane traffic** (i.e., including both the initial authentication process and the communication with the remote Kubernetes API server) **flows outside the VPN tunnel** interconnecting the two clusters (still, TLS is used to ensure secure communications).
Indeed, this tunnel is dynamically started in a later stage of the peering process, and it is leveraged only for cross-cluster pods traffic.

The single cross-cluster traffic flow required by this approach is schematized at a high level in the figure below (agnostic from how services are exposed, which is presented in the [dedicated installation requirements section](InstallationRequirementsOutOfBandControlPlane)).

![Out-of-band control plane peering representation](/_static/images/features/peering/out-of-band.drawio.svg)

Overall, the out-of-band control plane approach:

* Supports clusters under the control of **different administrative domains**, as each party interacts only with its own cluster: the provider retrieves an authentication token that is subsequently shared with and leveraged by the consumer to start the peering process.
* Is characterized by **high dynamism**, as upon parameters modifications (e.g., concerning VPN setup) the negotiation process ensures synchronization between clusters and the peering automatically re-converges to a stable status.
* Requires each cluster to expose **three different endpoints** (i.e., the Liqo authentication service, the Liqo VPN endpoint and the Kubernetes API server), making them accessible from the pods running in the remote cluster.

(FeaturesPeeringInBandControlPlane)=

### In-band control plane

The alternative peering approach is referred to as **in-band control plane**, since the **Liqo control plane traffic flows inside the VPN tunnel** interconnecting the two clusters.
In this case, the tunnel is statically established at the beginning of the peering process (i.e., part of the negotiation process is carried out directly by the Liqo CLI tool), and it is leveraged from that moment on for all inter-cluster traffic.
The three different cross-cluster traffic flows required by this approach are schematized at a high level in figure below (agnostic from how services are exposed, which is presented in the [dedicated installation requirements section](InstallationRequirementsInBandControlPlane)).

![In-band control plane peering representation](/_static/images/features/peering/in-band.drawio.svg)

Overall, the in-band control plane approach:

* Requires the administrator starting the peering process to have **access to both clusters** (although with limited permissions), as the network parameters negotiation is performed through the Liqo CLI tool (which interacts at the same time with both clusters).
The remainder of the peering process, instead, is completed as usual, although the entire communication flows inside the VPN tunnel.
* **Statically configures** the cross-cluster **VPN tunnel** at peering establishment time, hence requiring manual intervention in case of configuration changes causing connectivity loss.
* **Relaxes** the **connectivity requirements**, as only the Liqo VPN endpoint needs to be reachable from the pods running in the remote cluster.
Specifically, the Kubernetes API service is not required to be exposed outside the cluster.
