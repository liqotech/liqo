# Offloading

**Workload offloading** is enabled by a **virtual node**, which is spawned in the local (i.e., consumer) cluster at the end of the peering process, and represents (and aggregates) the subset of resources shared by the remote cluster.

This solution enables the **transparent extension** of the local cluster, with the new node (and its capabilities) seamlessly taken into account by the vanilla Kubernetes scheduler when selecting the best place for the workloads execution.
At the same time, this approach is fully compliant with the **standard Kubernetes APIs**, hence allowing to interact with and inspect offloaded pods just as if they were executed locally.

(FeatureOffloadingAssignedResources)=

## Assigned resources

By default, the virtual node is assigned with 90% of the resources available in the remote cluster. For example:

* If the remote cluster has 100 vCPUs available, the virtual node created with 90 vCPUs.
* If now the remote cluster starts some applications that consume 50 vCPUs (i.e., pods _requesting_ resources), the virtual node is resized to 45 vCPUs (i.e., 90% of (100-50)).
* If the remote cluster has some autoscaling mechanism that, at some point, double the size of the cluster, which reaches 200 vCPUs (all of them unused by any pod), the virtual node will be resized with 180 vCPUs.

This mechanism applies to all the physical resources available in the remote cluster, e.g., CPUs, RAM, GPUs and more.
The percentage of sharing can be customized also at run-time using the `--sharing-percentage` option, as documented in the proper [section](InstallControlPlaneFlags) of the Liqo installation.

```{warning}
Pay attention to _math rounding_. For instance, if your remote cluster has 1 GPU, with default settings the virtual node will be set with 0.9 GPUs. Since numbers must be integers, you may end up with a virtual node with _zero_ GPUs.
```

```{admonition} More granular resource definitions with external Resource Plugins
The `--sharing-percentage` option is a unique and global parameter for the cluster. Hence, currently Liqo cannot differentiate the resources assigned to different peered clusters.
For a more granular definition of the resources, you should consider to instal an external [Resource Plugin](https://github.com/liqotech/liqo-resource-plugins), or create your own.
```

## Virtual kubelet

The virtual node abstraction is implemented by an extended version of the [**Virtual Kubelet project**](https://github.com/virtual-kubelet/virtual-kubelet#liqo-provider).
A virtual kubelet replaces a traditional kubelet when the controlled entity is not a physical node.
In the context of Liqo, it interacts with both the local and the remote clusters (i.e., the respective Kubernetes API servers) to:

1. Create the **virtual node resource** and reconcile its status with respect to the negotiated configuration.
2. **Offload the local pods** scheduled onto the corresponding (virtual) node to the remote cluster, while keeping their status aligned.
3. Propagate and synchronize the **accessory artifacts** (e.g., *Services*, *ConfigMaps*, *Secrets*, ...) required for proper execution of the offloaded workloads, a feature we call **resource reflection**.

For each remote cluster, a different instance of the Liqo virtual kubelet is started in the local cluster, ensuring isolation and segregating the different authentication tokens.

## Virtual node

A **virtual node** summarizes and abstracts the **amount of resources** (e.g., CPU, memory, ...) shared by a given remote cluster.
Specifically, the virtual kubelet automatically propagates the negotiated configuration into the *capacity* and *allocatable* entries of the node status.

**Node conditions** reflect the current status of the node, with periodic and configurable **healthiness checks** performed by the virtual kubelet to assess the reachability of the remote API server.
This allows to mark the node as *not ready* in case of repeated failures, triggering the standard Kubernetes eviction strategies based on the configured *pod tolerations* (e.g., to enforce service continuity).

Finally, each virtual node includes a set of **characterizing labels** (e.g., geographical region, underlying provider, ...) suggested by the remote cluster.
This enables the enforcement of **fine-grained scheduling policies** (e.g., through *affinity* constraints), in addition to playing a key role in the namespace extension process presented below.

(FeatureOffloadingNamespaceExtension)=

## Namespace extension

To enable seamless workload offloading, **Liqo extends Kubernetes namespaces** across the cluster boundaries.
Specifically, once a given namespace is selected for offloading (see the [namespace offloading usage section](/usage/namespace-offloading) for the operational procedure), Liqo proceeds with the automatic creation of **twin namespaces** in the subset of selected remote clusters.

Remote namespaces host the actual **pods offloaded** to the corresponding cluster, as well as the **additional resources** propagated by the resource reflection process.
This behavior is presented in the figure below, which shows a given namespace existing in the local cluster and extended to a remote cluster.
A group of pods is contained in the local namespace, while a subset (i.e., those faded-out) is scheduled onto the virtual node and offloaded to the remote namespace.
Additionally, the resource reflection process propagated different resources existing in the local namespace (e.g., *Services*, *ConfigMaps*, *Secrets*, ...) in the remote one (represented faded-out), to ensure the correct execution of offloaded pods.

![Representation of the namespace extension concept](/_static/images/features/offloading/namespace-extension.drawio.svg)

The Liqo namespace extension process features a high degree of customization, mainly enabling to:

* Select a **specific subset of the available remote clusters**, by means of standard selectors matching the label assigned to the virtual nodes.
* Constraint whether pods should be scheduled onto **physical nodes only, virtual nodes only, or both**.
The extension of a namespace, forcing at the same time all pods to be scheduled locally, enables the consumption of local services from the remote cluster, as shown in the [*service offloading* example](/examples/service-offloading).
* Configure whether the **remote namespace name** should match the local one (although possibly incurring in conflicts), or be automatically generated, such as to be unique.

(FeaturePodOffloading)=

## Pod offloading

Once a **pod is scheduled onto a virtual node**, the corresponding Liqo virtual kubelet (indirectly) creates a **twin pod object** in the remote cluster for actual execution.
Liqo supports the offloading of both **stateless** and **stateful** pods, the latter either relying on the provided [**storage fabric**](/features/storage-fabric) or leveraging externally managed solutions (e.g., persistent volumes provided by the cloud provider infrastructure).

**Remote pod resiliency** (hence, service continuity), even in case of temporary connectivity loss between the two control planes, is ensured through a **custom resource** (i.e., *ShadowPod*) wrapping the pod definition, and triggering a Liqo enforcement logic running in the remote cluster.
This guarantees that the desired pod is always present, without requiring the intervention of the originating cluster.

The virtual kubelet takes care of the automatic propagation of **remote status changes** to the corresponding local pod (remapping the appropriate information), allowing for complete **observability** from the local cluster.
Advanced operations, such as **metrics and logs retrieval**, as well as **interactive command execution** inside remote containers, are transparently supported, to comply with standard troubleshooting operations.

Additional details concerning how pods are propagated to remote clusters are provided in the [resource reflection usage section](/usage/reflection).

(FeatureResourceReflection)=

## Resource reflection

The **resource reflection** process is responsible for the propagation and synchronization of selected control plane information into remote clusters, to enable the seamless execution of offloaded pods.
Liqo supports the reflection of the resources dealing with **service exposition** (i.e., *Ingresses*, *Services* and *EndpointSlices*), **persistent storage** (i.e., *PersistentVolumeClaims* and *PersistentVolumes*), as well as those storing **configuration data** (i.e., *ConfigMaps* and *Secrets*).

All resources of the above types that live in a **namespace selected for offloading** are automatically propagated into the corresponding twin namespaces created in the selected remote clusters.
Specifically, the local copy of each resource is the source of trust leveraged to realign the content of the **shadow copy** reflected remotely.
Appropriate **remapping** of certain information (e.g., *endpoint IPs*) is transparently performed by the virtual kubelet, accounting for conflicts and different configurations in different clusters.

You can refer to the [resource reflection usage section](/usage/reflection) for a detailed characterization of how the different resources are reflected into remote clusters.
