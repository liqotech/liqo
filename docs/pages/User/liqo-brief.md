---
title: Liqo in brief
weight: 1
---

This section gives a brief insight about Liqo, its features, and the main concepts we need to know in order to operate properly with it.

## Overview
Liqo is an open source add-on for Kubernetes that allows to seamlessly and securely share resources and services between multiple clusters, enabling to run your workloads on remote clusters.

Differently from existing federation mechanisms, Liqo leverages the same highly successful "peering" model of the Internet, without any central point of control, nor any "master" cluster.
New peering relationships can be established dynamically, whenever needed, even automatically.
In this respect, Liqo supports automatic discovery of local and remote clusters, to further simplify the peering process.

Sharing and peering operations are strictly enforced by policies: each cluster retains full control of its infrastructure,
deciding what to share, how much, with whom. Each cluster can advertise the resources allocated for sharing to other peers, which may accept that offer. In that case, a contract is signed and both parties are requested to fulfill their obligations.
Each cluster can control exactly the amount of shared resources, which can be differentiated for each peer.

Security is very important in Liqo. In this respect, Liqo leverages all the features available in Kubernetes, such as
Role-Based Access Control (RBAC), Pod Security Policies (PSP), hardened Container Runtimes Interfaces (CRI) implementations.

Kubernetes users will experience the usual environment also after starting Liqo: all administrative tasks are the same, performed in the usual way and with the well-known tools (e.g. `kubectl`). The only difference is that your cluster can become more powerful, as resources and services can be borrowed from the other clusters you peered with.

With Liqo, you can leverage an unlimited amount of resources by simply peering with other clusters. Similarly, resource providers can leverage their infrastructure by selling their resources to many different peers, in a highly dynamic way.

## Terminology
We call **home** cluster the one under your control, while the **foreign** cluster is the one that you peer with, which is usually under the control of different organizations.

A cluster can either **offer** resources and services to other foreign clusters, or **consume** resources and services in another foreign cluster. A cluster can also offer (local resources) and consume (foreign resources) at the same time; a possible example is a cluster that offers traditional computing resources to other organizations, while at the same time it consumes GPU computing resources in a foreign cluster.

In other words, the concepts of home/foreign and offering/consuming are orthogonal.


## Peering basics

The peering process in Liqo enables two clusters to *selectively* start sharing resources and services between them.
In this asymmetric process, an **home** cluster can ask for resources and services to a remote **foreign** cluster; bi-directional sharing can be achieved generating a peering request also in the other direction.

When a first cluster asks for a peering, a Peering Request is generated towards the foreign cluster, which may answer with
an Advertisement. The advertisement contains a list of resources (CPUs, memory, persistent storage, services) that the contacted cluster is willing to share with the requester, and (optionally) their cost.

If the requesting cluster accepts the advertisement, it creates a **virtual node** (a sort of *digital twin* of the foreign cluster) and it establishes the proper **network connections** (e.g., secure network tunnels) for the inter-cluster traffic.
The virtual node models the resources available in the foreign cluster and allows the home cluster to schedule pods on it. Instead, the cluster interconnection allows to reach pods (and services) scheduled on the foreign cluster as if they were running on the home infrastructure.

Finally, in order to establish a peering to another cluster, you need to *know another cluster*. Liqo offers multiple mechanisms to [discover other clusters](../configure/discovery): LAN discovery, DNS discovery, Manual discovery.


## Working Modes

Liqo supports two orchestration models, called **Working Modes**.

### Autonomous

In the **_Autonomous_** mode (default), **the cluster uses its own intelligence** for any scheduling decision, i.e. it connects to its local Kubernetes API server and lets the local orchestrator to control the scheduling. This way:

1. It can work as a _stand-alone cluster_, **consuming only its own resources**.
    * Useful if there is no internet connection or there are no available peers.
2. It can connect to multiple peers, both **consuming foreign resources** and **sharing its local resources** to other peers, leveraging at best the Liqo technology.

![Autonomous mode](/images/tray-agent/autonomous-mode.png)


### Tethered

When working in **_Tethered_** mode, the cluster can **peer** to a **single** foreign cluster, such as the one controlling the corporate infrastructure, allowing the foreign orchestrator (i.e., the Kubernetes API server) to fully control the usage of its local resources.
When the tethered mode is on, the local Kubernetes API server becomes uneffective.

This mode is particularly meaningful for single-node clusters (e.g., a laptop running Liqo) when they connect to the enterprise infrastructure, which has the right to control all the computing nodes to their full extent.

When the tethered peering is established:
  * The device turns off its intelligence.
  * The foreign peer, working in _Autonomous_ mode, uses its own _API Server_ and takes full control of the local cluster.
  * Every resource request made by the local cluster is forwarded to the foreign cluster that will perform a proper scheduling. In this way:

      * A device with few resources can leverage additional external power in order to complete its jobs.
      * The enterprise cluster can achieve a more efficient resource allocation, easily sharing its computational power and application logic with the employees' devices, which may result in a significant cost reduction.

When the tethered peering ends, the home cluster intelligence (e.g., the API server running on the laptop) takes back control of the local resources.

![Tethered mode](/images/tray-agent/tethered-mode.png)

> **NOTE**: Since tethering requires only one unidirectional peering, the transition from _Autonomous_ to _Tethered_ mode is allowed only in presence of **at most one peering** in which the local cluster is **offering** resources.

<!-- TODO I am not sure about the last part of the sentence, "in which the local cluster is **offering** resources." I believe it should be "we should have only one peer active", without any other conditions. -->