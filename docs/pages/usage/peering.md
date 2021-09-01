---
title: Peer with a cluster
weight: 1
---

## Overview

In Liqo, peering establishes an administrative connection between two clusters and enables the resource sharing across them.
It is worth noticing that peering is unidirectional. 
This implies that resources can be shared only from a cluster to another and not the vice-versa. Obviously, it can be optionally be enabled bi-directionally, enabling a two-way resource sharing.

### Peer with a new cluster

To peer with a cluster, you should provide some information to Liqo in order to reach that cluster.
*liqoctl* can help you by generating the command to peer with a specific cluster.

The installation of *liqoctl* is quick and you can find the complete instructions in a [dedicated section](/installation#liqoctl)

Let's suppose that you want to peer from cluster A to cluster B.
This means that we would like to offload pods in A to the cluster B.

#### Generate the peer command

First, pointing cluster B:

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

Than, to peer A with a B cluster, you can just run the command you obtained by the previous step, after having exported the cluster A KUBECONFIG:

```bash
export KUBECONFIG=kubeconfig-cluster-a
liqoctl add cluster clusterB --auth-url https://172.18.0.5:32714 \ 
    --id 3623623b0bd-3c32-4dec-994b-fc80d9d0d91d \
    --token b13b6932ee6fd890a1abe212dc21253aa6d74565fead54
```

If this command is executed successfully, you have completed a peering. 

### Check the status of the peering

To check the result of this addition, you can observe if new foreign clusters are available.
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

The first node of the list is a virtual node pointing to the cluster B you just peered.

### What's next?

* If you are not familiar with Liqo pod offloading, you can check out our tutorials in [Getting Started section](/gettingstarted).
* If you are familiar with Liqo offloading, you may want to get in touch with advanced features explained in [namespace offloading](/namespace_offloading) section.
