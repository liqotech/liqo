---
title: Configuration Reference
weight: 1
---

Liqo installer automatically installs a default configuration in your cluster. You can find this configuration in `deployments/liqo_chart/templates/clusterconfig.yaml`.
The configuration can be modified through `kubectl` or the Liqo dashboard.

## Discovery Configuration

### Edit your cluster name
The ClusterName is the nickname of your cluster, a simple and understandable name that the other clusters can see when they discover your.
It is set during installation, but you can easily change it whenever you want by editing your `ClusterConfig`, through the dashboard or `kubectl`.

To modify the `ClusterConfig` via kubectl use the following command:
```bash
kubectl edit clusterconfig
```
and modify the field: 
```yaml
discoveryConfig: 
   clusterName: your_cluster_name
```

## Advertisement configuration

In this section you can configure your cluster behaviour regarding the Advertisement broadcasting and acceptance,
and the parameters for the [keepalive check](#keepalive-check):
* **OutgoingConfig** defines the behaviour for the creation of the Advertisement for other clusters.
  - `enableBroadcaster` flag allows you to enable/disable the broadcasting of your Advertisement to the foreign clusters your cluster knows
  - `resourceSharingPercentage` defines the percentage of your cluster resources that you will share with other clusters
* **IngoingConfig** defines the behaviour for the acceptance of Advertisements from other clusters.
  - `maxAcceptableAdvertisement` defines the maximum number of Advertisements that can be accepted over time
  - `acceptPolicy` defines the policy to accept or refuse a new Advertisement from a foreign cluster. The possible policies are:
    - `AutoAcceptMax`: every Advertisement is automatically checked considering the configured maximum;
    AutoAcceptAll policy can be achieved by setting MaxAcceptableAdvertisement to 1000000, a symbolic value representing infinite; AutoRefuseAll can be achieved by setting MaxAcceptableAdvertisement to 0
    - `ManualAccept`: every Advertisement needs to be manually accepted or refused; this mode is not implemented yet.

### Keepalive check

After establishing a sharing with a foreign cluster (i.e. you have received an Advertisement and are using that cluster resources), a keepalive mechanism starts,
in order to know if the foreign cluster is reachable or not. In the AdvertisementConfig you can configure:
* `KeepaliveThreshold`: the number of failed attempts to contact the foreign cluster your cluster will tolerate before deleting it.
* `KeepaliveRetryTime`: the time between an attempt and the next one.

## Network configuration

### Setting the cluster gateway

All the network traffic between two clusters is delivered through a special node that acts as gateway between the local cluster and the remote ones.

The install script will select (randomly) one of the existing nodes of your cluster as gateway.
In case you would like to select a precise node, you have to label it as follows:

```bash
kubectl label no your_gateway_node net.liqo.io/gateway=true
```
where `your__gateway__node` is the name of the node that has to be selected as gateway (e.g., `k8s-2-node-1`).

To get the list of your nodes, you can use the following command:

```
kubectl get no
```

