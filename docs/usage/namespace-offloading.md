# Namespace Offloading

This section presents the operational procedure to **offload a namespace**, which allows to extend a namespace across (possibly a subset of) the **remote clusters** peered with the local cluster.
Hence, enabling **pod offloading**, as well as triggering the [**resource reflection**](/usage/reflection) process, that enables the cross-cluster availability of the k8s resources in the offloaded namespace: additional details about namespace extension in Liqo are provided in the dedicated [namespace extension features section](FeatureOffloadingNamespaceExtension).

## Overview

The offloading of a namespace can be easily controlled through the dedicated **[liqoctl](/installation/liqoctl.md)** commands, which abstract the creation and update of the appropriate custom resources.
In this context, the most important one is the ***NamespaceOffloading*** resource, which enables the offloading of the corresponding namespace, configuring at the same time the subset of target remote clusters, additional constraints concerning pod offloading and the naming strategy.
Moreover, different namespaces can be characterized by different configurations, hence achieving a high degree of flexibility.
Finally, the *NamespaceOffloading* status reports for each remote cluster a **summary about its status** (i.e., whether the remote cluster has been selected for offloading, and the twin namespace has been correctly created).

## Offloading a namespace

A given namespace *foo* can be offloaded, leveraging the default configuration, through:

```bash
liqoctl offload namespace foo
```

Alternatively, the underlying *NamespaceOffloading* resource can be generated and output (either in *yaml* or *json* format) leveraging the dedicated `--output` flag:

```bash
liqoctl offload namespace foo --output yaml
```

Then, the resulting manifest can be applied with *kubectl*, or through automation tools (e.g., by means of GitOps approaches).

```{admonition} Note
Possible race conditions might occur in case a *NamespaceOffloading* resource is created at the same time (e.g., as a batch) as pods (or higher level abstractions such as *Deployments*), preventing them from being considered for offloading until the *NamespaceOffloading* resource is not processed.

This situation can be prevented manually labeling in advance the hosting namespace with the *liqo.io/scheduling-enabled=true* label, hence enabling the Liqo mutating webhook and causing pod creations to be rejected until pod offloading is possible.
Still, this causes no problems, as the Kubernetes abstractions (e.g., *Deployments*) ensure that the desired pods get eventually created correctly.
```

Regardless of the approach adopted, namespace offloading can be further configured in terms of the three main parameters presented below, each one exposed through a dedicated CLI flag.

### Namespace mapping strategy

When a namespace is offloaded, Liqo needs to create a *twin* namespace on the clusters where the namespaces is extended.
The *namespace mapping strategy* defines the naming strategy used to create the remote namespaces, and can be configured through the `--namespace-mapping-strategy` flag.
The accepted values are:

* **DefaultName** (default): to **prevent conflicts** on the target cluster, remote namespace names are generated as the concatenation of the local namespace name and the cluster name of the local cluster (e.g., `foo` could be mapped to `foo-name-of-the-local-cluster`).
* **EnforceSameName**: remote namespaces are named after the local cluster's namespace.
This approach ensures **naming transparency**, which is required by certain applications (e.g., Istio), as well as guarantees that **cross-namespace DNS queries** referring to reflected services work out of the box (i.e., without adapting the target namespace name).
However, it can lead to **conflicts** in case a namespace with the same name already exists inside the selected remote clusters, ultimately causing the remote namespace creation request to be rejected.
* **SelectedName**: you can specify the name of the remote namespace through the `--remote-namespace-name` flag.
This flag is ignored in case the *namespace mapping strategy* is set to *DefaultName* or *EnforceSameName*.

More details and examples can be found in the following sections.

```{admonition} Note
Once configured for a given namespace, the *namespace mapping strategy* is **immutable**, and any modification is prevented by a dedicated Liqo webhook.
In case a different strategy is desired, it is necessary to first *unoffload* the namespace, and then re-offload it with the new parameters.
```

### Cluster selector

A user might want to extend the offloaded namespace only on a subset of remote clusters.
The *cluster selector* provides the possibility to **restrict the set of remote clusters** (in case more than one peering is active) selected as targets for offloading the given namespace.
The *twin* namespace is not created in clusters that do not match the cluster selector, as well as the resource reflection mechanism is not activated for those namespaces.
Yet, different *cluster selectors* can be specified for different namespaces, depending on the desired configuration.

The cluster selector follows the standard **label selector** syntax, and refers to the Kubernetes labels characterizing the **virtual nodes**.
Specifically, these include both the set of labels suggested by the remote cluster during the peering process and automatically propagated by Liqo, as well as possible additional ones added by the local cluster administrators.

The cluster selector can be expressed through the `--selector` flag, which can be optionally repeated multiple times to specify alternative requirements (i.e., in logical OR).
For instance:

* `--selector 'region in (europe,us-west), !staging'` would match all clusters located in the *europe* or *us-west* region, *AND* not including the *staging* label.
* `--selector 'region in (europe,us-west)' --selector '!staging'` would match all clusters located in the *europe* or *us-west* region, *OR* not including the *staging* label.

In case no *cluster selector* is specified, all remote clusters are selected as targets for namespace offloading.
In other words, an empty *cluster selector* matches all virtual clusters.

## Pod offloading

The remote clusters are backed by a Liqo Virtual Node, which allows the vanilla Kubernetes scheduler to address the remote cluster as target for pod scheduling.
However, by default the Liqo virtual nodes have a [Taint](https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration/) applied to them, which prevents pods from being scheduled on them, unless a *namespace offloading* is enabled in the namespace where the pod is running.

You have two different ways to determine whether a pod can be scheduled on a virtual node (so on a remote cluster) and they are mutually exclusive per Liqo installation:

* Defining a **pod offloading strategy** for the offloaded namespaces (default), which tells where the pods created on that namespace should be scheduled (whether in the local cluster, the remote clusters, or both letting the vanilla K8s scheduler decide).
* Setting the Liqo **RuntimeClass** in the pod, in this case, the namespace offloading strategy is ignored, and the pod will be scheduled to the virtual nodes.

### Pod offloading strategy

The *pod offloading strategy* defines high-level constraints about pod scheduling, and can be configured through the `--pod-offloading-strategy` flag.
The accepted values are:

* **LocalAndRemote** (default): pods deployed in the local namespace can be scheduled **both onto local nodes and onto virtual nodes**, hence possibly offloaded to remote clusters. This will leave the Kubernetes scheduler to decide about the best placement, based on the available resources and the pod requirements. You can still influence the scheduler decision on which pods should be scheduled onto virtual nodes using the [standard Kubernetes mechanisms to assign Pods to Nodes](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/).
* **Local**: pods deployed in the local namespace are enforced to be scheduled onto **local nodes only**, hence never offloaded to remote clusters.
* **Remote**: pods deployed in the local namespace are enforced to be scheduled onto **remote nodes only**, hence always offloaded to remote clusters.

It is worth mentioning that, independently from the selected pod offloading strategy, the services that expose them are propagated to the entire namespace (both locally and in the remote cluster), hence enabling the above pods to be consumed from anywhere in the Liqo domain, as shown in the [service offloading example](../examples/service-offloading.md).

```{admonition} Note
The *pod offloading strategy* applies to pods only, while the other objects that live in namespaces selected for offloading, and managed by the resource reflection process, are always replicated to (possibly a subset of) the remote clusters, as specified through the *cluster selector* (more details below).
```

```{warning}
Due to current limitations of Liqo, the pods violating the *pod offloading strategy* are not automatically evicted following an update of this policy to a more restrictive value (e.g., *LocalAndRemote* to *Remote*) after the initial creation.
```

### RuntimeClass

At Liqo install or upgrade time, you can specify a flag to enable the creation of a [RuntimeClass](https://kubernetes.io/docs/concepts/containers/runtime-class/) to be used to specify the pods that should be offloaded to the virtual nodes.

```bash
liqoctl install [...] --set offloading.runtimeClass.enable=true
```

or

```bash
helm install liqo liqo/liqo [...] --set offloading.runtimeClass.enable=true
```

The RuntimeClass is created with the name `liqo`, and it is configured to add a Toleration to the virtual node taint for pods selecting it and to set a node selector to the virtual node's label.

(UsageOffloadingClusterSelector)=

## Unoffloading a namespace

The offloading of a namespace can be disabled through the dedicated *liqoctl* command, causing in turn the deletion of all resources reflected to remote clusters (including the namespaces themselves), and triggering the rescheduling of all offloaded pods locally:

```bash
liqoctl unoffload namespace foo
```

```{warning}
Disabling the offloading of a namespace is a **destructive operation**, since all resources created in remote namespaces (either automatically or manually) get removed, including possible **persistent storage volumes**.
Before proceeding, double-check that the correct namespace has been selected, and ensure no important data is still present.
```
