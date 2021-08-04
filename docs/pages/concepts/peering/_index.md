---
title: "Peering"
weight: 2
---

## Overview

The peering process allows to manage the control plane of the shared resources among different clusters.
Periodic Advertisement messages embedding cluster capabilities are periodically sent to other peers; these messages are
then used to build a local virtual-node where jobs can be scheduled: if a job is assigned to a 
virtual-node, it will be actually sent to the respective foreign cluster.