---
title: Peer to a foreign cluster
weight: 2
---

Before peering your home cluster with a foreign cluster, we suggest to have a look at the [Liqo peering basics](../../liqo-brief/#peering-basics).


## Explore available clusters

```
kubectl get foreignclusters
```

Default policy tries to activate peering with a remote cluster, when it is discovered.

Peering can be enabled by setting the `Join` property in `ForeignCluster` to `True` or via the dashboard.

<!-- TODO: The above sentence looks not obvious for an occasional user. Please be more user-friendly. -->


## Peering checking

### Presence of the virtual-node

If the peering has been correctly performed, you should see a virtual node in addition to your physical nodes: 

```
kubectl get no

NAME                                      STATUS   ROLES    AGE     VERSION          LABELS
rar-k3s-01                                Ready    master   3h18m   v1.18.6+k3s1     beta.kubernetes.io/arch=amd64,beta.kubernetes.io/instance-type=k3s,beta.kubernetes.io/os=linux,k3s.io/hostname=rar-k3s-01,k3s.io/internal-ip=10.0.2.4,kubernetes.io/arch=amd64,kubernetes.io/hostname=rar-k3s-01,kubernetes.io/os=linux,liqonet.liqo.io/gateway=true,node-role.kubernetes.io/master=true,node.kubernetes.io/instance-type=k3s
liqo-remote-cluster   Ready    agent    3h5m    v1.17.2-vk-N/A   alpha.service-controller.kubernetes.io/exclude-balancer=true,beta.kubernetes.io/os=linux,kubernetes.io/hostname=vk-e582fe9d-03d1-4788-ad85-4d04674a6437,kubernetes.io/os=linux,kubernetes.io/role=agent,**type=virtual-node**
```

## Verify that the resulting infrastructure works correctly

You are now ready to verify that the resulting infrastructure works correctly, which is presented in the [next step](../test).

