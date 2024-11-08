# Declarative peering

Declarative peerings are supported starting from Liqo 1.0, which means that you can create a set of CRs describing the peering with a remote cluster, which is automatically set up once the CRs are applied on both sides.
This simplifies automation, GitOps and continuous delivery: for example, you might have your Git repository with the manifests describing the peerings, and an instance of [ArgoCD](https://argo-cd.readthedocs.io) which synchronizes the changes on the clusters, creating and destroying the peerings.

This documentation page analizes how to declaratively configure each of the Liqo modules:

- Networking
- Authentication
- Offloading

## Tenant namespace

Before starting to configure the peerings, you need to create a tenant namespace in both clusters that you need to peer.
This tenant namespace must **refer to the peering with a specific cluster**, hence a distinct tenant namespace must be created per each peering.
**All the resources needed to configure the peering must be created in those namespaces**.
A tenant namespace **can have an arbitrary name**, but **it must have the following labels**:

```text
liqo.io/remote-cluster-id: <PEER_CLUSTER_ID>
liqo.io/tenant-namespace: "true"
```

Where `PEER_CLUSTER_ID` is the cluster id of the peer cluster defined at installation time, it can be obtained by launching the following command on the **remote cluster**:

`````{tab-set}
````{tab-item} liqoctl

```bash
liqoctl info --get clusterid
```
````
````{tab-item} kubectl

```bash
kubectl get configmaps -n liqo liqo-clusterid-configmap \
  --template {{.data.CLUSTER_ID}}
```
````
`````

### Configuring the tenant namespace on consumer cluster

The following is an example of tenant namespace named `liqo-tenant-cl-provider`, which refers to the peering with a cluster with id `cl-provider`:

```yaml
apiVersion: v1
kind: Namespace
metadata:
  labels:
    liqo.io/remote-cluster-id: cl-provider
    liqo.io/tenant-namespace: "true"
  name: liqo-tenant-cl-provider
spec: {}
```

### Configuring the tenant namespace on the provider cluster

The following is an example of tenant namespace on the provider cluster, named `liqo-tenant-cl-consumer`, which refers to the peering with the consumer cluster with id `cl-consumer`:

```yaml
apiVersion: v1
kind: Namespace
metadata:
  labels:
    liqo.io/remote-cluster-id: cl-consumer
    liqo.io/tenant-namespace: "true"
  name: liqo-tenant-cl-consumer
spec: {}
```

```{admonition} Note
When peering is configured, the tenant namespace names of provider and consumer clusters do not need to match or follow any specific pattern
```

## Declarative network configuration

By default, the network connection between clusters is established using a secure channel created via [Wireguard](https://www.wireguard.com/).
In this case, one cluster (usually the provider) needs to host a server gateway that exposes a UDP port that must be reachable from the client gateway (usually running on the consumer cluster).

In this guide, **we will configure the client gateway on the consumer cluster and the server gateway on the provider cluster**, which is the most common setup.

However, given that the setup of the network peering is independent from the offloading role of the cluster (i.e., consumer vs. provider), you may choose to invert the client/server roles in case this is more convenient for your setup.

### Creating and exchanging the network configurations (both clusters)

The clusters that needs to be connected requires the network configuration of the peer cluster, which is provided via the `Configuration` CR.
You can check an example of the resources to apply at the [following documentation page](./inter-cluster-network.md#definition-of-the-network-configuration-configuration-crds).

**The Configuration resource should be applied in both clusters** and it **must contain the pod and external CIDR of the peer cluster**. (cluster A has the CIDR config of B and vice versa).

The CIDRs are defined at installation time, you can check the values of the pod and the external CIDR configured in the local cluster by running:

```bash
kubectl get networks.ipam.liqo.io -n liqo external-cidr -o=jsonpath={'.status.cidr'}

10.70.0.0/16
```

### Creating and exchanging the Wireguard keys (both clusters)

To enable authentication and encryption, Wireguard requires a key pair, one for each gateway.

Those keys can be created [via the Wireguard utility tool](https://www.wireguard.com/quickstart/#key-generation) or via OpenSSL like so:

```bash
openssl genpkey -algorithm X25519 -outform der -out private.der
openssl pkey -inform der -in private.der -pubout -outform der -out public.der

# Get the Wireguard private key
echo "Private key:"; cat private.der | tail -c 32 | base64
# Get the Wireguard public key
echo "Public key:"; cat public.der | tail -c 32 | base64
```

At this point, you need to create **in each cluster** a secret containing this pair of keys:

```yaml
apiVersion: v1
kind: Secret
metadata:
  labels:
    liqo.io/remote-cluster-id: <REMOTE_CLUSTER_ID>
  name: gw-keys
  namespace: <TENANT_NAMESPACE>
type: Opaque
data:
  privateKey: <WIREGUARD_PRIVATE_KEY>
  publicKey: <WIREGUARD_PUBLIC_KEY>
```

where WIREGUARD_PRIVATE_KEY and WIREGUARD_PUBLIC_KEY are the previously generated key pairs for that specific cluster (**each cluster has its pair of keys**).

Additionally, each cluster should have a PublicKey resource **with the public key of the peer cluster** (cluster A has the public key of B and vice versa):

```yaml
apiVersion: networking.liqo.io/v1beta1
kind: PublicKey
metadata:
  labels:
    liqo.io/remote-cluster-id: <HERE_THE_CLUSTER_ID_OF_PEER_CLUSTER>
    networking.liqo.io/gateway-resource: "true"
  name: gw-publickey
  namespace: <TENANT_NAMESPACE>
spec:
  publicKey: <REMOTE_WIREGUARD_PUBLIC_KEY>
```

In order to make things work, make sure that the PublicKey resource has the labels:

```yaml
liqo.io/remote-cluster-id: <HERE_THE_CLUSTER_ID_OF_PEER_CLUSTER>
networking.liqo.io/gateway-resource: "true"
```

### Configuring the server gateway (provider cluster)

By default, a Wireguard tunnel connects the clusters peered with Liqo.
This section shows how to configure the gateway server, where the client will connect to.

The `GatewayServer` resource describes the configuration of the gateway server, and should be applied on the cluster acting as server for the tunnel creation.
You can check [here](./inter-cluster-network.md#creation-of-a-gateway-server) an example of the `GatewayServer` CR.

When you create the `GatewayServer` resource, **make sure to specify the `secretRef`** pointing to the key pairs we created before.

Note that under `.spec.endpoint` of the `GatewayServer` resource you can configure a fixed `nodePort` or `loadBalancerIP` (if supported by your provider) to have a precise UDP port or IP address for the gateway, so that the configuration of the client that connects to it can be defined in advance.

The following is an example of the `GatewayServer` resource, configured in the provider cluster, exposing the gateway using `NodePort` on port `30742`:

```yaml
apiVersion: networking.liqo.io/v1beta1
kind: GatewayServer
metadata:
  labels:
    liqo.io/remote-cluster-id: <CONSUMER_CLUSTER_ID>   # the remote cluster ID
  name: server
  namespace: <PROVIDER_TENANT_NAMESPACE>
spec:
  endpoint:
    port: 51840
    serviceType: NodePort
    nodePort: 30742
  mtu: 1340
  secretRef:
    name: <WIREGUARD_KEYS_SECRET_NAME>
  serverTemplateRef:
    apiVersion: networking.liqo.io/v1beta1
    kind: WgGatewayServerTemplate
    name: wireguard-server
    namespace: liqo
```

Where:

- `CONSUMER_CLUSTER_ID` is the cluster ID of the consumer, where the gateway client runs;
- `PROVIDER_TENANT_NAMESPACE` is the tenant namespace on the provider cluster, where, in this case, we are configuring the gateway server;
- `WIREGUARD_KEYS_SECRET_NAME` is the name of the secret with the Wireguard key pairs we created before.

### Configuring the client gateway (consumer cluster)

The other cluster, in this case the consumer, needs to run the client gateway connecting to the service exposed by the provider cluster.

The `GatewayClient` resource describes the configuration of the gateway client, **it should contain the parameters** to connect to service exposed by the gateway server.

The following is an example of the `GatewayClient` CR to connect to the gateway server that we previously configured:

```yaml
apiVersion: networking.liqo.io/v1beta1
kind: GatewayClient
metadata:
  creationTimestamp: null
  labels:
    liqo.io/remote-cluster-id: <PROVIDER_CLUSTER_ID>   # the remote cluster ID
  name: client
  namespace: <CONSUMER_TENANT_NAMESPACE>
spec:
  clientTemplateRef:
    apiVersion: networking.liqo.io/v1beta1
    kind: WgGatewayClientTemplate
    name: wireguard-client
    namespace: liqo
  secretRef:
    name: <WIREGUARD_KEYS_SECRET_NAME>
  endpoint:
    addresses:
    - <REMOTE_IP>
    port: 30742
    protocol: UDP
  mtu: 1340
```

Where:

- `PROVIDER_CLUSTER_ID` is the cluster ID of the provider, where the gateway server is running;
- `CONSUMER_TENANT_NAMESPACE` is the tenant namespace on the consumer cluster, where, in this case, we are configuring the gateway client;
- `WIREGUARD_KEYS_SECRET_NAME` is the name of the secret with the Wireguard key pairs we created before;
- `REMOTE_IP`: is the IP address of one of the nodes of the provider cluster, as we configured a `NodePort` service. If the service was a `LoadBalancer` the IP would be the one of the load balancer ar a FQDN pointing to it.

### Summary of network configuration

To sum up, to set up the network, **both clusters need**:

- a `Configuration` resource with the network configuration of the peer cluster
- a `Secret` containing the Wireguard public and private keys
- a `PublicKey` with the Wireguard public key **of the peer cluster**

In addition:

- the **provider cluster will have a `GatewayServer`** resource
- the **consumer cluster a `GatewayClient` resource** connecting to the peer Gateway server.

Once you applied all the required resources, the client should be able to connect to the server and create the tunnel.

You can get the `Connection` resource to check the status of the tunnel, as shown [here](./inter-cluster-network.md#connection-crds).

## Declarative configuration of clusters authentication

This section shows how to configure the authentication between the clusters, allowing the consumer cluster to ask for resources.

When authentication is manually configured, **the user is in charge of providing the credentials with the permission required** by the consumer cluster to operate.

If your are not familiar with how authentication works in Kubernetes you can check [this documentation page](https://kubernetes.io/docs/reference/access-authn-authz/authentication/).
You can also check [here to know how to issue a certificate for a user](https://kubernetes.io/docs/reference/access-authn-authz/certificate-signing-requests/#normal-user).

```{warning}
Note that with EKS authentication via client certificate [is not directly supported](https://aws.amazon.com/blogs/containers/managing-access-to-amazon-elastic-kubernetes-service-clusters-with-x-509-certificates/).
You can check [here](https://docs.aws.amazon.com/eks/latest/userguide/cluster-auth.html) how access control works in eks.
```

### Consumer cluster role binding (provider cluster)

Once we created (in the provider cluster) the credentials the consumer can work with, we need to provide the minimum permission required by the consumer to operate.
Note that **the consumer cluster will never directly create workloads on the remote cluster** and, at this stage, it should have only the permissions to create the liqo resources to ask for the approval of a `ResourceSlice`.

To do so, you need to bind the newly created user to the `liqo-remote-controlpane` role.
This can be done **by creating the following `RoleBinding` resource in the tenant namespace of the provider cluster**:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  labels:
    liqo.io/remote-cluster-id: <CONSUMER_CLUSTER_ID>
  name: liqo-binding-liqo-remote-controlplane
  namespace: <TENANT_NAMESPACE>
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: liqo-remote-controlplane
subjects:
- apiGroup: rbac.authorization.k8s.io
  kind: User
  name: <USER_COMMON_NAME>
```

where, when the user authenticates via a certificate signed by the cluster CA, `USER_COMMON_NAME` is the `CN` field of the certificate.

### Creation of a tenant for the consumer cluster (provider cluster)

On the provider side, to allow the authentication of a consumer, we will need to create a `Tenant` resource for it.
This resource is useful to control the remote consumer (e.g. if the provider would like to prevent a remote consumer to negotiate more resources, it can set the tenant condition to `Cordoned`, stopping any other resources negotiation).

Note that, in the case of declarative configuration, there will not be any handshake between the clusters, so we will need to configure the tenant so that it accepts the `ResourceSlice` of the given consumer, even though no handshake occurred.
This can be done setting `TolerateNoHandshake` as `authzPolicy` like in the following example:

```yaml
apiVersion: authentication.liqo.io/v1beta1
kind: Tenant
metadata:
  labels:
    liqo.io/remote-cluster-id: <CONSUMER_CLUSTER_ID>
  name: my-tenant
  namespace: <TENANT_NAMESPACE>
spec:
  clusterID: <CONSUMER_CLUSTER_ID>
  authzPolicy: TolerateNoHandshake
  tenantCondition: Active
```

### Add the credentials on the consumer cluster (consumer cluster)

The previously created credentials on the provider cluster should be given to the consumer cluster.
To do so, you should create a `Secret` containing the kubeconfig with the credentials to operate on the provider cluster, having the following labels:

```yaml
liqo.io/identity-type: ControlPlane
liqo.io/remote-cluster-id: <PROVIDER_CLUSTER_ID>
```

and annotation:

```yaml
liqo.io/remote-tenant-namespace: <PROVIDER_TENANT_NAMESPACE>
```

Where the `PROVIDER_TENANT_NAMESPACE` is the tenant namespace that we created on the provider cluster for the peering with this consumer.

The following is an example of identity secret:

```yaml
apiVersion: v1
data:
  kubeconfig: <BASE64_KUBECONFIG>
kind: Secret
metadata:
  labels:
    liqo.io/identity-type: ControlPlane
    liqo.io/remote-cluster-id: <PROVIDER_CLUSTER_ID>
  annotations:
    liqo.io/remote-tenant-namespace: <PROVIDER_TENANT_NAMESPACE>
  name: cplane-secret
  namespace: <TENANT_NAMESPACE>
```

Once you create this secret, the `liqo-crd-replicator` starts the replication of the resources, and enables the creation of `ResourceSlice` resources targetting the provider cluster. This allows the consumer to start negotiating resources with the provider cluster.

After the secret creation, the logs of the `liqo-crd-replicator` should contain the following records:

```text
 k logs -n liqo liqo-crd-replicator-68f9f55dfc-bc4l8 --tail 5

I1107 10:05:29.249872       1 crdReplicator-operator.go:94] Processing Secret "liqo-tenant-cl02/kubeconfig-controlplane-cl02"
I1107 10:05:29.254977       1 reflector.go:90] [cl02] Starting reflection towards remote cluster
I1107 10:05:29.255001       1 reflector.go:131] [cl02] Starting reflection of offloading.liqo.io/v1beta1, Resource=namespacemaps
I1107 10:05:29.255035       1 reflector.go:131] [cl02] Starting reflection of authentication.liqo.io/v1beta1, Resource=resourceslices
I1107 10:05:29.355741       1 reflector.go:163] [cl02] Reflection of authentication.liqo.io/v1beta1, Resource=resourceslices correctly started
```

### Summary of authentication configuration

To sum up, to set up the authentication, **on the provider cluster** you will need to:

- Create the credentials to be used by the consumer
- Bind the credentials to the `liqo-remote-controlplane` role
- Create a `Tenant` resource for the consumer

While, **on the consumer side**:

- Create a new secret containing the kubeconfig with the credentials to access to the provider cluster

## Declarative configuration of namespace offloading

While offloading is independent from the network, which means that it is possible to negotiate resources and configure a namespace offloading without the inter-cluster network enabled, **a [working authentication configuration](#declarative-configuration-of-clusters-authentication) is a pre-requisite to enable offloading**.

### Ask for resources: configure a ResourceSlice (consumer cluster)

The `ResourceSlice` resource is the CR that defines the computational resources requested by the consumer to the provider cluster.
It should be created on the tenant namespace of the consumer cluster, and it is automatically forwarded to the provider cluster, which can accept or reject it.

The following is an example of `ResourceSlice`:

```yaml
apiVersion: authentication.liqo.io/v1beta1
kind: ResourceSlice
metadata:
  annotations:
    liqo.io/create-virtual-node: "true"
  creationTimestamp: null
  labels:
    liqo.io/remote-cluster-id: <PROVIDER_CLUSTER_ID>
    liqo.io/remoteID: <PROVIDER_CLUSTER_ID>
  name: test
  namespace: <CONSUMER_TENANT_NAMESPACE>
spec:
  class: default
  providerClusterID: <PROVIDER_CLUSTER_ID>
  resources:
    cpu: 20
    memory: 128Gi
```

If the request above is successfully accepted by the provider, a new (virtual) node, impersonating the provider cluster, will make available the requested resorces on the consumer cluster.

To know more about `ResourceSlice` and `VirtualNode` check [this section of the documentation](./offloading-in-depth.md#create-resourceslice).

### Enable offloading and K8s resources availability on remote clusters

By default, the virtual nodes are not eligible for task scheduling, unless offloading is enabled for the namespace where pod is running.
To do so, a `NamespaceOffloading`, like the following, should be created:

```yaml
apiVersion: offloading.liqo.io/v1beta1
kind: NamespaceOffloading
metadata:
  name: offloading
  namespace: demo
spec:
  clusterSelector:
    nodeSelectorTerms: []
  namespaceMappingStrategy: DefaultName
  podOffloadingStrategy: LocalAndRemote
```

The `NamespaceOffloading` resource should be created in the namespace that we would like to extend on the remote clusters.

For example, the resource above, extends the `demo` namespace on all the configured provider clusters.
Since the `podOffloadingStrategy` policy is `LocalAndRemote`, the a new pod could be executed either locally or remotely depending on the choice made by the vanilla Kubernetes scheduler (e.g., if the remote virtual node is plenty of free resources, it may be preferred against local nodes that are already used by other pods).

[Check here](../../usage/namespace-offloading.md#namespace-mapping-strategy) to know more about namespace offloading.

```{warning}
Currently, the `NamespaceOffloading` resource **must be created before scheduling a pod on a remote cluster**.

For example, if we configure a pod to run on the remote cluster, but the pod is created before setting the `NamespaceOffloading` resource, that pod will remain in a `Pending` state forever, also after the namespace is actually offloaded.

Therefore, **make sure to offload a namespace** before starting scheduling pods on it.
```
