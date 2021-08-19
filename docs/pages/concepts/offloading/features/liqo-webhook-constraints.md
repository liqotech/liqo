---
title: Liqo webhook constraints
weight: 4
---

As already seen in the [namespace offloading](/usage/namespace_offloading#introduction) section, pods scheduled in a *Liqo namespace* could be potentially offloaded inside remote clusters. 
The NamespaceOffloading resources contained in these namespaces provide the offloading constraints specified by the user.
When the user schedule a pod inside a Liqo namespace, the Liqo webhook gets the associated NamespaceOffloading object and processes it.
More precisely, the webhook considers just two fields of the resource spec:

1. The [PodOffloadingStategy](/usage/namespace_offloading#selecting-the-pod-offloading-strategy) field provides information about pod scheduling.
2. The [ClusterSelector](/usage/namespace_offloading#selecting-the-remote-clusters) field specifies which remote clusters are available for the offloading when the pod can be remotely scheduled.

According to these fields, the Liqo webhook forces NodeSelectorTerms inside the pod to make sure that the constraints specified are respected by the scheduler.

Virtual nodes expose a taint so that no pod can be scheduled on them without proper Toleration.
All pods that are not involved in the offloading process offered by Liqo do not have to be scheduled on virtual nodes.
The Liqo webhook applies this Toleration based on the PodOffloadingStrategy specified in the NamespaceOffloading resource.

### Different approaches for each pod offloading strategy

Considering a NamespaceOffloading with a fixed ClusterSelector, the webhook will force on the pod different NodeSelectorTerms and Toleration for the three possible PodOffloadingStategy.

A fixed ClusterSelector field could be:

```yaml
clusterSelector:
  nodeSelectorTerms:
  - matchExpressions:
    - key: liqo.io/region
      operator: In
      values:
      - us-west-1
``` 

All virtual nodes that expose the label `liqo.io/region=us-west-1` could be selected as a target to run the pod.
Considering now the three different strategies:

1. [LocalAndRemote](/usage/namespace_offloading#selecting-the-pod-offloading-strategy)

The pods could be scheduled both on virtual-nodes that expose that region label and on all local nodes.
So there is the necessity of two NodeSelectorTerms:

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

* the first NodeSelectorTerm selects the virtual nodes that expose the right region label.
* the second one selects all the local nodes.

The pods could be scheduled if they match one of these NodeSelectorTerms.
With this strategy, the approach used is to add to the NodeSelectorTerms provided by the ClusterSelector an additional NodeSelectorTerm that allows pods to be also scheduled locally.

Since pods are enabled to be scheduled even remotely, the webhook must also add on them the Toleration to the Liqo Taint.

2. [Local](/usage/namespace_offloading#selecting-the-pod-offloading-strategy)

This is the simplest case pods can be scheduled only locally as if Liqo was not present. 
Consequently, the webhook does not have to apply the virtual-node Toleration.
Since pods could not be scheduled remotely, the ClusterSelector must not be enforced by the webhook.

3. [Remote](/usage/namespace_offloading#selecting-the-pod-offloading-strategy)

Pods can be scheduled only on virtual-nodes.
In this case a single NodeSelectorTerm with two MatchExpressions is sufficient:

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

* the first MatchExpression selects the remote cluster with that region label.
* the second makes sure that a local node is not selected, in the unusual case that a local node exposes the same labels of the virtual-nodes.

With this strategy, the approach used is to add to every NodeSelectorTerms provided by the ClusterSelector an additional MatchExpression which prevents the pod from being scheduled on local nodes.

Since pods must be scheduled remotely, the webhook must also add the Toleration to the Liqo Taint.

#### Possible conflicts

If the user deploys pods already with some NodeSelectorTerms, these must be compliant with the ones enforced by the webhook.
In case of conflicts, pods will remain pending and unscheduled, as seen in the violation constraints example of the [Liqo Extended tutorial](#).
