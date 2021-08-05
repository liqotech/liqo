---
title: Set up the authentication
weight: 2
---

## Introduction

The Liqo **authentication mechanism** allows your cluster to control who can peer to it. This is particularly important 
if your cluster exposes its services to the Internet, hence avoiding that unknown organizations establish a peering with 
your cluster. The Authentication is similar to the bootstrap TLS: a unique secret is used to get an identity to be 
authenticated with.

### Peer with a new cluster

To peer with a new cluster, you have to create a ForeignCluster CR.

#### Add a new ForeignCluster

A `ForeignCluster` resource needs the authentication service URL and the port to be set: it is the backend of the
authentication server (mandatory to peer with another cluster).

The address is or the __hostname__ or the __IP address__ where it is reachable.
If you specified a name during the installation, it is reachable through an Ingress (you can get it with `kubectl get
ingress -n liqo`), if an ingress is not configured, the other cluster is exposed with a NodePort Service, you can get 
one if the IPs of your cluster's nodes (`kubectl get nodes -o wide`).

The __port__ where the remote cluster's auth service is reachable, if you are
using an Ingress, it should be `443` by default. Otherwise, if you are using a NodePort Service you 
can get the port executing `kubectl get service -n liqo liqo-auth`, an output example could be:

```txt
NAME           TYPE       CLUSTER-IP    EXTERNAL-IP   PORT(S)         AGE
liqo-auth   NodePort   10.81.20.99   <none>        443:30740/TCP   2m7s
```

An example of `ForeignCluster` resource can be:

```yaml
apiVersion: discovery.liqo.io/v1alpha1
kind: ForeignCluster
metadata:
  name: my-cluster
spec:
  outgoingPeeringEnabled: "Yes"
  foreignAuthUrl: "https://<ADDRESS>:<PORT>"
```

When you create the ForeignCluster, the Liqo control plane will contact the `foreignAuthUrl` (i.e. the public URL of a cluster 
authentication server) to retrieve all the required cluster information.

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

### 1. Get the home secure token

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

### 2. Add the secure token for a foreign cluster

In the home cluster you have to provide the token to Liqo.

To perform this operation:
1. fetch the cluster-id from the ForeignCluster resource
2. Create the secret resource in the home cluster and label it.

#### 2.1 Fetch the foreign cluster-id

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