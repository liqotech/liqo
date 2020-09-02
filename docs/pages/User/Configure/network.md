---
title: Network
weight: 3
---

## Setting the cluster gateway

All the network traffic between two clusters is delivered through a special node that acts as gateway between the local cluster and the remote ones.

The install script will select (randomly) one of the existing nodes of your cluster as gateway.
In case you would like to select a precise node, you have to label it as follows:

```bash
kubectl label no your_gateway_node net.liqo.io/gateway=true
```
where `your__gateway__node` is the name of the node that has to be selected as gateway (e.g., `k8s-2-node-1`).

To get the list of your nodes, you can use the following command:

```
kubectl get no
```


## Current limitations
Given the early stage developement of the Liqo project, Liqo networking has the following limitations:

1. You can have a **single** gateway node for your cluster:

    * All the foreign clusters will be reachable through the same gateway
    * No gateway redundancy is available in case your gateway fails.

<!-- TODO: what happens if the gateway dies? Will liqo select automatically another gateway? -->

2. A service can be deployed in your home cluster *and* in a *single* foreign cluster. In other words, if your service has multiple components (let say pods `A`, `B`, `C`), they can talk to each other only if they are deployed in the home cluster and one of your foreign clusters you peer to. Instead, pods will not be able to communicate if you deploy `A` in your home cluster, `B` in a first foreign cluster,  `C` in a second foreign cluster.

    * It is worth noting that the current networking model supports multiple foreign clusters at the same time, although given the above limitation.