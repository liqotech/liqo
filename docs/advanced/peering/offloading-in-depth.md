# Offloading in depth

## Overview

```{warning}
The aim of the following section is to give an idea of how Liqo works under the hood and to provide the instruments to address the more complex cases.
For the major part of the cases the `liqoctl peer` is enough, so [stick with it](/usage/peer) if you do not have any specific needs.
```

This document will go over the process of acquiring resources and making them available as a `Node` in the consumer cluster.

You can add a `VirtualNode` to a consumer cluster in two different ways:

1. By creating a `ResourceSlice` in the tenant namespace in the consumer cluster.
2. By creating a `VirtualNode` in the consumer cluster.

Note that the `ResourceSlice` method is the **preferred way** to add a `VirtualNode` to a consumer cluster, but it requires the *Authentication module* to be enabled (that is enabled by default when using `liqoctl peer`, `liqoctl authenticate` or the [manual configuration](/advanced/peering/inter-cluster-authentication)).
In the following steps, when we are using `ResourceSlices`, we will assume that the *Authentication module* is enabled and the authentication between the clusters is established, either with [`liqoctl peer`](/usage/peer) or with the [manual configuration](/advanced/peering/inter-cluster-authentication).

## Create ResourceSlice

This is the preferred way to add a `VirtualNode` to a consumer cluster.

`````{tab-set}

````{tab-item} liqoctl
```{code-block} bash
:caption: "Cluster consumer"
liqoctl create resourceslice mypool --remote-cluster-id cool-firefly
```
````

````{tab-item} YAML
```yaml
apiVersion: authentication.liqo.io/v1beta1
kind: ResourceSlice
metadata:
  annotations:
    liqo.io/create-virtual-node: "true"
  creationTimestamp: null
  labels:
    liqo.io/remote-cluster-id: cool-firefly
    liqo.io/remoteID: cool-firefly
    liqo.io/replication: "true"
  name: mypool
  namespace: liqo-tenant-cool-firefly
spec:
  class: default
  providerClusterID: cool-firefly
status: {}
```
````

`````

This command will create a `ResourceSlice` named `mypool` in the consumer cluster, and it will be associated with the `cool-firefly` cluster.

If no resource is specified, the provider cluster will fill them with the default values.
You can specify the resources you want to acquire by adding:

* `--cpu` to specify the amount of CPU.
* `--memory` to specify the amount of memory.
* `--pods` to specify the number of pods.

To add other resources like `ephemeral-storage`, `gpu` or any other custom resources, you can use the `-o yaml` flag for the `liqoctl create resourceslice` command and edit the `ResourceSlice` spec manifest before applying it.

```{code-block} bash
:caption: "Cluster consumer"
kubectl get resourceslices.authentication.liqo.io -A
```

```text
NAMESPACE                  NAME     AUTHENTICATION   RESOURCES   AGE
liqo-tenant-cool-firefly   mypool   Accepted         Accepted    19s
```

At the same time, in the **provider cluster**, a `Quota` will be created to limit the resources that can be used by the consumer cluster.

```{code-block} bash
:caption: "Cluster provider"
kubectl get quotas.offloading.liqo.io -A
```

```text
NAMESPACE                   NAME                  ENFORCEMENT   CORDONED   AGE
liqo-tenant-wispy-firefly   mypool-c34af51dd912   None                     36s
```

After a few seconds, in the **consumer cluster**, a new `VirtualNode` will be created automatically.

```{code-block} bash
:caption: "Cluster consumer"
kubectl get virtualnodes.offloading.liqo.io -A
```

```text
NAMESPACE                  NAME     CLUSTERID      CREATE NODE   AGE
liqo-tenant-cool-firefly   mypool   cool-firefly   true          59s
```

A new `Node` will be available in the consumer cluster with the name `mypool` providing the resources specified in the `ResourceSlice`.

```{code-block} bash
:caption: "Cluster consumer"
kubectl get node
```

```text
NAME                            STATUS   ROLES           AGE   VERSION
cluster-1-control-plane-fsvkj   Ready    control-plane   30m   v1.27.4
cluster-1-md-0-dzl4s            Ready    <none>          29m   v1.27.4
mypool                          Ready    agent           67s   v1.27.4
```

### Custom Resource Allocation

To customize the behavior of resource sharing, you can specify a custom Resource Slice class. This can be done either in the YAML specification or by using the `--class` flag with `liqoctl`:

`````{tab-set}

````{tab-item} liqoctl
```bash
liqoctl create resourceslice mypool --remote-cluster-id cool-firefly --class custom-class
```
````

````{tab-item} YAML
```yaml
apiVersion: authentication.liqo.io/v1beta1
kind: ResourceSlice
metadata:
  name: mypool
  namespace: liqo-tenant-cool-firefly
spec:
  class: custom-class
  providerClusterID: cool-firefly
```
````

`````

The provider cluster can handle this custom class with a dedicated controller.
Liqo provides a template repository for implementing such controllers.

When using a custom class:

1. Specify the class in the `ResourceSlice` spec.
2. The provider's custom controller should fill the resources in the status and set the condition regarding resources.
3. The `VirtualNode` and `Quota` will be created according to the resources specified by the custom controller.

This approach allows for more flexible and dynamic resource allocation based on specific policies or requirements defined by the provider cluster.

For more information on implementing a custom Resource Slice controller, refer to the [Liqo Resource Slice Controller template repository](https://github.com/liqotech/resource-slice-class-controller-template).

### Delete ResourceSlice

You can revert the process by deleting the `ResourceSlice` in the consumer cluster.

```{code-block} bash
:caption: "Cluster consumer"
kubectl delete resourceslice mypool -n liqo-tenant-cool-firefly
```

## Create VirtualNode

Alternatively, you can create a `VirtualNode` directly in the consumer cluster.

### With Existing ResourceSlice

If you have already created a `ResourceSlice` in the consumer cluster, you can create a `VirtualNode` that will use the resources specified in the `ResourceSlice`.

`````{tab-set}

````{tab-item} liqoctl
```{code-block} bash
:caption: "Cluster consumer"
liqoctl create virtualnode --remote-cluster-id cool-firefly --resource-slice-name mypool mynode
```
````

````{tab-item} YAML
```yaml
apiVersion: offloading.liqo.io/v1beta1
kind: VirtualNode
metadata:
  creationTimestamp: null
  labels:
    liqo.io/remote-cluster-id: cool-firefly
  name: mynode
  namespace: liqo-tenant-cool-firefly
spec:
  clusterID: cool-firefly
  createNode: true
  disableNetworkCheck: false
  kubeconfigSecretRef:
    name: kubeconfig-resourceslice-mypool
  labels:
    liqo.io/provider: kubeadm
    liqo.io/remote-cluster-id: cool-firefly
  resourceQuota:
    hard:
      cpu: "4"
      ephemeral-storage: 20Gi
      memory: 8Gi
      pods: "110"
  storageClasses:
  - storageClassName: liqo
status: {}
```
````

`````

This command will create a `VirtualNode` named `mynode` in the consumer cluster, and it will be associated with the `cool-firefly` cluster.

```{code-block} bash
:caption: "Cluster consumer"
kubectl get virtualnodes.offloading.liqo.io -A
```

```text
NAMESPACE                  NAME     CLUSTERID      CREATE NODE   AGE
liqo-tenant-cool-firefly   mynode   cool-firefly   true          7s
```

A new `Node` will be available in the consumer cluster with the name `mynode` providing the resources specified in the `ResourceSlice`.

```{code-block} bash
:caption: "Cluster consumer"
kubectl get node
```

```text
NAME                            STATUS   ROLES           AGE   VERSION
cluster-1-control-plane-fsvkj   Ready    control-plane   52m   v1.27.4
cluster-1-md-0-dzl4s            Ready    <none>          52m   v1.27.4
mynode                          Ready    agent           22s   v1.27.4
```

```{warning}
If you create multiple `VirtualNodes` using the same `ResourceSlice`, the resources will be shared among them.
```

### With Existing Secret

If you have a `kubeconfig` secret in the consumer cluster, you can create a `VirtualNode` that will use the resources specified in the `kubeconfig` secret.

`````{tab-set}

````{tab-item} liqoctl
```{code-block} bash
:caption: "Cluster consumer"
liqoctl create virtualnode --remote-cluster-id cool-firefly --kubeconfig-secret-name kubeconfig-resourceslice-mypool mynode
```
````

````{tab-item} YAML
```yaml
apiVersion: offloading.liqo.io/v1beta1
kind: VirtualNode
metadata:
  creationTimestamp: null
  labels:
    liqo.io/remote-cluster-id: cool-firefly
  name: mynode
  namespace: liqo-tenant-cool-firefly
spec:
  clusterID: cool-firefly
  createNode: true
  disableNetworkCheck: false
  kubeconfigSecretRef:
    name: kubeconfig-resourceslice-mypool
  labels:
    liqo.io/remote-cluster-id: cool-firefly
  resourceQuota:
    hard:
      cpu: "2"
      memory: 4Gi
      pods: "110"
```
````

`````

The `kubeconfig` secret must be created in the consumer cluster in the same namespace where the `VirtualNode` will be created.
The secret must contain the `kubeconfig` file of the provider cluster.

```yaml
apiVersion: v1
data:
  kubeconfig: <base64-encoded-kubeconfig>
kind: Secret
metadata:
  labels:
    liqo.io/remote-cluster-id: cool-firefly
  name: kubeconfig-resourceslice-mypool
  namespace: liqo-tenant-cool-firefly
type: Opaque
```

### Check shared resources and virtual nodes

Via `liqoctl` it is possible to check the amount of shared resources and the virtual nodes configured for a specific peerings looking at [the peering status](../../usage/peer.md#check-status-of-peerings).

### Delete VirtualNode

You can revert the process by deleting the `VirtualNode` in the consumer cluster.

```{code-block} bash
:caption: "Cluster consumer"
kubectl delete virtualnode mynode -n liqo-tenant-cool-firefly
```

## Multiple VirtualNodes

You can create multiple `VirtualNodes` associated with the same provider cluster.
This can be done in two ways:

1. By creating multiple `ResourceSlices` in the consumer cluster targeting the same provider cluster.
2. By creating multiple `VirtualNodes` associated with the same `ResourceSlice`.

The first option is recommended since the resource enforcement is more straightforward to manage, as the `VirtualNode` can use the resources assigned to the `ResourceSlice`.
If you want more resources, you can simply create another `ResourceSlice`.

The second option is still possible for maximum flexibility, but requires the user to allocate `VirtualNodes` resources more carefully, considering that the sum of the resources used by the `VirtualNodes` cannot exceed what is given to the shared `ResourceSlice` (if resource enforcement is enabled).

A provider cluster can allocate more resources than the ones it currently has.
The default enforcement (which can be disabled) only checks that the consumer does not use more resources than the ones negotiated in the `ResourceSlice`.
This is done to allow use cases where the provider cluster has an autoscaler so it can expand dynamically its resources.

See [VirtualNode customization](/advanced/virtualnode-customizations) for more information on how to customize the `VirtualNode` resources.
