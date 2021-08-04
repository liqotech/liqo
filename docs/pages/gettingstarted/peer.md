---
title: Peer to a foreign cluster
weight: 3
---

Once Liqo is installed in your cluster, you can start establishing new *peerings*. This tutorial relies on [LAN Discovery](/configuration/discovery#lan-discovery) since our Kind clusters are in the same L2 broadcast domain.

## LAN Discovery

Liqo can automatically discover any available clusters or make any clusters discoverable on the same LAN.

Using kubectl, you can also manually obtain the list of discovered foreign clusters:

```bash
kubectl get foreignclusters
NAME                                   AGE
ff5aa14a-dd6e-4fd2-80fe-eaecb827cf55   101m
```

The `foreigncluster` object is used in Liqo to model remote clusters discovered.

To check whether Liqo attempts to peer with the foreign cluster automatically,
you can check the `join` property of the specific `ForeignCluster` resource:

```bash
kubectl get foreignclusters ${FOREIGN_CLUSTER} --template={{.spec.join}}
true
```

## Peering checking

### Presence of the virtual-node

If the peering has succeded, you should see a virtual node (named `liqo-*`) in addition to your physical nodes:

```
kubectl get nodes

NAME                                      STATUS   ROLES
master-node                               Ready    master
worker-node-1                             Ready    <none>
worker-node-2                             Ready    <none>
liqo-9a596a4b-591c-4ac6-8fd6-80258b4b3bf9 READY    agent    <-- This is the virtual node
```

## Verify that the resulting infrastructure works correctly

You are now ready to verify that the resulting infrastructure works correctly, as presented in the [next step](../test).
