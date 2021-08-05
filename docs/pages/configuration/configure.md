---
title: Configuration
weight: 1
---

### Edit your cluster name

The ClusterName is your cluster's nickname, a simple and understandable name that the other clusters can see when they 
discover your. It is set during installation, but you can easily change it whenever you want by editing your 
`ClusterConfig`, through the dashboard or `kubectl`.

To modify the `ClusterConfig` via kubectl use the following command:
```bash
kubectl edit clusterconfig
```
and modify the field: 
```yaml
discoveryConfig: 
   clusterName: your_cluster_name
```