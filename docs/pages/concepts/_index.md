---
title: "Concepts"
weight: 7
---

## Introduction

Liqo enables resource sharing across Kubernetes clusters.
This section presents the architectural documentation of Liqo, composed of four main sections:

* [Discovery](./discovery): how Liqo detects other clusters to peer with;
* [Peering](./peering): how Liqo exchanges information about the resources (and associated costs) available in the other (_foreign_) cluster, and eventually establishing a _peering_ relationship with it;
* [Networking](./networking): how the network interconnection between different clusters works;
* [Offloading](./offloading): how Kubernetes resources and services present in one cluster are propagated in the peered ones, leveraging the _big cluster_ / _big node_ model based on the Virtual Kubelet.
