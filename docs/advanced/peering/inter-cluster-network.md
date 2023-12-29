# Inter-cluster Network Connectivity

## Overview

The following resources are involved in the network connectivity:

* **GatewayServer**: this resource is used to deploy the Liqo Gateway on the cluster, it exposes a service to the outside of the cluster.
* **GatewayClient**: this resource is used to connect to the Liqo Gateway of a remote cluster.
* **Connection**: this resource shows the status of the connection between two clusters.

With the different methods of network configuration, you will create and manage these resources with different levels of automation and customization.

## Automatic

When you [create a peering](/usage/peer) between two clusters, Liqo automatically deploys a Liqo Gateway for each cluster in the tenant namespace, no further configuration is required.
The cluster that is requesting resources and where the virtual node will be created will be configured as a client, while the cluster that is providing resources is configured as a server.

```{figure} /_static/images/usage/inter-cluster-network/automatic.drawio.svg
---
align: center
---
Automatic network configuration
```

The unpeer process will automatically remove the Liqo Gateway from the tenant namespace.

## Manual on cluster couple

When you have access to both clusters, you can configure the network connectivity for all the successive peering creations.

First, you need to initialize the network:

```bash
liqoctl network init \
  --kubeconfig $CLUSTER_1_KUBECONFIG_PATH \
  --remote-kubeconfig $CLUSTER_2_KUBECONFIG_PATH \
  --wait
```

You should see the following output:

```text
INFO   (local) Cluster identity correctly retrieved
INFO   (remote) Cluster identity correctly retrieved
INFO   (local) Network configuration correctly retrieved
INFO   (remote) Network configuration correctly retrieved
INFO   (local) Network configuration correctly set up
INFO   (remote) Network configuration correctly set up
INFO   (local) Configuration applied successfully
INFO   (remote) Configuration applied successfully
```

This command will share and configure the required resources between the two clusters.
You will find in both your clusters a new Configuration in the tenant namespace.

```bash
kubectl get configurations.networking.liqo.io -A

NAMESPACE                      NAME        DESIRED POD CIDR    REMAPPED POD CIDR   AGE
liqo-tenant-dry-paper-5d16c0   dry-paper   10.243.0.0/16       10.71.0.0/16        4m48s
```

Now, you can establish the connection between the two clusters:

```bash
liqoctl network connect \
  --kubeconfig $CLUSTER_1_KUBECONFIG_PATH \
  --remote-kubeconfig $CLUSTER_2_KUBECONFIG_PATH \
  --server-service-type NodePort \
  --wait
```

You should see the following output:

```text
INFO   (local) Cluster identity correctly retrieved
INFO   (remote) Cluster identity correctly retrieved
INFO   (local) Network correctly initialized
INFO   (remote) Network correctly initialized
INFO   (remote) Gateway server correctly set up
INFO   (remote) Gateway pod gw-crimson-rain is ready
INFO   (remote) Gateway server Service created successfully
INFO   (local) Gateway client correctly set up
INFO   (local) Gateway pod gw-damp-feather is ready
INFO   (remote) Gateway server Secret created successfully
INFO   (local) Public key correctly created
INFO   (local) Gateway client Secret created successfully
INFO   (remote) Public key correctly created
INFO   (remote) Connection created successfully
INFO   (local) Connection created successfully
INFO   (local) Connection is established
INFO   (remote) Connection is established
```

This command will deploy a Liqo Gateway for each cluster in the tenant namespace and establish the connection between them.
In the first cluster, the Liqo Gateway will be configured as a client, while in the second cluster, it will be configured as a server.

```{admonition} Note
You can see further configuration options with `liqoctl network connect --help`.

For instance, in the previous command we have used the `--server-service-type NodePort` option to expose the Liqo Gateway service as a NodePort service.
Alternatively, you can use the `--server-service-type LoadBalancer` option to expose the Liqo Gateway service as a LoadBalancer service (if supported by your cloud provider).
```

In cluster 1 you will find the following resources:

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

In cluster 2 you will find the following resources:

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

You can check the status of the connection to see if it is working correctly.

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

Optionally, you can remove the network configuration with the following command:

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

You can configure how to expose the Liqo Gateway Server service by using the following flags for the `liqoctl network connect` command on the server side:

* `--server-service-type` (default `LoadBalancer`): the type of the Gateway service, it can be `NodePort` or `LoadBalancer`.
* `--server-port` (default `51820`): the port of the Gateway service.
* `--node-port` (default `0`): set it to force the NodePort binding to a specific port. If set to `0`, the system will allocate a port automatically.
* `--load-balancer-ip` (default `""`): set it to force the LoadBalancer service to bind to a specific IP address. If set to `""`, the system will allocate an IP address automatically.
* `--mtu` (default `1340`): the MTU of the Gateway interface. Note that the MTU must be the same on both sides.

Use the `liqoctl network connect --help` command to see all the available options.

## Manual on single cluster

When you don't have access to both clusters, or you want to configure it in a declarative way, you can configure it by applying CRDs.
The process consists of the following steps:

1. **Cluster client and server**: the clusters that need to connect have to exchange a `Configuration` resource, containing the `CIDR` of each remote cluster.
2. **Cluster server**: one of the clusters defines a `GatewayServer`, which exposes a service acting as server for the inter-cluster communication.
3. **Cluster client**: the other cluster defines a `GatewayClient` resource, which will configure a client that will connect to the gateway server exposed on the other cluster.
4. **Cluster client and server**: the cluster client and server need to exchange the public keys to allow secure communication.

### Definition of the network configuration (Configuration CRDs)

In this step, each cluster needs to exchange the network configuration. Therefore, you will need to apply in **both clusters** a `Configuration` resource, paying attention to apply in each cluster, the network configuration of the other one.

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

You can find *REMOTE_CLUSTER_ID* these parameters in the output of the

```bash
liqoctl get cluster-config --cluster-id
```

command in the remote cluster.

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
    port: 51820
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

```bash
kubectl get gatewayservers --template {{.status.endpoint}}
```

```text
map[addresses:[172.19.0.9] port:32701 protocol:UDP]
```

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
   kubectl apply -f server-client.yaml
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

This allows the user to reference custom-made templates, giving also the possibility to implement custom technologies different from WireGuard.

The `examples/networking` folder contains a bunch of template manifests, showing possible customizations to the default WireGuard template.

```{admonition} Tip
A field with a value in the form of `'{{ EXAMPLE }}'` (**go-template** syntax) will be templated automatically by the Gateway operator with the value(s) from the `GatewayServer`/`GatewayClient` resource.
```
