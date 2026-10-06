# Resource Reflection

This section characterizes the [**resource reflection**](FeatureResourceReflection) process (including also [**pod offloading**](FeaturePodOffloading)), detailing how the different resources are propagated to remote clusters and which fields are mutated.

Briefly, the set of supported resources includes (by category):

* [**Workload**](UsageReflectionPods): *Pods*
* [**Exposition**](UsageReflectionExposition): *Services*, *EndpointSlices*, *Ingresses*
* [**Storage**](UsageReflectionStorage): *PersistentVolumeClaims*, *PersistentVolumes*
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
* Patch the default [`VkOptionsTemplate`](VkOptionsTemplate) CR or upgrade Liqo with the new Helm values (the changes are automatically propagated to the existing virtual nodes, whose *virtual-kubelet* deployment is updated accordingly).

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

When the local *Ingress* specifies an *IngressClass* managed by Liqo (e.g., `liqo`), the offloading infrastructure tracks the status of the reflected *Ingress* resources in the remote clusters through *ShadowIngressStatus* custom resources.
Each remote cluster reports the status of its own reflected *Ingress* (e.g., the assigned load-balancer IPs), and the Liqo controller manager aggregates these statuses back into the local *Ingress* status.
This allows the origin cluster to present a unified view of the ingress endpoints, combining the load-balancer information from all peered clusters where the *Ingress* has been offloaded.

### Gateway API

Liqo supports the reflection of the [Gateway API](https://gateway-api.sigs.k8s.io/) **Gateway**, **HTTPRoute**, **GRPCRoute** and **ReferenceGrant** resources.
Other routes (i.e., *TCPRoutes*, *TLSRoutes* and *UDPRoutes*) are not reflected, and a *FailedReflection* warning event is generated for each of them in the offloaded namespaces.
Similarly to the ingress classes, the reflection is enabled when the provider cluster offers any *GatewayClass*, used for the reflected *Gateways*, or any **shared Gateway**, which the reflected routes are attached to.
The *GatewayClasses* are configured in the **provider cluster** through the following Helm value, either at install time or later, upgrading the installation:

```bash
liqoctl install ... --set "offloading.reflection.gateway.gatewayClasses[0].name=envoy"
```

A change of the offered *GatewayClasses* applies to the existing peerings as well, not only to the new ones: the *ResourceSlices* of the consumer clusters are updated when the Liqo controller manager of the provider cluster restarts with the new configuration, and the virtual kubelets of the consumer clusters are restarted accordingly.

The shared *Gateways* are instead declared at runtime by the administrator of the **provider cluster**, labeling them with `liqo.io/shared-gateway=true`.
When multiple *Gateways* are shared, the one labeled with `liqo.io/shared-gateway=default` (or the first one, by namespace and name, otherwise) is used:

```bash
kubectl label gateway -n infra public liqo.io/shared-gateway=default
```

The offered *GatewayClasses* and shared *Gateways* are propagated to the consumer clusters through the *ResourceSlice* and *VirtualNode* resources, and they are kept up to date when they change.
When multiple *GatewayClasses* are offered, the one marked as `default: true` (or the first one otherwise) is used.
The same information can be specified when manually creating a virtual node, through the `--gateway-classes` and `--shared-gateways` flags of `liqoctl create virtualnode`.
Alternatively, the reflection can be configured through the following virtual kubelet flags (e.g., through the `virtualKubelet.extra.args` Helm value in the consumer cluster):

* `--enable-gateway-api`: enables the reflection of the Gateway API resources.
* `--remote-real-gateway-class-name`: the *GatewayClass* offered by the remote cluster, used for the reflected *Gateways*.
* `--enable-remote-shared-gateway`: whether the remote cluster offers a shared *Gateway*, which the reflected routes are attached to when their *Gateways* are not reflected.

The consumer cluster is not aware of the actual shared *Gateway*: the reflected routes refer to it through the `liqo-shared-gateway` **placeholder**, which is replaced with the shared *Gateway* by the Liqo webhook of the provider cluster, when the route is created or updated.
Hence, changing the shared *Gateway* does not require any change in the consumer clusters, and the virtual kubelets are restarted only when the provider cluster starts (or stops) offering shared *Gateways*.

If the Gateway API CRDs are installed in the local cluster, Liqo creates the **liqo** virtual *GatewayClass* (the name can be customized through the `offloading.reflection.gateway.virtualGatewayClassName` Helm value).
The *Gateways* of this class are not served by the local cluster, but they are **reflected to the remote clusters** where their namespace is offloaded, using the *GatewayClass* offered by the remote cluster.
This enables the exposition of the offloaded applications in the remote clusters, e.g., to implement a global ingress.
*Gateways* of other classes are served by the local cluster, and they are not reflected.

The reflected *Gateways* are translated as follows:

* The namespaces of the **certificate references** are translated to the corresponding remote namespaces.
  Listeners referring to certificates in namespaces not offloaded to the remote cluster are dropped.
* Listeners allowing the attachment of routes from **all namespaces** are restricted to the remote namespaces hosting the workloads offloaded from the local cluster, while the ones leveraging **label selectors** are restricted to the same namespace, since selectors refer to the labels of the local namespaces.
* Fields referring to the local cluster (i.e., `addresses` and `infrastructure.parametersRef`), or affecting other tenants of the remote cluster (i.e., `defaultScope` and `allowedListeners`), are dropped.

Alternatively, a *Gateway* of the virtual class can be **mapped to the shared Gateway** offered by the remote cluster, rather than reflected as a new *Gateway*.
In this case, no *Gateway* is created in the remote cluster, and the routes attached to the local *Gateway* are attached to the shared one.
This happens if the *Gateway* is annotated with `liqo.io/remote-gateway-mode: shared`, or automatically if the remote cluster offers a shared *Gateway*, but no *GatewayClass*:

```yaml
apiVersion: gateway.networking.k8s.io/v1
kind: Gateway
metadata:
  name: web
  annotations:
    liqo.io/remote-gateway-mode: shared
spec:
  gatewayClassName: liqo
  listeners:
  - name: http
    port: 80
    protocol: HTTP
```

The listeners of a *Gateway* mapped to the shared one are not applied in the remote cluster, where the hostnames, ports and certificates are the ones configured for the shared *Gateway* by the administrator of the remote cluster.
Similarly, the `sectionName` and `port` fields of the parent references of the attached routes are dropped.
The same *Gateway* may be mapped to the shared one in some remote clusters (e.g., the ones not offering any *GatewayClass*), and reflected in the others.

The consumer cluster is not allowed to access the shared *Gateway*, hence the remote cluster reports it through the `liqo.io/shared-gateway` annotation of the reflected routes attached to it (set by the Liqo webhook, when replacing the placeholder), and its **addresses** through the `liqo.io/shared-gateway-addresses` annotation (set by the Liqo controller manager, once the shared *Gateway* is programmed).
The *Gateway* mapped to the shared one reports such addresses, and it is *Programmed* in the remote cluster, as soon as at least _one_ route reflected in the same namespace is attached to the shared *Gateway*.
Until then, or if the Liqo version installed in the remote cluster does not report the addresses, the *Gateway* is not *Programmed* in that cluster, while the attached routes are anyway served by the shared *Gateway*.

**Routes** are reflected independently of the class of their *Gateways*, since they can be attached to the shared *Gateway* offered by the remote cluster even if their *Gateways* are not reflected (e.g., to expose an offloaded application through the *Gateway* preconfigured by the infrastructure team of the remote cluster, as envisioned by the Gateway API role-oriented design).
They are translated as follows:

* **Parent references** to *Gateways* of the virtual class are translated to the remote copies of the *Gateways*, preserving the listener they are attached to.
  References to other *Gateways*, which are not reflected, as well as to the *Gateways* mapped to the shared one, are replaced by a single reference to the **shared Gateway** offered by the remote cluster (i.e., to the placeholder, replaced by the remote cluster).
  The shared *Gateway* must allow the attachment of routes from the remote namespaces hosting the offloaded workloads (i.e., through the `allowedRoutes` field of its listeners).
  References to *Services* (i.e., mesh routes) are preserved, translating their namespace.
* **Backend references** to *Services* in the same namespace are preserved as is, while the ones to *Services* in other namespaces are translated to the corresponding remote namespace, if offloaded to the same cluster.
  References to namespaces not offloaded to the remote cluster, as well as to other kinds of backends, are dropped.
* **Filters** not referring to other objects are preserved as is, while *RequestMirror* filters are dropped if their backend cannot be translated.

**ReferenceGrants** are reflected to allow cross-namespace references between namespaces offloaded to the same remote cluster.
The namespaces the references are granted from are translated to the corresponding remote namespaces, while the ones not offloaded to the remote cluster are dropped.

The provider cluster does not rely on the consumer clusters to translate the resources correctly, and the **Liqo webhook enforces** the following constraints on the Gateway API resources in the namespaces hosting offloaded workloads:

* *Gateways* shall belong to one of the *GatewayClasses* offered to the consumer clusters.
* Routes shall be attached only to *Gateways* in the same namespace (i.e., reflected), to the shared *Gateways*, or to *Services* in namespaces offloaded by the same consumer cluster, and they shall not include *ExtensionRef* filters.
  The parent references are validated only when added: the routes attached to a *Gateway* which is no longer shared are not detached, while new routes cannot be attached to it.
  If no *Gateway* is shared anymore, the placeholder is removed (i.e., the route is detached), and the consumer cluster reports the route as not *Accepted*.

Resources are **never reflected in a less restrictive form**: *Gateways* including a Gateway-level TLS configuration (which may enforce the authentication of clients and backends), routes including *ExtensionRef* filters (which refer to implementation-specific resources, possibly enforcing security policies), or *ExternalAuth* filters whose backend cannot be translated, are not reflected at all.
The same applies to resources which would be meaningless in the remote cluster, such as routes not attached to any parent, or *Gateways* without listeners.

The **status** of the reflected *Gateways* and routes is reported back to the local cluster.
Since the same resource may be reflected to multiple remote clusters, each virtual kubelet reports the status of the resources in its remote cluster through *ShadowGatewayStatus* and *ShadowRouteStatus* custom resources, which are then aggregated by the Liqo controller manager:

* The virtual *GatewayClass* is marked as *Accepted*, as Liqo acts as its controller.
* The *Gateways* of the virtual class report the union of the **addresses** assigned in all remote clusters (e.g., to configure a DNS record pointing to all of them), and they are *Programmed* if programmed in at least one remote cluster.
  Warning: a *Programmed* *Gateway* guarantees that it is correctly reflected in _at least_ one remote cluster, not that it has been correctly reflected in _all_ remote clusters.
  The listeners report the total number of routes attached in all remote clusters.
* Routes report one entry for each parent reflected, managed by the `liqo.io/gateway-controller` controller, whose conditions are satisfied if satisfied in **at least one** remote cluster, and whose messages identify the clusters where they are not.
  Similarly, an *Accepted* route guarantees that it is accepted in _at least_ one remote cluster, not in _all_ of them: check the condition message (or the *PartiallyAccepted* events) to identify the clusters where it is not.
  The entries managed by other controllers (e.g., the local one serving the *Gateways* of other classes) are preserved.

Both *Gateways* and routes are hence considered working as soon as they serve traffic in at least one remote cluster, consistently with the addresses of the *Gateways*, which are the union of those assigned in all clusters (each of them can be used to reach the offloaded workloads).
A failure in a single remote cluster (e.g., a broken peering, or a cluster not supporting the route) does not mark the whole route as not *Accepted*: the condition message lists the clusters where it is not accepted, together with the reason reported by each of them, and a *PartiallyAccepted* warning event is generated on the route whenever such clusters change.

If a resource cannot be reflected to a remote cluster, the failure is also reported in its status, with the `ReflectionFailed` reason: routes are not *Accepted* with respect to the parents they would be attached to, and *Gateways* are not *Programmed* in that cluster.
This happens, for instance, if the resource is rejected by the remote API server (e.g., since the remote cluster runs a different version of the Gateway API with different validation rules), if it cannot be reflected without altering its semantic, or if a resource with the same name, not managed by Liqo, already exists in the remote namespace.
The message identifies the remote cluster and includes the error, so that the failure remains visible after the corresponding events expire.

For example, consider a namespace offloaded to `cluster-a` and `cluster-b`, where only `cluster-a` rejects a route (e.g., due to a stricter validation rule), and a *Gateway* with the same name as the reflected one already exists in `cluster-a`:

```yaml
# Route: accepted in cluster-b, with the reason why it is not accepted in cluster-a. If not accepted in any cluster,
# the condition is False, and the reason is the one of the first cluster where the condition is not satisfied.
status:
  parents:
  - controllerName: liqo.io/gateway-controller
    parentRef:
      name: web
    conditions:
    - type: Accepted
      status: "True"
      reason: Accepted
      message: 'Condition satisfied in cluster(s) cluster-b; not satisfied in cluster "cluster-a": reflection failed: ... retry.codes: duplicate entries for key [=500]'
---
# Gateway: programmed in cluster-b, with the reason why it is not programmed in cluster-a.
status:
  conditions:
  - type: Programmed
    status: "True"
    reason: Programmed
    message: 'Programmed in cluster(s) cluster-b (not programmed in cluster "cluster-a": an object with the same name, not managed by Liqo, already exists)'
```

Liqo generates Kubernetes events on the local resources to notify about the outcome of the reflection, including the ID of the remote cluster and the name of the virtual node, to simplify troubleshooting when a namespace is offloaded to multiple clusters.
In particular, a *PartialReflection* warning event lists the references dropped during the translation, while a *FailedReflection* warning event reports why the resource could not be reflected.

```{warning}
The Gateway API resources available in both clusters are detected when the virtual kubelet starts.
If a resource is available in the local cluster only, or the Liqo version installed in the remote cluster does not allow the virtual kubelet to operate on it, the corresponding local resources are not reflected, and a *FailedReflection* warning event is generated for each of them.
The virtual kubelet must be restarted in case the Gateway API CRDs are installed in either cluster afterwards, as well as the Liqo controller manager and webhook of the cluster where they are installed.
```

(UsageReflectionStorage)=

## Persistent storage

The reflection of **PersistentVolumeClaims (PVCs)** and **PersistentVolumes (PVs)** is a key to enable the cross-cluster [Liqo storage fabric](/features/storage-fabric).
Specifically, the process is triggered when a PVC requiring the *Liqo storage class* is bound for the first time, and the requesting pod is scheduled in a virtual node (i.e., remote cluster).
Upon this event, the **PVC is propagated verbatim** to the remote cluster, replacing the requested *StorageClass* with the one negotiated during the peering process.

Once created, the **resulting PV is reflected backwards** (i.e., from the remote to the local cluster), and the proper **affinity selectors** are added to **bind it to the virtual node**.
Hence, subsequent pods mounting that *PV* will be scheduled on that virtual node, and eventually offloaded to the same remote cluster.

When offloading a pod to a remote cluster, Liqo allows to override the **StorageClass** and **AccessModes** used for the remote PVC.
This can be achieved by adding the following annotations to the local PVC:

* `liqo.io/remote-storage-class`: overrides the StorageClass of the remote PVC. If not set, the StorageClass configured in the Liqo storage class is used.
* `liqo.io/remote-access-modes`: overrides the AccessModes of the remote PVC. The value must be a comma-separated list of Kubernetes access modes (e.g., `ReadWriteOnce` or `ReadWriteOnce,ReadOnlyMany`). If not set, the AccessModes of the local PVC are used.

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

It is also possible to enforce a remote RuntimeClass to all pods scheduled on a virtual node, by specifying it in the *OffloadingPatch* of the virtualnode (`.spec.offloadingPatch.runtimeClassName`).
If you are using liqoctl to create the virtual node, you can leverage the `--runtime-class-name` flag.  

If these options are used in combination, the following priority (from higher to lower priority) will be used to determine the remote RuntimeClass:

1. pod Annotation (`liqo.io/remote-runtime-class-name`).
2. pod RuntimeClass (`.spec.runtimeClassName`). It is ignored if set to `liqo`.
3. virtualNode OffloadingPatch (`.spec.offloadingPatch.runtimeClassName`).
