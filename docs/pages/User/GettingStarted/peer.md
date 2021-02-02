---
title: Peer to a foreign cluster
weight: 2
---

Once Liqo is installed in your cluster, you can start establishing new *peerings*.
Specifically, you can rely on three different methods to peer with other clusters:

1. **LAN Discovery**: automatic discovery of neighboring clusters available in the same LAN. This looks similar to the automatic discovery of Wi-Fi hotspots and it is particularly suitable when your cluster is composed of a single node (e.g., in combination of [K3s](https://k3s.io)).
2. **DNS Discovery**: automatic discovery of the clusters associated with a specific DNS domain (e.g.; *liqo.io*), by scraping the existence of specific DNS entries. This looks similar to the discovery of voice-over-IP SIP servers and it is mostly oriented to big organizations that wish to adopt Liqo in production.
3. **Manual Configuration**: manual addition of specific clusters to the list of known ones. This method is particularly appropriate for testing purposes, as well as to try Liqo outside LAN, without requiring any DNS configuration.

## LAN Discovery

Liqo is able to automatically discover any available clusters running on the same LAN, as well as to make your cluster discoverable by others.

The currently available and established peerings can be easily monitored through the dashboard.
Just log in, and the home view will present you the list of "Available Peers" (i.e. all discovered foreign clusters).
By default, Liqo tries to establish peerings with on-LAN remote clusters as soon as they are discovered.
Hence, you may already observe a remote cluster presented inside the "Connected Peers" section of the dashboard.

{{%expand "Using kubectl, you can also manually obtain the list of discovered foreign clusters:" %}}

```bash
kubectl get foreignclusters
NAME                                   AGE
ff5aa14a-dd6e-4fd2-80fe-eaecb827cf55   101m
```

To check whether Liqo is configured to automatically attempt to peer with the foreign cluster,
you can check the `join` property of the specific `ForeignCluster` resource:
```bash
kubectl get foreignclusters ${FOREIGN_CLUSTER} --template={{.spec.join}}
true
```
{{% /expand %}}


## DNS Discovery

The DNS discovery procedure requires two orthogonal actions to be enabled.
1. Register your own cluster into your DNS server to make it discoverable by others (the required parameters are presented in the section below).
2. Connect to a foreign cluster, specifying the remote domain.

### Register the home cluster

To allow other cluster to peer with your cluster(s), you need to register a set of DNS records that specify the cluster(s) available in your domain, with the different parameters required to establish the connection.

#### Get the Required Values

First, you have to retrieve how to reach the Auth Service of your cluster.

The first required value is or the __hostname__ or the __IP address__ where it is reachable.
If you specified a name during the installation, it will be reachable with an Ingress (you can get it with `kubectl get ingress -n liqo`),
if not it is exposed with a NodePort Service, so you can get one if the IPs of the nodes of your cluster (`kubectl get nodes -o wide`).

The second required value is the __port__ where it is reachable.
If you are using an Ingress it should be reachable at port `443`, Else if you are using a NodePort Service you can get the port executing
`kubectl get service -n liqo auth-service`, an output example could be:

```txt
NAME           TYPE       CLUSTER-IP    EXTERNAL-IP   PORT(S)         AGE
auth-service   NodePort   10.81.20.99   <none>        443:30740/TCP   2m7s
```
where "30740" is the port where the service is listening.

#### DNS Configuration

Now, it is possible to configure the records necessary to enable the DNS discovery process.
In the following, we present a `bind9`-like configuration for an hypothetical domain `example.com`. It exposes one Liqo-enabled cluster named `liqo-cluster`, with the Auth Service accessible at `1.2.3.4:443`.
Remember to adapt the configuration according to your setup.
```txt
example.com.                  PTR     liqo-cluster.example.com.

liqo-cluster.example.com.     SRV     0 0 443 auth.server.example.com.

auth.server.example.com.      A       1.2.3.4
```

{{%expand "Expand here to know more about the meaning of each record." %}}

* the `PTR` record lists the Liqo clusters exposed for the specific domain (e.g. `liqo-cluster`).
* the `SRV` record specifies the network parameters needed to connect to the Auth Service of the cluster.
  Specifically, it has the following format:
  ```txt
   <cluster-name>._liqo._tcp.<domain>. SRV <priority> <weight> <auth-server-port> <auth-server-name>.
  ```
  where the priority and weight fields are unused and should be set to zero. In this case, the API server is reachable at the address `liqo-cluster-api.server.example.com` through port `6443`.
* The `A` record assigns an IP address to the DNS name of the Auth Service server, in this case `1.2.3.4`.

{{% /expand %}}

### Connect to a remote cluster

In order to leverage the DNS discovery to peer to a remote cluster, it is necessary to specify the remote domain.
This operation can be easily performed through the graphical dashboard: click on the "+" icon located near Available Peers and then select "Add domain".
Here, you need to configure the following parameters:
1. **Domain**: the domain where the cluster you want to peer with is located.
2. **Name**: a mnemonic name to identify the domain.
3. **Join**: to specify whether to automatically trigger the peering procedure.

{{%expand "Using kubectl, it is also possible to perform the same configuration." %}}

```
cat << "EOF" | kubectl apply -f
apiVersion: discovery.liqo.io/v1alpha1
kind: SearchDomain
metadata:
  name: example.com
spec:
  domain: example.com
  autojoin: true
EOF
```

{{% /expand %}}

## Manual Configuration

If the cluster you want to peer with is not present in your LAN, and you do not want to configure the DNS discovery,
it is possible to set-up a manual peering via the graphical dashboard.

First, you have to collect some information about the foreign cluster as described [above](#get-the-required-values). In particular:

1. The **Auth Service Hostname/IP**: the address of the foreign cluster. It should be accessible without NAT from the Home cluster.
2. The **Auth Service Port**: the port where the foreign Auth Service is listening.

<!-- TODO: are the dashboard instructions still valid? -->
Now, through the dashboard of the home cluster, you can configure the new peering. Click on the "+" icon located near Available Peers and then select "Add Remote Peer". Here, you need to configure the parameters obtained during the previous step. Additionally, you also have to set the following values:

 1. **Name**: Name of the cluster (i.e. a mnemonic name, which can also be identical to the clusterID)
 2. **Join**: True (i.e. automatically trigger the peering procedure)

> **Note:** When the peering is active, a new ForeignCluster will be created in the remote cluster. There is no need to repeate this procedure.
> You can enable the "join" flag in the ForeignCluster CR spec to enable a bi-directional peering.

{{%expand "Using kubectl, it is also possible to perform the same configuration." %}}

```
cat << "EOF" | kubectl apply -f
apiVersion: discovery.liqo.io/v1alpha1
kind: ForeignCluster
metadata:
  name: foreign-cluster
spec:
  join: true        # optional (defaults to false)
  authUrl: https://${AUTH_SERVICE_ADDRESS}:${AUTH_SERVICE_PORT}
EOF
```

{{% /expand %}}

## Peering checking

### Presence of the virtual-node

If the peering has been correctly performed, you should see a virtual node (named `liqo-*`) in addition to your physical nodes:

```
kubectl get nodes

NAME                                      STATUS   ROLES
master-node                               Ready    master
worker-node-1                             Ready    <none>
worker-node-2                             Ready    <none>
liqo-9a596a4b-591c-4ac6-8fd6-80258b4b3bf9 READY    agent    <-- This is the virtual node
```

## Verify that the resulting infrastructure works correctly

You are now ready to verify that the resulting infrastructure works correctly, which is presented in the [next step](../test).
