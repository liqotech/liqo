---
title: "Liqo Glossary"
---

Liqo leverages CRDs to expose its API and store its state.

## Configuration

* **ClusterConfig**: contains the Liqo Configuration.

## Discovery and Peering

* **PeeringRequest**: Notify a foreign cluster the interest to received subscription    
      * Policy can be enforced manually (i.e. UI) for Advertisement broadcasting
* **ForeignCluster**: represents the existence of a discovered (e.g.; in the LAN or via DNS) or 
manually added remote cluster to connect to.
* **SearchDomain**: represents the DNS domain where cluster are searched.

## Advertisement Management and Policies

* **Advertisement**: represents the offer of resources from a foreign cluster to an home cluster.

## Resource Sharing

* **SchedulingNode**: encapsulates all complementary information about known peers from scheduling 
perspective. It is always associated to a corresponding node. 

### VirtualKubelet

- **NamespaceNattingTable**: keeps track of namespaces created on foreign clusters, mapping them to local 
cluster.



