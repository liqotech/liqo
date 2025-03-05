# Use only offloading

Thanks to Liqo modularity, it is possible to enable [**resource reflection**](/usage/reflection) and [**namespace offloading**](/usage/namespace-offloading) without the need to use the default Liqo networking module to establish pod-to-pod network connectivity between the clusters.
In this section, we will see how to enable/disable this module and how to use this feature.

## Overview

Turning off the networking module feature allows resource reflection and namespaces' offloading without requiring network connectivity between clusters.

This feature is useful in scenarios where the offloaded application **does not need to perform cross-cluster communication** but only needs to access local resources.
For example, a batch processing application does not need to communicate with other clusters, but it needs to access a database external to the cluster.
In this case, a network interconnection between clusters is not required and may lead to unnecessary **security issues** and **interdependencies** between clusters.

(AdvancedUseOnlyOffloadingDisableModule)=

## Disable the Networking module

It is possible to disable the networking module at Liqo installation time. In that case, the networking will not be available for all peerings since the Liqo controllers and DaemonSets responsible for that module will not be deployed.

The networking module is **enabled** by default and can be disabled or configured with **two** different **feature flags** at install time (see the [reference](/installation/install.md)):

* `--set networking.enabled=false` to disable the networking module
* `--set networking.reflectIPs=false` to disable the reflection of the IP addresses. (i.e., you will find Pods with `None` IP addresses, and no EndpointSlices resources will be populated)

### networking.enabled=false

This flag disables the internal network.
When this flag is set to `false`, the Liqo network controllers are not deployed on the cluster.
The Liqo Network Manager is responsible for creating the `gatewayserver` and `gatewayclient` resources, which are used to establish the network connectivity between the clusters.

When the networking is disabled, the Liqo Network Fabric is not enabled and **no parameter negotiation or IP remapping is performed**.
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

(AdvancedUseOnlyOffloadingSpecificCluster)=

## Disable Networking for a specific Cluster

It is also possible to disable the networking module only for the peering with a specific cluster.
This can be done by adding the `--disable-networking` flag to the `liqoctl peer` command when establishing a peering.

```bash
liqoctl --kubeconfig=$CONSUMER_KUBECONFIG_PATH peer \
    --remote-kubeconfig $PROVIDER_KUBECONFIG_PATH \
    --disable-networking
```

The `--disable-networking` flag will prevent Liqo from configuring the networking between the clusters for that specific peering. While the resource reflection and offloading is still possible.

Alternatively, if you are using the [Advanced Peering](/advanced/manual-peering) method, you can disable the networking module simply by skipping the Inter-cluster Network Connectivity step.

In that scenario, the Pod IPs will still be reflected in the remote cluster, but they will not be remapped. You can still connect to the remote pods using an external network tool.

In order to disable the IP reflection, you can add the `--disable-ip-reflection` argument to the respective VirtualNode spec by adding it to the `virtualKubelet.extra.args` field in the `values.yaml` file at installation time or by adding a custom `VkOptionsTemplate` resource to your consumer cluster.

Example of the `values.yaml` file:

```yaml
virtualKubelet:
  extra:
    args:
      - --disable-ip-reflection
```

Example of peer command with `--disable-networking` flag:

```bash
liqoctl --kubeconfig=$CONSUMER_KUBECONFIG_PATH peer \
    --remote-kubeconfig $PROVIDER_KUBECONFIG_PATH \
    --disable-networking
```
