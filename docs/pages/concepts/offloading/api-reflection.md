---
title: API reflection
weight: 5
---

### Overview

In addition to the well-known kubelet features, our virtual kubelet implementation provides a feature we called "*reflection*": the offloaded Pods may need some resources to be properly executed in the remote clusters, such as:

* `Services`.
* `Endpoints`.
* `Secret`.
* `ConfigMaps`.

The virtual kubelet itself is in charge of replicating those APIs in the remote cluster by properly operating some translations (e.g., the endpoints addresses have to be translated to point to the home cluster).

{{% notice note %}}
This documentation section is a work in progress
{{% /notice %}}