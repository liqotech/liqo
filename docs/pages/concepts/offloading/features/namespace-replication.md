---
title: Namespace replication
weight: 2
---

### Replication context

The core element in the remote namespaces' replication is the *Liqo namespace*.
This Liqo term corresponds to a standard v1.Namespace and an associated [NamespaceOffloading](#) CRD.
You have already seen in [the usage section](#) how to define a NamespaceOffloading resource activating the replication of the local namespace on peered clusters.

In this section, you are provided with a deeper explanation of how replication logic works

### Replication scenario

This figure shows the various steps of the replication process:

![](/images/namespace-replication/replication.png)

The NamespaceOffloading is a resource for the user interface, while the *NamespaceMap* is a technical Liqo CRD.
Each virtual-node has a NamespaceMap resource associated.
This resource keeps track of all namespaces mapped on that remote cluster.
The purpose of the NamespaceMap fields represented in the figure will be clarified step by step during the following explanation.

### Replication workflow

When the user deploys a NamespaceOffloading object in a local namespace (Step 1 in the figure), Liqo processes the resource spec.
In particular, it considers the ClusterSelector field. 

After having detected the virtual-nodes compliant with selector, a namespace creation request is entered in every NamespaceMap associated with a selected cluster.
The request format is an entry consisting of the local namespace name as a key and the name of the remote namespace as a value.
The NamespaceMaps are filled with these requests. 
More precisely, the `spec.DesiredMapping` field keeps track of them (Step 2 in the figure).

Once the request is received, Liqo tries to fulfill it, creating the remote namespace.
The creation outcome is stored in the corresponding NamespaceMap.
More precisely, it is saved in the `status.CurrentMapping` field, as you can see in step 3 of the figure.
The result format is similar to request one: the key is the name of the local namespace, while the value is composed of the remote name and the actual remote namespace phase.
The NamespaceMap status is the only place where all the cluster resources receive information about the remote namespaces conditions.

Liqo periodically checks that the requested remote namespaces are present.
Whenever it detects a change in the namespaces state, it immediately updates the NamespaceMap resources.
The NamespaceOffloading status is updated thanks to NamespaceMap status changes (step 4 in the figure).
As already seen in the [NamespaceOffloading status description](/usage/namespace_offloading/#check-the-namespaceoffloading-resource-status), the fields that provide the user with all the information about the replication phase are the RemoteNamespaceConditions and the OffloadingPhase (step 5 in the figure). 

Starting from the replication request in the NamespaceOffloading resource, it is possible to observe its completion through the status of the resource itself.

### Deletion workflow

When the user decides to delete the NamespaceOffloading resource, its OffloadingPhase is set to Terminating, and the corresponding creation requests are removed from the NamespaceMaps.
Liqo reacts to this event by requesting the deletion of the remote namespaces that are no longer required.

When a remote namespace is deleted, the corresponding entry in the NamespaceMap status is removed and consequently the RemoteNamespacesCondition in the NamespaceOffloading resource.
In this way, the user can keep track of how the deletion process is evolving.

Once all the remote namespaces have been removed and, therefore, all entries from the NamespaceMaps, then the NamespaceOffloading resource is finally removed, and the deletion process is complete.





