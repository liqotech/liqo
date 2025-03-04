# Inter-cluster Networking

## Overview

Liqo needs a proper network connectivity to connect two clusters, which may involve both network resources as well as logical parameters (e.g., dedicated IP routes).
By default, Liqo creates a WireGuard tunnel to provision the above inter-cluster network connectivity.
In order to setup this feature, the following Kubernetes Custom Resources (CRs) are involved:

* **GatewayServer**: used to deploy the Liqo Gateway acting as _server_ on one cluster; it exposes a Kubernetes service that has to be reachable from outside of the cluster.
* **GatewayClient**: used to deploy a Liqo Gateway acting as a _client_ toward a remote cluster. The deployed pod must be able to reach the Kubernetes service exported by the gateway server on the remote cluster.
* **Connection**: it shows the status of the connection between two clusters.

This section presents how to deal with the above resources in order to achieve different levels of automation and customization.

However, before delving into the configuration details, let use recap what happens, from the networking standpoind, when you use the [automatic peering](/usage/peer) between two clusters.
In this case, Liqo automatically deploys a Gateway in the tenant namespace of each cluster, and no further configuration is required.
The cluster that is requesting resources and where the virtual node will be created is configured as a _client_, while the cluster that is providing resources is configured as a _server_.

```{figure} /_static/images/usage/inter-cluster-network/automatic.drawio.svg
---
align: center
---
Automatic network configuration
```

By default, with the automatic process the _provider cluster_ is also acting as _gateway server_ from the networking point of view; this is not strictly needed, and the roles from the networking (i.e., gateway _server_ and _client_) and offloading (i.e., cluster _provider_ and _consumer_) can actually be decoupled using the `--gw-server-gateway-location` parameter with the `liqoctl peer` command or when using a advanced peering process.

Finally, it is worth remembering that the network configuration is applied independently per each peering; for instance, a first cluster A can act as gateway client toward a second cluster B, but it can act as gateway server with respect to the peering with cluster C.

## Setup the inter-cluster network via `liqoctl network` command

When your management machine has access to the Kubernetes API server of both clusters, you can configure the inter-cluster network connectivity via the `liqoctl network` command.

Note that the `liqoctl network` command assumes that the remote kubeconfig/context (`--remote-kubeconfig`, `--remote-context`) is the cluster that acts as gateway server for the Wireguard tunnel, independently from the role of the above cluster from the offloading point of view (i.e., _provider_ or _consumer_).

To establish a connection between two clusters, you can run the following command:

```bash
liqoctl network connect \
  --kubeconfig $CLUSTER_1_KUBECONFIG_PATH \
  --remote-kubeconfig $CLUSTER_2_KUBECONFIG_PATH \
  --gw-server-service-type NodePort \
  --wait
```

You should see an output like the following:

```text
 INFO   (local) Network configuration correctly retrieved
 INFO   (remote) Network configuration correctly retrieved
 INFO   (local) Network configuration correctly set up
 INFO   (remote) Network configuration correctly set up
 INFO   (local) Configuration applied successfully
 INFO   (remote) Configuration applied successfully
 INFO   (remote) Gateway server template "wireguard-server/liqo" correctly checked
 INFO   (local) Gateway client template "wireguard-client/liqo" correctly checked
 INFO   (local) Network correctly initialized
 INFO   (remote) Network correctly initialized
 INFO   (remote) Gateway server correctly set up
 INFO   (remote) Gateway pod gw-cl01 is ready
 INFO   (remote) Gateway server Service created successfully
 INFO   (local) Gateway client correctly set up
 INFO   (local) Gateway pod gw-cl02 is ready
 INFO   (remote) Gateway server Secret created successfully
 INFO   (local) Public key correctly created
 INFO   (local) Gateway client Secret created successfully
 INFO   (remote) Public key correctly created
 INFO   (remote) Connection created successfully
 INFO   (local) Connection created successfully
 INFO   (local) Connection is established
 INFO   (remote) Connection is established
```

This command deploys a Liqo Gateway in the tenant namespace of each cluster and sets up the connection between them.
The Liqo Gateway of the first cluster will act as a client, while the one in the second cluster will act as a server.

```{admonition} Note
You can see further configuration options with `liqoctl network connect --help`.

For instance, in the previous command we have used the `--gw-server-service-type NodePort` option to expose the service associated to the Liqo Gateway server as a NodePort.
Alternatively, you can use the `--gw-server-service-type LoadBalancer` option to expose the Liqo Gateway service as a LoadBalancer (if supported in your Kubernetes cluster).
```

In the first cluster (acting as gateway client), you can find the following resources:

```bash
kubectl get gatewayclients.networking.liqo.io -A
```

```text
NAMESPACE             NAME      TEMPLATE NAME      IP           PORT    AGE
liqo-tenant-cl02      cl02      wireguard-client   172.19.0.8   32009   28s
```

```bash
kubectl get connections.networking.liqo.io -A
```

```text
NAMESPACE          NAME            TYPE     STATUS      AGE
liqo-tenant-cl02   gw-cl02         Client   Connected   76s
```

In the second cluster (acting as gateway server) you can find the following resources:

```bash
kubectl get gatewayservers.networking.liqo.io -A
```

```text
NAMESPACE          NAME        TEMPLATE NAME      IP           PORT    AGE
liqo-tenant-cl01   cl01        wireguard-server   172.19.0.8   32009   69s
```

```bash
kubectl get connections.networking.liqo.io -A
```

```text
NAMESPACE             NAME      TYPE     STATUS      AGE
liqo-tenant-cl01      cl01      Server   Connected   51s
```

The command above applied the following changes to the clusters:

* Exchanged the network configuration to configure the IPs remapping, which allows to reach pods and services in the other cluster
* it deployed a Liqo Gateway for each cluster in the tenant namespace and established the connection between them.
  By default, in the first cluster, the Liqo Gateway is configured as a client, while in the second cluster, is configured as a server.

```{admonition} Note
You can see further configuration options with `liqoctl network connect --help`.

For instance, in the previous command we have used the `--gw-server-service-type NodePort` option to expose the Liqo Gateway service as a NodePort service.
Alternatively, you can use the `--gw-server-service-type LoadBalancer` option to expose the Liqo Gateway service as a LoadBalancer service (if supported by your cloud provider).
```

In **cluster 1**, which, in this case, **hosts the client gateway**, you will find the following resources:

* A `Configuration` resource describing how the POD cidr of the other cluster is remapped in the current cluster:

  ```bash
  kubectl get configurations.networking.liqo.io -A
  ```

  ```text
  NAMESPACE           NAME     DESIRED POD CIDR    REMAPPED POD CIDR   AGE
  liqo-tenant-cl02    cl02     10.243.0.0/16       10.71.0.0/16        4m48s
  ```

* A `GatewayClient` resource, which describes the configuration of the gateway acting as a **client** for establishing the tunnel between the two clusters:

  ```bash
  kubectl get gatewayclients.networking.liqo.io -A
  ```

  ```text
  NAMESPACE             NAME      TEMPLATE NAME      IP           PORT    AGE
  liqo-tenant-cl02      cl02      wireguard-client   172.19.0.8   32009   28s
  ```

* A `Connection` resource, describing the status of the tunnel with the peer cluster:

  ```bash
  kubectl get connections.networking.liqo.io -A
  ```

  ```text
  NAMESPACE          NAME            TYPE     STATUS      AGE
  liqo-tenant-cl02   gw-cl02         Client   Connected   76s
  ```

In **cluster 2**, which, in this case, **hosts the server gateway**, you will find the following resources:

* A `Configuration` resource describing how the POD cidr of the other cluster is remapped in the current cluster:

  ```bash
  kubectl get configurations.networking.liqo.io -A
  ```

  ```text
  NAMESPACE           NAME     DESIRED POD CIDR    REMAPPED POD CIDR   AGE
  liqo-tenant-cl01    cl01     10.243.0.0/16       10.71.0.0/16        4m48s
  ```

* A `GatewayServer` resource, which describes the configuration of the gateway acting as a **server** for establishing the tunnel between the two clusters:

  ```bash
  kubectl get gatewayservers.networking.liqo.io -A
  ```

  ```text
  NAMESPACE          NAME        TEMPLATE NAME      IP           PORT    AGE
  liqo-tenant-cl01   cl01        wireguard-server   172.19.0.8   32009   69s
  ```

* A `Connection` resource, describing the status of the tunnel with the peer cluster:

  ```bash
  kubectl get connections.networking.liqo.io -A
  ```

  ```text
  NAMESPACE             NAME      TYPE     STATUS      AGE
  liqo-tenant-cl01      cl01      Server   Connected   51s
  ```

### Tear down

You can remove the network connection between the two clusters with the following command:

```bash
liqoctl network disconnect \
  --kubeconfig $CLUSTER_1_KUBECONFIG_PATH \
  --remote-kubeconfig $CLUSTER_2_KUBECONFIG_PATH --wait
```

You should see the following output:

```text
disconnect is a potentially destructive command.
Are you sure you want to continue? [y/N]yes
INFO   (local) Cluster identity correctly retrieved
INFO   (remote) Cluster identity correctly retrieved
INFO   (local) Gateway client correctly deleted
INFO   (remote) Gateway server correctly deleted
```

Optionally, you can remove all the network configuration (and the associated CRs) with the following command:

```bash
liqoctl network reset \
  --kubeconfig $CLUSTER_1_KUBECONFIG_PATH \
  --remote-kubeconfig $CLUSTER_2_KUBECONFIG_PATH \
  --wait
```

You should see the following output:

```text
reset is a potentially destructive command.
Are you sure you want to continue? [y/N]yes
INFO   (local) Cluster identity correctly retrieved
INFO   (remote) Cluster identity correctly retrieved
INFO   (local) Gateway client correctly deleted
INFO   (remote) Gateway server correctly deleted
INFO   (local) Network configuration correctly deleted
INFO   (remote) Network configuration correctly deleted
```

### Customization

You can configure how to expose the Liqo Gateway Server service by using the following flags for the `liqoctl network connect` command with respect to the server side:

* `--gw-server-service-type` (default `LoadBalancer`): the type of the Gateway service, it can be `NodePort` or `LoadBalancer`.
* `--gw-server-service-port` (default `51840`): Port of the Gateway Server service.
* `--gw-server-service-nodeport` (default `""`): force the NodePort of the Gateway Server service. Leave empty to let Kubernetes allocate a random NodePort.
* `--gw-server-service-loadbalancerip` (default `""`): force LoadBalancer IP of the Gateway Server service. Leave empty to use the one provided by the LoadBalancer provider.
* `--mtu` (default `1340`): the MTU of the Gateway interface. Note that the MTU must be the same on both sides.

Use the `liqoctl network connect --help` command to see all the available options.

## Manually setup the inter-cluster network on each cluster separately

When you do not have contemporary access to both clusters, or you would like to configure the inter-cluster network in a declarative way, you can configure this component by applying the proper CRDs on each cluster separately.

1. **Cluster client and server**: the clusters that need to connect have to exchange a `Configuration` resource, containing the `CIDR` of each remote cluster.
2. **Cluster server**: one of the clusters defines a `GatewayServer`, which exposes a service acting as server for the inter-cluster communication.
3. **Cluster client**: the other cluster defines a `GatewayClient` resource, which will configure a client that will connect to the gateway server exposed on the other cluster.
4. **Cluster client and server**: the cluster client and server need to exchange the public keys to allow secure communication.

More info about the declarative setup of the network can be found [here](./peering-via-cr.md#declarative-network-configuration).

### Definition of the network configuration (Configuration CRDs)

In this step, each cluster needs to exchange the network configuration. Therefore, you will need to apply a `Configuration` resource in **both clusters**, paying attention to apply, _in each cluster_, the network configuration _of the other one_.

This is an example of a possible `Configuration` resource:

```yaml
apiVersion: networking.liqo.io/v1beta1
kind: Configuration
metadata:
  labels:
    liqo.io/remote-cluster-id: <REMOTE_CLUSTER_ID>   # the remote cluster ID
  name: dry-paper
spec:
  remote:
    cidr:
      external: 10.70.0.0/16        # the external CIDR of the remote cluster
      pod: 10.243.0.0/16            # the pod CIDR of the remote cluster
```

You can find the value of the *REMOTE_CLUSTER_ID* by launching the following command on the **remote cluster**:

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

```{admonition} Tip
You can generate this file with the command `liqoctl generate configuration` executed in the remote cluster.
```

```{important}
You need to apply this resource in both clusters.
```

### Gateway CRDs

#### Creation of a gateway server

In the inter-cluster communication, one of the clusters will expose a gateway server, where a client on the other cluster will connect to.
Therefore, in the cluster that will expose the service and act as a server, you need to apply the `GatewayServer` resource:

```yaml
apiVersion: networking.liqo.io/v1beta1
kind: GatewayServer
metadata:
  labels:
    liqo.io/remote-cluster-id: <CLIENT_CLUSTER_ID>   # the remote cluster ID
  name: server
spec:
  endpoint:
    port: 51840
    serviceType: NodePort
  mtu: 1340
  serverTemplateRef:
    apiVersion: networking.liqo.io/v1beta1
    kind: WgGatewayServerTemplate
    name: wireguard-server
    namespace: liqo
```

````{admonition} Tip
You can generate this file with the following command, and then edit it:

``` bash
liqoctl create gatewayserver server \
  --remote-cluster-id <CLIENT_CLUSTER_ID> \
  --service-type NodePort -o yaml
```
````

After some seconds, you will be able to see an IP address and a port assigned to the GatewayServer resource:

```bash
kubectl get gatewayservers.networking.liqo.io -A
```

```text
NAMESPACE   NAME     TEMPLATE NAME      IP           PORT    AGE
default     server   wireguard-server   10.42.3.54   32133   84s
```

`````{tab-set}
````{tab-item} liqoctl

```bash
liqoctl info peer <REMOTE_CLUSTER_ID> --get network.gateway
```
````

````{tab-item} kubectl

```bash
kubectl get gatewayservers --template {{.status.endpoint}} -n <GATEWAY_NS> <GATEWAY_NAME>
```

```text
map[addresses:[172.19.0.9] port:32701 protocol:UDP]
```
````
`````

#### Creation of a gateway client

The other cluster will need to connect to the gateway server and act as a client.
To configure the client, you need to apply the GatewayClient resource, containing the IP address and port where the `GatewayServer` is reachable and all the parameters required for the connection to the server:

```yaml
apiVersion: networking.liqo.io/v1beta1
kind: GatewayClient
metadata:
  creationTimestamp: null
  labels:
    liqo.io/remote-cluster-id: <SERVER_CLUSTER_ID>   # the remote cluster ID
  name: client
  namespace: default
spec:
  clientTemplateRef:
    apiVersion: networking.liqo.io/v1beta1
    kind: WgGatewayClientTemplate
    name: wireguard-client
    namespace: liqo
  endpoint:
    addresses:
    - <REMOTE_IP>
    port: <REMOTE_PORT>
    protocol: UDP
  mtu: 1340
```

The *REMOTE_IP* and *REMOTE_PORT* are the IP and the port of the GatewayServer service in the server cluster, generated in the previous section.

````{admonition} Tip
You can generate this file with the command:

``` bash
liqoctl create gatewayclient client --remote-cluster-id <SERVER_CLUSTER_ID> \
  --addresses <REMOTE_IP> --port <REMOTE_PORT> -o yaml
```
````

### Public keys exchange (PublicKey CRDs)

Finally, to allow secure communication between the clusters, they need to generate a key pair and exchange the **public key**.

In the client cluster, you will run the following command:

```bash
liqoctl generate publickey --gateway-type client --gateway-name client
```

You should see the following output:

```yaml
apiVersion: networking.liqo.io/v1beta1
kind: PublicKey
metadata:
  creationTimestamp: null
  labels:
    liqo.io/remote-cluster-id: <CLIENT_CLUSTER_ID>   # the remote cluster ID
    networking.liqo.io/gateway-resource: "true"
  name: <CLIENT_CLUSTER_ID>
spec:
  publicKey: <PUBLIC_KEY>
```

You need to apply this resource in the server cluster.

In the server cluster, you will run the following command:

```bash
liqoctl generate publickey --gateway-type server --gateway-name server
```

You should see the following output:

```yaml
apiVersion: networking.liqo.io/v1beta1
kind: PublicKey
metadata:
  creationTimestamp: null
  labels:
    liqo.io/remote-cluster-id: <SERVER_CLUSTER_ID>   # the remote cluster ID
    networking.liqo.io/gateway-resource: "true"
  name: <SERVER_CLUSTER_ID>
spec:
  publicKey: <PUBLIC_KEY>
```

You need to apply this resource in the client cluster.

### Connection CRDs

Now, you will find the **Connection** resource in both clusters.

In each cluster, you can run the following command:

```bash
kubectl get connections.networking.liqo.io -A
```

On the server cluster, you will see:

```text
NAMESPACE   NAME                  TYPE     STATUS      AGE
default     <CLIENT_CLUSTER_ID>   Server   Connected   2m
```

On the client cluster, you will see:

```text
NAMESPACE   NAME                  TYPE     STATUS      AGE
default     <SERVER_CLUSTER_ID>   Client   Connected   2m
```

### Summary

Resuming, these are the steps to be followed by the administrators of each of the clusters to manually complete the configuration of the inter-cluster network:

1. **Cluster client**: creates the configuration to be given to the **cluster server** administrator:

   ```bash
   liqoctl generate configuration > conf-client.yaml
   ```

2. **Cluster server**: applies the client configuration and generates its own to be applied by the **cluster client**:

   ```bash
   kubectl apply -f conf-client.yaml
   liqoctl generate configuration > conf-server.yaml
   ```

3. **Cluster client**: applies the server configuration:

   ```bash
   kubectl apply -f conf-server.yaml
   ```

4. **Cluster server**: sets up the `GatewayServer` and provides to the cluster client administrator port and address where the server is reachable:

   ```bash
   liqoctl create gatewayserver server \
    --remote-cluster-id $CLUSTER_ID_CLIENT \
    --service-type NodePort
   ```

5. **Cluster client**: Sets up the client that connects to the `GatewayServer`:

   ```bash
   liqoctl create gatewayclient client \
    --remote-cluster-id $CLUSTER_ID_SERVER \
    --addresses $ADDRESS_SERVER \
    --port $PORT_SERVER
   ```

6. **Cluster server**: generates a key pair and generates a `PublicKey` resource to be applied by the **cluster client**:

   ```bash
   liqoctl generate publickey \
    --gateway-type server \
    --gateway-name server > publickey-server.yaml
   ```

7. **Cluster client**: applies the `PublicKey` resource of the server and generates its own:

   ```bash
   kubectl apply -f publickey-server.yaml
   liqoctl generate publickey \
    --gateway-type client \
    --gateway-name client > publickey-client.yaml
   ```

8. **Cluster server**: applies the `PublicKey` resource of the client:

   ```bash
   kubectl apply -f publickey-client.yaml
   ```

You can check whether the procedure completed successfully by checking [the peering status](../../usage/peer.md#check-status-of-peerings).

## Custom templates

Gateway resources (i.e., `GatewayServer` and `GatewayClient`) contain a reference to the template CR implementing the inter-cluster network technology.

The default technology used by Liqo is an implementation of a WireGuard VPN tunnel to connect the gateway client and gateway server of the two peered clusters.
The template is referenced in the spec of the API, as shown here:

```yaml
spec:
  serverTemplateRef:
    apiVersion: networking.liqo.io/v1beta1
    kind: WgGatewayServerTemplate
    name: wireguard-server
    namespace: liqo
```

This allows you to reference custom-made templates, giving also the possibility to implement custom technologies different from WireGuard.

The [`examples/networking`](https://github.com/liqotech/liqo/tree/master/examples/networking) folder contains a bunch of template manifests, showing possible customizations to the default WireGuard template.

### Template Variables and Usage

The Gateway operator automatically injects the following variables into your templates:

For GatewayServer templates:

* `{{ .Spec }}`: Access to all fields from the GatewayServer spec
* `{{ .Name }}`: Name of the GatewayServer resource
* `{{ .Namespace }}`: Namespace of the GatewayServer resource
* `{{ .GatewayUID }}`: Unique identifier of the Gateway resource
* `{{ .ClusterID }}`: ID of the remote cluster
* `{{ .SecretName }}`: Name of the referenced secret

For GatewayClient templates:

* `{{ .Spec }}`: Access to all fields from the GatewayClient spec
* `{{ .Name }}`: Name of the GatewayClient resource
* `{{ .Namespace }}`: Namespace of the GatewayClient resource
* `{{ .GatewayUID }}`: Unique identifier of the Gateway resource
* `{{ .ClusterID }}`: ID of the remote cluster
* `{{ .SecretName }}`: Name of the referenced secret

These variables can be used in the following template fields:

* `metadata.name`
* `metadata.namespace`
* `metadata.labels`
* `metadata.annotations`
* `spec` (any field)

Optional fields in your template should start with a `?` prefix.
These fields will be included only if the templated value is not empty.
For example:

```yaml
spec:
  # Required field - always included
  name: "{{ .Name }}"
  # Optional field - included only if .Spec.ExtraConfig is not empty
  ?extraConfig: "{{ .Spec.ExtraConfig }}"
```
