---
title: Enable peering
weight: 3
---

Once Liqo is installed on your clusters, you can start establishing new *peerings*. 


## Create the desired multi-cluster architecture

From now on, the *cluster-1* is the *home-cluster*. 
You can enable the *home-cluster* to peer with the other 2 clusters.

### Enable peering

```
export KUBECONFIG=$KUBECONFIG_2
liqoctl generate-add-command
```

You will obtain an output like the following:

```bash
liqoctl add cluster cluster-2 --auth-url https://172.18.0.5:32714 \ 
    --id 3623b0bd-3c32-4dec-994b-fc80d9d0d91d \
    --token b13b6932ee6fd890a1abe212dc21253aa6d74565fead54
```

This output represents the command to use to enable an outgoing peering from another cluster to cluser-2.
Therefore, you can take the output obtained and launch it aftering having sourced the KUBECONFIG_1:

```
export KUBECONFIG=$KUBECONFIG_1
liqoctl add ... # the output you obtained from the liqoctl generate-add-command
```

You can do the same with cluster-3:

```
export KUBECONFIG=$KUBECONFIG_3
liqoctl generate-add-command
export KUBECONFIG=$KUBECONFIG_1
liqoctl add ... # the output you obtained from the liqoctl generate-add-command
```

You can check now if the peerings are effectively enabled.

### Check peering status

Using *kubectl*, you can obtain the list of foreign clusters discovered by the *home-cluster*:

```bash
export KUBECONFIG=$KUBECONFIG_1
kubectl get foreignclusters
```

There should be two *ForeignCluster* resources in that state:

```bash
NAME                                   OUTGOING PEERING PHASE   INCOMING PEERING PHASE   NETWORKING STATUS   AUTHENTICATION STATUS
cluster-2                                Established                   None                 Established           Established             
cluster-3                                Established                   None                 Established           Established             
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
REMOTE_CLUSTER_ID_2=$(kubectl get nodes --selector=liqo.io/provider=provider-2 -o name | cut -d "-" -f2-)
echo $REMOTE_CLUSTER_ID_2 
REMOTE_CLUSTER_ID_3=$(kubectl get nodes --selector=liqo.io/provider=provider-3 -o name | cut -d "-" -f2-)
echo $REMOTE_CLUSTER_ID_3 
```

The following sections will guide you to discover and use the most notable Liqo features.
You can move forward to the first one: [Selective Offloading mechanism](../select_clusters).


