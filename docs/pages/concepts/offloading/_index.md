---
title: Resource Offloading
weight: 4
---

## Overview

The peering process terminates by creating a new virtual node in the home cluster.
According to the Liqo terminology, the new virtual node is called *big node*, as it represents (and aggregates) a subset of the resources available in the foreign cluster. 
Conversely, the home cluster becomes what we call a *big cluster*, as it represents a cluster whose resources span (transparently) across multiple physical clusters.

In Kubernetes, each physical node is managed by the [kubelet](https://kubernetes.io/docs/reference/command-line-tools-reference/kubelet/), a process running in the hosting machine that interfaces with the Kubernetes API server and handles the lifecycle of the pods scheduled on it.

Since the foreign cluster is represented by a virtual node in the home cluster, offloading a pod on the remote cluster corresponds to scheduling it on a specific node.
When the [virtual kubelet](#virtual-kubelet) creates a new pod in the foreign cluster, the foreign scheduler elects one *remote, physical* node as host for the received pod, while the *remote, physical* kubelet managing that pod takes care of the containers' execution.

According to this mechanism, the offloading of a pod to the foreign cluster is fully compliant with the Kubernetes control plane: the home cluster can control all the remote jobs by interacting with the big node that represents the remote cluster. 

### Virtual Kubelet

The *big node* is a virtual node, so it cannot have a real kubelet process such as normal physical nodes.
Liqo leverages a custom version of the [virtual kubelet project](https://github.com/virtual-kubelet/virtual-kubelet) for the management of the virtual node.
In a nutshell, a cluster that peers with `N` foreign clusters has `N` big nodes representing the clusters and runs `N` instances of the virtual kubelet process.

Generally speaking, a standard kubelet is in charge of accomplishing two tasks:

1. Handling the node resource and reconciling its status.
2. Taking the received pods, starting the containers, and reconciling their status in the pod resource.

Similarly, the virtual kubelet is in charge of:

1. Creating the virtual node resource and reconciling its status, as described in the [node-management](/concepts/offloading/node-management/#overview) section.
2. Offloading the local pod scheduled on the virtual node to the remote cluster, as described in the [computing](/concepts/offloading/computing#overview) section.

Also, our implementation provides a feature we called "reflection", described [here](/concepts/offloading/api-reflection/#overview).

### Namespace mapping

The virtual kubelet has to face the namespace replication problem to make the pods in a certain namespace suitable to be offloaded.
Further details can be found in the dedicated [section](/concepts/offloading/namespace-replication/#the-liqo-namespace-model).

### Scheduling behavior

The virtual node is created with a specific taint. To make a pod available to be scheduled on a virtual node that taint must be tolerated. 
The toleration is added by the Liqo MutatingWebhook watching the pods created in all the namespaces labeled with the label `liqo.io/enabled="true"`.
By default, the Kubernetes scheduler selects the eligible node with the highest score (scores are computed on several parameters, among which the available resources).
