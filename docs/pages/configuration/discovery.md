---
title: Discovery
weight: 2
---

## Overview

Once Liqo is installed in your cluster, you can start discovering new *peers*.
Specifically, you can rely on three different methods to discover other clusters:

1. **LAN Discovery**.
2. **Manual Configuration**: manual addition of specific clusters to the list of known ones. This method is particularly
appropriate outside LAN, without requiring any DNS configuration.
3. **DNS Discovery**: automatic discovery of the clusters associated with a specific DNS domain (e.g.; *liqo.io*). 
This is achieved by querying specific DNS entries. This looks similar to the discovery of voice-over-IP SIP servers, and 
it is mostly oriented to big organizations that wish to adopt Liqo in production.

Depending of your environment, you only have to decide the proper discovery method. They are detailed below:

### LAN Discovery

Automatic discovery of neighboring clusters available in the same LAN. It looks similar to the automatic discovery 
of Wi-Fi hotspots, and it is particularly suitable when your cluster has a single node (e.g., in a combination with 
[K3s](https://k3s.io)).
Liqo can automatically discover any available clusters running on the same L2 Broadcast Domain. 
Besides, mDNS discovery implies also that your cluster is discoverable by others.

#### Enable and Disable the discovery on LAN

The discovery on LAN can be enabled and disabled by updating the flags in the ClusterConfig CR. Lan Discovery can be 
disabled to avoid unwanted peering with neighbors.

You can enable discovery, by typing:
```bash
kubectl patch clusterconfigs liqo-configuration \
  --patch '{"spec":{"discoveryConfig":{"enableDiscovery": true, "enableAdvertisement": true}}}' \
  --type 'merge'
```

Or disabling it with:
```bash
kubectl patch clusterconfigs liqo-configuration \
  --patch '{"spec":{"discoveryConfig":{"enableDiscovery": false, "enableAdvertisement": false}}}' \
  --type 'merge'
```

The automatic LAN discovery is enabled by default.

### Manual Configuration

In Liqo, remote clusters are defined as `ForeignClusters`: each time a new `ForeignCluster` resource is added in a Liqo
cluster, it is possible to peer with it.

#### Forging the ForeignCluster

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
liqo-auth      NodePort   10.81.20.99   <none>        443:30740/TCP   2m7s
```

An example of `ForeignCluster` resource can be:

```yaml
apiVersion: discovery.liqo.io/v1alpha1
kind: ForeignCluster
metadata:
  name: my-cluster
spec:
  join: true # optional (defaults to false)
  authUrl: "https://<ADDRESS>:<PORT>"
```

When you create the ForeignCluster, the Liqo control plane will contact the `authURL` (i.e. the public URL of a cluster 
authentication server) to retrieve all the required cluster information.

#### Access the cluster configurations

You can get the cluster configurations exposed by the Auth Service endpoint of the other cluster. This allows retrieving
the information necessary to peer with the remote cluster.

```bash
curl --insecure https://<ADDRESS>:<PORT>/ids
```

```json
{"clusterId":"0558de48-097b-4b7d-ba04-6bd2a0f9d24f","clusterName":"LiqoCluster0692","guestNamespace":"liqo"}
```

### DNS Discovery

In addition to LAN discovery and manual configuration, Liqo supports DNS-based discovery: such a mechanism is useful, 
for instance, when dealing with multiple clusters, which are dynamically spawned and decommissioned. The DNS discovery 
procedure requires two orthogonal actions to be enabled:

1. Register your cluster into your DNS server to make it discoverable by others (the required parameters are available 
in the section below).
2. Connect to a foreign cluster, specifying the remote domain.

#### Register the home cluster

To allow the other clusters to peer with your cluster(s), you need to register a set of DNS records that specify the 
cluster(s) available in your domain, with the different parameters required to establish the connection.

In a scenario where we have to manage multiple clusters' discovery, it can be useful to manage the entire set updating 
it in a unique place. We only have to know how the Authentication Service is reachable from the external world.

#### DNS Configuration

In the following example, we present a `bind9`-like configuration for a hypothetical domain `example.com`. It exposes 
two Liqo-enabled cluster named `liqo-cluster` and `liqo-cluster-2`. The first one exposes the Auth Service at 
`1.2.3.4:443`, while the second at `2.3.4.1:8443`.

```txt
example.com.                  PTR     liqo-cluster.example.com.
                                      liqo-cluster-2.example.com.

liqo-cluster.example.com.     SRV     0 0 443 auth.server.example.com.
liqo-cluster-2.example.com.   SRV     0 0 8443 auth.server-2.example.com.

auth.server.example.com.      A       1.2.3.4
auth.server-2.example.com.    A       2.3.4.1
```

Remember to adapt the configuration according to your setup, modifying the urls, ips and ports accordingly.

{{%expand "Expand here to know more about the meaning of each record." %}}

* the `PTR` record lists the Liqo clusters exposed for the specific domain (e.g. `liqo-cluster`).
* the `SRV` record specifies the network parameters needed to connect to the cluster's Auth Service. You should have a 
record for each cluster present in the `PTR` record.
  Specifically, it has the following format:
  ```txt
   <cluster-name>._liqo._tcp.<domain>. SRV <priority> <weight> <auth-server-port> <auth-server-name>.
  ```
  where the priority and weight fields are unused and should be set to zero. In this case, the API server is reachable 
  at the address `liqo-cluster-api.server.example.com` through port `6443`.
* The `A` record assigns an IP address to the DNS name of the Auth Service server ( `1.2.3.4` in the above example).

{{% /expand %}}

#### Connect to a remote cluster

To leverage the DNS discovery to peer to a remote cluster, it is necessary to specify the remote domain called 
`SearchDomain`. When a `SearchDomain` is configured, Liqo performs periodical queries on specific search domains looking
for new clusters to peer with.
For any `SearchDomain`, you need to configure the following parameters:

1. **Domain**: the domain where the cluster you want to peer with is located.
2. **Name**: a mnemonic name to identify the domain.
3. **Join**: to specify whether to trigger the peering procedure automatically.

Using kubectl, it is also possible to perform the following configuration. A `SearchDomain` for the `example.com` 
domain, may be look like:

```bash
cat << "EOF" | kubectl apply -f -
apiVersion: discovery.liqo.io/v1alpha1
kind: SearchDomain
metadata:
  name: example.com
spec:
  domain: example.com
  autojoin: true
EOF
```

### Get discovered clusters

Using kubectl, you can manually obtain the list of discovered foreign clusters by typing:

```bash
kubectl get foreignclusters
NAME                                   AGE
ff5aa14a-dd6e-4fd2-80fe-eaecb827cf55   101m
```