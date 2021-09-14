---
title: Configuration
weight: 1
---

### Edit your cluster name

The ClusterName is your cluster's nickname, a simple and understandable name that the other clusters can see when they peer with it. 

You can easily modify it whenever you want using `liqoctl install`. The `liqoctl install` workflow is idempotent and can be executed multiple times to enforce the desired configuration.

For example:

```
liqoctl install ${YOUR_PROVIDER} --cluster-name ${YOUR_CLUSTER_NAME}
```

where `${YOUR_PROVIDER}` is the provider for your cluster and `${YOUR_CLUSTER_NAME}` is the name you want to assign.
