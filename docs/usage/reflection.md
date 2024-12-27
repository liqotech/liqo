# Resource Reflection

This section characterizes the [**resource reflection**](FeatureResourceReflection) process (including also [**pod offloading**](FeaturePodOffloading)), detailing how the different resources are propagated to remote clusters and which fields are mutated.

Briefly, the set of supported resources includes (by category):

* [**Workload**](UsageReflectionPods): *Pods*
* [**Exposition**](UsageReflectionExposition): *Services*, *EndpointSlices*, *Ingresses*
* [**Storage**](UsageReflectionStorage): *PersistentVolumeClaims*, *PresistentVolumes*
* [**Configuration**](UsageReflectionConfiguration): *ConfigMaps*, *Secrets*, *ServiceAccounts*
* [**Event**](UsageReflectionEvent): *Events*

(UsageReflectionPolicies)=

## Reflection policies

Liqo implements two different reflection policies:

* ***DenyList***: reflects all the resources available in the liqo-enabled namespaces, excluding the ones with the `liqo.io/skip-reflection` annotation.
* ***AllowList***: do not reflect any resource in the liqo-enabled namespaces, but the ones with the `liqo.io/allow-reflection` annotation.

You can configure the preferred reflection policy for each resource type through the Helm value `offloading.reflection.<resource>.type`:

```bash
liqoctl install ... --set "offloading.reflection.secret.type=AllowList"
```

````{warning}
* ***DenyList*** is the **default** reflection policy for all resources.
* Only the *Pods*, *PVCs*, and *ServiceAccounts* reflectors follow a **custom** Liqo logic and can't be customized.
* The *EndpointSlice* reflector inherits the reflection policy from the *Service* reflector, and follows the following policy:
  * an endpointslice is (not) reflected if the associated service is (not) reflected
  * you can bypass the above behavior if you explicitly annotate the endpointslice itself (i.e., reflect the endpointslice using `liqo.io/allow-reflection` annotation, do not reflect using `liqo.io/skip-reflection`)
````

````{admonition} Note
The number of workers to use for the reflection of a given type of resource is customizable through the Helm value `offloading.reflection.<resource>.workers`.
Additionally, you can set the number of workers to 0 to **completely disable the reflection** of a given type of resource (e.g., *Secrets*) towards remote clusters:

```bash
liqoctl install ... --set "offloading.reflection.secret.workers=0"
```
````

(UsageReflectionLabelsAnnots)=

## Disabling the reflection of specific labels and annotations

In some cases, it could be useful to **not propagate** to the remote clusters some labels/annotations present on reflected resources.
This can be useful to avoid reflecting labels/annotations that lead to conflicts between the local and remote resources (e.g., the ones added by cloud providers and that are tied to the configuration of the hosting cluster), thus **preventing infinite reconciliations** of the reflected resource.

You can disable the reflection of custom labels and annotations by configuring at install-time respectively the Helm values `offloading.reflection.skip.labels` and `offloading.reflection.skip.annotations` with the list of **keys** that must not be reflected.
To modify the list of not-reflected labels/annotations if Liqo is already installed, or if you want to customize it for each virtual node, you can either:

* Set the [`OffloadingPatch`](OffloadingPatch) of the individual virtual nodes using the fields `spec.offloadingPatch.labelsNotReflected` and `spec.offloadingPatch.annotationsNotReflected`.
* Reference a custom [`VkOptionsTemplate`](VkOptionsTemplate) CR in the virtual node spec.
* Patch the default [`VkOptionsTemplate`](VkOptionsTemplate) CR or upgrade Liqo with the new Helm values (but keep in mind that existing virtual nodes will not be updated dynamically, so you should unpeer first and peer again to apply the new changes to existing peers).

(UsageReflectionPods)=

## Pods offloading

Liqo leverages a custom resource, named *ShadowPod*, combined with an appropriate enforcement logic to ensure **remote pod resiliency** even in case of temporary connectivity loss between the local and remote clusters.

**Pod specifications** are propagated to the remote cluster **verbatim**, except for the following fields that are mutated:

* Removal of **scheduling constraints** (e.g., *Affinity*, *NodeSelector*, *SchedulerName*, *Preemption*, ...), as referring to the local cluster.
* Mutation of **service account** related information, to allow offloaded pods to transparently interact with the local (i.e., origin) API server, instead of the remote one.
* Enforcement of the properties concerning the usage of **host namespaces** (e.g., network, IPC, PID) to *false* (i.e., disabled), as potentially invasive and troublesome.

````{admonition} Note
*Anti-affinity presets* can be leveraged to specify predefined scheduling constraints for offloaded pods, spreading them across different nodes in the remote cluster.
This feature is enabled through the `liqo.io/anti-affinity-preset` pod annotation, which can take three values:

* `propagate`: the anti-affinity constraints of the pod are propagated *verbatim* when offloaded to the remote cluster.
  Make sure that they match both the virtual node in the local cluster and at least one physical node in the remote cluster, otherwise the pod will fail to be scheduled (i.e., remain in pending status).
* `soft`: the pods sharing the same labels are *preferred* to be scheduled on different nodes (i.e., it is translated into a *preferredDuringSchedulingIgnoredDuringExecution* anti-affinity constraint).
* `hard`: the pods sharing the same labels are *required* to be scheduled on different nodes (i.e., it is translated into a *requiredDuringSchedulingIgnoredDuringExecution* anti-affinity constraint).

When set to *soft* or *hard*, the `liqo.io/anti-affinity-labels` annotation allows to select a subset of the pod label keys to build the anti-affinity constraints:

```yaml
annotations:
  liqo.io/anti-affinity-preset: soft
  liqo.io/anti-affinity-labels: app.kubernetes.io/name,app.kubernetes.io/instance
```

Given that affinity constraints are *immutable*, the addition/removal of the annotations to/from an already existing pod *does not have any effect*.
Make sure that the annotations are configured appropriately in the template of the managing object (e.g., *Deployment*, or *StatefulSet*).
````

Differently, **pod status** is propagated from the remote cluster to the local one, performing the following modifications:

* The *PodIP* is **remapped** according to the network fabric configuration, such as to be reachable from the other pods running in the same cluster.
* The *NodeIP* is replaced with the one of the corresponding virtual kubelet pod.
* The number of **container restarts** is augmented to account for the possible deletions of the remote pod (whose presence is enforced by the controlling *ShadowPod* resource).

````{admonition} Note
A pod living in a namespace not enabled for offloading, but manually forced to be scheduled in a virtual node, remains in *Pending* status, and it is signaled with the *OffloadingBackOff* reason.
For instance, this can happen for system *DaemonSets* (e.g., CNI plugins), which tolerate all *taints* (hence, including the one associated with virtual nodes) and thus get scheduled on *all nodes*.

To prevent this behavior, it is necessary to explicitly modify the involved *DaemonSets*, adding a suitable *affinity* constraint excluding virtual nodes:
```yaml
affinity:
  nodeAffinity:
    requiredDuringSchedulingIgnoredDuringExecution:
      nodeSelectorTerms:
        - matchExpressions:
          - key: liqo.io/type
            operator: NotIn
            values:
            - virtual-node
```
````

(UsageReflectionExposition)=

## Service exposition

The reflection of **Service** and **EndpointSlice** resources is a key element to allow the seamless **intercommunication** between microservices spread across multiple clusters, enabling the usage of standard DNS discovery mechanisms.
In addition, the propagation of **Ingresses** enables the definition of multiple points of entrance for the external traffic.

### Services

**Services** are reflected **verbatim** into remote clusters, except for what concerns the *ClusterIP*, *LoadBalancerIP* and *NodePort* fields (when applicable), which are left empty (hence defaulted by the remote cluster), as likely conflicting.
Still, the usage of **standard DNS discovery** mechanisms (i.e., based on service name/namespace) abstracts away the *ClusterIP* differences, with each pod retrieving the correct IP address.

```{admonition} Note
In case *node port* correspondence across clusters is required, its propagation can be enforced adding the `liqo.io/force-remote-node-port=true` annotation to the involved service.
```

(UsageReflectionEndpointSlices)=

### EndpointSlices

In the local cluster, Services are transparently handled by the vanilla Kubernetes control plane, since it has **full visibility of all pods** (even those offloaded), hence leading to the creation of the corresponding **EndpointSlice** entries.
Differently, the control plane of each remote cluster perceives **only the pods running in that cluster**, and the standard *EndpointSlice* creation logic alone is not sufficient (as it would not include the pods hosted by other clusters).

This gap is filled by the Liqo **EndpointSlice reflection** logic, which takes care of propagating all *EndpointSlice* entries (i.e. endpoints) not already present in the destination cluster.
During the propagation process, endpoint addresses are appropriately **remapped** according to the **network fabric** configuration, ensuring that the resulting IPs are reachable from the destination cluster.

Thanks to this approach, **multiple replicas** of the same microservice spread across different clusters, and backed by the same service, are handled transparently.
Each pod, no matter where it is located, contributes with a distinct *EndpointSlice* entry, either by the standard control plane or through resource reflection, hence becoming eligible during the **Service load-balancing process**.

```{admonition} Note
Even in a scenario where a single cluster is peered with multiple remote ones, the **EndpointSlice reflection** logic ensures that a **pod** scheduled **remotely** is reachable from every cluster through its **service**.
```

### Ingresses

The propagation of **Ingress** resources enables the configuration of multiple points of entrance for **external traffic**.
*Ingress* resources are propagated **verbatim** into remote clusters, except for the *IngressClassName* field, which is left empty.
Hence, selecting the default *ingress class* in the remote cluster, as the local one (i.e., the one in the origin cluster) might not be present.

(UsageReflectionStorage)=

## Persistent storage

The reflection of **PersistentVolumeClaims (PVCs)** and **PersistentVolumes (PVs)** is a key to enable the cross-cluster [Liqo storage fabric](/features/storage-fabric).
Specifically, the process is triggered when a PVC requiring the *Liqo storage class* is bound for the first time, and the requesting pod is scheduled in a virtual node (i.e., remote cluster).
Upon this event, the **PVC is propagated verbatim** to the remote cluster, replacing the requested *StorageClass* with the one negotiated during the peering process.

Once created, the **resulting PV is reflected backwards** (i.e., from the remote to the local cluster), and the proper **affinity selectors** are added to **bind it to the virtual node**.
Hence, subsequent pods mounting that *PV* will be scheduled on that virtual node, and eventually offloaded to the same remote cluster.

(UsageReflectionConfiguration)=

## Configuration data

**ConfigMaps** and **Secrets** typically hold **configuration data** consumed by pods, and both types of resources are propagated by Liqo **verbatim** into remote clusters.
In this respect, Liqo features also the propagation of **ServiceAccount tokens**, to enable offloaded pods to contact the Kubernetes API server of the origin cluster, as well as to support those applications leveraging *ServiceAccounts* for internal authentication purposes.

````{warning}
*ServiceAccount* tokens are stored within *Secret* objects when propagated to the remote cluster.
This implies that any entity authorized to access *Secret* objects (or the mounting pods) might **retrieve the tokens and impersonate the offloaded workloads**.
Hence, gaining the possibility to interact with the Kubernetes API server of the origin cluster, with the same permissions granted to the corresponding service account.

If this is a security concern in your scenario (e.g., the clusters are under the control of different administrative domains), it is possible to disable this feature setting the `--enable-apiserver-support=false` virtual kubelet flag at install time:
```bash
liqoctl install ... --set "virtualKubelet.extra.args={--enable-apiserver-support=false}"
```
````

(UsageReflectionEvent)=

## Events

Remote events are reflected to the local cluster to improve debuggability and visibility.
More specifically, an event is propagated if it belongs to an offloaded namespace and its associated resource is one of the following: *pods*, *services*, *endpointslices*, *ingresses*, *configmaps*, *secrets*, *PVCs*.

```{admonition} Note
The event reflector is the only one that propagates a resource from the remote cluster to the local cluster.
Local events are not reflected to the remote cluster.
```

(UsageReflectionRuntimeClass)=

## RuntimeClass

The **RuntimeClass** (`.spec.runtimeClassName` field) is reflected from the local pod to the remote one.

If you are using the [Liqo RuntimeClass](../usage/namespace-offloading.md#runtimeclass), you cannot specify the RuntimeClass name as the field is already used.
To overcome this problem, you can annotate the pod with `liqo.io/remote-runtime-class-name: <MY_RUNTIMECLASS_NAME>`.

It is also possible to enforce a remote RuntimeClass to all pods scheduled on a virtual node, by specifying it in the *OffladingPatch* of the virtualnode (`.spec.offloadingPatch.runtimeClassName`).
If you are using liqoctl to create the virtual node, you can leverage the `--runtime-class-name` flag.  

If these options are used in combination, the following priority (from higher to lower priority) will be used to determine the remote RuntimeClass:

1. pod Annotation (`liqo.io/remote-runtime-class-name`).
2. pod RuntimeClass (`.spec.runtimeClassName`). It is ignored if set to `liqo`.
3. virtualNode OffloadingPatch (`.spec.offloadingPatch.runtimeClassName`).
