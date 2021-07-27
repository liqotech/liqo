---
title: Enable peering
weight: 3
---

Once Liqo is installed on your clusters, you can start establishing new *peerings*. This tutorial relies on [LAN Discovery method](/configuration/discovery#lan-discovery) since the Kind clusters are in the same L2 broadcast domain.

## Create the desired multi-cluster architecture

From now on, the *cluster-1* is the *home-cluster*. 
You can enable the *home-cluster* to peer with the other 2 clusters.

### Enable peerings

Using *kubectl*, you can obtain the list of foreign clusters discovered by the *home-cluster*:

```bash
export KUBECONFIG=$KUBECONFIG_1
kubectl get foreignclusters
```

There should be two *ForeignCluster* resources in that state:

```bash
NAME                                   OUTGOING PEERING PHASE   INCOMING PEERING PHASE   NETWORKING STATUS   AUTHENTICATION STATUS
b07938e3-d241-460c-a77b-e286c0f733c7   None                     None                     None                Established             
b38f5c32-a877-4f82-8bde-2fd0c5c8f862   None                     None                     None                Established             
```

{{% notice note %}}
When discovered using LAN discovery, the ForeignCluster object name is the cluster-id of the corresponding remote cluster.
{{% /notice %}}

To enable an unidirectional peering it is sufficient to edit a field of the ForeignCluster resource:

```bash
kubectl patch foreignclusters <your-ForeignCluster-name> \
--patch '{"spec":{"outgoingPeeringEnabled":"Yes"}}' \
--type 'merge'
```

Repeat the same operation for the other resource.

After the previous patches to ForeignCluster resources, you should obtain:

```bash
export KUBECONFIG=$KUBECONFIG_1
kubectl get foreignclusters
```

```bash
NAME                                   OUTGOING PEERING PHASE   INCOMING PEERING PHASE   NETWORKING STATUS   AUTHENTICATION STATUS
b07938e3-d241-460c-a77b-e286c0f733c7   Established              None                     None                Established          
b38f5c32-a877-4f82-8bde-2fd0c5c8f862   Established              None                     None                Established          
```

Only the "*OUTGOING PEERING PHASE*" is set to Established, so only the *home-cluster* can use the resources of the other two clusters:

| Concept        | Description |
|-----           | ----------- |
| **Outgoing**   | Unidirectional peering from home-cluster to another cluster. |
| **Incoming**   | Unidirectional peering from another cluster to home-cluster.|

To have more details about the peering mechanism, look at the [dedicated peering section](/concepts/peering#overview).


### Check the virtual node presences

After few seconds, you should see two *Liqo Big Nodes* (named *"liqo-"*) in addition to the physical node.

```bash
export KUBECONFIG=$KUBECONFIG_1
kubectl get nodes 
```

The *home-cluster* has these two virtual nodes, relying on the two unidirectional peerings with *cluster-2* and *cluster-3*.
You can now check the labels that they expose:

```bash
kubectl get nodes --selector=liqo.io/type --show-labels
```

The output is truncated to see only the labels of interest:

```bash
NAME                                        STATUS   ROLES    LABELS
liqo-b07938e3-d241-460c-a77b-e286c0f733c7   Ready    agent    "topology.liqo.io/region"="eu-east", "liqo.io/provider"="provider-3"
liqo-b38f5c32-a877-4f82-8bde-2fd0c5c8f862   Ready    agent    "topology.liqo.io/region"="us-west", "liqo.io/provider"="provider-2"
```

According to the cluster labels:

* **liqo-b38f5c32-a877-4f82-8bde-2fd0c5c8f862** is the virtual node relying on the unidirectional peering with the *cluster-2*.
* **liqo-b07938e3-d241-460c-a77b-e286c0f733c7** is the virtual node relying on the unidirectional peering with the *cluster-3*.

The virtual-node name is composed of a prefix (*liqo-*) plus the *cluster-id* of the corresponding remote cluster.
You can export these *cluster-id* as environment variables:

```bash
REMOTE_CLUSTER_ID_2=$(kubectl get nodes --selector=liqo.io/provider=provider-2 | cut -d " " -f1 | cut -d "-" -f2-)
echo $REMOTE_CLUSTER_ID_2 
REMOTE_CLUSTER_ID_3=$(kubectl get nodes --selector=liqo.io/provider=provider-3 | cut -d " " -f1 | cut -d "-" -f2-)
echo $REMOTE_CLUSTER_ID_3 
```

The following sections will guide you to discover and use the most notable Liqo features.
You can move forward to the first one: [Selective Offloading mechanism](../select_clusters).


