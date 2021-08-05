---
title: Namespace extension
weight: 3
---
### Introduction

The Liqo namespace model extends the Kubernetes namespace concept introducing the support for remote namespaces, "twins" of the local namespace. 
Those "twin" namespaces map the home cluster namespace and its associated resources (e.g., Service, Configmap) on remote clusters. 
Regarding pod offloading, remote namespaces host the offloaded pods belonging to the home twin namespace, as if they were executed in the home cluster.

### Quick offloading

Liqo provides a label-based mechanism to set up namespace offloading over remote clusters.
To enable it, you should label a target namespace:

```bash
kubectl label namespace target-namespace liqo.io/enabled=true
```

Pods scheduled in the "*target-namespace*" will be potentially offloaded inside remote clusters. 
With the *quick offloading* approach, you are selecting all peered clusters suitable for offloading. 
If you need a fine-grained approach, you can rely on custom offloading and the *NamespaceOffloading* resource.

### Custom offloading

To control all different aspects of the namespace extension, Liqo provides a NamespaceOffloading resource. 
The policies defined inside the NamespaceOffloading object specify how the local namespace can be replicated on peered clusters.

Any namespace inside the home cluster can have its NamespaceOffloading resource and its corresponding offloading policy.
In other words, the NamespaceOffloading object defines "*per-namespace boundaries*", limiting the scope where pods can be remotely offloaded.

As presented in the following example, the NamespaceOffloading resource is composed of three main fields:

{{% render-code file="static/examples/namespace-offloading-default.yaml" language="yaml" %}}

{{% notice warning %}}
The resource name must always be "*offloading*" to ensure the uniqueness of a single configuration for each local namespace. 
A resource created with a different name will not trigger the topology creation.
{{% /notice %}}

#### Selecting the namespace mapping strategy

The *NamespaceMappingStrategy* defines the naming strategy used to create the remote namespaces. 
The accepted values are:

| Value               | Description |
| --------------      | ----------- |
| **DefaultName** (Default)    | The remote namespaces have the name of the local namespace followed by the local cluster-id to guarantee the absence of conflicts. |
| **EnforceSameName** | The remote namespaces have the same name as the namespace in the local cluster (this approach can lead to conflicts if a namespace with the same name already exists inside the selected remote clusters). |

{{% notice info %}}
The DefaultName value is recommended if you do not have particular constraints related to the remote namespaces name. 
However, using the DefaultName policy, the namespace name cannot be longer than 63 characters according to the [RFC 1123](https://datatracker.ietf.org/doc/html/rfc1123). 
Since the cluster-id is 37 characters long, the home namespace name can have at most 26 characters.
{{% /notice %}}

#### Selecting the pod offloading strategy

The *PodOffloadingStrategy* defines constraints about pod scheduling.

| Value              | Description |
| --------------     | ----------- |
| **LocalAndRemote** (Default) | The pods deployed in the local namespace can be scheduled both locally and remotely. |
| **Local**          | The pods deployed in the local namespace are always scheduled inside the local cluster, never remotely. |
| **Remote**         | The pods deployed in the local namespace are always scheduled inside the remote clusters, never locally. |

{{% notice note %}}
Unlike pods, standard Kubernetes Services are always replicated inside all the selected clusters.
{{% /notice %}}

The LocalAndRemote strategy does not impose any constraints, and it leaves the scheduler the choice to select both local and remote nodes.
The Remote and Local strategies force the pods to be scheduled respectively only remotely and only locally.

{{% notice warning %}}
If you specify pod NodeSelectorTerms not compatible with these constraints, the pod will remain unscheduled and pending.
{{% /notice %}}

#### Selecting the remote clusters

The *ClusterSelector* specifies the NodeSelectorTerms to target specific clusters of the topology. 
Such NodeSelectorTerms can be specified by using the [Kubernetes NodeAffinity syntax](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#node-affinity).  
These selector terms specified inside the NamespaceOffloading resource are applied on *every pod* created inside the namespace.

If not specified at creation time, the ClusterSelector will target all virtual nodes available, enabling the offloading on all peered clusters. 
More precisely, the default value corresponds to:

```yaml
clusterSelector:
  nodeSelectorTerms:
  - matchExpressions:
    - key: liqo.io/type
      operator: In
      values:
      - virtual-node
``` 

{{% notice info %}}
In *Liqo 0.3*, the NamespaceOffloading object must contain the configuration at creation time. 
If you want to modify its structure at run-time, you should delete the resource and recreate it with a new configuration. 
This triggers the deletion of all remote "twin" namespaces and the creation of new ones.
{{% /notice %}}

The MatchExpressions specified in the ClusterSelector term select labels attached to Liqo virtual nodes.
More precisely, at Liqo installation time, you may identify a set of labels to expose the most relevant features of your cluster. 
After the peering phase, the virtual node will expose those labels, enabling the possibility to select it during the offloading configuration. 
Moreover, the scheduler will also be able to use these virtual nodes' labels to impose affinity/anti-affinity policies at run-time.

It is worth noting that there is no restriction on the labels to choose. 
Labels can characterize your clusters showing their geographical location, the underlying provider, or the presence of specific hardware devices.

{{% notice tip %}}
If you want to create deployment topologies that include all available clusters, you are not required to choose labels at installation time. 
All virtual nodes expose the label `liqo.io/type = virtual-node` by default.
{{% /notice %}}

### NamespaceOffloading resource in quick offloading

Also the *quick offloading* approach relies on the NamespaceOffloading resource. 
In fact, when the label `liqo.io/enabled = true` is added to a namespace, this event triggers the creation of a default NamespaceOffloading resource inside the namespace.

The generated resource is equal to the template [seen above](#custom-offloading), setting all fields to the default values:

| Field                         | Value |
| --------------                | ----------- |
| **NamespaceMappingStrategy**  | DefaultName. |
| **PodOffloadingStrategy**     | LocalAndRemote. |
| **ClusterSelector**           | All remote clusters selection. |

{{% notice info %}}
Using the *quick offloading* approach, it is not possible to customize the generated NamespaceOffloading resource. 
If a NamespaceOffloading object is already present in the namespace, you should remove the label first and create the new resource.
{{% /notice %}}

{{% notice tip %}}
If you create a custom NamespaceOffloading resource inside the namespace, the `liqo.io/enabled = true` label is not necessary.
As previously said, this label is just a way to configure a *quick offloading*. 
{{% /notice %}}

### Check the NamespaceOffloading resource status

The Liqo controllers update the NamespaceOffloading object every time there is a change in the deployment topology. 
The resource status provides different information:

```bash
kubectl get namespaceoffloading offloading -n target-namespace -o yaml
```

#### OffloadingPhase

The *OffloadingPhase* informs you about the namespaces offloading status.
It can assume different values:

| Value                 | Description |
| --------------        | ----------- |
| **Ready**             |  Remote Namespaces have been correctly created inside previously selected clusters. |
| **NoClusterSelected** |  No cluster matches user constraints or constraints are not specified with the right syntax (in this second case, an annotation is also set on the NamespaceOffloading resource, specifying what is wrong with the syntax)        |
| **SomeFailed**        |  There was an error during some remote namespaces creation. |
| **AllFailed**         |  There was an error during all remote namespaces creation. |
| **Terminating**       |  Remote namespaces are undergoing graceful termination. |

#### RemoteNamespacesConditions

The *RemoteNamespacesConditions* allows you to verify remote namespaces' presence and their status inside all remote clusters.
This field is a map that has the *remote cluster-id* as key and as value, a *vector of conditions* for the namespace created inside that remote cluster.
There are two types of conditions:

1. **Ready**

   | Value     | Description |
   | -------   | ----------- |
   | **True**  |  The remote namespace is successfully created. |
   | **False** |  There was a problem during the remote namespace creation. |

2. **OffloadingRequired**

   | Value     | Description |
   | -------   | ----------- |
   | **True**  |  The creation of a remote namespace inside this cluster is required |
   | **False** |  The creation of a remote namespace inside this cluster is not required. |

{{% notice note %}}
The RemoteNamespacesConditions syntax is the same of the standard [v1.NamespaceCondition](https://pkg.go.dev/k8s.io/api/core/v1@v0.21.0#NamespaceCondition).
{{% /notice %}}

### Offloading termination

To terminate the offloading, you can remove the NamespaceOffloading resource inside the namespace or, in case of the *quick offloading* approach, the previously inserted label `liqo.io/enabled = true`.

{{% notice warning %}}
Deleting the NamespaceOffloading object or removing the label, all remote namespaces will be deleted with everything inside them.
{{% /notice %}}