---
title: Pod offloading
weight: 3
---

Differently from a traditional kubelet, which starts the containers of a Pod on the designated Node, the Liqo virtual kubelet maps each operation to a corresponding *twin* Pod object in the remote cluster for actual execution.
At the same time, it takes care of the automatic propagation of *remote status changes* to the corresponding local Pod, allowing for complete observability from the local cluster.
Finally, advanced operations, such as metrics and logs retrieval, as well as interactive command execution inside remote containers, are transparently supported, to comply with standard troubleshooting operations.

### Remote Pod resiliency

Due to the permanent link between the two twin Pod representations (in the local and in the remote cluster), different behaviors can occur when deleting an offloaded Pod:

- The Pod is deleted in the **local cluster** (intentionally or by eviction): the remote one has to be immediately deleted. In fact, the local cluster is the owner of the pod, therefore whatever modification to the Pod is set, this has to be reflected in the remote Pod.
- The Pod is deleted in the **remote cluster** (intentionally or by eviction): not only the local Pod must not be deleted, but the remote Pod has to be recreated as soon as possible. This recovery process shall not require the local cluster intervention, to reduce the overhead and ensure service continuity even in case of temporary connectivity loss between the two control planes.

Accounting for the desired Pod resiliency, the virtual kubelet does not directly create a vanilla Pod in the remote cluster.
Differently, it proceeds with the remote creation of a *ShadowPod*, a custom resource wrapping the Pod definition and triggering the custom enforcement logic running in the remote cluster.
Ultimately, this leads to the generation of a standard Pod, while transparently ensuring it is recreated in case of deletion, independently of the connectivity with the originating cluster.

Summarizing, all local Pod operations (i.e., creations, updates and deletions) are translated by the virtual kubelet to corresponding ones on remote ShadowPods, while automatic remapping is performed to propagate Pod status updates in the local cluster.

### Pod offloading workflow

The complete Pod offloading workflow, which involves a two-steps scheduling process, is detailed in the following:

1. A user requests the execution of a new Pod, either directly or through higher level abstractions (e.g., *Deployments*).
2. The vanilla Kubernetes scheduler in the local cluster binds the Pod to a virtual node (first scheduling step).
3. The corresponding virtual kubelet takes charge of it, creating the twin ShadowPod in the remote cluster.
4. The remote enforcement logic observes the ShadowPod creation and generates the corresponding Pod object.
5. The vanilla Kubernetes scheduler in the remote cluster binds the Pod to a node for execution (second scheduling step). Although in most scenarios this second node is a physical one, nothing prohibits it from being yet another virtual node, thus incurring in an additional indirection level.
6. The virtual kubelet constantly keeps the local Pod status aligned with the remote one, hence tracking its lifecycle. During this process, remapping operations are automatically performed whenever necessary (e.g., to translate the [Pod IP field](/concepts/networking/components/network-manager/#ip-addresses-translation-of-offloaded-pods) for conflict resolution).

![A schematic representation of the Pod offloading workflow](/images/offloading/pod-offloading.svg)
