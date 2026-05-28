# Resources sharing and reservation

Liqo enables a cluster, named *consumer*, to acquire resources from another remote cluster, named *provider*.
This happens through an explicitly negotiated **slice** of the provider cluster: the `ResourceSlice` resource carries the request, the `Quota` resource enforces it on the provider, and the `VirtualNode` resource exposes it on the consumer.

However, some questions arise, such as *how many resources are desired* (by the consumer), *how resources are reserved* (by the provider), *how can we guarantee that the consumer does not consume more resources than planned*, and more.

This section of the documentation presents a detailed view about how resource reservation is handled with Liqo, and the possible limitations of the current approach.
Particularly, it describes how that slice is reserved end-to-end, from both the *provider* and the *consumer* point of view: how to configure the default slice size, how to request a specific amount, how strictly the provider enforces it at runtime, and how to suspend or reclaim a reservation that is no longer needed.

## Overview

A reservation in Liqo is the result of an exchange between two clusters:

* The **consumer** declares how much it would like to obtain by creating a `ResourceSlice` in its tenant namespace, with the requested quantities in `spec.resources`.
* The **provider** decides how much it actually grants by writing the accepted quantities back into `status.resources` of the same object (the slice is replicated across clusters by the *crd-replicator*).

Once the slice is `Accepted`, two derived resources materialize the reservation:

* On the **provider**, a `Quota` (`offloading.liqo.io/Quota`) is created in the tenant namespace, mirroring the accepted resources and the desired enforcement strictness.
* On the **consumer**, a `VirtualNode` is created with `spec.resourceQuota.hard` set to the same quantities, and the corresponding Kubernetes `Node` advertises that capacity to the local scheduler.

By default, a single `ResourceSlice` is shared across all of the consumer's [offloaded namespaces](/usage/namespace-offloading.md): pods consume from the same `Quota` regardless of which offloaded namespace they live in.
When multiple `VirtualNode`s are created (one per `ResourceSlice`), pods consume the quota based on the `VirtualNode` they are scheduled on, not by namespace.

The diagram below summarizes the flow and makes explicit on which cluster each object is created.

```{figure} /_static/images/usage/resource-reservation/sequence.svg
:align: center
:alt: Sequence diagram of the ResourceSlice to Quota and VirtualNode flow across the consumer and provider clusters.
```

From that point on, the reservation has two enforcement points:

* the **consumer's Kubernetes scheduler**, which sees the Liqo virtual `Node` as a node with finite capacity;
* the **provider's `ShadowPod` admission webhook**, which rejects offloaded pods that would push the consumer past its `Quota`.
This double mechanism prevents *consumers* from using more resources than what was negotiated with the provider cluster.

```{warning}
To make sure the consumer cluster does not exceed the quota of shared resources, the offloaded pods need to be created with the resources `limits` set.

Check the [Resource quota enforcement](#resource-quota-enforcement) section to know more about how to configure Liqo to enforce the quota of resources that the provider has granted to the consumer cluster.
```

(ResourceReservationDefaultSlice)=

## Defining the amount of resources to share

By default, the `liqoctl peer` command requests the provider a default amount of resources, defined at installation time on the provider, which are:

* cpu: "4"
* memory: "8Gi"
* pods: "110"
* ephemeral-storage: "20Gi"

These defaults only apply to resource keys the consumer did not explicitly request.
For example, if a consumer asks for `cpu: "8"` and nothing else, the resulting `Quota` will have `cpu: "8"` from the request and `memory: "8Gi"`, `pods: "110"`, `ephemeral-storage: "20Gi"` from the defaults.

```{admonition} Note
Setting `offloading.defaultNodeResources` to a small value is the simplest way to limit how much capacity an unconfigured peering can claim: a consumer that does not explicitly negotiate larger amounts will always receive the defaults.
```

These defaults are configured through the Helm value `offloading.defaultNodeResources`:

```{code-block} yaml
:caption: "values.yaml (provider)"
offloading:
  defaultNodeResources:
    cpu: "4"
    memory: "8Gi"
    pods: "110"
    ephemeral-storage: "20Gi"
```

The default resources can be customized during installation or later with an upgrade, by setting the `offloading.defaultNodeResources` value. This can be done using either `liqoctl install` or Helm (`helm install`/`helm upgrade`). For more details, see the [customization options](/installation/install.md#customization-options) page.

```{code-block} bash
:caption: "Cluster provider"
liqoctl install [...ARGS] \
  --set offloading.defaultNodeResources.cpu="2" \
  --set offloading.defaultNodeResources.memory="4Gi"
```

### Overriding the defaults at peering time

To define different values for a specific peering, the consumer can provide the `--cpu`, `--memory`, `--pods`, and `--resource` arguments to the `liqoctl peer` command:

```{code-block} bash
:caption: "Cluster consumer"
liqoctl peer \
  --kubeconfig=$CONSUMER_KUBECONFIG_PATH \
  --remote-kubeconfig=$PROVIDER_KUBECONFIG_PATH \
  --cpu=8 \
  --memory=16Gi \
  --pods=200 \
  --resource=nvidia.com/gpu=2
```

Any resource key supported by the Kubernetes `ResourceList` can be requested through the `--resource` command line parameter (repeatable), including extended resources advertised by the provider (e.g. `nvidia.com/gpu`).

## Request a slice after peering

After a peering has been established, a consumer can change the amount of resources granted by either editing an existing `ResourceSlice` or creating a new one.

### By editing an existing `ResourceSlice`

The amount of resources granted by an existing slice can be increased or decreased by updating its `spec.resources`.
The provider re-evaluates the request, the `Quota` is updated to match `status.resources`, and the `VirtualNode` reflects the new capacity.
If the provider rejects the new request — only possible with a [custom class controller](#custom-resource-allocation-policies), since the default class always accepts — the `ResourceSlice`'s `Resources` condition transitions to `Denied`, the existing `Quota` is left unchanged, and the `VirtualNode` continues to expose the previously granted capacity.

```{warning}
Reducing the granted resources below what the consumer is currently using will not evict pods immediately: existing offloaded pods continue to run, but new pods that would exceed the new `Quota` will be denied at admission.
For an immediate effect, see the [suspending and reclaiming a reservation](#suspend-and-reclaim-a-reservation) section below.
```

### After peering, with a separate `ResourceSlice`

When the peering already exists, additional capacity can be requested by creating a new `ResourceSlice`.
This is also the right path when you want a separate slice and a separate `VirtualNode` for a distinct pool of resources — for example, a dedicated GPU pool kept apart from the main CPU/memory slice — rather than enlarging the original slice:

`````{tab-set}

````{tab-item} liqoctl

```{code-block} bash
:caption: "Cluster consumer"
liqoctl create resourceslice gpu-pool \
  --remote-cluster-id cool-firefly \
  --cpu 4 --memory 8Gi --pods 30 \
  --resource nvidia.com/gpu=2
```
````

````{tab-item} YAML

```{code-block} yaml
:caption: "Cluster consumer"
apiVersion: authentication.liqo.io/v1beta1
kind: ResourceSlice
metadata:
  name: gpu-pool
  namespace: liqo-tenant-cool-firefly
  labels:
    liqo.io/remote-cluster-id: cool-firefly
    liqo.io/replication: "true"
spec:
  class: default
  providerClusterID: cool-firefly
  resources:
    cpu: "4"
    memory: 8Gi
    pods: "30"
    nvidia.com/gpu: "2"
```
````

`````

Each `ResourceSlice` is associated with one `Quota` on the provider, and with one or more `VirtualNodes` on the consumer (one by default); multiple slices toward the same provider originate multiple virtual nodes, which can be also useful to expose heterogeneous resources (for example, separate ARM and x86 pools — see the [multiple virtual nodes](/advanced/peering/offloading-in-depth.md#multiple-virtualnodes) section).

## Inspect the reservation

On the **consumer**, the resulting Liqo virtual `Node` exposes the granted capacity to the local scheduler as a regular node.
`liqoctl info peer` shows the info about the shared resource and of the peering status:

```{code-block} bash
:caption: "Cluster consumer"
liqoctl info peer cool-firefly --get authentication.resourceslices
```

```text
─ Resource slices ──────────────────────────────────────────────
  ─ gpu-pool ──────────────────────────────────────────────────
    ✔  Resource slice accepted
    Action: Create
    ─ Resources ──────────────────────────────────────────────
      cpu:               4
      memory:            8Gi
      pods:              30
      nvidia.com/gpu:    2
```

```{admonition} Pro tip
For lower-level debugging, the underlying `ResourceSlice` (on the consumer) and `Quota` (on the provider) custom resources can also be inspected directly with `kubectl`.
```

On the **consumer**, the `ResourceSlice` shows what was requested and whether it was accepted:

```{code-block} bash
:caption: "Cluster consumer"
kubectl get resourceslices.authentication.liqo.io -A
```

```text
NAMESPACE                  NAME       AUTHENTICATION   RESOURCES   AGE
liqo-tenant-cool-firefly   gpu-pool   Accepted         Accepted    21s
```

On the **provider**, the `Quota` shows what is being enforced:

```{code-block} bash
:caption: "Cluster provider"
kubectl get quotas.offloading.liqo.io -A
```

```text
NAMESPACE                   NAME                    ENFORCEMENT   CORDONED   AGE
liqo-tenant-wispy-firefly   gpu-pool-c34af51dd912   None                     35s
```

## Resource quota enforcement

A `Quota` defines an upper bound, but whether the consumer can ever exceed it at runtime depends on two further options on the provider, both under `controllerManager.config`:

* `enableResourceEnforcement` (default `true`): turns on the `ShadowPod` validating webhook that rejects new offloaded pods if they would push the consumer past its `Quota`. However, this is not on its own a strict guarantee: if `limits` are higher than `requests` or if neither is set at all, the consumer can still consume more than the quota at runtime.
* `defaultLimitsEnforcement` (default `None`): controls how strictly the webhook interprets the `requests` and `limits` of each offloaded pod. It can take three values:

  * **`None`** (default): offloaded pods may have neither `requests` nor `limits` set, so the consumer might end up using more than the resources negotiated via the `ResourceSlice`.
  * **`Soft`**: offloaded pods must declare `requests`; pre-allocated resources stay within the quota, but if a pod uses more than its `requests` (because `limits` are higher or unset), actual usage can drift over.
  * **`Hard`**: offloaded pods must declare both `requests` and `limits`, with `limits == requests`. This is the strictest mode and the only one that turns the per-peering quota into a runtime guarantee, since the kubelet on the provider will throttle or kill containers that try to exceed their declared `limits`.

The selected mode is propagated to every `Quota` created on the provider via the `Quota.spec.limitsEnforcement` field. Both options can be set at installation time or updated later. For example, to set `defaultLimitsEnforcement` to `Hard`:

```{code-block} bash
:caption: "Cluster provider"
liqoctl install [...ARGS] --set controllerManager.config.defaultLimitsEnforcement=Hard
```

```{warning}
Switching to `Hard` enforcement after workloads are already being offloaded will cause new `ShadowPod` admissions to fail for any pod that does not declare matching `requests` and `limits`.
Verify that the workloads being offloaded already comply, or set sensible defaults via a mutating policy, before applying this change to a production provider.
```

## Custom resource allocation policies

By default, the `ResourceSlice` class controller shipped with Liqo accepts every incoming request and only fills in the missing keys with `offloading.defaultNodeResources`. This behavior can be customized by defining a **custom `ResourceSlice` class** to implement stricter or cluster-wide policies (for example, capping the total accepted slices at a fraction of cluster capacity, or per-tenant quotas).

A reusable starting point is provided by the [resource-slice-class-controller-template](https://github.com/liqotech/resource-slice-class-controller-template) repository, and the general mechanics of class controllers are described in the [Custom Resource Allocation](/advanced/peering/offloading-in-depth.md#custom-resource-allocation) section of the offloading-in-depth page.

```{warning}
The class is chosen by the **consumer**, and the built-in default class controller is always running alongside any custom one — Liqo does not provide a built-in way to disable it.
A consumer can therefore bypass a strict custom controller simply by selecting `class: default` (or omitting the class), which routes the request to the lenient built-in controller.
To actually enforce a stricter policy at the provider, the custom controller must be paired with an external mechanism that prevents the lenient path from being used — for example, a Kubernetes admission webhook on the provider that rejects or rewrites the `spec.class` of incoming `ResourceSlice` objects.
```

(ResourceReservationSuspendReclaim)=

## Suspend and reclaim a reservation

Liqo exposes two operations to act on an active reservation without unpeering: **cordon** (stop new allocations, keep existing ones) and **drain** (reject new allocations and revoke existing ones).
Both operations can be applied at the granularity of a single `ResourceSlice` or of an entire `Tenant` (which covers all slices of that consumer).

### Cordon a `ResourceSlice`

Cordoning a slice prevents the provider from accepting new resource requests on that slice while leaving existing offloaded workloads running:

```{code-block} bash
:caption: "Cluster provider"
liqoctl cordon resourceslice gpu-pool --remote-cluster-id cool-firefly
```

Internally, this adds the `liqo.io/cordoned-resource` annotation to the slice, which causes the `Quota` to be marked `cordoned: true`.
The `ShadowPod` admission webhook then rejects any new offloaded pod from that consumer.
The reverse operation is `liqoctl uncordon resourceslice`.

### Cordon a `Tenant`

Cordoning a tenant has the same effect as cordoning all of its slices, plus it stops the provider from accepting **new** `ResourceSlice` objects from that consumer:

```{code-block} bash
:caption: "Cluster provider"
liqoctl cordon tenant cool-firefly
```

This sets `spec.tenantCondition` to `Cordoned` on the provider's `Tenant` (`authentication.liqo.io/Tenant`) for that consumer.
The `RemoteResourceSliceReconciler` honors the condition by leaving previously-accepted slices untouched and denying new ones.

### Drain a `Tenant`

Draining is the strongest operation: it suspends new allocations *and* invalidates existing ones, so that the provider stops admitting offloaded pods for that consumer altogether:

```{code-block} bash
:caption: "Cluster provider"
liqoctl drain tenant cool-firefly
```

This sets `spec.tenantCondition` to `Drained` on the same `Tenant`; the `RemoteResourceSliceReconciler` then denies the resources of all existing slices.
The reverse operation is `liqoctl uncordon tenant`, which restores the tenant to `Active`.

The full reference for these commands is available on the [`liqoctl cordon`](/usage/liqoctl/liqoctl_cordon.md), [`liqoctl drain`](/usage/liqoctl/liqoctl_drain.md), and [`liqoctl uncordon`](/usage/liqoctl/liqoctl_uncordon.md) pages.

```{important}
Cordon and drain are administrative operations on the **provider**: they affect what the provider is willing to grant.
Cordoning a tenant on the provider does not unpeer the consumer nor remove its `VirtualNode`.
In other words, pods scheduled on the virtual node may still appear `Running` until they are evicted or the slice is fully released.
```

## Limitations

A few aspects of the current design are worth keeping in mind when designing a reservation policy:

* The default `ResourceSlice` class controller does not perform a **cluster-wide capacity check**: it accepts every request and may therefore grant more resources than the provider physically has, leaving the final arbitration to the standard Kubernetes scheduler on the provider. Cross-peering reservation requires a custom class controller **paired with an admission webhook (or equivalent mechanism) that prevents consumers from selecting the lenient default class** — Liqo does not ship that mechanism.
* Reducing the granted resources on a slice does not evict pods that are already running; cordon and drain are the supported way to actively reclaim capacity.

## What Liqo does not

Liqo provides the basic mechanisms to negotiate and reserve resources, but a full-functional service may require the cooperation between Liqo and external components that take care of some preconditions that are out of scope for Liqo.
This section lists some information and/or actions that may be required, and that must be carried out outside Liqo; skipping them may cause the reservation to fail or to complete with incorrect outcomes.

* **Discover what to ask for.**
  The consumer must learn from the provider (out of band) which amounts and resource types are actually available before creating a `ResourceSlice`.
  Liqo has no inventory advertisement and the default class (on the provider side) accepts any request, so an improper reservation is automatically accepted by Liqo, and only manifests later with pods that may not be able to start (hence, in `Pending` state) on the provider cluster.

* **Exchange cluster identities and peering credentials.**
  Cluster IDs and a peering kubeconfig (typically generated on the provider with `liqoctl generate peering-user`) must be exchanged through a secure channel before `liqoctl peer` runs.
  Liqo provides no directory or discovery service across clusters.

* **Authorize the peering relationship.**
  Someone with organizational authority must decide that the consumer is allowed to peer at all.
  Liqo's handshake verifies *who* a peer is, not *whether they should be admitted*; there is no built-in approval workflow or admission queue at the organization level. A custom `ResourceSlice` class controller can implement such a workflow by accepting or denying slices based on organizational policy.

* **Choose the provider's reservation posture.**
  The provider operator must set the Helm chart values `offloading.defaultNodeResources` and `controllerManager.config.defaultLimitsEnforcement` before any consumer peers.
  Both can be updated later via a Helm upgrade: `defaultLimitsEnforcement` then propagates to existing `Quota` objects on the next reconciliation, while `defaultNodeResources` only affects future slice negotiations — slices already accepted retain the amounts they were granted.
  The chart ships defaults suited to demos (4 CPU, 8Gi, `None` enforcement); using them unchanged in production typically grants more than the operator intends and enforces nothing at runtime.

* **Agree on what each resource key physically represents.**
  Kubernetes resource keys (`cpu`, `memory`, `nvidia.com/gpu`, ...) are opaque strings — a limitation of how resource allocation worked in Kubernetes prior to Dynamic Resource Allocation (DRA), which Liqo inherits. What `nvidia.com/gpu=2` actually means — A100s versus H100s, full cards versus MIG slices, with or without NVLink — must be agreed out of band.
  Such a mismatch — for example, the consumer asking for `nvidia.com/gpu=2` expecting two H100 cards while the provider grants two A100 cards under the same key — is accepted at negotiation and only surfaces as wrong performance or runtime failure once workloads are scheduled.

* **Ensure offloaded pods comply with the enforcement mode.**
  When `defaultLimitsEnforcement` is `Soft` or `Hard`, the consumer's pods must declare `requests` (Soft) or matching `requests` and `limits` (Hard) before being offloaded (see [Resource quota enforcement](#resource-quota-enforcement) for the per-mode semantics).
  The `ShadowPod` admission webhook rejects non-compliant pods at the provider, leaving the local pod in `Pending` state; making pod specs compliant is the consumer application teams' responsibility, not Liqo's.

* **Provision the capacity advertised by the provider.**
  The provider must actually have the physical or cloud capacity to honor what `offloading.defaultNodeResources` and granted `ResourceSlice` objects promise.
  The default class controller does not verify against real capacity, so over-promising surfaces only at runtime with pods resulting in a `Pending` state; cluster sizing — whether manual, via `cluster-autoscaler` or Karpenter, or through cloud quota requests — is the provider operator's job.

