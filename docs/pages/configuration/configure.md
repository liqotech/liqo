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

### Scheduling a pod in a remote cluster using the 'liqo.io/enabled' label

First, you need to configure a Kubernetes namespace that also spans across foreign clusters, which can be achieved by 
setting the `liqo.io/enabled=true` label, as follows (which refers to namespace `liqo-demo`):

```
# Create a new namespace named 'liqo-demo'
kubectl create namespace liqo-demo
# Associate the 'liqo.io/enabled' label to the above namespace
kubectl label namespace liqo-demo liqo.io/enabled=true
```
