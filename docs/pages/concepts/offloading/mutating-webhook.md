---
title: Mutating webhook
weight: 5
---

This section explains how the Liqo Webhook mutates Pods created within a Liqo-enabled namespace, enabling them to be possibly offloaded to remote clusters and enforcing user-specified constraints.

As detailed in the [namespace offloading](/usage/namespace_offloading) section, the creation of a *NamespaceOffloading* resource inside a given namespace extends that namespace to remote clusters (possibly remapped, to prevent naming conflicts).
Additionally, each *NamespaceOffloading* resource states the offloading constraints specified by the user, and leveraged by the webhook to appropriately mutate the Pods in that namespace (i.e., adding the *toleration* to the virtual nodes *taint*, and modifying the *node affinities* field).
In particular:

1. The [PodOffloadingStrategy](/usage/namespace_offloading#selecting-the-pod-offloading-strategy) field, which determines at a high-level whether Pod offloading is enabled.
2. The [ClusterSelector](/usage/namespace_offloading#selecting-the-remote-clusters) field, which optionally selects a subset of virtual clusters for Pod offloading.

### Pod mutations

This section shows in detail the webhook behavior with each of the three possible `PodOffloadingStrategy` values, while considering a fixed `ClusterSelector` that targets all virtual nodes with label `liqo.io/region=us-west-1`:

```yaml
clusterSelector:
  nodeSelectorTerms:
  - matchExpressions:
    - key: liqo.io/region
      operator: In
      values:
      - us-west-1
```

#### LocalAndRemote PodOffloadingStrategy

The `LocalAndRemote` strategy allows Pods to be scheduled both on local nodes and a subnet of the virtual nodes.
Hence, the webhook adds the *toleration* to the virtual nodes *taint*, as well as mutates the pod affinity introducing two (ORed) `NodeSelectorTerms`:

* The first `NodeSelectorTerm` enforces the constraints specified through the `ClusterSelector`, and applies to virtual nodes.
* The second `NodeSelectorTerm` enables Pod scheduling on all physical nodes, as they might not match the `ClusterSelector`.

Hence, the resulting `NodeAffinity` constraint applied to a Pod would resemble the following:

```yaml
nodeAffinity:
  requiredDuringSchedulingIgnoredDuringExecution:
    nodeSelectorTerms:
      - matchExpressions:
        - key: liqo.io/region
          operator: In
          values:
          - us-west-1
      - matchExpressions:
        - key: liqo.io/type
          operator: NotIn
          values:
          - virtual-node
```

In case a given Pod already contains a set of `NodeAffinity` constraints, they are merged appropriately by the Liqo webhook with the ones above, ensuring that only the Nodes respecting the intersection of the restrictions can be selected as valid scheduling targets.

#### Local PodOffloadingStrategy

The `Local` strategy does not allow Pods to be scheduled on virtual nodes, hence behaving from this point of view as if Liqo was not present (although other resources, such as services, might be reflected to enable remote consumption).
Consequently, in this case the webhook is not activated, and Pods are neither appended the *toleration* nor any additional *NodeAffinity* constraints.

#### Remote PodOffloadingStrategy

The `Remote` strategy forces Pods to be scheduled on virtual nodes only.
Hence, the webhook adds the *toleration* to the virtual nodes *taint*, as well as mutates the pod affinity introducing a single `NodeSelectorTerm` with two (ANDed) `MatchExpressions`:

* The first `MatchExpression` term enforces the constraints specified through the `ClusterSelector`.
* The second `MatchExpression` ensures Pods are scheduled on virtual nodes only (i.e., with the `liqo.io/type=virtual-node` label).

The resulting `NodeAffinity` constraint applied to a Pod resembles the following (in case it already contained a set of constraints, they would be merged accordingly):

```yaml
nodeAffinity:
  requiredDuringSchedulingIgnoredDuringExecution:
    nodeSelectorTerms:
      - matchExpressions:
        - key: liqo.io/region
          operator: In
          values:
          - us-west-1
        - key: liqo.io/type
          operator: In
          values:
          - virtual-node
```

### Constraints violation

The `NamespaceOffloading` resource specifies a set of constraints that applies to all pods of that namespace.
Additionally, users can define additional and more specific requirements at the Pod level (i.e., through the `NodeAffinity` configuration), automatically merged by the Liqo webhook with the ones originating from the `NamespaceOffloading` resource.
However, be careful in preventing conflicting requirements, which would cause the Pod to remain in `Pending` status as unschedulable, as seen in the violation constraints example of the [Liqo Extended tutorial](/gettingstarted/extended/hard_constraints/).
