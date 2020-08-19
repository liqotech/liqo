---
title: Resource Sharing
---


# The Liqo Sharing Model

The peering process in Liqo introduces the possibility to share resources among clusters. Liqo enforces Kubernetes objects 
replication (e.g. primarily pods) by mapping them across different clusters. Namespaces included in liqo, will be replicated
have a "shadow" namespace on each foreign cluster.

![](/images/home/architecture.png)

## Liqo Tenant Model

* Pods are replicated, not deployments. 
* Several objects are kept synchronized between namespaces by Liqo:
   * Services/Endpoints
   * Configmaps/Secrets