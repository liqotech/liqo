---
title: Tenant Namespace
weight: 4
---

### Overview

Each _Liqo Peering_ needs to be isolated from the other ones, each cluster can access only the resources related to its peerings.
Liqo assigns to each remote cluster a different namespace where the resources related to its peering are stored; this namespace is the _Tenant Namespace_.

Each cluster participating in the peering has to provide a Tenant Namespace where the two clusters share the ownership over the replicated resources.
During the peering it will grant to the remote identity namespace specific permissions only.

The Tenant Namespace is a standard _Kubernetes Namespace_ that has two labels:

* `discovery.liqo.io/tenant-namespace`: indicates that this is a _Liqo Tenant Namespace_.
* `discovery.liqo.io/cluster-id`: indicates which remote cluster this namespace is assigned to.

```yaml
apiVersion: v1
kind: Namespace
metadata:
  labels:
    discovery.liqo.io/cluster-id: 48bb07b7-a054-44a0-aa3b-16a9014cfe5b
    discovery.liqo.io/tenant-namespace: "true"
  name: liqo-tenant-48bb07b7-a054-44a0-aa3b-16a9014cfe5b
```

It is created the first time Liqo needs to store a resource in it, and it will reused for every interconnection with that remote cluster.
