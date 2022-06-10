# Namespace Offloading

This section presents the operational procedure to **offload a namespace** to (possibly a subset of) the **remote clusters** peered with the local cluster.
Hence, enabling **pod offloading**, as well as triggering the [**resource reflection**](/usage/reflection) process: additional details about namespace extension in Liqo are provided in the dedicated [namespace extension features section](FeatureOffloadingNamespaceExtension).

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

The *namespace mapping strategy* defines the naming strategy used to create the remote namespaces, and can be configured through the `--namespace-mapping-strategy` flag.
The accepted values are:

* **DefaultName** (default): to **prevent conflicts** on the target cluster, remote namespace names are generated as the concatenation of the local namespace name, the cluster name of the local cluster and a unique identifier (e.g., *foo* could be mapped to *foo-lively-voice-dd8531*).
* **EnforceSameName**: remote namespaces are named after the local cluster's namespace.
This approach ensures **naming transparency**, which is required by certain applications, as well as guarantees that **cross-namespace DNS queries** referring to reflected services work out of the box (i.e., without adapting the target namespace name).
Yet, it can lead to **conflicts** in case a namespace with the same name already exists inside the selected remote clusters, ultimately causing the remote namespace creation request to be rejected.

```{admonition} Note
Once configured for a given namespace, the *namespace mapping strategy* is **immutable**, and any modification is prevented by a dedicated Liqo webhook.
In case a different strategy is desired, it is necessary to first *unoffload* the namespace, and then re-offload it with the new parameters.
```

### Pod offloading strategy

The *pod offloading strategy* defines high-level constraints about pod scheduling, and can be configured through the `--pod-offloading-strategy` flag.
The accepted values are:

* **LocalAndRemote** (default): pods deployed in the local namespace can be scheduled **both onto local nodes and onto virtual nodes**, hence possibly offloaded to remote clusters.
* **Local**: pods deployed in the local namespace are enforced to be scheduled onto **local nodes only**, hence never offloaded to remote clusters.
The extension of a namespace, forcing at the same time all pods to be scheduled locally, enables the consumption of local services from the remote cluster, as shown in the [*service offloading* example](/examples/service-offloading).
* **Remote**: pods deployed in the local namespace are enforced to be scheduled onto **remote nodes only**, hence always offloaded to remote clusters.

```{admonition} Note
The *pod offloading strategy* applies to pods only, while the other objects that live in namespaces selected for offloading, and managed by the resource refletion process, are always replicated to (possibly a subset of) the remote clusters, as specified through the *cluster selector* (more details below).
```

```{warning}
Due to current limitations of Liqo, the pods violating the *pod offloading strategy* are not automatically evicted following an update of this policy to a more restrictive value (e.g., *LocalAndRemote* to *Remote*) after the initial creation.
```

(UsageOffloadingClusterSelector)=

### Cluster selector

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

## Unoffloading a namespace

The offloading of a namespace can be disabled through the dedicated *liqoctl* command, causing in turn the deletion of all resources reflected to remote clusters (including the namespaces themselves), and triggering the rescheduling of all offloaded pods locally:

```bash
liqoctl unoffload namespace foo
```

```{warning}
Disabling the offloading of a namespace is a **destructive operation**, since all resources created in remote namespaces (either automatically or manually) get removed, including possible **persistent storage volumes**.
Before proceeding, double-check that the correct namespace has been selected, and ensure no important data is still present.
```
