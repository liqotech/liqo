---
title: Namespace replication
weight: 2
---

### The Liqo Namespace Model

The Liqo resource replication model is made by a set of components that replicate each resource with an object-specific logic.
One important example is represented by namespaces. 
More precisely, namespace replication across multiple clusters implements a core mechanism to seamlessly extend a cluster to some others.

In the Liqo model, the namespace replication operates on a standard *v1.Namespace*, associated with a Liqo-specific [NamespaceOffloading](/usage/namespace_offloading#custom-offloading) CR; this new CR allows to specify the remote clusters on which the application can be scheduled.

Section [Namespace Extensions](/usage/namespace_offloading) shows how to define and use a NamespaceOffloading resource that enables the replication of the local namespace on peered clusters.

This section shows how the Liqo namespace model works under the hood, understanding which resources and controllers are activated when a new NamespaceOffloading is manipulated.

#### Resources

Liqo namespace replication involves instances of two kinds of CRDs:

* **NamespaceOffloading**: this CR represents the initial trigger of the namespace replication process; alternatively, the namespace labeling can also be used as explained in the [Namespace Extension](/usage/namespace_offloading) section.
  On the one hand, the NamespaceOffloading spec describes the replication properties, such as the name of the replicated namespaces.
  On the other hand, the status collects information about the actual conditions of remote namespaces, e.g., if the replication succeeded or not.
* **NamespaceMap**: this CR contains the list of namespaces that must be created on the remote cluster associated with this resource.
  The spec collects the list of desired namespaces for a specific remote cluster while the status keeps the updated information about their actual creation.  
  Each NamespaceMap is filled by several NamespaceOffloading resources targeting the same cluster for a remote namespace.
  For example, if three different NamespaceOffloading require a remote namespace on the same cluster, the NamespaceMap associated with that virtual node will see three new creation requests in its spec field.
  It is worth noting that the NamespaceMap status represents the source of truth to know the status of replicated namespaces on foreign clusters.

#### Controllers

The namespace replication logic is performed by several controllers responsible for different aspects of this process:

* **NamespaceOffloading Controller**: it processes the NamespaceOffloading spec and creates the proper namespace entries into NamespaceMap resources.
* **NamespaceMap Controller**: it takes care of the required namespaces' creation on remote clusters and updates their status in the NamespaceMap resources.
* **OffloadingStatus Controller**: it updates the NamespaceOffloading status gathering information about namespace replication from the NamespaceMap resources' status.

#### Workflow

The figure below figures out the overall workflow steps of the replication process:

![](/images/namespace-replication/replication.png)

The mechanism can be resumed in the following steps:

1. When the user creates a NamespaceOffloading object in a Liqo-enabled namespace **(Step 1 in the figure)**, the Liqo logic processes the resource spec. 
After having detected the virtual nodes compliant with the NamespaceOffloading selector, the NamespaceOffloading controller fills the NamespaceMap resources of the selected nodes.
More precisely, the controller sets the creation requests in the DesiredMapping spec field of the various NamespaceMap resources **(Step 2 in the figure)**. 
This logic is recalled every time a new virtual node joins the topology to check if it is compliant with the NamespaceOffloading selector.

2. Once the NamespaceOffloading controller has filled the NamespaceMap with the requests, the NamespaceMap controller should enforce the remote namespaces' creation.
The operations outcome is saved in the CurrentMapping status field (**Step 3 in the figure**).
The NamespaceMap Controller periodically checks if each entry in the DesiredMapping spec field has an associated remote namespace. 
In case of absence, it immediately enforces a new namespace creation.
Furthermore, the controller performs health probes on these namespaces. 
Whenever it detects a change in the namespaces state, it immediately updates the NamespaceMap resources. 

3. The OffloadingStatus Controller is responsible for the NamespaceOffloading status reconciliation **(Step 4 in the figure)**. It periodically checks the status of all NamespaceMaps in the clusters, and for each NamespaceOffloading object, it updates the RemoteNamespaceConditions and the OffloadingPhase fields. As already detailed in the [NamespaceOffloading status description](/usage/namespace_offloading/#check-the-namespaceoffloading-resource-status), these fields provide the user with all the information about the replication process **(Step 5 in the figure)**. 

The following figure provides an example of this mechanism:

![](/images/namespace-replication/replication-example.png)

The steps represented are the same as described above. More precisely, the constraints specified in the NamespaceOffloading require the creation of a remote namespace only on the *regionB-cluster*. Consequently, only the corresponding NamespaceMap will be updated.

### Deletion workflow

The deletion mechanism can be summarized in these core steps:

1. When the user decides to delete the NamespaceOffloading resource, the termination of the replication process starts, and the OffloadingStatus controller sets the OffloadingPhase field to Terminating. 

2. The NamespaceOffloading controller removes all the entries associated with that resource from the DesiredMapping field of the NamespaceMap objects.

3. The NamespaceMap controller reacts to this event enforcing the deletion of all the remote namespaces no longer required. In particular, when a remote namespace is deleted, it removes the corresponding entry from the NamespaceMap CurrentMapping field. 

4. When the NamespaceMap controller removes an entry from the CurrentMapping field of one NamespaceMap resource, the OffloadingStatus controller deletes the remote conditions associated with that namespace in the NamespaceOffloading resource. Once all the remote namespaces have been removed and, therefore, all associated remote namespace conditions, then the NamespaceOffloading resource is finally removed, and the deletion process is complete.