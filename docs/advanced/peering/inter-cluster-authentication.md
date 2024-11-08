# Inter-cluster Authentication

## Overview

To enable the [**namespace offloading**](/usage/namespace-offloading) along with the [**resource reflection**](/usage/reflection) on a cluster provider, the cluster consumer should be able to interact with the provider K8s API server to create some CRs (`NamespaceOffloading` and `ResourceSlice`), used by the consumer to ask for the authorization to offload on a given namespace and to use a certain amount of resources on the cluster provider.

The authentication process is a handshake between the clusters, allowing the cluster consumer to obtain a valid identity to interact with the other cluster (i.e. access to the Kubernetes API server).
**This identity grants only limited permissions on the Liqo-related resources** used during the offloading process.

The `liqoctl peer` command automatically sets up the network pairing and performs the authentication process.
However, **authentication and resource reflection are independent of the network pairing**, which means that it is possible to offload tasks and reflect resources without setting up the network.

The authentication process can be manually performed using `liqoctl authenticate` command or by manually applying the CRs.

## Authentication via liqoctl authenticate

```{warning}
The aim of the following section is to give an idea of how Liqo works under the hood and to provide the instruments to address the more complex cases.
For the major part of the cases the `liqoctl peer` is enough, so [stick with it](/usage/peer) if you do not have any specific needs.
```

The `liqoctl authenticate` command starts an authentication process, **this command can be used only when the user has access to both the clusters that need to be paired**.

For example, given a *cluster consumer* (which starts the authentication and wants to offload the resources on another cluster) and a *cluster provider* (which offers its resources to the consumer), the following command starts an authentication process between the consumer and the provider:

```{code-block} bash
:caption: "Cluster consumer"
liqoctl authenticate --kubeconfig $CONSUMER_KUBECONFIG_PATH --remote-kubeconfig $PROVIDER_KUBECONFIG_PATH
```

When successful, the command above generates the following output:

```text
 INFO   (local) Tenant namespace correctly ensured
 INFO   (remote) Tenant namespace correctly ensured
 INFO   (remote) Nonce secret ensured
 INFO   (remote) Nonce generated successfully
 INFO   (remote) Nonce retrieved
 INFO   (local) Signed nonce secret ensured
 INFO   (local) Nonce is signed
 INFO   (local) Signed nonce retrieved
 INFO   (local) Tenant correctly generated
 INFO   (remote) Tenant correctly applied on provider cluster
 INFO   (remote) Tenant status is filled
 INFO   (remote) Identity correctly generated
 INFO   (local) Identity correctly applied on consumer cluster
 INFO   (local) Identity status is filled
 ```

At the end of the process, the cluster consumer will have a new identity resource, which represents the identity used by the consumer to ask the provider for resources:

```{code-block} bash
:caption: "Cluster consumer"
kubectl get identities -A
NAMESPACE                 NAME                       AGE   TYPE           KUBECONFIGSECRET
liqo-tenant-cl-provider   controlplane-cl-provider   88s   ControlPlane   kubeconfig-controlplane-cl-provider
```

As it is possible to notice, the `Identity` resource points to a secret, in this case called `kubeconfig-controlplane-cl-provider`, containing the kubeconfig of the newly created user in the cluster provider, with the permissions to manage Liqo-related resources used during the offloading process (`NamespaceOffloading` and `ResourceSlice`) in the `tenant` namespace dedicated to this pairing (`liqo-tenant-cl-provider`).

On the other side, in the cluster provider, we will be able to see a new `Tenant` resource with status `ACTIVE`, saying that the cluster consumer is currently able to ask for new resources:

```{code-block} bash
:caption: "Cluster provider"
kubectl get tenant
NAME   AGE   CONDITION
cl01   16m   Active
```

Check the [offloading guide](/advanced/peering/offloading-in-depth) to understand how to start the pod offloading and the reflection of the resources.

### In-Band

If you can't make the Kubernetes API Server of the **Provider** cluster reachable from the **Consumer**, you can leverage on **in-band** peering.
You can enable this by setting the `--in-band` flag in the `liqoctl authenticate` command, which automatically configure all the features needed for this mechanism to work (i.e., the API server proxy and the IP remapping).

```{admonition} Note
For this feature to work, the Liqo **networking module** must be enabled.
```

### Undo the authentication

`liqoctl unauthenticate` allows to undo the changes applied by the `authenticate` command. Even in this case, the user should be able to access both the involved clusters.

For example, given the previous case, if the user wants to undo the authentication between cluster consumer and provider, the following command can be used:

```{code-block} bash
:caption: "Cluster consumer"
liqoctl unauthenticate --kubeconfig $CONSUMER_KUBECONFIG_PATH --remote-kubeconfig $PROVIDER_KUBECONFIG_PATH
```

When successful, **the identity** used to operate on the cluster provider, **and the tenant resource** on the provider **are removed**. Therefore, from this point on, the cluster consumer is not authorized to offload and reflect resources on the provider.

## Manual authentication

```{warning}
The aim of the following section is to give an idea of how Liqo works under the hood and to provide the instruments to address the more complex cases. For the major part of the cases the `liqoctl peer` is enough, so [stick with it](/usage/peer) if you do not have any specific needs.
```

When the user has no access to both the clusters, it is possible to perform the authentication by calling some commands in each of the clusters.

The authentication process consists of the following steps (where we call *consumer* the cluster offloading resources, *provider* the one offering resources):

1. **Cluster provider**: generates a nonce to be signed
2. **Cluster consumer**: generates a `Tenant` resource to be applied on the provider cluster. It contains:
    - the provider's signed nonce (each cluster in Liqo has a pair of keys and certificate, at the moment the certificate is self-signed and created at installation time)
    - a *CSR* for the certificate to use to authenticate to the provider's K8S API server
3. **Cluster provider**: applies the consumer-generated `Tenant` resource and if the signature of the nonce is valid, the consumer-provided CSR is signed, and it is possible to generate an `Identity` resource containing the signed certificate.
4. **Cluster consumer**: applies the provider-generated `Identity`, which will trigger the automatic creation of the `kubeconfig` to interact with the provider K8S API server.

### Nonce and tenant namespace generation (cluster provider)

In this step, starting from the cluster provider, we need to:

- Create a **tenant namespace**, which will be used for the resource related to new peering;
- Create a secret to contain the nonce

These operations can be performed via `liqoctl`, executing the following command **from the cluster provider**:

```{code-block} bash
:caption: "Cluster provider"
liqoctl create nonce --remote-cluster-id $CLUSTER_CONSUMER_ID
```

When successful the output of the command will be something like the following:

```text
 INFO  Nonce created
 INFO  Nonce generated successfully
 INFO  Nonce retrieved: 5IDZQiNG7EUv+GPmS5RvzcTAo8ksSfM3LUTFaBYnPfvY5WndhNGkJVFOFmTVc0ArT4B/FkH6QO7Kfhk6Q1B4ww==
```

```{admonition} Tip
Take note of the **nonce** as it can will be used in the following step.
```

As a result of this command, you should be able to see:

- a new **tenant namespace**:

  ```{code-block} bash
  :caption: "Cluster provider"
  $ kubectl get namespaces -l liqo.io/tenant-namespace=true -l liqo.io/remote-cluster-id=cl-consumer

  NAME                      STATUS   AGE
  liqo-tenant-cl-consumer   Active   4m7s
  ```

- inside this namespace, a secret containing the nonce:

  ```{code-block} bash
  :caption: "Cluster provider"
  $ kubectl get secret -n liqo-tenant-cl-consumer

  NAME         TYPE     DATA   AGE
  liqo-nonce   Opaque   1      5m5s
  ```

### Creation of the Tenant resource (cluster consumer)

Once on the cluster provider side, the nonce has been generated, it is possible to create the Tenant resource. This involves:

- The sign of nonce with the cluster consumer private key
- The creation of a *CSR* for the certificate to be used to authenticate to the cluster provider K8S API server

To do so, **on the cluster consumer**, launch the following `liqoctl` command:

```{code-block} bash
:caption: "Cluster consumer"
liqoctl generate tenant --remote-cluster-id $CLUSTER_PROVIDER_ID --nonce $NONCE
```

As a result, the command above **generates a `Tenant` resource to be applied on the cluster provider**, which contains the signed nonce and the CSR:

```yaml
apiVersion: authentication.liqo.io/v1beta1
kind: Tenant
metadata:
  creationTimestamp: null
  labels:
    liqo.io/remote-cluster-id: cl-consumer
  name: cl-consumer
spec:
  clusterID: cl-consumer
  csr: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURSBSRVFVRVNULS0tLS0KTUlHZ01GUUNBUUF3SVRFUU1BNEdBMVVFQ2hNSGJHbHhieTVwYnpFTk1Bc0dBMVVFQXhNRVkyd3dNVEFxTUFVRwpBeXRsY0FNaEFPTDlqaWg5clpkNXVPelVadkk1LytWZ2hWbFlnb29EaDhlQ3hXak1YMEZzb0FBd0JRWURLMlZ3CkEwRUFQbytpenBBRXljM2FsNjYyZUhBZ3RyQXlHenozelFpVzA0ZHQzazNJYXNWdVBpcE1wSnBmQnA5aEk1YWYKM0c2a0Y3cGVjWW5zeThYWUlqVyt6QnNORHc9PQotLS0tLUVORCBDRVJUSUZJQ0FURSBSRVFVRVNULS0tLS0K
  publicKey: 4v2OKH2tl3m47NRm8jn/5WCFWViCigOHx4LFaMxfQWw=
  signature: K5GP+UkTKJ5ao02l8RBUJLwaUPp+gE85HtCYdJ2i2g8IW/pkmBmzjdAw1RujU3//Mnp+zdgExlQA2MWiv7jTBw==
```

As a result of the command, you should be able to see:

- a new **tenant namespace**:

  ```{code-block} bash
  :caption: "Cluster provider"
  $ kubectl get namespaces -l liqo.io/tenant-namespace=true -l liqo.io/remote-cluster-id=cl-provider

  NAME                      STATUS   AGE
  liqo-tenant-cl-provider   Active   4m7s
  ```

- inside the namespace, a secret containing the **signed** nonce.

  ```{code-block} bash
  :caption: "Cluster consumer"
  $ kubectl get secrets -n liqo-tenant-cl-provider

  NAME                TYPE     DATA   AGE
  liqo-signed-nonce   Opaque   2      78s
  ```

```{admonition} Note
If you want to use the [in-band](UsagePeeringInBand) approach, use the `spec.proxyURL` field inside the `Tenant` CRD.
Check the [Kubernetes API Server Proxy](/advanced/k8s-api-server-proxy.md) page
```

### Creation of the Identity resource (cluster provider)

At this point, **on the cluster provider**, it is possible to apply the `Tenant` resource previously generated on the cluster consumer.

```{code-block} bash
:caption: "Cluster provider"
kubectl apply -f $TENANT_RESOURCE_YAML_PATH
```

Once the resource is applied, the CSR is signed by the cluster provider:

```bash
$ kubectl get csr
NAME                  AGE     SIGNERNAME                            REQUESTOR                                            REQUESTEDDURATION   CONDITION
liqo-identity-k47v5   2m59s   kubernetes.io/kube-apiserver-client   system:serviceaccount:liqo:liqo-controller-manager   <none>              Approved,Issued
```

Once signed, we have the certificate to be used by the cluster consumer to authenticate to the provider's K8S API server. So, it is possible to generate the `Identity` resource to be applied on the consumer:

```{code-block} bash
:caption: "Cluster provider"
liqoctl generate identity --remote-cluster-id $CLUSTER_CONSUMER_ID
```

When successful, the command above generates an Identity resource like the following:

```yaml
apiVersion: authentication.liqo.io/v1beta1
kind: Identity
metadata:
  creationTimestamp: null
  labels:
    liqo.io/remote-cluster-id: cl-provider
  name: controlplane-cl-provider
spec:
  authParams:
    apiServer: https://172.19.0.3:6443
    ca: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSURCVENDQWUyZ0F3SUJBZ0lJU3V5THpXWVRWWTB3RFFZSktvWklodmNOQVFFTEJRQXdGVEVUTUJFR0ExVUUKQXhNS2EzVmlaWEp1WlhSbGN6QWVGdzB5TkRBM01UVXdPVEUwTkRKYUZ3MHpOREEzTVRNd09URTVOREphTUJVeApFekFSQmdOVkJBTVRDbXQxWW1WeWJtVjBaWE13Z2dFaU1BMEdDU3FHU0liM0RRRUJBUVVBQTRJQkR3QXdnZ0VLCkFvSUJBUURHMEZyQ3VndlhQZzdmR3VBWmZXSFRObzIvdEFnaFJQUGJ6ZVZEN0M3VFhJQ0ExNjNDd2x0cy9CZWsKanpXWUVNQWtSTEFseXR3QlBGbEdjMVRsL21vQS9xWXJHcWkyU2NjRkxNTWNlV3VwSkZMOEVOOGN3VW1Wck4ydQpSTEgzYWJmYTdSR2Z2cHdvVUlsaFRSd2lOUE50Q0VYSWRNbUg4RHVGdCtnaGhKMXg3N29qWlRjL29lell6UENRCjV3cmJRNC8rZzdGM01aM2xCWWNvS1pkcXhLdGFwZXhxWkUydEhBYTBSYzZHVCthODk2MnNCUVZrUU9LcWg0TGoKemU3Mjg5NHVYNzBWRHEyQmR6bWFFa2Q4Wml0QkZrY0lnV0xpOFJPbDJSQ0FZV3BPcUdUNnByZEhGcmZxaHhJcQpOOFZibVhRNEVIeHR5bnppSDcwUWwwdnNIUU5sQWdNQkFBR2pXVEJYTUE0R0ExVWREd0VCL3dRRUF3SUNwREFQCkJnTlZIUk1CQWY4RUJUQURBUUgvTUIwR0ExVWREZ1FXQkJUY0ZqNCszQ0F2WkhCdTRNdy8zN3krc3M2SlpEQVYKQmdOVkhSRUVEakFNZ2dwcmRXSmxjbTVsZEdWek1BMEdDU3FHU0liM0RRRUJDd1VBQTRJQkFRQUoxeTE2SUY1UwpDOW1JTk1PKzNRR002bkM1Y0RLeGdQakZQblltMnlsZUoxazJaYWFBeStQM0xqNWdTVHNLakpMQ2tGWWo1aUU0ClhjeUlWdVo2OHRYOC9vQkV4c2ppVUhQbHBjZks2UlhLYVNoUnlUMUdoV0xDNUpvWExnL3dic1I2bDdpWmtIaUgKZkR2TElIbE9oNDUwZ3RONnR0NWR6VHVzZzI1VDZTRzV3Ryt4aWRQbUpBeVFHZDY3UjNkYWRVNUY0YzdkdGVjYgpxUzM2MXlWdDVYZXJiRGE5cHBYbDJsbVIwVTc1eTFMdldQWEpBa2RMSTBBQlNjMjNnendNWlN6SHhhclg3Q2M4Ci91QVpYMXl2Y2RoRHZaajBoU2ZWb2o5d1N1aitDNTJaRENTT29QVEcrNk1xQ1FlYVJCZ282YkJUS2FXVzNWTXEKY29RQ0pNVnI1OXMwCi0tLS0tRU5EIENFUlRJRklDQVRFLS0tLS0K
    signedCRT: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUNIVENDQVFXZ0F3SUJBZ0lSQUlDbGtUMmdDbjdKZVVvd2FwT1ZUc013RFFZSktvWklodmNOQVFFTEJRQXcKRlRFVE1CRUdBMVVFQXhNS2EzVmlaWEp1WlhSbGN6QWVGdzB5TkRBM01qSXdOalF5TkRGYUZ3MHlOVEEzTWpJdwpOalF5TkRGYU1DRXhFREFPQmdOVkJBb1RCMnhwY1c4dWFXOHhEVEFMQmdOVkJBTVRCR05zTURFd0tqQUZCZ01yClpYQURJUURpL1k0b2ZhMlhlYmpzMUdieU9mL2xZSVZaV0lLS0E0Zkhnc1ZvekY5QmJLTldNRlF3RGdZRFZSMFAKQVFIL0JBUURBZ1dnTUJNR0ExVWRKUVFNTUFvR0NDc0dBUVVGQndNQ01Bd0dBMVVkRXdFQi93UUNNQUF3SHdZRApWUjBqQkJnd0ZvQVUzQlkrUHR3Z0wyUndidURNUDkrOHZyTE9pV1F3RFFZSktvWklodmNOQVFFTEJRQURnZ0VCCkFMVU5xVS9DMXk2S0ZXU1R3UjgrSUV6T3NzL3hrTURsVnlvdk9UUnYwR051bWk3L2l1T25zN3dZZEtaekpkbFoKTzdKZWJ2WjVKa3hTMWltblhkQk5QUHZrdXpHMGxHNHkzTXRWblkyTnJMaVZFOWh4SE1XYjZ3SU5yVUQzUmJOLwp6eHoxaHU4WUlwSGo4Zk0vWVJRejJpY01Eb1VIZmc1VXlyS1FIQnFDRlhNWVFaQmNJdnlVV0E1Q3g5cTNmS1VqCkNMaE1lZVlDSmxHWlhnd2RBNy9DYWdWMFpqTTdQYUJYZ29PUVdTSzdtRWlWa0VsN3dRVGlGL1FFcGhaTXVCcHMKK2lrQ1J1cTNBVVBuSnFVQWRvL3lYRHJla3YzTHhqN1VUQ1R5QlJMUVc3eThrdTF2T2ZGekszaDcvL05TcThESQpLZVdBSmN6NXRWdk1oc2ZTbGRsZVU5MD0KLS0tLS1FTkQgQ0VSVElGSUNBVEUtLS0tLQo=
  clusterID: cl-provider
  namespace: liqo-tenant-cl-consumer
  type: ControlPlane
status: {}
```

Which can be applied **on the cluster consumer**, making sure that the resource is created in the tenant namespace (`liqo-tenant-cl-consumer` in this case):

```{code-block} bash
:caption: "Cluster consumer"
kubectl apply -f $IDENTITY_RESOURCE_YAML_PATH -n liqo-tenant-cl-consumer
```

Once the Identity resource is correctly applied, the clusters are able to negotiate the resources for the [offloading](/advanced/peering/offloading-in-depth).

### Summary

All in all, these are the steps to be followed by the administrators of each of the clusters to manually complete the authentication process:

1. **Cluster provider**: creates the nonce to be provided to the **cluster consumer** administrator:

   ```bash
   liqoctl create nonce --remote-cluster-id $CLUSTER_CONSUMER_ID
   liqoctl get nonce --remote-cluster-id $CLUSTER_CONSUMER_ID > nonce.txt
   ```

2. **Cluster consumer**: generates the `Tenant` resource to be applied by the **cluster provider**:

   ```bash
   liqoctl generate tenant --remote-cluster-id $CLUSTER_PROVIDER_ID --nonce $(cat nonce.txt) > tenant.yaml
   ```

3. **Cluster provider**: applies `tenant.yaml` and generates the `Identity` resource to be applied by the consumer:

   ```bash
   kubectl apply -f tenant.yaml
   liqoctl generate identity --remote-cluster-id $CLUSTER_CONSUMER_ID > identity.yaml
   ```

4. **Cluster consumer** applies `identity.yaml` in the tenant namespace:

   ```bash
   kubectl apply -f identity.yaml -n $TENANT_NAMESPACE
   ```

You can check whether the procedure completed successfully by checking [the peering status](../../usage/peer.md#check-status-of-peerings).
