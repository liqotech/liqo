---
title: Peering
weight: 4
---

## Overview

The last step to perform to join two clusters is peering. It allows establishing a connection between the two 
clusters and to exchange resources.

## Enable peering

The LAN and DNS discovery have the autojoin feature set by default, i.e., once the clusters are discovered and
authenticated, the peering happens automatically. If the foreign cluster has been added through a manual configuration,
you can enable the peering by setting its join flag as follows:

```bash
kubectl patch foreignclusters "$foreignClusterName" \
  --patch '{"spec":{"join":true}}' \
  --type 'merge'
```

## Disable peering

To disable the peering, it is enough to patch the `ForeignCluster` resource as follows:

```bash
kubectl patch foreignclusters "$foreignClusterName" \
  --patch '{"spec":{"join":false}}' \
  --type 'merge'
```
