---
title: Authentication
weight: 3
---

## Introduction

The Liqo **authentication mechanism** allows your cluster to control who can peer to it. This is particularly important 
if your cluster exposes its services to the Internet, hence avoiding that unknown organizations establish a peering with 
your cluster. The Authentication is similar to the bootstrap TLS: a unique secret is used to get an identity to be 
authenticated with.

##  Disable the authentication

The authentication in Liqo is enabled by default; in some environments, such as playgrounds or development contexts, you
may want to disable it. To do so, use the following command:

```bash
kubectl patch clusterconfig liqo-configuration --patch '{"spec":{"authConfig":{"allowEmptyToken": true}}}' --type 'merge'
```

> __NOTE__: Disabling authentication will automatically accept peering with any other Liqo instances in the network your 
cluster is exposed to.

## Authentication mechanism

The inter-cluster authentication is on a 2-step basis:
1. Get the authentication token of the foreign cluster.
2. Create a new secret in the home cluster with the authentication token and label it. This operation
will trigger the following authentication procedure:
    1. The discovery component posts the authentication token to the authentication server
    2. The authentication server compares the received token with the correct one; if the two are matching, the
peering cluster is authenticated.
    3. the authentication server creates a set of permissions, forges an identity bound to them, and gives it to the 
peering cluster.

This new Identity will be uniquely assigned to who made the request, giving him per-user access, only with permissions 
on its resources. It will be used for any future request to the API Server once the peering will be enabled.

Below, the 2 steps are detailed:

### 1. Get the foreign cluster token

> __NOTE__: Since a secret token is required for peering, you can authenticate with another cluster if and only if you
> have access to that cluster. Keep the secret confidential! Everyone with that token can peer with your cluster and use
> your resources.

To get the authentication token of the foreign cluster, set the kubeconfig to use the foreign cluster and type:

```bash
token=$(kubectl get secret -n liqo auth-token -o jsonpath="{.data.token}" | base64 -d)
echo "Token: $token"
```

The output should be similar to:

```txt
Token: 502da93c20bb07ff289e4db7f0a9e12e2254a071f37ef6d580070715d38271c2429a4cbe2610202c79062f260eb0de96a881bb3b88eb3cd5222f8238f3e9928e
```

### 2. Create a secret in the home cluster

In the home cluster you have to provide the token to Liqo.

To perform this operation:
1. fetch the cluster-id from the ForeignCluster resource
2. Create the secret resource in the home cluster and label it.

#### 1.1 Fetch the foreign cluster-id

Each Liqo cluster is uniquely identified by a cluster-id. Once a new Liqo cluster has been discovered, a new 
ForeignCluster resource is created in your cluster. The cluster-id of the foreign cluster is part of the specific 
ForeignCluster. To get the id of the cluster you want to authenticate with, identify the corresponding ForeignCluster 
and type:

```bash
FOREIGN_CLUSTER_ID=$(kubectl get foreigncluster <foreign-cluster> -o=jsonpath="{['spec.clusterIdentity.clusterID']}")
```

>__NOTE__: in v0.2 the foreign cluster resource name is the cluster-id, therefore it is enough to list the
>ForeignClusters and get the desired one's name. This will change in future releases.

#### 1.2 Create the secret

Once you identified the cluster-id of the cluster you want to authenticate with, create the secret with the 
authentication token as follows:

First create a script by running:

```bash
cat >authenticate.sh <<'EOL'
#!/bin/bash

set -e

if [ "$#" -ne 1 ]; then
  echo "Usage: authenticate.sh <ForeignCluster CR name>"
  exit 1
fi

liqoNamespace="${NAMESPACE:-liqo}"
fcName="$1"

clusterId=$(kubectl get foreignclusters "$fcName" \
  -o jsonpath="{.spec.clusterIdentity.clusterID}")

echo "Insert token:"
read -r token


# create local secret

secret_name="remote-token-$clusterId"

kubectl create secret generic "$secret_name" \
  -n "$liqoNamespace" \
  --from-literal=token="$token"


# label it

kubectl label secret "$secret_name" \
  -n "$liqoNamespace" \
  discovery.liqo.io/cluster-id="$clusterId" \
  discovery.liqo.io/auth-token=""


# patch foreign cluster (optional)

kubectl patch foreignclusters "$fcName" \
  --patch '{"status":{"authStatus":"Pending"}}' \
  --type 'merge'

EOL

chmod +x authenticate.sh
```

then, launch the script providing the foreign cluster-id:
```bash
./authenticate.sh $FOREIGN_CLUSTER_ID
```

You can see the created secret by typing:

```bash
kubectl get secret -n liqo -l discovery.liqo.io/auth-token --show-labels 

NAME                                                TYPE     DATA   AGE    LABELS
remote-token-3114c478-173d-4344-8f01-eb21efb95aea   Opaque   1      100s   discovery.liqo.io/auth-token=,discovery.liqo.io/cluster-id=3114c478-173d-4344-8f01-eb21efb95aea
```

The secret creation triggers authentication procedure of the discovery component: it will post the authentication token
embedded in the secret previously forged to the authentication server of the foreign cluster. 

## Check the Auth Status

The outcome of the authorization procedure can be found in the corresponding foreignCluster resource by typing:

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

## Troubleshooting

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