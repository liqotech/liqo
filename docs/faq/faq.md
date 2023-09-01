# FAQ

This section contains the answers to the most frequently asked questions by the community (Slack, GitHub, etc.).

## Table of contents

* [General](FAQGeneralSection)
  * [Cluster limits](FAQClusterLimits)
  * [Why DaemonSets pods (e.g., Kube-Proxy, CNI pods) scheduled on Virtual Nodes are in OffloadingBackOff?](FAQDaemonsetBackOff)
* [Installation](FAQInstallationSection)
  * [Upgrade the Liqo version installed on a cluster](FAQUpgradeLiqo)
  * [How to install Liqo on DigitalOcean](FAQInstallLiqoDO)
* [Peering](FAQPeeringSection)
  * [How to force unpeer a cluster?](FAQForceUnpeer)
  * [Is it possible to peer clusters using an ingress?](FAQPeerOverIngress)

(FAQGeneralSection)=

## General

(FAQClusterLimits)=

### Cluster limits

The official Kubernetes documentation presents some [general best practices and considerations for large clusters](https://kubernetes.io/docs/setup/best-practices/cluster-large/), defining some cluster limits.
Since Liqo still relies on Kubernetes, most of its limits are still present in Liqo.
Hence, we do not enforce any limitations in Liqo, it's just something that comes from general experience in using Kubernetes.
Some limitations of K8s, though, do not apply.
For instance, the limitation of 110 pods per node is not enforced on Liqo virtual nodes, as they abstract an entire remote cluster and not a single node.
The same consideration applies to the maximum number of nodes (5000) since all the remote nodes are hidden by a single virtual node.
You can find additional information [here](https://github.com/liqotech/liqo/issues/1863).

(FAQDaemonsetBackOff)=

### Why DaemonSets pods (e.g., Kube-Proxy, CNI pods) scheduled on virtual nodes are in OffloadingBackOff?

The majority of DaemonsSets pods (e.g., Kube-Proxy, CNI pods) that are scheduled on Liqo virtual nodes will result in the **OffloadingBackOff** state.
This generally does not represent a problem.
It is recommended to prevent the scheduling of a DaemonSet pod on a virtual node if needed, using the following *NodeAffinity*:

```yaml
nodeAffinity:
  requiredDuringSchedulingIgnoredDuringExecution:
    nodeSelectorTerms:
    - matchExpressions:
      - key: liqo.io/type
        operator: DoesNotExist
```

(FAQInstallationSection)=

## Installation

(FAQUpgradeLiqo)=

### Upgrade the Liqo version installed on a cluster

Unfortunately, this feature is not currently fully supported.
At the moment, upgrading through `liqoctl install` or `helm update` will update manifests and Docker images (excluding the *virtual-kubelet* one as it is created dynamically by the *controller-manager*), but it will not update any CRD-related changes (see this [issue](https://github.com/liqotech/liqo/issues/1831) for further details).
The easiest way is to unpeer all existing clusters and then uninstall and reinstall Liqo on all clusters (make sure to have the same Liqo version on all peered clusters).

(FAQInstallLiqoDO)=

### How to install Liqo on DigitalOcean

The installation of Liqo on a Digital Ocean's cluster does not work out of the box.
The problem is related to the `liqo-gateway` service and DigitalOcean load balancer health check (which does not support a health check based on UDP).
This [issue](https://github.com/liqotech/liqo/issues/1668) presents a step-by-step solution to overcome this problem.

(FAQPeeringSection)=

## Peering

(FAQForceUnpeer)=

### How to force unpeer a cluster?

It is highly recommended to first unpeer all existing foreignclusters before upgrading/uninstalling Liqo.
If using `liqoctl unpeer` command does not fix the problem (probably due to some cluster setup misconfiguration), you can try to manually unpeer the cluster by force deleting all Liqo resources associated with that ForeignCluster.
To do this, force delete all resources (look also in the tenant namespace) with the following types (possibly in this order):

* `TunnelEndpoint`
* `NetworkConfig`
* `ResourceOffers`
* `ResourceRequests`
* `NamespaceMap`

Make sure to also manually remove possible finalizers.
At this point, you should be able to delete the ForeignCluster.
If there are no peerings left you can uninstall Liqo if needed.

```{warning}
This is a not recommended solution, use this only as a last resort if no other viable options are possible. 
Future upgrades will make it easier to unpeer a cluster or uninstall Liqo. 
```

(FAQPeerOverIngress)=

### Is it possible to peer clusters using an ingress?

It is possible to use an ingress to expose the `liqo-auth` service instead of a NodePort/LoadBalancer using Helm values.
Make sure to set `auth.ingress.enable` to `true` and configure the rest of the values in `auth.ingress` according to your needs.

```{admonition} Note
The `liqo-gateway` service can't be exposed through a common ingress (proxies like nginx which works with HTTP only) because it uses UDP.
```
