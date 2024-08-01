# Storage Fabric

The Liqo **storage fabric** subsystem enables the seamless offloading of **stateful workloads** to remote clusters.
The solution is based on two main pillars:

- **Storage binding deferral** until its first consumer is scheduled onto a given cluster (either local or remote).
This ensures that **new storage pools** are created in the exact location where their associated pods have just been scheduled for execution.
- **Data gravity**: a set of **automatic policies** to attract pods in the appropriate cluster, guaranteeing that pods requesting **existing pools of storage** (e.g., after a restart) are scheduled onto the cluster physically hosting the corresponding data.

These approaches extend standard Kubernetes practice to multi-cluster scenarios, simplifying at the same time the configuration of **high availability** and **disaster recovery** scenarios.
To this end, one relevant use case is represented by database instances that need to be replicated and synchronized across different clusters, which is shown in the [stateful applications example](/examples/stateful-applications).

Under the hood, the Liqo storage fabric leverages a **virtual storage class**, which embeds the logic to **create the appropriate storage pools** in the different clusters.
Whenever a new *PersistentVolumeClaim (PVC)* associated with the virtual storage class is created, and its consumer is bound to a node, the Liqo logic goes into action, based on the target node:

- If it is a **real node** (a node of the local cluster), the `PVC` associated with the *Liqo storage class* is remapped to a second one, associated with the corresponding **real storage class**, to transparently **provision the requested volume**.
- In the case of **virtual nodes**, the reflection logic is responsible for creating the **remote shadow PVC**, remapping to the negotiated storage class, and **synchronizing the PersistentVolume information**, to allow pod binding.

In both cases, **locality constraints** are automatically embedded within the resulting *PersistentVolumes (PVs)*, to make sure each pod is scheduled only onto the cluster where the associated storage pools are available.

Additional details about the **configuration of the Liqo storage fabric**, as well as concerning the possibility to **move storage pools** among clusters through the Liqo CLI tool, are presented in the [stateful applications usage section](/usage/stateful-applications).

```{admonition} Note
In addition to the provided storage class, Liqo supports the execution of pods leveraging cross-cluster storage managed by external solutions (e.g., persistent volumes provided by the cloud provider infrastructure).
```
