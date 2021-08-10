---
title: Namespace Offloading
weight: 3
---

This page collects some common questions about namespace replication in Liqo.
You can check the [dedicated section](/usage/namespace_offloading) to configure the replication of your namespaces on remote clusters.

### After having created the NamespaceOffloading resource, how can I check if the remote namespaces are created inside the clusters?

You can check that the [NamespaceOffloading](/usage/namespace_offloading#custom-offloading) resource is correctly reconciled inside the local namespace you want to replicate:

{{% notice note %}}
The resource name must be *offloading*.
Liqo controllers do not reconcile a resource with a different name.
{{% /notice %}}

```bash
kubectl get namespaceoffloadings offloading -n <your-namespace> -o yaml
```
In the status, you can observe how the namespace replication is progressing.

Liqo tracks the global status of the namespace reconciliation.
More precisely, the resource offloading status called [OffloadingPhase](/usage/namespace_offloading#offloadingphase) can assume one of the following values in case of problems:

| Value                  | Description |
| -------                | ----------- |
| **AllFailed**          |  There was a problem during the creation of all remote namespaces. |
| **SomeFailed**         |  There was a problem during the creation of some remote namespaces (one or more). |
| **NoClusterSelected**  |  No cluster matches the desired policies, or specified constraints do not follow some syntax rules.|       

```bash
kubectl get namespaceoffloadings offloading -n <your-namespace> -o=jsonpath="{['status.offloadingPhase']} "
```

In the case of *NoClusterSelected* status, you are potentially facing:

1. There may be an error in the specified [ClusterSelector](/usage/namespace_offloading#selecting-the-remote-clusters) syntax.
   The NamespaceOffloading resource should expose an annotation with key `liqo.io/scheduling-enabled` that signals the syntax error:
   
   ```bash
   kubectl get namespaceoffloadings -n <your-namespace> offloading -o=jsonpath="{['metadata.annotations.liqo.io/scheduling-enabled']}"
   ```

2. The [cluster labels](/usage/namespace_offloading#cluster-labels-concept) may not have been inserted correctly at Liqo install time.
   You can check the labels that every virtual-node expose:

   ```bash
   kubectl get nodes --selector=liqo.io/type --show-labels
   ```

   The labels you get here are the same label you should use in the namespaceOffloading resource.

   You can set the cluster labels at install time. For example, you may follow the [Liqo Extended tutorial](#).

In the case of *SomeFailed* or *AllFailed* status:

1. The name of your local namespace could be too long.
   Using the [DefaultName](/usage/namespace_offloading#selecting-the-namespace-mapping-strategy) policy, the remote namespace name cannot be longer than 63 characters according to the [RFC 1123](https://datatracker.ietf.org/doc/html/rfc1123).
   Since the cluster-id is 37 characters long, the home namespace name can have at most 26 characters.

2. A namespace with the same name as your remote namespace could already exist inside the target clusters.
   To check which clusters have experienced problems during the namespace replication, you can look at the [RemoteNamespaceConditions](/usage/namespace_offloading#remotenamespacesconditions) in the NamespaceOffloading.

   Liqo controllers create the remote namespaces with an annotation to distinguish them from others namespaces.
   If you are not sure about the namespace creation, you can check the presence of that annotation.
   This should have `liqo.io/remote-namespace` as key and as value, the *local cluster-id*:
   
   ```bash
   kubectl get namespace <your-remote-namespace> -o=jsonpath="{['metadata.annotations.liqo.io/remote-namespace']}" 
   ```

### The remote namespaces are correctly generated, but the pods deployed inside the local namespace remain pending or are not correctly scheduled

When dealing with pods that remain pending, you are potentially facing one or more of the following issues:

1. Make sure the NamespaceOffloading contains the desired [PodOffloadingStrategy](/usage/namespace_offloading#selecting-the-pod-offloading-strategy) value:

    ```bash
    kubectl get namespaceoffloadings offloading -n <your-namespace> -o=jsonpath="{['spec.podOffloadingStrategy']} "
    ```

2. You can check if the [cluster labels](/usage/namespace_offloading#cluster-labels-concept) exported by the virtual-nodes are the ones desired:

    ```bash
    kubectl get nodes --selector=liqo.io/type --show-labels
    ```

   If you have problems setting the cluster labels at Liqo install time, follow the [Liqo Extended tutorial](#).


3. The Liqo webhook adds NodeSelectorTerms at pod deployment to enforce the scheduling policies specified in the NamespaceOffloading.
   If you have provided the pod with additional NodeSelectorTerms, it could remain pending because the resulting NodeSelectorTerms might be antithetical.
   You can look at the [pod scheduling section](#) to better understand the NodeSelectorTerms imposed by the webhook for the different PodOffloadingStrategy.
   