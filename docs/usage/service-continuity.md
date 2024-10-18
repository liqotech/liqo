# Service Continuity

This section provides additional details regarding service continuity in Liqo.
It reports the main architectural design choices and the options to better handle eventual losses of components of the multi-cluster (e.g., control plane, nodes, network, liqo pods, etc.).

For simplicity, we consider a simple consumer-provider setup, where the consumer/local cluster offloads an application to a provider/remote cluster.
Since a single peering is unidirectional and between two clusters, all the following considerations can be extended to more complex setups involving bidirectional peerings and/or multiple clusters.

(ServiceContinuityHA)=

## High-availability Liqo components

Liqo allows to deploy the most critical Liqo components in high availability.
This is achieved by deploying multiple replicas of the same component in an **active/passive** fashion.
This ensures that, even after eventual pod restarts or node failures, exactly one replica is always active while the remaining ones run on standby.

The supported components (pods) in high availability are:

- ***liqo-controller-manager*** (active-passive): ensures the Liqo control plane logic is always enforced. The number of replicas is configurable through the Helm value `controllerManager.replicas`
- ***wireguard gateway server and client*** (active-passive): ensures no cross-cluster connectivity downtime. The number of replicas is configurable through the Helm value `networking.gatewayTemplates.replicas`
- ***webhook*** (active-passive): ensures the enforcement of Liqo resources is responsive, as at least one liqo webhook pod is always active and reachable from its Service. The number of replicas is configurable through the Helm value `webhook.replicas`
- ***virtual-kubelet*** (active-passive): improves VirtualNodes responsiveness when the leading virtual-kubelet has some failures or is restarted. The number of replicas is configurable through the Helm value `virtualKubelet.replicas`
- ***ipam*** (active-passive): ensures IPs and Networks management is always up and responsive. The number of replicas is configurable through the Helm value `ipam.internal.replicas`

## Resilience to cluster failures/unavailability

Liqo performs periodic checks to ensure the availability and readiness of all peered clusters.
In particular, for every peered cluster, it checks for:

- readiness of the foreign cluster's **API server**
- availability of the **VPN tunnel** for cross-cluster connectivity (*liqo-gateway*)

The **ForeignCluster** CR contains the status conditions indicating the current status of the above checks (named respectively *APIServerStatus* and *NetworkStatus*).
A peered cluster is considered ready/healthy if **all** the above checks are successful.

### Remote cluster failure

In this scenario the remote cluster is unavailable/unhealthy.
Following the standard K8s protocol, the virtual node is marked as *NotReady* after `node-monitor-grace-period` seconds (default: 40s).
This allows the control plane of the local cluster (which has visibility on all pods in a Liqo-enabled namespace, i.e. both local and remote pods) to mark all endpointslices associated to remote pods as not ready, preventing services to redirect traffic towards them in the same way services will not backend standards Kubernetes nodes.
Also, new pods are scheduled on the remaining local nodes.
As the virtual node transparently implements the standard Kubernetes interface, service continuity in the local cluster is guaranteed by Kubernetes in the event of unavailability of the remote cluster.
Look at the [official guide](https://kubernetes.io/docs/concepts/services-networking/endpoint-slices/#conditions) for further details.

### Local cluster failure

In this scenario the local cluster is unavailable/unhealthy.
Since the virtual node is not present on the remote cluster, Liqo logic ensures service continuity.

#### Remote pod resiliency

Remote pod resiliency (hence, service continuity) is ensured, even in case of temporary connectivity loss between the two control planes, through a custom resource (i.e., **ShadowPod**) wrapping the pod definition, and triggering a Liqo enforcement logic running in the remote cluster.
This guarantees that the desired pod is always present, without requiring the intervention of the originating cluster.
The virtual kubelet takes care of the automatic propagation of remote status changes to the corresponding local pod (remapping the appropriate information).

```{figure} /_static/images/usage/service-continuity/shadowpod.drawio.svg
---
align: center
---
Schematic representation of the pod offloading workflow. 
Solid lines refer to liqo-related tasks, while dashed ones to standard Kubernetes logic.
Double circles indicate the pod in execution (i.e., whose containers are running).
Blue rectangles refers to liqo-related resources.
```

#### Remote endpointslices

The endpointslices of all **local** pods must be disabled to prevent services to redirect traffic towards pods running on the local cluster to ensure service continuity on the remote cluster when the local cluster has a failure.
Note that since the control plane of each remote cluster perceives only the pods running in its cluster, the Liqo EndpointSlice reflection fills the gaps by creating the necessary endpointslices associated with local pods (as explained [here](UsageReflectionEndpointSlices)).

Liqo provides a more robust mechanism that offers better resiliency to cluster failures since version v0.8.2.
It introduces an intermediate resource, the **ShadowEndpointSlice** CR, similar to the one adopted for the pods (i.e., ShadowPod).
In this case, it is an abstraction that serves as a template for the desired configuration of the **remote** endpointslice.
The virtual kubelet forges the remote shadow resource of a reflected endpointslice and creates it on the remote cluster.
A controller in Liqo runs in the remote cluster and enforces the presence of the actual endpointslice, using the shadow resource as a source of truth.
At the same time, it periodically checks the **local cluster status** (monitoring the above-described conditions) and dynamically updates the *Ready* condition of the endpoints in the endpointslices, depending on the cluster status.
More specifically, endpoints are set ready only if both the VPN tunnel and the API server of the foreign cluster are ready.
Note that a remote endpoint is updated only when the local endpointslice (and therefore the shadowendpointslice) has the *Ready* condition set to *True* or unspecified.
If it is set to *False*, the remote endpoint condition is set to *False* regardless of the current status of the foreign cluster to preserve the local cluster's desired behavior.

```{figure} /_static/images/usage/service-continuity/shadoweps.drawio.svg
---
align: center
---
Schematic representation of the endpointslice reflection workflow. 
Solid lines refer to liqo-related tasks, while dashed ones to standard Kubernetes logic.
Blue rectangles refer to liqo-related resources.
```

In summary, Liqo ensures that when a peered cluster is unavailable the endpoints of the local pods are temporarily disabled, and re-enabled when the cluster becomes ready again (if not explicitly disabled by the originating cluster).

## Resilience to worker nodes failures

This section describes scenarios where one or more worker nodes are unavailable/unhealthy, with all control planes ready and the cross-cluster network up and running.

### Worker node failure on the local cluster

Pods running on the local cluster are scheduled on regular worker nodes and therefore their entire lifecycle is handled by Kubernetes as explained in the [official guide](https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle/).

### Worker node failure on the remote cluster

Offloaded pods are scheduled on the virtual node in the local cluster and run on regular worker nodes in the remote cluster.
As explained in the [pod offloading section](FeaturePodOffloading) the ShadowPod abstraction guarantees remote pod resiliency (hence, service continuity) in case of unavailability of the local cluster, enforcing the presence of the desired pod (scheduled on a regular worker node) without requiring the intervention of the originating cluster.

If a remote worker node becomes *NotReady* the Kubernetes control plane marks all pods scheduled on that node for deletion, leaving them in a *Terminating* state **indefinitely** (until the node becomes ready again or a manual eviction is performed).
Due to design choices in Liqo, a pod that is (1) offloaded, (2) *Terminating*, (3) running on a failed node is **not** replaced by a new one on a healthy worker node (like in vanilla Kubernetes).
The consequence is that in case of remote worker node failure, the expected workload (i.e., the number of replicas actively running) of a deployment could be less than expected.

Since Liqo v0.7.0, it is possible to overcome this issue. You can configure Liqo to make sure the expected workload is always running on the remote cluster, setting the Helm value `controllerManager.config.enableNodeFailureController=true` at install/upgrade time.
This flag enables a custom Liqo controller that checks for all offloaded and *Terminating* pods running on *NotReady* nodes.
A pod matching all conditions is force-deleted by the controller.
This way, the ShadowPod controller will enforce the presence of the remote pod by creating a new one on a healthy remote worker node, therefore ensuring the expected number of replicas is actively running on the remote cluster.

As explained in the [pod reflection section](UsageReflectionPods), the local cluster has the feedback on what is happening on the remote cluster because the remote pod status is propagated to the local pod and the number of **container restarts** is augmented to account for possible deletions of the remote pod (e.g., the Liqo controller force-deletes the *Terminating* pod on the failed node).

```{warning}
Enabling the controller can have some minor drawbacks: when the pod is force-deleted, the resource is removed from the K8s API server.
This means that in the (rare) case that the failed node becomes ready again and without an OS restart, the containers in the pod will not be gracefully deleted by the API server because the entry is not in the database anymore.
The side effect is that zombie processes associated with the pod will remain in the node until the next OS restart or manual cleanup.
```
