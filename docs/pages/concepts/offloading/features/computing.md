---
title: Computing
weight: 3
---

### Overview

The offloading of a pod in a foreign cluster involves a two-steps scheduling process, which can be summarized as follows:
- the scheduler of the home cluster selects the _big node_ as the best target to run the pod;
- the big node, being virtual, cannot run any pod. Hence, the virtual kubelet handling the big node involves the 
scheduler of the foreign cluster, which will select the best (physical) node on the foreign cluster to run the pod.

In addition, the proper reconciliation processes are executed by the Virtual Kubelet in order to allow the pods in the
home cluster to be up-to-date with the local representation of the remote ones, to ensure complete visibility.

### Resiliency of the remote pod 

Due to the properties of the two-step scheduling process, different behaviours can be spotted when deleting a pod 
running in a foreign cluster:
- The pod is deleted in the **home** cluster (intentionally or by eviction): the remote one has to be deleted 
immediately. In fact, the home cluster is the owner of Pod, therefore whatever modification to the pod is set, 
this has to be reflected on the remote pod. Hence, when the pod is deleted in the home cluster, the status of the 
remote cluster has to be aligned, leading to the deletion of the remote Pod.
- The pod is deleted in the **foreign** cluster (intentionally or by eviction): not only the local pod must not be 
deleted, but the remote pod has to be re-created as soon as possible. The requested pod must be running in the foreign 
cluster, without involving the home cluster in any un-necessary detail, unless an unrecoverable condition is detected.

To implement this behavior, a new pod scheduled in the virtual node, triggers the Virtual Kubelet to create a new 
[ReplicaSet](https://kubernetes.io/docs/concepts/workloads/controllers/replicaset/), that ultimately leads to the 
creation of the Pod (through the foreign controller-manager).

Creating a remote ReplicaSet addresses the problem of the resiliency of the remote pod: once a ReplicaSet is created in 
the remote cluster, the remote ReplicaSet-controller reconciles its status, leading to the desired number of running 
pods at any time (the desired amount of replicas for those ReplicaSet is always one).

### Computing resources offloading and reconciliation

The scheme below describes the offloading workflow.
The local pod is referred to as _shadow pod_ (because it is a mere local representation of the remote pod).

![](/images/offloading/computing-offloading-overview.svg)

1. A user creates a deployment in the local cluster.
2. The controller-manager detects the new deployment, then
    1. Creates the corresponding ReplicaSet;
    2. Detects the ReplicaSet creation;
    3. Creates the specified amount of Pod replicas.
3. The scheduler detects the creation of the new Pod.
4. The scheduler binds some Pods to the virtual node.
5. The Virtual Kubelet detects that a Pod has been scheduled on the virtual node managed by this process.
7. The Virtual Kubelet creates a remote ReplicaSet having the local Pod as `PodTemplate` field and _one_ replica.
8. The remote controller-manager detects the ReplicaSet.
9. The remote controller-manager creates one Pod starting from the `PodTemplate` field.
10. The Virtual Kubelet detects the creation of the remote offloaded Pod.
11. The Virtual Kubelet keeps the local Pod status updated with the remote one.
