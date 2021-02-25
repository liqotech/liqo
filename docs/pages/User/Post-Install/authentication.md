---
title: Authentication
weight: 3
---

The inter-cluster authentication procedure makes possible that a cluster can get an Identity (with an associated Role)
that makes possible to create the required resources needed for the clusters' interconnection.

Liqo support two different methods:
* __Empty Token__: any request will be accepted if there is no Identity related to the same clusterID
* __Token Matching__: a request will be accepted only if it contains the correct token that has to be delivered out of band

## Setting the Authentication Method

You can select the authentication method editing the ClusterConfig resource with the following command:
```bash
kubectl edit clusterconfig
```

and changing the field:
```yaml
spec:
  authConfig:
    allowEmptyToken: true
```

## Get and Insert Authentication Credentials

If in the remote cluster the `emptyToken` is disabled, you will see in th home cluster, in the ForeignCluster resource
status, something like:
```yaml
status:
  authStatus: EmptyRefused
```

this means that the Discovery component has made an attempt to get an Identity with an empty token, but the remote cluster
refused it.

### Get the token in the Remote Cluster

If you have the access to the remote cluster, you can get the token required for the authentication running this example
script in the remote cluster:

```bash
token=$(kubectl get secret -n liqo auth-token -o jsonpath="{.data.token}" | base64 -d)
echo -e "Token:\t$token"
```

that will print you something like:
```txt
Token:	502da93c20bb07ff289e4db7f0a9e12e2254a071f37ef6d580070715d38271c2429a4cbe2610202c79062f260eb0de96a881bb3b88eb3cd5222f8238f3e9928e
```

> NOTE: keep it confidential! Everyone with this token can peer with your cluster and use your resources.

### Insert the token to authenticate the Home Cluster

In the home cluster you have to create a Secret containing this token and to add two labels to make it visible to the
Discovery component and linkable to the correct ForeignCluster resource:

* `discovery.liqo.io/cluster-id` has to be set equals to the clusterID of the cluster that we want to peer (you can
  find it in the ForeignCluster CR in `spec.clusterIdentity.clusterID`)
* `discovery.liqo.io/auth-token` has to exist in the secret to tell to Liqo that an authentication token is stored in this secret

An example script that, given the ForeignCluster resource name and the token, create the secret can be:

```bash
#!/bin/bash

set -e

if [ "$#" -ne 1 ]; then
  echo "Usage: authenticate.sh <ForeignCluster CR name>"
  exit 1
fi

fcName="$1"

clusterId=$(kubectl get foreignclusters "$fcName" \
  -o jsonpath="{.spec.clusterIdentity.clusterID}")

echo "Insert token:"
read -r token


# create local secret

secret_name="remote-token-$clusterId"

kubectl create secret generic "$secret_name" \
  -n liqo \
  --from-literal=token="$token"


# label it

kubectl label secret "$secret_name" \
  -n liqo \
  discovery.liqo.io/cluster-id="$clusterId" \
  discovery.liqo.io/auth-token=""


# patch foreign cluster (optional)

kubectl patch foreignclusters "$fcName" \
  --patch '{"status":{"authStatus":"Pending"}}' \
  --type 'merge'
```

## Check the Auth Status

You can check the current Auth status in the ForeignCluster resource status with the command:
```bash
kubectl get foreignclusters.discovery.liqo.io <FC NAME> -o jsonpath="{.status.authStatus}"
```

The result will be one of the following:

| Value          | Description |
| -------------- | ----------- |
| `Pending`      | The request has not been sent or there is still no answer from the remote cluster |
| `EmptyRefused` | An empty token request was sent to the remote cluster, but it has been refused |
| `Accepted`     | The request has been accepted and there is an Identity stored in a local Secret |
| `Refused`      | The request has been refused, no other retries will be done. You can still change the Token Secret and change this filed to `Pending` to restart the process |

![](/images/auth/get_identity_flowchart_complete.png)

## Witch resources will be created during the process?

Some Kubernetes resources will be created in both the clusters involved in this process.

### In the Home Cluster

| Resource | Name                     | Description |
| -------- | ------------------------ | ----------- |
| Secret   | remote-token-$CLUSTER_ID | A secret containing a token to authenticate to a remote cluster    |
| Secret   | remote-identity-*        | A secret containing the identity retrieved from the remote cluster |

> NOTE: these Secret will not be deleted after the ForeignCluster deletion. Do not delete the "remote identit" Secret,
> you will not be able to retrieve it again.

### In the Foreign Cluster

| Resource           | Name               | Description |
| ------------------ | ------------------ | ----------- |
| ServiceAccount     | remote-$CLUSTER_ID | The service account assigned to the home cluster |
| Role               | remote-$CLUSTER_ID | This allows to manage _Secrets_ with a name equals to the clusterID in the `liqoGuestNamespace` (`liqo` by default) |
| ClusterRole        | remote-$CLUSTER_ID | This allows to manage _PeeringRequests_ with a name equals to the clusterID |
| RoleBinding        | remote-$CLUSTER_ID | Link between the Role and the ServiceAccount |
| ClusterRoleBinding | remote-$CLUSTER_ID | Link between the ClusterRole and the ServiceAccount |

> NOTE: delete the ServiceAccount if a remote cluster has lost its Secret containing the Identity, in that way a new
> ServiceAccount will be created are shared with that cluster.
