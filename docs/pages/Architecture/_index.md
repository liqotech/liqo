---
title: "Architecture"
weight: 2
---

## Introduction

Liqo is a framework to create easy-to-setup multi-cluster topologies. More precisely, Liqo interconnects multiple clusters by creating a (1) virtual node to offload workloads and (2) an ad-hoc VPN to let be routed across clusters. This section presents the architectural documentation of Liqo by (1) detailing the Liqo wokrflow and (2) presenting its main components.

In the schema presented below, we can identify the 5 steps of Liqo Workflow:

![Liqo Workflow](/images/architecture/LiqoWorkflow.png)

1. [Discovery](./discovery): detection and characterization of other clusters to peer with.;
2. [Peering](./peering): resource sharing control plane;
3. [Network Interconnection](./networking): interconnection between different clusters.
4. [Resource Management](./computing): resource sharing data plane;
5. **Usage**
