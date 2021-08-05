---
title: Replication controller logic
weight: 3
---

The previous section figures out the core concepts of the namespace replication and the resources involved.
As you can see, the control plane component that performs a particular action is not specified, but the generic term "Liqo" is used instead.
In this section, both the "Replication workflow" and the "Deletion workflow" are considered again, paying particular attention to which Liqo controllers are involved in the process and what their function is.

This image, compared to the previous section one, adds the Liqo controllers' role:

As the [previous figure shows](/concepts/offloading/features/namespace_replication), the core controllers involved in the replication procedure are:

* **namespace-offloading-controller**: processes the spec fields and submits creation requests to the NamespaceMaps (step 2 in the figure).
* **namespace-map-controller**: satisfies the requests creating the remote namespaces and updates the status of the NamespaceMaps (step 3 in the figure).
* **offloading-status-controller**: updates the status of the NamespaceOffloading resources by reading the NamespaceMaps one (step 4 in the figure).

### Replication workflow (controllers view)

#### Namespace-offloading-controller

The namespace-offloading-controller reconciles the NamespaceOffloading resources.
It processes the resource spec fields and carries out two core features for the replication process:

* It sets the namespace-replicas name according to the NamespaceMappingStrategy field.
* It identifies which virtual-nodes are compliant with the ClusterSelector, and fills the corresponding NamespaceMap resources with the namespace creation requests. This operation is performed every time a new virtual-node is generated after the peering phase.

{{% notice note %}}
The PodOffloadingStrategy is not involved in the namespaces' replication, it provides relevant information only for [the pod scheduling](#).
{{% /notice %}}

#### Namespace-map-controller

The namespace-map-controller reconciles the NamespaceMap resources.
More precisely, it performs the following tasks for every NamespaceMap object in the cluster:

* It creates remote namespaces associated with requests.
* It periodically checks that each entry in the `spec.desiredMapping` field corresponds to a remote namespace. 
* It performs health checks on the remote namespaces and updates the `status.CurrentMapping` field in the NamespaceMap resource

#### Offloading-status-controller

The offloading-status-controller reconciles the NamespaceOffloading resources.
It periodically checks the status of all NamespaceMaps in the clusters, and for each NamespaceOffloading object:

* It updates the RemoteNamespaceConditions with the actual remote namespaces status.
* It changes the global OffloadingPhase according to the previously set remote conditions. 

### Deletion workflow (controllers view)

This paragraph shows the role that the previous controller assume during the deletion phase. 
The resources reconciled are always the same.

#### Namespace-offloading-controller

* It removes the creation requests from the `spec.desiredMapping` field of the NamespaceMap resources.
* It deletes the NamespaceOfflaoding when there are no more remote namespaces associated with this resource.

#### Namespace-map-controller

* It checks which requests are removed and deletes the remote namespaces that are no longer required.
* When a remote namespace is deleted, it removes the corresponding entry from the `status.CurrentMapping` field of the NamespaceMap resource. 

#### Offloading-status-controller

* It sets the OffloadingPhase of the NamespaceOffloading resource to Terminating. 
* When an entry is removed from the `status.CurrentMapping` field of one NamespaceMap resource, it deletes the remote conditions associated with that namespace in the NamespaceOffloading resource.



