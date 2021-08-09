---
title: Authentication
weight: 3
---

## Introduction

The Liqo **authentication mechanism** allows your cluster to control who can peer to it. This is particularly important if your cluster exposes its services to the Internet, hence avoiding that unknown organizations can establish a peering with your cluster. The authentication is similar to the bootstrap TLS: a unique secret is used to get an identity to be authenticated with.

## Disable the authentication

Liqo enables the authentication by default; in some environments, such as playgrounds or development contexts, you may want to disable it. To do so, use the following command:

```bash
kubectl patch clusterconfig liqo-configuration --patch '{"spec":{"authConfig":{"enableAuthentication": false}}}' --type 'merge'
```

{{% notice note %}}
When you disable the authentication, your cluster will automatically accept peering with any other Liqo instances in the network where it is exposed to.
{{% /notice %}}

## Authentication mechanism

The inter-cluster authentication is on a 2-step basis:

1. Get the authentication token from the foreign cluster.
2. Create a new secret in the home cluster with the authentication token and label it. This operation will trigger the following authentication procedure:
   1. The local discovery component forges a private key and a new certificate signing request (CSR)
   2. The local discovery component posts the authentication token and the CSR to the remote authentication server
   3. The remote authentication server compares the received token with the correct one. If the two values are equal, it issues a new certificate for that identity that it will return to the local cluster.

The identity certificate is uniquely assigned to the local cluster, giving him per-user access. Liqo generates this certificate using the local clusterID as the CommonName and _liqo.io_ as the organization. Kubernetes will bind these values to the [respective user and group](https://kubernetes.io/docs/reference/access-authn-authz/authentication/#x509-client-certs).

When a Liqo cluster accepts a new identity request, it creates a new [Tenant](https://github.com/clastix/capsule/blob/master/docs/operator/references.md#customer-resource-definition) to handle its permission and to ensure isolation over multiple peering.

### 1. Get the authentication token

{{% notice note %}}
Since a secret token is required for peering, you can authenticate with another cluster if and only if you have access to that cluster. Keep the secret confidential! Everyone with that token can peer with your cluster and use your resources.
{{% /notice %}}

To get the authentication token from the cluster, set the kubeconfig to use the right cluster and type:

```bash
token=$(kubectl get secret -n liqo auth-token -o jsonpath="{.data.token}" | base64 -d)
echo "Token: $token"
```

The output should be similar to:

```txt
Token: 502da93c20bb07ff289e4db7f0a9e12e2254a071f37ef6d580070715d38271c2429a4cbe2610202c79062f260eb0de96a881bb3b88eb3cd5222f8238f3e9928e
```

### 2. Create a secret in another cluster

In the other cluster, you have to provide the token to Liqo.

When you identified the name of the ForeignCluster resource you want to authenticate with, create the authentication secret as follows:

First, create a script by running:

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
EOL

chmod +x authenticate.sh
```

then, launch the script providing the foreign cluster-id:
```bash
./authenticate.sh $FOREIGN_CLUSTER_NAME
```

You can see the created secret by typing:

```bash
kubectl get secret -n liqo -l discovery.liqo.io/auth-token --show-labels 

NAME                                                TYPE     DATA   AGE    LABELS
remote-token-3114c478-173d-4344-8f01-eb21efb95aea   Opaque   1      100s   discovery.liqo.io/auth-token=,discovery.liqo.io/cluster-id=3114c478-173d-4344-8f01-eb21efb95aea
```

The secret creation triggers the authentication procedure of the discovery component: it will post the authentication token embedded in the secret previously forged to the authentication server of the foreign cluster.

## Check the Auth Status

The outcome of the authorization procedure can be found in the corresponding foreignCluster resource by typing:

```bash
kubectl get foreignclusters.discovery.liqo.io <FC NAME> -o jsonpath="{.status.peeringConditions[?(@.type == 'AuthenticationStatus')].status} {.status.peeringConditions[?(@.type == 'AuthenticationStatus')].reason}"
```

Expected output:

```txt
Established IdentityAccepted
```

The result will be one of the following:

| Value             | Description |
| ----------------- | ----------- |
| `None` (or empty) | The request has not been sent yet |
| `Pending`         | There is still no answer from the remote cluster |
| `EmptyDenied`     | Liqo sent an empty token request to the remote cluster, but it has been denied |
| `Established`     | The request has been accepted, and there is an Identity stored in a local Secret |
| `Denied`          | The request with the provided token has been refused |

![](/images/auth/get_identity_flowchart_complete.png)

## Troubleshooting

### Which resources will be created during the process?

Some Kubernetes resources will be created in both the clusters involved in this process.

#### In the Local Cluster

| Resource | Name                             | Namespace | Description |
| -------- | -------------------------------- | --------- | ----------- |
| Secret   | remote-token-$FOREIGN_CLUSTER_ID | `liqo`    | A secret containing a token to authenticate to a remote cluster    |
| Secret   | remote-identity-*                | The Liqo Tenant Namespace associated with this clusterID (i.e. `liqo-tenant-$FOREIGN_CLUSTER_ID`) | A secret containing the identity retrieved from the remote cluster |

{{% notice note %}}
These Secret will not be deleted after the ForeignCluster deletion. Do not delete the "remote identity" Secret: you will not be able to retrieve it again.
{{% /notice %}}

#### In the Remote Cluster

| Resource           | Namespace               | Description |
| ------------------ | ----------------------- | ----------- |
| RoleBinding        | The Liqo Tenant Namespace associated with this clusterID (i.e. `liqo-tenant-$FOREIGN_CLUSTER_ID`) | Link between the ClusterRole and the Identity, it provides the basic permission over peering resources (i.e. `ResourceRequest`) in the related namespace |
