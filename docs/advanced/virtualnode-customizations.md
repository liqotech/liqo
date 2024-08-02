# VirtualNode customizations

Since Liqo 1.0 it is possible to granularly customize each *virtualnodes* (and their associated *virtual-kubelets*), thanks to the VirtualNode CR.
This page will describe how to tune the VirtualNode resource according to the user's needs or use cases.

(OffloadingPatch)=

## Offloading Patch

The VirtualNode CR allows to configure a set of patches to apply to all offloaded pods or reflected resources by its *virtual-kubelet*.
We call this feature `OffloadingPatch` and it is configurable directly on the VirtualNode spec.

In particular, it is possible to specify custom *NodeSelector*, *Tolerations*, and *Affinity* that will applied to remote offloaded pods.
For example, with node selectors, you can target specific pools of nodes on the provider cluster.
Another feature is the possibility to disable the reflection of specific labels and annotations on all reflected resources, as described [here](UsageReflectionLabelsAnnots).

```{admonition} Note
If any field of the offloading patch changes, the *virtual-kubelet* deployment is restarted.
```

(VkOptionsTemplate)=

## Virtual-kubelet options

To allow maximum flexibility, it is possible to configure the option to pass to the *virtual-kubelet*, for each *VirtualNode* CR.
By default it is created in the namespace `liqo` a default `VkOptionsTemplate` CR, containing the default settings for the virtualnodes (some fields are templated from Helm values).
When a user creates a new VirtualNode it can customize the *virtual-kubelet* options by referencing a custom template in the `.spec.vkOptionsTemplateRef` field.
If not specified, the default template installed by Liqo is used.

The `VkOptionsTemplate` contains a lot of possible customizations.
Some of the most important allows to:

* customize the *virtual-kubelet* pod image
* customize [reflectors policies](UsageReflectionPolicies)
* turn off [reflectors](UsageReflectionPolicies) for specific resources
* customize some default fields to use in the VirtualNode *OffloadingPatch* (e.g., [disabling the reflection of specific labels and annotations](UsageReflectionLabelsAnnots))
* add extra args, labels, and annotations to the *virtual-kubelet* deployment
* add extra labels and annotations to the virtual k8s node
* turn off metrics or change its address

## Disable creation of the k8s Liqo node

When you create a VirtualNode CR, automatically an associated K8s node to schedule workloads is created.
In some use cases, the user may want to only use the [**resource reflection**](/usage/reflection) to propagate resources (e.g., *Services*, *ConfigMaps*, *Secrets*, etc..) on remote clusters, while not needing a node to schedule workloads.
Even if the k8s node is absent, the *virtual-kubelet* pod is able to reflect on the remote cluster the above-mentioned resource, as they are not tied to a local k8s node. In this case, it may be convenient to completely disable the creation of the K8s node.

This feature can be set by customizing the `createNode` field in the `VirtualNode` CR or by referencing a `VkOptionsTemplate` with the `createNode` modified.

## Disable network condition check

By default, Liqo checks periodically if the networking between the consumer and provider is established correctly.
When there are network issues, the *virtual-kubelet* detects this problem and will update the `NetworkUnavailable` [node conditions](https://kubernetes.io/docs/reference/node/node-status/#condition).
This event makes the k8s scheduler prevent scheduling pods on this node.
If you **do not need this check** you can completely disable the update of this condition, and the condition will be always **ready**, allowing the user to schedule pods independently from the state of the network.

This feature can be set by customizing the `disableNetworkCheck` field in the `VirtualNode` CR or by referencing a `VkOptionsTemplate` with the `disableNetworkCheck` modified.

```{admonition} Note
In case the network module is not enabled for this pair of clusters (or disabled globally), Liqo will always set the network condition to **ready** (or equivalently not putting it at all). As a consequence pods can always be scheduled on the virtual node indipendently from the underlying network technology or connectivity.
```
