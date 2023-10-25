# Security Modes

## Overview

This section describes the different options in terms of **connectivity security modes** available in Liqo, which can be used to impose some restrictions to inter-cluster and intra-cluster traffic.

To better explain the concept, pictures and tables will use the following terminology:

* **Cx**: physical Cluster (e.g. Cluster 1).
* **Px**: Pod (e.g., Pod 1).
* **Sx**: Service (e.g., a Cluster IP service), pointing to one or more pods. Given the nature of Liqo, the service endpoints associated to this service can be pods in either clusters.
* **Ix**: Internet connectivity. For instance, I1 is the Internet connectivity provided by Cluster 1.
* **Virtual cluster**: refers to the combination of a physical cluster along with all its extensions in other clusters, which are created with Liqo. A virtual cluster includes all the physical resources of the local cluster, and all the resources it borrows from the remote cluster.
* **Offloaded namespace**: namespace that is expanded across the virtual cluster, enabling the seamless utilization of resources beyond the physical cluster boundaries. An offloaded namespace does not include all the resources of the local cluster, but only the resources belonging to the namespace that has been offloaded (in both local and remote cluster).

Furthermore, yellow cells in the connectivity matrices highlight the differences compared to the default behaviour (i.e., _full pod-to-pod_ connectivity).

## Security modes

Liqo provides two security modes:

* full pod-to-pod connectivity (default mode)
* intra-cluster traffic segregation

### Full pod-to-pod connectivity (default mode)

This mode does not enforce any restriction in terms of connectivity, and all pods of the local cluster can connect to all pods of the remote cluster (and vice versa), for all namespaces including non-offloaded ones.
This mode can be useful when the clusters trust each other (and/or belong to the same administrative domain) and there is no need to restrict the connectivity.
This may happen when both clusters are under control of the same organization, and Liqo is used simply to "cross the boundaries" among distinct clusters (e.g., because clusters are running in different geographical regions), providing a single point of control for many geographically distributed clusters.

In the following picture and table, there will be five pods with different characteristics:

* **P1**: pod in cluster C1, not belonging to the offloaded namespace.
* **P2**: pod in cluster C1, belonging to the offloaded namespace and used as target by the service S2.
* **P3**: pod in cluster C1, belonging to the offloaded namespace and not used as target by any service.
* **P4**: pod in cluster C2, belonging to the offloaded namespace; this pod has been offloaded by cluster C1.
* **P5**: pod in cluster C2, not belonging to the offloaded namespace; this pod has no relationships with cluster C1 and it is in full control from cluster C2.

In addition, three services (e.g., ClusterIP) are present, with different characteristics:

* **S1**: service under the control of cluster C1, not belonging to the offloaded namespace.
* **S2**: service under the control of cluster C1, belonging to the offloaded namespace (hence reflected also in C2).
* **S3**: service under the control of cluster C2, not belonging to the offloaded namespace.

Considering two clusters (C1 and C2) in which the former has started a peering toward the latter, and C1 is using some physical resources on the remote cluster C2, the following picture and table show the connectivity provided by Liqo to each pod when trying to connect to resources (other pods, services, and Internet connectivity) present in either one of the two clusters.

```{figure} /_static/images/usage/security-modes/security-modes-schema.drawio.svg
---
align: center
class: mb
---

```

```{figure} /_static/images/usage/security-modes/matrix-full-p2p.drawio.svg
---
align: center
class: mb
---

```

### Intra-cluster traffic segregation

This mode allows the full pod-to-pod connectivity only within each _physical_ cluster, e.g., all pods in the local cluster can talk to each other, and the same holds for all pods in the remote cluster irrespective of the namespace they belong to or whether they are offloaded or not.
With respect to the intra-cluster traffic, the local cluster can contact only its offloaded pods, while the remote cluster is allowed to contact only the _services_ offloaded on it.
This mode allows the full pod-to-pod connectivity only within each _physical_ cluster, e.g., all pods in the local cluster can talk to each other, and the same holds for all pods in the remote cluster irrespective of the namespace they belong to or whether they are offloaded or not.
In addition, the remote cluster is also allowed to contact the _services_ offloaded on it (e.g., P5 --> S2), and the local cluster can contact all its offloaded pods (e.g., P1 --> P4, but not vice versa).
This mode can be used when the local cluster wants to extend its full pod-to-pod connectivity with pods and services in the offloaded namespace running in the remote cluster, while it wants to limit the remote cluster's connectivity solely to offloaded services and related backend pods, preventing interaction with local pods that are not engaged in the multi-cluster topology.

Using the same rules and conventions already presented for the previous case (_full pod-to-pod_ connectivity), the following figure and table shows the connectivity provided by Liqo in this case:

```{figure} /_static/images/usage/security-modes/security-modes-schema.drawio.svg
---
align: center
class: mb
---

```

```{figure} /_static/images/usage/security-modes/matrix-traffic-segregation.drawio.svg
---
align: center
class: mb
---

```

``` {warning} Warning
Currently, when this feature is enabled, your offloaded pods will not be able to reach the local cluster's API Server.
This is due to the fact that the API Server is not exposed as a service, but it is directly reachable through the remapped cluster's IP address.
This limitation will be removed in future.

For the same reason, the [in-band](FeaturesPeeringInBandControlPlane) peer will not work in this mode.
```

## Selection of the security mode

The desired security mode can be selected by setting a **flag** at install time or by setting the proper Helm values.
More details are provided in the [Install page](/installation/install.md)).

With respect to the former method, the flags available in `liqoctl` are the following:

* `--set networking.securityMode=FullPodToPod` to select **full pod-to-pod connectivity** (default mode)
* `--set networking.securityMode=IntraClusterTrafficSegregation` to select **intra-cluster traffic segregation**

```{admonition} Note
Currently, the selection of security mode is possible only at installation time. In future, this feature will be extended to be configured at run-time, e.g., each time you setup a new peering.
```
