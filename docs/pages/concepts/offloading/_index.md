---
title: Offloading
weight: 4
---

## Overview

The peering process terminates by creating a new virtual node in the home cluster.
According to the Liqo terminology, the new virtual node is called _big node_, as it represents (and aggregates) a subset
of the resources available in the foreign cluster. Conversely, the home cluster becomes what we call a _big cluster_, 
as it represents a cluster whose resources span (transparently) across multiple physical clusters.

In Kubernetes, each physical node is managed by the 
[Kubelet](https://kubernetes.io/docs/reference/command-line-tools-reference/kubelet/), a process running in the hosting 
machine that interfaces with the Kubernetes API server, and handles the lifecycle of the pods scheduled on it.

Since the foreign cluster is represented by a local (virtual) node in the home cluster, offloading a pod on the remote 
cluster correspond to scheduling it on a specific node.
When the virtual Kubelet creates a new pod in the foreign cluster, the foreign scheduler elects one _remote, physical_ 
node as host for the received pod, while the _remote, physical_ kubelet managing that pod takes care of the containers' 
execution.

According to this mechanism, the offloading of a pod to the foreign cluster is fully compliant with the Kubernetes 
control plane: the home cluster can control all the remote jobs by interacting with the big node that models the remote 
cluster. 

### Virtual Kubelet

The _big node_ is a virtual node, hence it cannot have a real `kubelet` process such as normal (physical) nodes.
Liqo leverages a custom version of the [Virtual kubelet](https://github.com/virtual-kubelet/virtual-kubelet) project for
the management of the virtual node, with the kubelet managing the virtual node that runs as a (containerized) pod as a 
part of the Liqo control plane.
In a nutshell, a cluster that peers with `N` foreign clusters will have `N` big nodes, representing the `N` remote 
clusters, and it will run `N` instances of the virtual kubelet, each one dedicated to the mapping between the (local) 
big node and the corresponding (remote) foreign cluster.

Generally speaking, a real Kubelet is in charge of accomplishing two tasks:
* handling the node resources and reconciling its status
* taking the received pods, starting the containers, and reconciling their status in the pod resource.

Similarly, the virtual kubelet is in charge of:
* creating the virtual node resource and reconciling its status, as described in the
 [node-management](features/node-management) section;
* offloading the local pod scheduled on the virtual node to the remote cluster, as described in the 
[computing](features/computing) section.

Also, our implementation provides a feature we called "reflection", described [here](features/api-reflection).

### Namespace mapping

To make the pods in a certain namespace suitable to be offloaded in the remote cluster, the virtual Kubelet has to face 
with the problem of the offloading namespace, i.e., in which namespace of the remote cluster to create the pods.
Further details can be found in the dedicated [section](features/namespace-management).

### Scheduling behavior

The virtual node is created with a specific taint. To make a pod available to be scheduled on that node, that taint must
be tolerated. The toleration is added by a `MutatingWebhook` watching all the pods being created in all the namespaces 
labeled with the label `liqo.io/enabled="true"`.

By default, the Kubernetes scheduler selects the eligible node with the highest score (scores are computed on several 
parameters, among which the available resources).
