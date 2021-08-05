---
title: Add a remote cluster 
weight: 1
---

## Overview

In Liqo, peering establishes an administrative connection between two clusters and enables the resource sharing across them.
It is worth noticing that peering is uni-directional. This implies that resources can be shared only from a cluster to another and not the vice-versa. Obviously, it can be optionally be enabled bi-directionally, enabling a two-way resource sharing.

### Peer with a new cluster

To peer with a new cluster, you have to create a ForeignCluster CR.

#### Add a new ForeignCluster

A `ForeignCluster` resource needs the authentication service URL and the port to be set: it is the backend of the
authentication server (mandatory to peer with another cluster).

The address is or the __hostname__ or the __IP address__ where it is reachable.
If you specified a name during the installation, it is reachable through an Ingress (you can get it with `kubectl get
ingress -n liqo`), if an ingress is not configured, the other cluster is exposed with a NodePort Service, you can get 
one if the IPs of your cluster's nodes (`kubectl get nodes -o wide`).

The __port__ where the remote cluster's auth service is reachable, if you are
using an Ingress, it should be `443` by default. Otherwise, if you are using a NodePort Service you 
can get the port executing `kubectl get service -n liqo liqo-auth`, an output example could be:

```txt
NAME           TYPE       CLUSTER-IP    EXTERNAL-IP   PORT(S)         AGE
liqo-auth   NodePort   10.81.20.99   <none>        443:30740/TCP   2m7s
```

An example of `ForeignCluster` resource can be:

```yaml
apiVersion: discovery.liqo.io/v1alpha1
kind: ForeignCluster
metadata:
  name: my-cluster
spec:
  outgoingPeeringEnabled: "Yes"
  foreignAuthUrl: "https://<ADDRESS>:<PORT>"
```

When you create the ForeignCluster, the Liqo control plane will contact the `foreignAuthUrl` (i.e. the public URL of a cluster 
authentication server) to retrieve all the required cluster information.

#### Access the cluster configurations

You can get the cluster configurations exposed by the Auth Service endpoint of the other cluster. This allows retrieving
the information necessary to peer with the remote cluster.

```bash
curl --insecure https://<ADDRESS>:<PORT>/ids
```

```json
{"clusterId":"0558de48-097b-4b7d-ba04-6bd2a0f9d24f","clusterName":"LiqoCluster0692"}
```

## Enable peering

The LAN and DNS discovery have the autojoin feature set by default, i.e., once the clusters are discovered and
authenticated, the peering happens automatically. If the foreign cluster has been added through a manual configuration,
you can enable the peering by setting its join flag as follows:

```bash
kubectl patch foreignclusters "$foreignClusterName" \
  --patch '{"spec":{"outgoingPeeringEnabled":"Yes"}}' \
  --type 'merge'
```

## Disable peering

If you want to disable the peering, it is enough to patch the `ForeignCluster` resource as follows:

```bash
kubectl patch foreignclusters "$foreignClusterName" \
  --patch '{"spec":{"outgoingPeeringEnabled":"No"}}' \
  --type 'merge'
```
