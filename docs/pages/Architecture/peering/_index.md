---
title: "Peering"
weight: 2
---

## Overview

The interconnection of two clusters in Liqo is called "peering". Peering is a policy-driven, voluntary, and direct relationship of administratively separate clusters. Liqo leverages peerings as the fundamental step for creating the virtual node and a viable network configuration enabling interconnection between the clusters.

### Step in Liqo Workflow

Periodic Advertisement messages embedding cluster capabilities are periodically sent to other peers; these messages are
then used to build a local virtual-node where jobs can be scheduled: if a job is assigned to a 
virtual-node, it will be actually sent to the respective foreign cluster.
