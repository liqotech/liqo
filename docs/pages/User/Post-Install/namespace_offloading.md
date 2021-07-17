---
title: Built deployment topologies
weight: 3
---
## Introduction

Liqo allows you to deploy applications in complex multi-cluster topologies, however it is not always necessary to have
an offloading inside all available clusters. **Liqo 0.3** therefore allows you to create **deployment topologies** even on
a clusters' subset.

Let's try to understand what it means to create a deployment topology, looking at the figure below:

![]()

The cluster home (cluster 1) have a peering with clusters 2 and 3.

Let's say we want to deploy::
1. An "***application 1***" only locally and inside the remote cluster 2. (red)
2. An "***application 1***" only locally and inside the remote cluster 2, like the application 1 (blue)
3. An "***application 3***" only inside the remote cluster 3 and ***not locally*** (green)

A deployment topology is not created for a specific application, but has a 1:1 association with a
**deployment configuration**.

A deployment configuration is a set of constraints including:

- The selected remote clusters.
- the pod offloading strategy (local, remote, local and remote).
- and other constraints currently not relevant.

In our example, application 1 and 2 have the same deployment configuration:

"***select remote cluster 2 only and allow pod offloading both locally and remotely***"

These constraints are defined within a Liqo CRD, called "**NamespaceOffloading**". Once the resource has been 
configured, it is sufficient to insert it in a local namespace to create the desired deployment topology.
Liqo controllers read the resource and replicate the local namespace on each cluster selected, generating 
a namespaces network.

>__NOTE__: According to the Liqo terminology, the local namespace is called ***Liqo namespace***.

The configuration resource must be bound to a local namespace, because the specified constraints are valid
only for what is deployed within it. In the same cluster there can be "n" Liqo namespace each one with its own
configuration and its corresponding deployment topology.

We can say that the physical translation of the deployment topology concept is a network of namespaces that
respect the configuration constraints imposed by the resource:

   ***deployment configuration + namespaces network = deployment topology***

Considering the previous scenario:

![]()

Since applications 1 and 2 require the same deployment configuration we can create a single Liqo namespace
with the respective NamespaceOffloading. The application 3, on the other hand, requires a different configuration
therefore it is necessary to create another Liqo namespace (called topology-2) with its resource. In total we have 
2 topologies for 3 applications.

The namespaces networks that belong to different topologies are represented with different colors.

> __NOTE__ : these topologies are extremely dynamic. If an application needs clusters with certain
characteristics and there is a new peering with a cluster of this type, it is immediately added to the topology.

If this overview has caught your attention, continue reading to understand how to configure your topologies
in a few simple steps.

## Configure cluster labels at Liqo installation time

To identify only some of the available clusters we need labels that characterize them.

When you install Liqo on your cluster you can choose this set of labels that allows to identify its most relevant 
features in an intuitive way. If you want to see how to define these labels at Liqo installation time look 
at the [Getting started section](#).

There is no restriction on the labels to choose, however it is clear that selecting significant labels
makes it easier to group clusters with the same characteristics in one fell swoop.

We can observe the 3 clusters with Liqo already installed, each one has its own labels:

![]() 

When the peering process is complete each remote cluster is exposed as a virtual node in the home cluster.
Each virtual node shows its own labels so it is possible to use a label selector to choose the desired remote clusters.

![]() 

If you don't remember how to generate a peering between multiple clusters, take a look at the [appropriate section](#)

> __NOTE__: If you just want to create deployment topologies that include all available clusters you are not required to
> choose labels at installation time. All virtual nodes expose the label ***liqo.io/type = virtual-node*** by default,
> Liqo uses this to automatically select all clusters.

## NamespaceOffloading structure and how to configure it

The NamespaceOffloading is the resource that contains the deployment configuration for the topology.

The Liqo webhook ensures that the constraints specified in the configuration are always respected.
Your application is never offloaded inside an unselected cluster, you have always the full control of where
your pods are deployed and who can reach them.

A template of the NamespaceOffloading resource is as follows:

{{% render-code file="static/examples/namespace-offloading-default.yaml" language="yaml" %}}

The name of the resource must be always "**offloading**" to ensure the uniqueness of a single configuration for
each local namespace. A resource created with a different name will not trigger the topology creation.

Now let's see how to enforce configuration parameters in the ***NamespaceOffloading Spec***:

1. The **namespaceMappingStrategy** parameter can assume 2 values:

   | Value               | Description |
   | --------------      | ----------- |
   | **EnforceSameName** | The remote namespaces have the same name as the namespace in the local cluster (this approach can lead to conflicts if a namespace with the same name already exists inside the selected remote clusters). |
   | **DefaultName**     | The remote namespaces have the name of the local namespace followed by the local cluster-id to guarantee the absence of conflicts inside the remote clusters. |

   Other values are not accepted. If this field is omitted it will be set by default to the value of **DefaultName**.

   > __NOTE__: The **DefaultName** value is recommended if you do not have particular constraints related to the remote
   > namespaces name.


2. The **podOffloadingStrategy** parameter can assume the 3 values:

   | Value              | Description |
      | --------------     | ----------- |
   | **Local**          | The pods deployed in the local namespace are always scheduled inside the local cluster, never remotely.
   | **Remote**         | The pods deployed in the local namespace are always scheduled inside the remote clusters, never locally.
   | **LocalAndRemote** | The pods deployed in the local namespace can be scheduled both locally and remotely.
   
   Other values are not accepted. If this parameter is omitted it will be set by default to the value of **LocalAndRemote**.

   The pod offloading strategy **LocalAndRemote** does not impose constraints, it leaves the scheduler the choice
   to deploy locally or remotely. While the **Remote** and **Local** strategies force the pods to be scheduled 
   respectively only remotely and only locally.
   If the user tries to violate these constraints, adding conflicting restrictions, the pod will remain pending.

   If you select 2 remote clusters and a podOffloadingStrategy ***LocalAndRemote***, you don't know which cluster the
   pod will be scheduled inside. The scheduler may also decide to keep the pod locally based on available resources. if
   you want to force a pod to be scheduled on a specific cluster you have to add further restrictions on it.

   > __NOTE__: This strategy only applies to pods, services are always replicated inside all the selected clusters.

3. The **clusterSelector** allows us to select the clusters that will be part of the desired
   deployment topology, the[k8s NodeAffinity](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#node-affinity)
   syntax is used.

   The selection takes place through the labels that the various virtual nodes expose as we have previously said.
   
   If you want to generate deployment topologies with all the available clusters, you can leave this field empty,
   if it is omitted it will be set by default to the value:
   ```yaml
   clusterSelector:
     nodeSelectorTerms:
     - matchExpressions:
       - key: liqo.io/type
         operator: In
         values:
         - virtual-node
   ``` 
   i.e. all available remote cluster are selected.
   
   > __NOTE__: This selector will be imposed on ***every pod*** scheduled inside the Liqo namespace, therefore 
   > you have the guarantee that your pods can only be scheduled inside the clusters you have selected.


__ATTENTION__: In ***Liqo 0.3*** the NamespaceOffloading must contain the configuration at creation time,
once created it must ***no longer be modified***. If you want to change the offloading constraints you have
to delete the resource and recreate it with a new configuration. The old deployment topology is destroyed and a 
new one will be created.

## The two offloading scenarios

Now that we have seen all the elements necessary to understand the mechanism, we will see
in detail the few steps to generate a deployment topology. Based on your use
case we have identified two different scenarios:

- If your goal is to use the resources of all available clusters to deploy your workload regardless of
cluster type and how your pods are scheduled, then the **Default offloading** can be great
for your use case.

- If on the other hand your scenario is more complex, and you need a particular configuration like:
   * pods scheduled only inside "trusted" clusters.
   * pod deployed only locally with services that remotely expose them.
   * remote namespaces with the same name of the local one to activate particular services.
   * and many others.
then you have to use a **Custom offloading**.
     
## Default offloading

Let's consider our reference architecture with 3 clusters. We want to deploy an application that is able to exploit
the resources of the two remote clusters. It doesn't matter where the pods are scheduled or what is the 
remote namespaces name, the important thing is that the application is as efficient as possible. In this case
2 simple steps are enough:

### Start offloading

1. Create the Liqo namespace inside the local cluster.

```bash
  kubectl create namespace test-namespace
```

> #### Name constraints:
> The namespace name cannot be longer than 63 characters according to the [RFC 1123](https://datatracker.ietf.org/doc/html/rfc1123).
> Since adding the cluster-id requires 37 characters, your namespace name can have at most 26 characters


2. Now add the label **liqo.io/enabled = true** to your namespace.

```bash
  kubectl label namespace test-namespace liqo.io/enabled=true
```

When the label is inserted, a default NamespaceOffloading is automatically created. The resource is exactly 
equal to the template seen above, the parameters have all the default values:

1. **namespaceMappingStrategy** = **DefaultName**.
2. **podOffloadingStrategy** = **LocalAndRemote**.
3. **clusterSelector** = the default one previosly seen.

A remote namespace with ***DefaultName*** is created inside all available remote clusters (2 and 3), and the pods
can be scheduled both locally and remotely. The current situation is represented by the following figure, which shows
how some pods have been deployed locally while others inside the cluster 2. The scheduler is not required to schedule 
pods inside all previously selected remote clusters. The cluster 3 although selected does not receive any pods.

![]()

> __ATTENTION__: if you decide to use the label it is not possible to create a custom resource as there is already a
> **NamespaceOffloading** in that Liqo namespace. You have to remove the label first and then create the new resource.


### Offloading termination

To terminate the offloading, simply remove the previously inserted label **liqo.io/enabled = true** or directly
delete the resource inside the namespace.
> __ATTENTION__: ending the offloading, all remote namespaces will be deleted with everything inside them.


## Custom offloading

> __NOTE__ : Liqo 0.2.1 allowed you to use only the label mechanism. In Liqo 0.3
> the custom configuration is introduced and so the possibility to configure the offloading process.

Let's consider our reference architecture. We want to deploy an application locally or only inside the cluster 2.

### Start offloading

1. Create the Liqo namespace inside the local cluster. (remember the [name constraint](#name-constraints)).

```bash
  kubectl create namespace test-namespace
```

2. Now create a **NamespaceOffloading** resource inside the namespace. Consider a resource like this:

{{% render-code file="static/examples/namespace-offloading-custom.yaml" language="yaml" %}}

As we recall from our reference topology, cluster 2 was installed with exactly the labels present in the cluster 
selector. A remote namespace with the same name as the local one (** EnforceSameName **) will be created
inside the cluster 2. The pods can be scheduled both locally and inside the cluster 2. The situation described in the
figure below shows how the scheduler decided to schedule all pods locally in this case:

![]()

### Offloading termination

If you wish to terminate offloading, simply remove the previously created resource or directly delete 
the local namespace.

## Dynamic peering

Liqo makes your deployment topologies extremely dynamic. If you have selected a clusters set
with certain features, and you decide to add more resources with a new cluster of the same type you will not need
to terminate the offloading and reconfigure it. Your topology will be automatically updated, and your applications
from now on could also be scheduled on the new cluster.

Similarly, if you need to disconnect one of the clusters, just disable peering to still have an updated topology.

## Check the offloading status

The liqo controllers update the NamespaceOffloading every time there is a change in the topology
deployment. The resource status provides different information:

```bash
  kubectl get namespaceoffloading offloading -n test-namespace -o wide
```

The **remoteNamespaceName** must match the namespaceMappingStrategy chosen.

### Offloading phase

The global offloading status (**OffloadingPhase**) can assume different values:

| Value                 | Description |
| --------------        | ----------- |
| **Ready**             |  Remote Namespaces have been correctly created inside previously selected clusters. |
| **NoClusterSelected** |  No cluster matches user constraints or constraints are not specified with the right syntax (in this second case an annotation is also set on the namespaceOffloading, specifying what is wrong with the syntax)        |
| **SomeFailed**        |  There was an error during creation of some remote namespaces. |
| **AllFailed**         |  There was an error during creation of all remote namespaces. |
| **Terminating**       |  Remote namespaces are undergoing graceful termination. |

### Remote namespace conditions

If you want more detailed information about the offloading status, you can check the **remoteNamespaceConditions**
inside the NamespaceOffloading resource:

```bash
   kubectl get namespaceoffloading offloading -n test-namespace -o yaml
```

The **remoteNamespaceConditions** field is a map which has as its key the ***remote cluster-id*** and as its value
a ***vector of conditions for the namespace*** created inside that remote cluster. There are two types of conditions:

1. **Ready**

   | Value   | Description |
   | ------- | ----------- |
   | **True**  |  The remote namespace is successfully created. |
   | **False** |  There was a problems during the remote namespace creation. |

2. **OffloadingRequired**

   | Value   | Description |
   | ------- | ----------- |
   | **True**  |  The creation of a remote namespace inside this cluster is required (the condition ***OffloadingRequired = true*** is removed when the remote namespace acquires a ***Ready*** condition). |
   | **False** |  The creation of a remote namespace inside this cluster is not required. |

> __NOTE__: The **RemoteNamespaceCondition** syntax is the same of the standard [NamespaceCondition](https://pkg.go.dev/k8s.io/api/core/v1@v0.21.0#NamespaceCondition).
