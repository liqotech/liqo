# Stateful Applications

As introduced in the [storage fabric features section](/features/storage-fabric.md), Liqo supports **multi-cluster stateful applications** by extending the classical approaches adopted in standard Kubernetes clusters.

(UsageStatefulApplicationsVirtualStorageClass)=

## Liqo virtual storage class

The Liqo virtual storage class is a [*Storage Class*](https://kubernetes.io/docs/concepts/storage/storage-classes/) that embeds the logic to manage some virtual `PersistentVolumeClaims` and `PersistentVolumes`, bound to a real copy of those resources, managed by the storage class configured in the target cluster.  depending on the target cluster the mounting pod is scheduled onto.
The storage binding the deferred until its first consumer is scheduled onto a given cluster (either local or remote), ensuring that the **new storage pools** are created where their associated pods have just been scheduled.

All operations performed on virtual objects (i.e., *PersistentVolumeClaims (PVCs)* and *PersistentVolumes (PVs)* associated with the *liqo* storage class) are then automatically propagated by Liqo to the corresponding real ones.

Additionally, once a real *PV* gets created, the corresponding virtual one is enriched with a set of policies to attract mounting pods in the appropriate cluster, guaranteeing that pods requesting **existing pools of storage** are scheduled onto the cluster physically hosting the corresponding data, following the **data gravity** approach.

This process is **completely transparent** from the management point of view, with the only difference being the name of the storage class.

```{warning}
The deletion of the virtual *PVC* will cause the deletion of the real *PVC/PV*, and the stored data will be **permanently lost**.
```

The Liqo control plane handles the binding of virtual PVC resources (i.e., associated with the *liqo* storage class) differently depending on the cluster where the mounting pod gets eventually scheduled onto, as detailed in the following.

### Local cluster binding

In case a virtual *PVC* is bound to a pod initially **scheduled onto the local cluster** (i.e., a physical node), the Liqo control plane takes care of creating a twin *PVC* (in turn originating the corresponding twin *PV*) in the *liqo-storage* namespace, while mutating the *storage class* to that configured at Liqo installation time (with a fallback to the default one).
A virtual *PV* is eventually created by Liqo to mirror the real one, effectively allowing pods to mount it and enforcing the *data gravity* constraints.

The resulting configuration is depicted in the figure below.

```{figure} /_static/images/usage/stateful-applications/virtual-storage-class-local.drawio.svg
---
align: center
---
Virtual Storage Class Local
```

```{admonition} Note
Currently, the virtual storage class does not support the configuration of [Kubernetes mount options](https://kubernetes.io/docs/concepts/storage/storage-classes/#mount-options) and parameters.
```

### Remote cluster binding

In case a virtual *PVC* is bound to a pod initially **scheduled onto a remote cluster** (i.e., a virtual node), the Liqo control plane takes care of creating a twin *PVC* (in turn originating the corresponding twin *PV*) in the *offloaded* namespace, while mutating the *storage class* to that configured at Liqo installation time in the remote cluster (with a fallback to the default one).
A virtual *PV* is eventually created by Liqo to mirror the real one, effectively allowing pods to mount it and enforcing the *data gravity* constraints.

The resulting configuration is depicted in the figure below.

```{figure} /_static/images/usage/stateful-applications/virtual-storage-class-remote.drawio.svg
---
align: center
---
Virtual Storage Class Remote
```

```{warning}
The tearing down of the peering and/or the deletion of the offloaded namespace will cause the deletion of the real PVC, and the stored data will be **permanently lost**.
```

### Move PVCs across clusters

Once a PVC is created in a given cluster, subsequent pods mounting that volume will be forced to be **scheduled onto the same cluster** to achieve storage locality, following the *data gravity* approach.

Still, if necessary, you can **manually move** the storage backing a virtual *PVC* (i.e., associated with the *liqo* storage class) from a cluster to another, leveraging the appropriate *liqoctl* command.
Then, subsequent pods will get scheduled in the cluster the storage has been moved to.

```{warning}
This procedure requires the *PVC/PV* not to be bound to any pods during the entire process.
In other words, live migration is currently not supported.
```

A given *PVC* can be moved to a target node (either physical, i.e., local, or virtual, i.e., remote) through the following command:

```bash
liqoctl move volume $PVC_NAME --namespace $NAMESPACE_NAME --target-node $TARGET_NODE_NAME
```

Where:

* `$PVC_NAME` is the name of the *PVC* to be moved.
* `$NAMESPACE_NAME` is the name of the namespace where the *PVC* lives in.
* `$TARGET_NODE_NAME` is the name of the node where the *PVC* will be moved to.

Under the hood, the migration process leverages the Liqo cross-cluster network fabric and the [Restic project](https://restic.net/) to back up the original data in a temporary repository, and then restore it in a brand-new *PVC* forced to be created in the target cluster.

```{warning}
*Liqo* and *liqoctl* **are not** backup tools. Make sure to properly back up important data before starting the migration process.
```

(NativeStorageClass)=

## Externally managed storage

In addition to the virtual storage class, Liqo supports the offloading of pods that bind to ***cross-cluster* storage managed by external solutions** (e.g., managed by the cloud provider, or manually provisioned).
Specifically, the *volumes* stanza of the pod specification is propagated verbatim to the offloaded pods, hence allowing to specify volumes available only remotely.

```{admonition} Note
In case a piece of externally managed storage is available only in one remote cluster, it is likely necessary to manually force pods to get scheduled exactly in that cluster.
To prevent scheduling issues (e.g., the pod is marked as *Pending* since the local cluster has no visibility on the remote *PVC*), it is suggested to configure the target *NodeName* in the pod specifications to match that of the corresponding virtual nodes, hence bypassing the standard Kubernetes scheduling logic.
```

```{warning}
Due to current Liqo limitations, the remote namespace, including any *PVC* therein contained, will be **deleted** in case the local namespace is unoffloaded/deleted, or the peering is torn down.
```
