---
title: Resource reflection
weight: 4
---

In addition to Node and Pods lifecycle handling, the Liqo virtual kubelet implements a feature we call *resource reflection*.
It is responsible for the propagation and synchronization of selected control plane information into remote clusters, to enable the seamless execution of offloaded Pods. Currently, the virtual kubelet supports the reflection of the following Kubernetes resources:

* `Services`, abstracting the access to workloads spread across multiple clusters, while enabling the usage of standard DNS discovery mechanisms.
* `EndpointSlices`, synchronizing the endpoints associated with the reflected services, regardless of whether the corresponding Pods are running in the same or in a different cluster.
* `ConfigMaps`, typically holding configuration data consumed by Pods.
* `Secrets`, typically holding secret data consumed by Pods.

Once a given namespace is enabled for [Liqo extension]((/usage/namespace_offloading)) (i.e., through the creation of a `NamespaceOffloading` resource, either manually or through the appropriate `liqoctl` command), the virtual kubelet starts reflecting all resources belonging to the above categories into the subset of selected clusters (i.e., by means of the `ClusterSelector` field).
The local copy of each resource is the source of trust leveraged to realign the content of the *shadow* copy reflected in each remote cluster.
However, appropriate remapping of certain information (e.g., [endpoints IP](/concepts/networking/components/network-manager/#reflection)) is transparently performed by the virtual kubelet, accounting for conflicts and different configurations in different clusters.

### Service and EndpointSlice reflection

The reflection of Service and EndpointSlice resources is a key element to allow the seamless intercommunication between microservices spread across multiple clusters.

As a matter of example, let consider the scenario shown in the figure below, depicting an application composed of three Pods (partially hosted by a local cluster and partially offloaded to a remote one through a virtual node) and exposed through their respective Service (then, Kubernetes automatically creates the corresponding EndpointSlice).

Considering first a local Pod (P<sub>1</sub>), it can directly contact an offloaded Pod (P<sub>3</sub>) through the corresponding Service.
As a matter of fact, the local Kubernetes control plane perceives P<sub>3</sub> as executed locally and, since its possibly remapped IP address is present as part of its status, it creates the corresponding EndpointSlice entry (i.e., E<sub>3</sub>) as usual.

In the opposite scenario (e.g., the remote pod P<sub>3</sub> willing to communicate with a local one), the outgoing reflection takes action.
First, it creates a shadow copy of the local services, to enable transparent DNS discovery without requiring IP correspondence.
Second, it configures the appropriate EndpointSlice entries (possibly remapping the IP addresses, according to the network fabric configuration) to include the local service endpoints.
Indeed, these cannot be managed automatically by Kubernetes, as the corresponding Pods are not present in the remote cluster.

According to this approach, multiple replicas of the same microservice spread across different clusters, and backed by the same service, are handled transparently.
Each Pod, no matter where it is located, contributes with a distinct EndpointSlice entry, either by the standard control plane or through outgoing reflection, hence becoming eligible during the service load-balancing process.
