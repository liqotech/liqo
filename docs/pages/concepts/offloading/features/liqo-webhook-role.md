---
title: Liqo webhook role
weight: 4
---

This section explains how the Liqo Webhook mutates pods created within a Liqo enabled namespace. 
As already seen in the [namespace offloading](/usage/namespace_offloading#introduction) section, pods created within a *Liqo namespace* could be potentially scheduled remotely, so the webhook must guarantee their possible offloading.

The NamespaceOffloading resource contained in a Liqo enabled namespace provides the offloading constraints specified by the user, so when they schedule pods within this namespace, the webhook gets the associated NamespaceOffloading object and processes it.
More precisely, it considers two fields of the resource spec:

1. The [PodOffloadingStategy](/usage/namespace_offloading#selecting-the-pod-offloading-strategy) field provides information about pod scheduling.
2. The [ClusterSelector](/usage/namespace_offloading#selecting-the-remote-clusters) field specifies which remote clusters are available for the offloading process.

Therefore, according to the PodOffloadingStrategy field, the webhook can force inside pods:

1. The ClusterSelector to impose user constraints.
2. The Toleration to the virtual node Taint allowing the remote offloading.

The next section shows in detail the webhook behavior with each of the three possible strategies.

### Different pod mutations 

Starting with a fixed ClusterSelector, which selects all virtual nodes with region `us-west-1`, we can analyze pod mutations according to the three different pod offloading strategies.

```yaml
clusterSelector:
  nodeSelectorTerms:
  - matchExpressions:
    - key: liqo.io/region
      operator: In
      values:
      - us-west-1
``` 

#### 1. LocalAndRemote

This strategy allows pods to be scheduled both on selected virtual nodes and on all local nodes, so there is the necessity of at least two NodeSelectorTerms:

* The first NodeSelectorTerm is the one specified in the NamespaceOffloading ClusterSelector.
* The second one selects all local nodes.

```yaml
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

More generally, the webhook provides an additional NodeSelectorTerm to the ClusterSelector, allowing pods to be also scheduled locally. Therefore, the webhook mutates pods by adding these NodeSelectorTerms and the Toleration to the Liqo Taint since pods are enabled to be scheduled remotely. 

#### 2. Local

This is the simplest case where pods can be scheduled only locally as if Liqo was not present. 
Consequently, the webhook does not apply either the virtual node Toleration or the ClusterSelector.

#### 3. Remote

This strategy allows pods to be scheduled only on selected virtual nodes, so a single NodeSelectorTerm with two MatchExpressions is sufficient:

* The first MatchExpression is the one specified in the NamespaceOffloading ClusterSelector.
* The second makes sure that a local node is not selected, in the unusual case that a local node exposes the same virtual nodes labels.

```yaml
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
With this strategy, the webhook provides an additional MatchExpression to every NodeSelectorTerm in the ClusterSelector field, preventing pods from being scheduled on local nodes. Therefore, the webhook mutates pods by adding these NodeSelectorTerms and the Toleration to the Liqo Taint since pods are enabled to be scheduled only remotely.

### Constraints violation

If users deploy pods already with some NodeSelectorTerms, these must be compliant with the ones enforced by the webhook.
In case of conflicts, pods will remain pending and unscheduled, as seen in the violation constraints example of the [Liqo Extended tutorial](/gettingstarted/extended/hard_constraints/).
