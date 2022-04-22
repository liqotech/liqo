---
title: Node lifecycle
weight: 1
---


The first task handled by the virtual kubelet regards the creation and the management of the *virtual node* abstracting the resources shared by the remote cluster.
Specifically, it aligns the Node status with respect to the negotiated configuration, according to the content of two main Liqo CRs:

* `ResourceOffer`, which states the amount of resources (e.g., CPU, memory, ...) made available by the remote cluster, automatically reflected into the *capacity* and *allocatable* entries of the Node status. In addition, it lists a set of characterizing labels suggested by the remote cluster, propagated by the virtual kubelet to the corresponding Node to enable fine-grained scheduling policies (e.g., through *affinity* constrains).
* `TunnelEndpoint`, which summarizes the status of the network interconnection between the two clusters. In particular, this information is reflected in the `NetworkUnavailable` Node condition.

In addition, the virtual kubelet performs periodic healthiness checks (configurable through the corresponding command line flags) to assess the reachability of the remote API server.
This allows to mark the Node as *not ready* in case of repeated failures, triggering the standard Kubernetes eviction policies based on the configured Pod tolerations.
