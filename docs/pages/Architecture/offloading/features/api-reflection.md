---
title: API reflection
weight: 4
---

### Overview

In addition to the well-known Kubelet features, our Virtual Kubelet implementation provides a feature we called 
"reflection": the offloaded Pods may need some resources to be properly executed in the remote cluster, such as
* `services`
* `endpoints`
* `secret`
* `configmaps`

The virtual Kubelet itself is in charge of replicating those APIs in the remote cluster, by properly operating some 
translations (e.g., the endpoints addresses have to be translated to point to the home cluster).

> This documentation section is a work in progress