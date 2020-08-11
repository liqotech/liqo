---
title: The Liqo Tenant Model 
weight: 1
---

The peering process in Liqo introduces the possibility to share resources among clusters. Liqo enforces Kubernetes objects 
replication (e.g. primarily pods) by mapping them across different clusters. Namespaces included in liqo, will be replicated
have a "shadow" namespace on each foreign cluster.

![](/images/home/architecture.png)

## Liqo Tenant Model

* Pods are replicated, not deployments. 
* Several objects are kept synchronized between namespaces by Liqo:
   * Services/Endpoints
   * Configmaps/Secrets

## Scheduling Pods on VirtualNodes

To schedule pods on other clusters, you have several options:

* spawn it in a namespace that can be alive also in a foreign cluster (this requires the namespace to be labelled  ```liqo.io/enabled=true```)
* add a toleration for taints:
```
    taints:
    - effect: NoExecute
      key: virtual-node.liqo.io/not-allowed
      value: "true"
```



