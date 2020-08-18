---
title: Peering 
weight: 1
---

## Introduction

The peering process in Liqo introduces the possibility to share resources among clusters. It is intended to be 
an asymmetric process where an **home** cluster can ask for resources and service to a remote **foreign** cluster discovered
 via different available mechanisms (i.e. Local discovery, DNS query, manual insertion).
 
When an home cluster asks for a peering, a peering request is forwarded to the remote cluster, which answers with
an Advertisement. The advertisement models the "sharing offers" it is willing to share in terms of CPU, memory, persistent
storage and other resources.

If the home cluster accepts the advertisement, this trigger the establishment of a virtual node and an intercluster
connection. The virtual nodes make the remote resources available for scheduling while the cluster interconnection make
pods scheduled on the foreign cluster available on the local one.


## Explore available clusters

```
kubectl get foreignclusters
```

Default policy tries to activate peering with a remote cluster, when it is discovered.

Peering can be enabled by setting the Join property in ForeignCluster to True or via the dashboard.

## Peering checking

### Presence of the virtual-node

If the peering has been correctly performed, you should see the creation of a virtual node in addition to your
nodes: 

```
NAME                                      STATUS   ROLES    AGE     VERSION          LABELS
rar-k3s-01                                Ready    master   3h18m   v1.18.6+k3s1     beta.kubernetes.io/arch=amd64,beta.kubernetes.io/instance-type=k3s,beta.kubernetes.io/os=linux,k3s.io/hostname=rar-k3s-01,k3s.io/internal-ip=10.0.2.4,kubernetes.io/arch=amd64,kubernetes.io/hostname=rar-k3s-01,kubernetes.io/os=linux,liqonet.liqo.io/gateway=true,node-role.kubernetes.io/master=true,node.kubernetes.io/instance-type=k3s
vk-remote-cluster   Ready    agent    3h5m    v1.17.2-vk-N/A   alpha.service-controller.kubernetes.io/exclude-balancer=true,beta.kubernetes.io/os=linux,kubernetes.io/hostname=vk-e582fe9d-03d1-4788-ad85-4d04674a6437,kubernetes.io/os=linux,kubernetes.io/role=agent,**type=virtual-node**
```