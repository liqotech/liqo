---
title: Install Liqo
weight: 2
---

## Install Liqo on your clusters

In this section, you can install Liqo on the clusters just created.

### Define your cluster labels

First, you should define the *cluster labels* that each cluster remotely exports during the peering process.
As detailed in the [namespace replication page](/usage/namespace_offloading/#cluster-labels-concept), each cluster can expose some labels meaningful, enabling the possibility to select it during the offloading configuration.

In this example, you export two labels for every cluster:

| Key                           | Description |
| --------------                | ----------- |
| **topology.liqo.io/region**   | Represents the region to which that cluster belongs. |
| **liqo.io/provider**          | Indicates the cloud provider that manages that cluster. |

### Installing Liqo

You can install Liqo on the first cluster:

```bash
export KUBECONFIG=$KUBECONFIG_1
liqoctl install kind --cluster-name cluster-1 \
   --cluster-labels="topology.liqo.io/region"="eu-west","liqo.io/provider"="provider-1" \
   --enable-lan-discovery=false
```

The "**--namespace**" option sets the namespace name in which the Liqo control plane is deployed.

| Key                                   | Type | Description |
|-----                                  |------|-------------|
| **name**      | *string* | Set a mnemonic name for your cluster. |
| **cluster-labels**    | *map*    | Set labels attached to the cluster when exposed remotely. |
| **enable-lan-discovery**         | *bool*   | If set to true, automatically join discovered cluster exposing the Authentication Service with a valid certificate. |

You can find additional details about the possible chart values by looking at [the dedicated section](/installation/chart_values#values).

If you set the *--enable-lan-discovery* to true, Liqo will automatically discover the other clusters and create a full mesh topology between them.
However, in this tutorial, the *--enable-lan-discovery* parameter is set to False to let you discover how to manually enable and disable peerings according to the Liqo selective peering feature.

You should install Liqo also on the other two clusters:

```bash
export KUBECONFIG=$KUBECONFIG_2
liqoctl install kind --cluster-name cluster-2 \
   --cluster-labels="topology.liqo.io/region"="us-west","liqo.io/provider"="provider-2" \
   --enable-lan-discovery=false
```

```bash
export KUBECONFIG=$KUBECONFIG_3
liqoctl install kind --cluster-name cluster-3 \
   --cluster-labels="topology.liqo.io/region"="eu-east","liqo.io/provider"="provider-3" \
   --enable-lan-discovery=false
```

liqoctl commands take a couple of minutes to complete. If liqoctl returns successfully, the installation process is complete.

## Check installation state

Liqo should be installed on all three clusters.

You can check by typing for example:

```bash
export KUBECONFIG=$KUBECONFIG_1
kubectl get pods -n liqo
```

Each cluster should provide an output similar to this:

```bash
NAME                                          READY   STATUS
liqo-auth-7df65db559-jtmvb                    1/1     Running
liqo-controller-manager-c7b996f8f-m5wgd       1/1     Running
liqo-crd-replicator-7d7b66d566-7z9lm          1/1     Running
liqo-discovery-5f7d7fffdd-wbvt2               1/1     Running
liqo-gateway-6f4fb8dcd9-pqx2d                 1/1     Running
liqo-network-manager-65dd9599d6-dvfgw         1/1     Running
liqo-route-sn5c8                              1/1     Running
liqo-webhook-6bcc9d4f76-5dwmw                 1/1     Running
```

If Liqo is installed and running on your clusters, you can start to [peer your clusters](../peer).
