# Offloading in depth

## Overview

```{warning}
The aim of the following section is to give an idea of how Liqo works under the hood and to provide the instruments to address more complex scenarios.
The `liqoctl peer` is enough for the most part of the cases, so [stick with it](/usage/peer) if you do not have any specific needs.
```

This document will go over the process of acquiring resources and making them available as a `Node` in the consumer cluster.

You can add a `VirtualNode` to a consumer cluster in two different ways:

1. By creating a `ResourceSlice` in the tenant namespace in the consumer cluster.
2. By creating a `VirtualNode` in the consumer cluster.

Note that the `ResourceSlice` method is the **preferred way** to add a `VirtualNode` to a consumer cluster, but it requires the *Authentication module* to be enabled (which is enabled by default when using `liqoctl peer`, `liqoctl authenticate`, or the [manual configuration](/advanced/peering/inter-cluster-authentication)).
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

To add other resources like `ephemeral-storage`, `gpu`, or any other custom resources, you can use the `-o yaml` flag for the `liqoctl create resourceslice` command and edit the `ResourceSlice` spec manifest before applying it.

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

The amount of resources shared by the provider cluster is managed by the _ResourceSlice class controller_, which decides whether to accept or deny a ResourceSlice based on a set of criteria and the available resources in the cluster.

The default _ResourceSlice class controller_ is quite simple, as it accepts any incoming ResourceSlice request from the consumer.
Consequently, the provider cluster might grant more resources than it currently has.
While this might seem problematic, it can be **useful in scenarios where the cluster has an autoscaler that dynamically acquires resources** as needed.

To support more complex scenarios and specific use cases, Liqo allows the definition of _custom ResourceSlice classes_.
These classes enable the implementation of **custom logic to determine whether to accept or reject a ResourceSlice** (e.g., based on a tenant's resource quota) and how much resources the provider cluster can share.
**This logic should be implemented in a _custom ResourceSlice class controller_**, whose template is available [in this repository](https://github.com/liqotech/resource-slice-class-controller-template).

The ResourceSlice class can be specified either in the YAML manifest or by using the `--class` flag with `liqoctl`:

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

Once a custom class is defined in the `ResourceSlice` spec, the custom ResourceSlice controller will be responsible for accepting or denying the ResourceSlices and updating their status with the amount of granted resources.
The custom controller might deny the request, fully accept it, or partially accept it by providing only a portion of the requested resources.
The `VirtualNode` in the consumer cluster and the `Quota` in the provider cluster will be created based on the resources granted by the custom controller.

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

Via `liqoctl` it is possible to check the amount of shared resources and the virtual nodes configured for a specific peering by looking at [the peering status](../../usage/peer.md#check-status-of-peerings).

### Delete VirtualNode

You can revert the process by deleting the `VirtualNode` in the consumer cluster.

```{code-block} bash
:caption: "Cluster consumer"
kubectl delete virtualnode mynode -n liqo-tenant-cool-firefly
```

## Multiple VirtualNodes

In some cases, it might be beneficial to have multiple `VirtualNodes` pointing to the same provider cluster.
For example, this can be useful to tune the Kubernetes scheduler.
For instance, if a `VirtualNode` is larger than other nodes in the cluster, the scheduler tends to place the majority of the pods on that node.

Additionally, you might want to divide resources into subgroups.
For example, if the provider cluster shares 50 CPUs through a `ResourceSlice`, but 40 of them are x86 and the remaining 10 are ARM, these resources cannot be used interchangeably, given their different nature.
In such a scenario, you should split them into two Liqo virtual nodes: one exposing the 10 ARM CPUs and the other exposing the remaining 40 x86 CPUs.

There are two strategies to create `VirtualNodes` associated with the same provider cluster:

1. By **creating multiple `ResourceSlices`** in the consumer cluster targeting the same provider cluster: this is the recommended strategy, as resource enforcement is more straightforward to manage. For each `ResourceSlice` that grants resources on the provider cluster, there is a single `VirtualNode` exposing them. If you need more resources, you can simply create another `ResourceSlice`.
2. By **creating multiple `VirtualNodes` associated with the same `ResourceSlice`**: this approach offers maximum flexibility but requires careful resource allocation. Multiple `VirtualNodes` will share the resources granted by the same `ResourceSlice`, and the total resources exposed by these `VirtualNodes` cannot exceed the resources granted by `ResourceSlice` (if resource enforcement is enabled).

## Resource Enforcement

To ensure that the pods scheduled on a Liqo virtual node do not exceed the resources granted by the ResourceSlice, pods should have resource `limits` set. This allows the consumer cluster scheduler to be aware of how many resources have already been allocated, and it can select another node for scheduling.

By default, Liqo enables a server-side check that ensures the requests of the pod do not exceed the quota (`controllerManager.config.enableResourceEnforcement` option enabled).
However, this is not a guarantee that the consumer is not using more than expected, as if limits are higher than requests or if the user does not set limits and requests at all, a consumer cluster can use more than the quota.
To define how strict is the server-side resources enforcement, ensuring that the consumer cluster never exceeds the quota, it is possible to act on the `controllerManager.config.defaultLimitsEnforcement` option, which can assume the following values:

* **None** (default): the offloaded pods might not have the resource `requests` or `limits`. Which involves that the consumer cluster might use more than the resources negotiated via the `ResourceSlice`.
* **Soft**: it forces the offloaded pods to have the `requests` set, which implies that pre-allocated resources will never go over the quota, but if the pods go over the requests, the total used resources might go over the quota.
* **Hard**: it forces the offloaded pods to have both `limits` and `requests` set, with `limits` equal to the `requests`. **This is the safest mode** as the consumer cluster cannot go over the quota negotiated via the `ResourceSlice`.

These options [need to be set at installation time](../../installation/install.md#customization-options), by defining them in the `values.yaml` or providing them via the `--set` argument to `helm install` or `liqoctl install`.
For example, to set the `defaultLimitsEnforcement` to `Hard`:

```bash
liqoctl install [...ARGS] --set controllerManager.config.defaultLimitsEnforcement=Hard
```
