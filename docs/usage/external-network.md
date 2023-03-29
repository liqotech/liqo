# External Network

Since Liqo v0.8.0, it is possible to enable [**resource reflection**](/usage/reflection) and [**namespace offloading**](/usage/namespace-offloading) without the need to establish network connectivity between the clusters.
This feature is called **External Network**. In this section, we will see how to enable/disable this feature and how to use it.

## Overview

The *External Network* feature allows resource reflection and namespaces offloading without requiring network connectivity between clusters.

This feature is useful in scenarios where the offloaded application **does not need to perform cross-cluster communication** but only needs to access local resources.
For example, a batch processing application does not need to communicate with other clusters, but it needs to access a database external to the cluster.
In this case, a network interconnection between clusters is not required and may lead to unnecessary **security issues** and **interdependencies** between clusters.

Another use case is when the clusters and the pods running on them are **already connected to the same network**.
In this case, you may leverage the *External Network* feature to enable resource reflection and namespaces offloading without having to establish another network connection between the clusters.

## Enable/Disable the External Network

This feature is **disabled** by default, and can be configured with **two** different **feature flags** at install time (see the [reference](/installation/install.md)):

* `--set networking.internal=false` to disable the internal network
* `--set networking.reflectIPs=false` to disable the reflection of the IP addresses

### networking.internal=false

This flag disables the internal network.
When this flag is set to `false`, the Liqo Gateway and the Liqo Route are not deployed on the cluster and the Liqo Network Manager is not started.
The Liqo Network Manager is responsible for creating the `tunnel-endpoint` resource, which is used to establish the network connectivity between the clusters.

When the internal network is disabled, the Liqo Network Fabric is not enabled and **no parameter negotiation or IP remapping is performed**.
The IP addresses of the remote pods are reflected as they are.

```{admonition} Note
The pod IPs are still reflected in the remote clusters, but they are not remapped.
This means that the shadow pods will see the same IP in each cluster.
It similarly happens with <a href="https://kubernetes.io/docs/concepts/services-networking/" target="_blank">EndpointSlices</a> resources.
If you have an external network tool that handles the connection, you will be able to connect to the remote pods.
```

### networking.reflectIPs=false

This flag disables the reflection of the IP addresses.
When this flag is set to `false`, the **IP addresses** of the remote pods **are not reflected** and both local and remote <a href="https://kubernetes.io/docs/concepts/services-networking/" target="_blank">EndpointSlices</a> resources are not populated.

All shadow pods will have an **empty IP address**, and will not be selected as targets by any Kubernetes service.
