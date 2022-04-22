---
title: Resource Offloading
weight: 4
---

## Overview

The peering process terminates with the creation of a new *virtual node* in the local cluster, which represents (and aggregates) the subset of resources made available by the remote one.
This solution enables the transparent extension of the local cluster, with the new Node (and its capabilities) seamlessly taken into account by the vanilla Kubernetes scheduler when selecting the best place for the workloads execution.
At the same time, this approach is fully compliant with standard Kubernetes APIs, hence allowing to interact with and inspect offloaded Pods just as if they were executed locally.

### Liqo Virtual Kubelet

In Kubernetes, each physical node is managed by the [kubelet](https://kubernetes.io/docs/reference/command-line-tools-reference/kubelet/), a process running in the hosting machine that interfaces with the Kubernetes API server and handles the lifecycle of both the Node itself and the Pods therein hosted.
Similarly, the virtual node abstraction is implemented through an extended version of the [Virtual Kubelet project](https://github.com/virtual-kubelet/virtual-kubelet#liqo-provider).
The virtual kubelet replaces a traditional kubelet when the controlled entity is not a physical node, allowing to control arbitrary objects through standard Kubernetes APIs.

In the context of Liqo, the virtual kubelet interacts with both the local and the remote clusters, and it is in charge of three main tasks:

1. Creating the (virtual) node resource and reconciling its status with respect to the negotiated configuration, as described in the [node lifecycle](/concepts/offloading/node-lifecycle) section.
2. Offloading the local Pods scheduled on the corresponding (virtual) node to the remote cluster, while aligning their status, as detailed in the [pod offloading](/concepts/offloading/pod-offloading) section.
3. Propagating and synchronizing the accessory artifacts required for proper execution of the offloaded workloads, a feature we call [resource reflection](/concepts/offloading/resource-reflection).

A different instance of the Liqo virtual kubelet is started (in the local cluster) for each remote cluster, ensuring isolation and segregating the different authentication tokens.

### Namespace mapping

The virtual kubelet has to face the namespace replication problem to make the Pods in a certain namespace suitable to be offloaded.
Further details can be found in the dedicated [section](/concepts/offloading/namespace-replication/#the-liqo-namespace-model).

### Scheduling behavior

The virtual node is created with a specific *taint*, preventing arbitrary Pods from being offloaded to remote clusters.
Only the Pods including the appropriate *toleration* are allowed to be scheduled on a virtual node.
The toleration is automatically added by the [Liqo Mutating Webhook](/concepts/offloading/mutating-webhook), based on whether offloading is enabled for the hosting namespace (see the [namespace offloading](/usage/namespace_offloading#introduction) section for additional information).
At this point, the Kubernetes scheduler selects the eligible node with the highest score (scores are computed on several parameters, among which the available resources), optionally filtered depending on additional constraints (e.g., *affinity* configurations).
