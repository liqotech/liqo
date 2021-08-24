---
title: Namespace replication
weight: 2
---

### The Liqo Namespace Model

The Liqo resource replication model is made by a set of components that replicate each object with a logic that is object-specific.
One important example is represented by namespaces. 
In particular, namespace replication across multiple clusters implements a key mechanism to extend seamleassly a cluster to some others.

In the Liqo model, the namespace replication operates on a a standard `v1.Namespace`, associated to a Liqo-specific [NamespaceOffloading](#) CR; this new CR enables to specify the (remote) clusters on which the application can be scheduled.

Section [Namespace Extensions](/usage/namespace_offloading) shows how to define and use a `NamespaceOffloading` resource that enables the replication of the local namespace on peered (remote) clusters.

This section shows how the Liqo namespace model works under the hood, understanding which resources and controllers are activated when a new `namespaceOffloading` is manipulated.

#### Resources

Liqo namespace replication involves instances of two kinds of CRDs:

* **NamespaceOffloading**: this CR represents the initial trigger of the namespace replication process; alternatively, the labelling of a namespace as explained in the [Namespace Extension](/usage/namespace_offloading)) section can also be used.
  On the one hand, the *NamespaceOffloading* spec describes the properties of the replication, such as the name of the replicated namespaces.
  On the other hand, the *status* collects the information about the actual status of remote namespaces, e.g., if the replication succeeded or not.
* **NamespaceMap**: this CR contains the list of namespaces associated to a specific node.
  The *spec* collects the list of desired namespaces for a specific remote cluster while the `status` keeps the updated information about their actual creation.  
  Each *NamespaceMap* can be alimented by several *NamespaceOffloading* instances that are targeting the same cluster for a remote namespace.
  It is worth noting that the NamespaceMap status represent the source of truth to know the status of replicated namesapces on foreign clusters.

#### Controllers

The namespace replication logic is made by several controllers, responsible for different aspects of namespace controllers:

* **NamespaceOffloading Controller**: it processes the NamespaceOffloading spec and creates the proper namespace entries into NamespaceMap specs.
* **NamespaceMap Controller**: it takes care of creating the required namespaces on remote clusters and updates their corresponding status in the NamespaceMaps.
* **OffloadingStatus Controller**: it updates the status of the NamespaceOffloading statuses gathering the information about namespace replication by reading the NamespaceMaps statuses.
* 
#### Workflow

The figure below presents a representation of the overall workflow steps of the replication process:

![](/images/namespace-replication/replication.png)

We can resume the steps in:

1. When the user creates/updates a **NamespaceOffloading** object in a Liqo-enabled namespace **(Step 1 in the figure)**, the Liqo logic processes the resource spec.

2. After having detected the virtual-nodes compliant with the **NamespaceOffloading** selector, the **NamespaceOffloading Controller** fills the *spec* in every NamespaceMap of a selected cluster with dedicated entries for the namespaces that should be created **(Step 2 in the figure)**.
In particular, it considers the **ClusterSelector** field to select on which cluster the current namespace should be replicated.

More precisely, the controller responsible for this reconciliation is the **NamespaceOffloading Controller**. 
It processes the *NamespaceOffloading* spec fields, by inserting the namespace creation requests in the *spec.DesiredMapping* field of **NamespaceMap** instances of selected clusters.
The request format is an entry consisting of the local namespace name as a key and the name of the remote namespace as a value.
In addition, this operation has to be performed every time a new virtual-node joins the cluster after a new peering is established.

3. Once a *NamespaceMap* is updated, the *NamespaceMap Controller* should create the corresponding remote namespace. 
More precisely, *NamespaceMap Controller* reconciles the NamespaceMap resources, by creating the remote namespaces and storing the operation results in the corresponding NamespaceMap.
To complete the namespace enforcement, the occurred creation is saved in the *status.CurrentMapping* field, as you can see in **Step 3 of the figure**.
The result format is similar to request one: the key is the name of the local namespace, while the value is composed of the remote name and the actual remote namespace phase.

In addition to the actual creation of namespaces, the *NamespaceMap Controller* (1) periodically checks that each entry in the *spec.desiredMapping* field has an associated remote namespace and (2) performs health checks on the remote namespaces. The result of all its operations are stored in the *status.CurrentMapping* field in the NamespaceMap resource.

4. Liqo periodically checks that the requested remote namespaces are present.
Whenever it detects a change in the namespaces state, it immediately updates the NamespaceMap resources.
The NamespaceOffloading status is updated thanks to NamespaceMap status changes **(step 4 in the figure)**.
The **OffloadingStatus Controller** is responsible for this NamespaceOffloading status reconciliation. It periodically checks the status of all *NamespaceMaps* in the clusters, and for each NamespaceOffloading object, (1) it updates the RemoteNamespaceConditions with the actual remote namespaces status and (2)  it changes the global OffloadingPhase according to the previously set remote conditions. 

As already detailed in the [NamespaceOffloading status description](/usage/namespace_offloading/#check-the-namespaceoffloading-resource-status), the fields that provide the user with all the information about the replication phase are the RemoteNamespaceConditions and the OffloadingPhase **(step 5 in the figure)**. 

### Deletion workflow

When the user decides to delete the NamespaceOffloading resource, the *Offloading status controller* sets the OffloadingPhase of the NamespaceOffloading resource to Terminating. 
The corresponding entries are removed from the NamespaceMaps by the *NamespaceMap Controller*. 
Liqo reacts to this event by requesting the deletion of the remote namespaces that are no longer required.
In particular, the *NamespaceOffloading Controller* removes the creation requests from the *spec.desiredMapping* field of the NamespaceMap resources.

Consequently, the *NamespaceMap Controller* checks enforces the deletion of the remote namespaces that are no longer required.
When a remote namespace is deleted, the *NamespaceMap Controller* removes the corresponding entry from the *status.CurrentMapping* field of the NamespaceMap resource. 

When an entry is removed from the *status.CurrentMapping* field of one NamespaceMap resource, the *OffloadingStatus Controller* deletes the remote conditions associated with that namespace in the NamespaceOffloading resource.

Once all the remote namespaces have been removed and, therefore, all entries from the NamespaceMaps, then the NamespaceOffloading resource is finally removed, and the deletion process is complete. More precisely, the *OffloadingStatus Controller* sets the OffloadingPhase of the NamespaceOffloading resource to Terminating. The *NamespaceOffloading Controller* deletes the NamespaceOffloading when there are no more remote namespaces associated with this resource.