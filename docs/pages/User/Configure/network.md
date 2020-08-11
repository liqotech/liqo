---
title: Configure Liqo
weight: 2
---

## Gateway

All the inter-cluster traffic is delivered through a special node that acts as gateway between the local cluster and the other peered clusters.
This node has to be manually labelled as gateway, as in the following command:
```bash
kubectl label no __your__gateway__node liqonet.liqo.io/gateway=true
```

To get the list of your nodes, you can use: 

```
kubectl get no
```