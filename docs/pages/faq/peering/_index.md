---
title: Peering 
weight: 4
---
## Access the cluster configurations

You can get the cluster configurations exposed by the Auth Service endpoint of the other cluster. This allows retrieving
the information necessary to peer with the remote cluster.

```bash
curl --insecure https://<ADDRESS>:<PORT>/ids
```

```json
{"clusterId":"0558de48-097b-4b7d-ba04-6bd2a0f9d24f","clusterName":"LiqoCluster0692","guestNamespace":"liqo"}
```
