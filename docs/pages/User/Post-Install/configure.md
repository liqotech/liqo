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

### Advertisement configuration

In this section, you can configure your cluster behavior regarding Advertisement broadcasting and acceptance,
and the parameters for the [keepalive check](#keepalive-check):
* **OutgoingConfig** defines the behaviour for the creation of the Advertisement for other clusters.
  - `enableBroadcaster` flag allows you to enable/disable the broadcasting of your Advertisement to the foreign clusters
   your cluster knows
  - `resourceSharingPercentage` defines the percentage of your cluster resources that you will share with other clusters
* **IngoingConfig** defines the behaviour for the acceptance of Advertisements from other clusters.
  - `maxAcceptableAdvertisement` defines the maximum number of Advertisements that can be accepted over time
  - `acceptPolicy` defines the policy to accept or refuse a new Advertisement from a foreign cluster. The possible 
  policies are:
    - `AutoAcceptMax`: every Advertisement is automatically checked considering the configured maximum;
    AutoAcceptAll policy can be achieved by setting MaxAcceptableAdvertisement to 1000000, a symbolic value representing
    infinite; AutoRefuseAll can be achieved by setting MaxAcceptableAdvertisement to 0
    - `ManualAccept`: every Advertisement needs to be manually accepted or refused; this mode is not implemented yet