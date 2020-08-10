---
title: "Discovery Protocol"
---

## Overview
This component's goal is to find other clusters running Liqo around us, get information needed to pair and start peering process

### Features
List of supported features
* ClusterID creation
  * if not already present, during component starting, it creates new ClusterID taking the UID of first master of our
   cluster or generates new UUID if no master is present (NOTE: in this case ID will be different if ConfigMap where it 
   is store is deleted)
* Make our cluster discoverable by other clusters
  * this feature can be enabled and disabled at runtime setting `enableAdvertisement` flag in `ClusterConfig` CR
  * register Liqo service on local mDNS server and answers when someone is looking for it
* Discovery on LAN
  * this feature can be enabled and disabled at runtime setting `enableDiscovery` flag in `ClusterConfig` CR
  * through mDNS is able to find registered Liqo services on current LAN, a new `ForeignCluster` CR will be created for each cluster
  * clusters' reachability is periodically checked, if a cluster doesn't answer for three consecutive times, related `ForeignCluster` is deleted
* Discovery on WAN
  * when a new `SearchDomain` CR is added, an operator retrieves data from DNS server, a new `ForeignCluster` CR will be created for each cluster registered to domain provided
  * if a cluster is no more in domain registered list, related `ForeignCluster` is deleted
* Detect IP changes
  * if remote cluster IP changes, it is updated in `ForeignCluster`
* Automatically join discovered clusters
  * this feature can be enabled and disabled at runtime setting `autojoin` flag in `ClusterConfig` CR
  * when new `ForeignCluster` is added, peering process will automatically begin
  * this value can be overwritten for WAN discovery setting it in `SearchDomain` CR
* Start peering process
  * when `join` flag becomes true in a specific `ForeignCluster`, peering process will start for that cluster
* Unjoin process
  * unjoin process can be triggered setting `join` flag to false
  * if remote cluster deletes our `Advertisement` related `ForeignCluster` will be notified and changes its status to not joined

### Limitations
List of known limitations
* There is no way to set default behavior if to connect or not
  * as for Wi-Fi connections we may remember if automatically connect to a specific service when found
* CA used on the remote cluster has to be the same used by Deployments to authenticate API Server
  * no way to use URL as API server address (only raw IP is supported)
* Local cluster does not handle remote cluster CA changes
* It is not possible to trust or not CAs and to authenticate remote cluster
* If API Server IP changes and the two clusters are joined, related advertisement is deleted as all remote resources

## Architecture and workflow

This component can be divided in two main blocks:

1. Components in charge to find clusters and create `ForeignCluster`s
    * mDNS resolver
    * DNS client
    * Search Domain Operator
2. `ForeignCluster` management
    * Foreign Cluster Operator

#### Cluster finding

The goal of this block is to find clusters, to collect data and to create `ForeignCluster`s CR.
This can be done by mDNS resolver, DNS client or by manual insertion.
If we are using DNS client, we use an additional sub-component, the `SearchDomain` operator. This operator watches 
`SearchDomain` resources, when a new one is added, it contacts DNS server to retrieve required data.

##### Merge Logic

This logic merges discovered clusters with already existent ones. Currently, it checks if there is a cluster with the
same ClusterId, if not it creates a new one.

#### ForeignCluster management

This block is a standard Kubernetes operator that is watching on `ForeignCluster` resources.
When new one is added this component retrieves CAData from the remote cluster and stores it in a secret. That secret will
be used in all next interactions with remote cluster to authenticate it using it as Certification Authority of remote
cluster TLS certificate.
When `join` flag becomes true in a `ForeignCluster`, this component creates a new `PeeringRequest` CR in the remote
cluster triggering peering process.
Vice versa when we set to false this flag, `PeeringRequest` will be deleted triggering peering delete.
Every 30 seconds it checks is everything is working as expected both in the local and in the remote cluster, 
if something is not it tries to reconcile them.

![](/images/discovery/peering-process.png)

### Workflow

The typical workflow consist of three main steps:

1. Cluster finding
2. Setting `join` flag
3. Start peering process

When we no longer need foreign resources we can disable `join` flag to trigger peering delete. This process consists of:

1. `PeeringRequest` delete
    * that triggers `Broadcaster` termination
2. `Advertisement` delete
    * that triggers `VirtualKubelet` termination

### Which fields are managed by who?

![](/images/discovery/foreign-cluster.png)
