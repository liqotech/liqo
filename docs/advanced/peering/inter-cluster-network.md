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

### NAT Firewall

In Case a **gateway server** is behind a NAT firewall, the following steps are required to establish the connection:
You can configure the settings for the connection by setting the following *annotations* in the `values.yaml` file or by using the `liqoctl` command line `--set` option:

Under the `networking.gatewayTemplates.server.service.annotations` key, you can set the following annotations:

* **liqo.io/override-address**: the public IP address of the NAT firewall.
* **liqo.io/override-port**: the public port of the NAT firewall.

```{admonition} Tip
In case you need to have multiple gateways behind the same NAT firewall, you need to override the port for each peer using the `--server-port` flag at peering time.
```

## Manual on cluster couple

When you have access to both clusters, you can configure the network connectivity for all the successive peering creations.

First, you need to initialize the network:

```bash
liqoctl network init --kubeconfig PATH_TO_CLUSTER_1_KUBECONFIG --remote-kubeconfig PATH_TO_CLUSTER_2_KUBECONFIG --wait
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
liqoctl network connect --kubeconfig PATH_TO_CLUSTER_1_KUBECONFIG --remote-kubeconfig PATH_TO_CLUSTER_2_KUBECONFIG --server-service-type NodePort --wait
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
NAMESPACE                          NAME            TEMPLATE NAME      IP           PORT    AGE
liqo-tenant-crimson-field-46ec75   crimson-field   wireguard-client   10.42.3.54   30316   61s
```

```bash
kubectl get connections.networking.liqo.io -A
```

```text
NAMESPACE                          NAME            TYPE     STATUS      AGE
liqo-tenant-crimson-field-46ec75   crimson-field   Client   Connected   76s
```

In cluster 2 you will find the following resources:

```bash
kubectl get gatewayservers.networking.liqo.io -A
```

```text
NAMESPACE                      NAME        TEMPLATE NAME      IP           PORT    AGE
liqo-tenant-dry-paper-5d16c0   dry-paper   wireguard-server   10.42.3.54   30316   69s
```

```bash
kubectl get connections.networking.liqo.io -A
```

```text
NAMESPACE                      NAME        TYPE     STATUS      AGE
liqo-tenant-dry-paper-5d16c0   dry-paper   Server   Connected   51s
```

You can check the status of the connection to see if it is working correctly.

### Tear down

You can remove the network connection between the two clusters with the following command:

```bash
liqoctl network disconnect --kubeconfig PATH_TO_CLUSTER_1_KUBECONFIG --remote-kubeconfig PATH_TO_CLUSTER_2_KUBECONFIG --wait
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
liqoctl network reset --kubeconfig PATH_TO_CLUSTER_1_KUBECONFIG --remote-kubeconfig PATH_TO_CLUSTER_2_KUBECONFIG --wait
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

### Configuration

You can configure how to expose the Liqo Gateway Server service by using the following flags for the `liqoctl network connect` command on the server side:

* `--server-service-type` (default `LoadBalancer`): the type of the Gateway service, it can be `NodePort` or `LoadBalancer`.
* `--server-port` (default `51820`): the port of the Gateway service.
* `--node-port` (default `0`): set it to force the NodePort binding to a specific port. If set to `0`, the system will allocate a port automatically.
* `--load-balancer-ip` (default `""`): set it to force the LoadBalancer service to bind to a specific IP address. If set to `""`, the system will allocate an IP address automatically.
* `--mtu` (default `1340`): the MTU of the Gateway interface. Note that the MTU must be the same on both sides.

## Manual on single cluster

When you don't have access to both clusters, or you want to configure it in a declarative way, you can configure it by applying CRDs.

You need to apply in both clusters the Configuration resource:

```yaml
apiVersion: networking.liqo.io/v1beta1
kind: Configuration
metadata:
  labels:
    liqo.io/remote-cluster-id: 506f88d4-9918-4848-ab74-2a75ae32647f     # the remote cluster ID
  name: dry-paper
spec:
  remote:
    cidr:
      external: 10.70.0.0/16        # the external CIDR of the remote cluster
      pod: 10.243.0.0/16            # the pod CIDR of the remote cluster
```

You can find these parameters in the output of the `liqoctl status` command in the remote cluster.

```{admonition} Tip
You can generate this file with the command `liqoctl generate configuration` run in the remote cluster.
```

Now, in the cluster that will expose the service and act as a server, you need to apply the GatewayServer resource:

```yaml
apiVersion: networking.liqo.io/v1beta1
kind: GatewayServer
metadata:
  labels:
    liqo.io/remote-cluster-id: 506f88d4-9918-4848-ab74-2a75ae32647f
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

You can generate this file with the following command, and then edit it:

``` bash
liqoctl create gatewayserver server --remote-cluster-id 35d83766-f466-46ef-b5b3-d253b6d465f1 --service-type NodePort -o yaml

Some seconds after you will find an assigned IP and a port in the status of the GatewayServer resource:

```bash
kubectl get gatewayservers.networking.liqo.io -A
```

```text
NAMESPACE   NAME     TEMPLATE NAME      IP           PORT    AGE
default     server   wireguard-server   10.42.3.54   32133   84s
```

Now, in the cluster that will connect to the service and act as a client, you need to apply the GatewayClient resource:

```yaml
apiVersion: networking.liqo.io/v1beta1
kind: GatewayClient
metadata:
  creationTimestamp: null
  labels:
    liqo.io/remote-cluster-id: ef7a1f41-c753-4a0e-83e5-bed013e7fe4f
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
    - 10.42.3.54
    port: 32133
    protocol: UDP
  mtu: 1340
```

You can generate this file with the command:

``` bash
liqoctl create gatewayclient client --remote-cluster-id ef7a1f41-c753-4a0e-83e5-bed013e7fe4f --addresses 10.42.3.54 --port 32133 -o yaml
```

Lastly, you need to exchange the public keys between the two clusters.

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
    liqo.io/remote-cluster-id: ef7a1f41-c753-4a0e-83e5-bed013e7fe4f
    networking.liqo.io/gateway-resource: "true"
  name: dry-paper
spec:
  publicKey: GExyYe7RJ3Lo5nQVa6K2Tp9zobXCvWmJawsLxu7hn10=
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
    liqo.io/remote-cluster-id: 506f88d4-9918-4848-ab74-2a75ae32647f
    networking.liqo.io/gateway-resource: "true"
  name: crimson-field
spec:
  publicKey: NB+3xA4a0SIVsta9zwWn/mPc4J86GPmUdSChJG5rxlk=
```

You need to apply this resource in the client cluster.

Now, you will find the Connection resource in both clusters.

Resuming, you can implement the network connectivity between two clusters with a script like the following:

```bash
#!/bin/bash

set -o nounset   # Fail if undefined variables are used

export KUBE_SERVER=PATH_TO_CLUSTER_1_KUBECONFIG
export KUBE_CLIENT=PATH_TO_CLUSTER_2_KUBECONFIG

# Create configuration
liqoctl --kubeconfig $KUBE_SERVER generate configuration > conf-server.yaml
liqoctl --kubeconfig $KUBE_CLIENT generate configuration > conf-client.yaml

kubectl --kubeconfig $KUBE_SERVER apply -f conf-client.yaml
kubectl --kubeconfig $KUBE_CLIENT apply -f conf-server.yaml

SERVER_CONFIGURATION_NAME=$(kubectl --kubeconfig $KUBE_SERVER get configuration -o json | jq -r '.items[0].metadata.name')
CLIENT_CONFIGURATION_NAME=$(kubectl --kubeconfig $KUBE_CLIENT get configuration -o json | jq -r '.items[0].metadata.name')

kubectl wait --for jsonpath='{.status.remote.cidr.pod}' configuration $SERVER_CONFIGURATION_NAME --timeout=300s --kubeconfig $KUBE_SERVER
kubectl wait --for jsonpath='{.status.remote.cidr.pod}' configuration $CLIENT_CONFIGURATION_NAME --timeout=300s --kubeconfig $KUBE_CLIENT

# Get cluster IDs
CLUSTER_ID_SERVER=$(kubectl get --kubeconfig $KUBE_SERVER -n liqo configmaps liqo-clusterid-configmap -o json | jq -r '.data.CLUSTER_ID')
CLUSTER_ID_CLIENT=$(kubectl get --kubeconfig $KUBE_CLIENT -n liqo configmaps liqo-clusterid-configmap -o json | jq -r '.data.CLUSTER_ID')

# Create gateways
liqoctl --kubeconfig $KUBE_SERVER create gatewayserver server --remote-cluster-id $CLUSTER_ID_CLIENT --service-type NodePort

kubectl wait --for jsonpath='{.status.endpoint.addresses[0]}' gatewayserver server --timeout=300s --kubeconfig $KUBE_SERVER
kubectl wait --for jsonpath='{.status.endpoint.port}' gatewayserver server --timeout=300s --kubeconfig $KUBE_SERVER
kubectl wait --for jsonpath='{.status.internalEndpoint.ip}' gatewayserver server --timeout=300s --kubeconfig $KUBE_SERVER

ADDRESS_SERVER=$(kubectl --kubeconfig $KUBE_SERVER get gatewayserver server -o json | jq -r '.status.endpoint.addresses[0]')
PORT_SERVER=$(kubectl --kubeconfig $KUBE_SERVER get gatewayserver server -o json | jq -r '.status.endpoint.port')

liqoctl --kubeconfig $KUBE_CLIENT create gatewayclient client --remote-cluster-id $CLUSTER_ID_SERVER --addresses $ADDRESS_SERVER --port $PORT_SERVER

kubectl wait --for jsonpath='{.status.internalEndpoint.ip}' gatewayclient client --timeout=300s --kubeconfig $KUBE_CLIENT

# Create publickeys
liqoctl --kubeconfig $KUBE_SERVER generate publickey --gateway-type server --gateway-name server > publickey-server.yaml
liqoctl --kubeconfig $KUBE_CLIENT generate publickey --gateway-type client --gateway-name client > publickey-client.yaml

kubectl --kubeconfig $KUBE_SERVER apply -f publickey-client.yaml
kubectl --kubeconfig $KUBE_CLIENT apply -f publickey-server.yaml
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
A field with a value in the form of `'{{ EXAMPLE }}'` will be templated automatically by the Gateway operator with the value(s) from the `GatewayServer`/`GatewayClient` resource.  
```
