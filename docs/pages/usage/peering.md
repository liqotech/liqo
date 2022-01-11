---
title: Peer with a cluster
weight: 1
---

## Overview

In Liqo, peering establishes an administrative connection between two clusters and enables the resource sharing across them.
It is worth noticing that peering is unidirectional.
This implies that resources can be shared only from a cluster to another and not the vice-versa.
Obviously, it can be optionally be enabled bi-directionally, enabling a two-way resource sharing.

### Peer with a new cluster

To peer with a cluster, you should provide some information to Liqo in order to reach that cluster.
*liqoctl* can help you by generating the command to peer with a specific cluster.

The installation of *liqoctl* is very simple; complete instructions are available in a [dedicated section](/installation/install#liqoctl).

Let's suppose that you are the Cluster A and you would like to peer with Cluster B.
This means that you would like to offload Cluster A's pods in Cluster B.

#### Generate the peer command

First, you should configure the KUBECONFIG of cluster B and trigger the command generation:

```bash
export KUBECONFIG=kubeconfig-cluster-b
liqoctl generate-add-command
```

You will obtain an output like the following:

```bash
liqoctl add cluster clusterB --auth-url https://172.18.0.5:32714 \
    --id 3623b0bd-3c32-4dec-994b-fc80d9d0d91d \
    --token b13b6932ee6fd890a1abe212dc21253aa6d74565fead54
```

#### Peer with a cluster

Now, to peer Cluster A with B, you can just (1) export the KUBECONFIG of Cluster A, then (2) run the command you obtained in the previous step:

```bash
export KUBECONFIG=kubeconfig-cluster-a
liqoctl add cluster clusterB --auth-url https://172.18.0.5:32714 \
    --id 3623623b0bd-3c32-4dec-994b-fc80d9d0d91d \
    --token b13b6932ee6fd890a1abe212dc21253aa6d74565fead54
```



To check if the above command completed successfully, you can observe if a new foreign cluster B is available.
To do so, you can use kubectl. For example:
```bash
kubectl get foreignclusters
```

You should observe an output similar to the following:

```bash
NAME                                   OUTGOING PEERING PHASE   INCOMING PEERING PHASE   NETWORKING STATUS   AUTHENTICATION STATUS   AGE
3623b0bd-3c32-4dec-994b-fc80d9d0d91d   Established              Established              Established         Established             1m
```

After several seconds, a new node should be available in your cluster:

```bash
kubectl get nodes
```

You should see something like:

```bash
liqo-3623b0bd-3c32-4dec-994b-fc80d9d0d91d   Ready    agent    1m   v1.19.11   alpha.service-controller.kubernetes.io/exclude-balancer=true,beta.kubernetes.io/os=linux,kubernetes.io/hostname=liqo-3623b0bd-3c32-4dec-994b-fc80d9d0d91d,kubernetes.io/role=agent,liqo.io/type=virtual-node,node.kubernetes.io/exclude-from-external-load-balancers=true,type=virtual-kubelet,liqo.io/provider=kubeadm
liqo-cluster-2-control-plane                Ready    master   87m   v1.19.11   beta.kubernetes.io/arch=amd64,beta.kubernetes.io/os=linux,kubernetes.io/arch=amd64,kubernetes.io/hostname=liqo-cluster-2-control-plane,kubernetes.io/os=linux,node-role.kubernetes.io/master=
liqo-cluster-2-worker                       Ready    <none>   86m   v1.19.11   beta.kubernetes.io/arch=amd64,beta.kubernetes.io/os=linux,kubernetes.io/arch=amd64,kubernetes.io/hostname=liqo-cluster-2-worker,kubernetes.io/os=linux
liqo-cluster-2-worker2                      Ready    <none>   86m   v1.19.11   beta.kubernetes.io/arch=amd64,beta.kubernetes.io/os=linux,kubernetes.io/arch=amd64,kubernetes.io/hostname=liqo-cluster-2-worker2,kubernetes.io/os=linux
```

The first node of the list is a virtual node pointing to the cluster B you have just peered with.

### What's next?

* If you are not familiar with Liqo pod offloading, you can check our tutorials in the [Getting Started](/gettingstarted) section.
* If you are already familiar with Liqo offloading, you may want to explore the advanced features presented in the [namespace offloading](../namespace_offloading) section.
