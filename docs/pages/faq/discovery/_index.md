---
title: Discovery 
weight: 4
---

### Which resources will be created during the process?

Some Kubernetes resources will be created in both the clusters involved in this process.

#### In the Home Cluster

| Resource | Name                     | Description |
| -------- | ------------------------ | ----------- |
| Secret   | remote-token-$FOREIGN_CLUSTER_ID | A secret containing a token to authenticate to a remote cluster    |
| Secret   | remote-identity-*        | A secret containing the identity retrieved from the remote cluster |

> NOTE: these Secret will not be deleted after the ForeignCluster deletion. Do not delete the "remote identit" Secret,
> you will not be able to retrieve it again.

#### In the Foreign Cluster

| Resource           | Name               | Description |
| ------------------ | ------------------ | ----------- |
| ServiceAccount     | remote-$FOREIGN_CLUSTER_ID | The service account assigned to the home cluster |
| Role               | remote-$FOREIGN_CLUSTER_ID | This allows to manage _Secrets_ with a name equals to the clusterID in the `liqoGuestNamespace` (`liqo` by default) |
| ClusterRole        | remote-$FOREIGN_CLUSTER_ID | This allows to manage _PeeringRequests_ with a name equals to the clusterID |
| RoleBinding        | remote-$FOREIGN_CLUSTER_ID | Link between the Role and the ServiceAccount |
| ClusterRoleBinding | remote-$FOREIGN_CLUSTER_ID | Link between the ClusterRole and the ServiceAccount |

> NOTE: delete the ServiceAccount if a remote cluster has lost its Secret containing the Identity, in that way a new
> ServiceAccount will be created are shared with that cluster.