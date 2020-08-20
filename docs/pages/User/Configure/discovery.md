---
title: Discovery
weight: 1
---

When a first (home) cluster would like to peer to a second (foreign) cluster, it needs to know of the required parameters
(e.g., IP address of the Kubernetes API server, etc).
Liqo simplifies this process by defining three way of discovering a remote cluster:

* **LAN Discovery**: automatic discovery of other **neighboring clusters**, reachable on the same local area network of your cluster. This looks similar to the automatic discovery of WiFi hotspots available nearby and it is particularly suitable when your cluster is made by a single node (e.g., using [K3s](https://k3s.io)).
* **DNS Discovery**: automatic discovery of the parameters associated to **remote clusters** through a proper set of DNS queries (e.g., *which are the Liqo parameters if I want to peer with domain `foo.bar`?*). This looks similar to the discovery of voice-over-IP SIP servers and it is mostly oriented to big organizations that have already adopted Liqo in production.
* **Manual Discovery**: when other methods are not available, Liqo allows the manual insertion of the parameters required for the peering. This medhod is particularly appropriate for testing purposes, when you want to connect to a remote cluster and the DNS discovery is not yet available.

## Required Parameters

We need some parameters to contact and to connect to a remote cluster:

* `ClusterID`: cluster's unique identifier, it is created by Liqo during first run
* `LiqoNamespace`: namespace where Liqo components are deployed and they expect to find resources required to work. If you installed Liqo with the [provided script](../../gettingstarted/install/) the namespace should be `liqo`
* `IP` and `Port` of the Kubernetes API Server

<!-- TODO As discussed in the weekly call on 18/08, not clear why we need to specify ClusterID, instead of allowing the system to discover that parameter automatically -->

<!-- TODO Alex, please help me here. Most of the following text doesn't look appropriate for this section, which is about "advanced config". It looks you should be a developer to understand most of the following. Should we move this text into another place? -->


## Peering Process

1. Each cluster allow create-only permission on `FederationRequest` resources to unathenticated user.
2. When the `Join` flag in the `ForeignCluster` CR becomes true (either automatically or manually), 
   an operator is triggered and creates a new `FederationRequest` CR on the _foreign cluster_.
   `FederationRequest` creation process includes the creation of new kubeconfig with management permission on
   `Advertisement` CRs.
3. On the foreign cluster, an admission webhook accept/reject `FederationRequest`s.
4. The `FederationRequest` is used to start the sharing of resources.


## Neighbor discovery

<!-- TODO Alex, should we move this into the 'architecture' section? -->

Discovery service allows two clusters to know each other, ask for resources and begin exchanging Advertisements.
The protocol is described by the following steps:

1. each cluster registers its master IP, its ClusterID and the namespace where Liqo is deployed to a mDNS service
2. the requesting cluster sends on local network a mDNS query to find available foreigns
3. when someone replies, the requesting cluster get required data from mDNS server
4. the home cluster stores this information in `ForeignCluster` CR along with their `clusterID`

Exchanged DNS packets are analogous to the ones exchanged in DNS discovery with exception of PTR record. 
In mDNS discovery list of all clusters will be the ones that replies on multicast query on `local.` domain. 
(See following section)

## DNS Discovery

Each DNS domain (e.g., `foo.bar` domain) can export one or more Liqo clusters, which are automatically discovered by issuing the proper queries to the DNS, starting from the domain name. In other words, we can discovery all the Liqo cluster by querying the the `foo.bar` domain.

The DNS discovery mechanism exploits a mixture of `PTR`, `SRV`, and `TXT` DNS records (in addition to traditional `A/AAAA/CNAME` ones) and is very similar to the service discovery defined by other services, such as the Session Initiation Protocol (SIP) for discovering voice-over-IP servers.

This mechanism is mostly oriented to big organizations that have already adopted Liqo in production and that would like to simplify the peering from a remote cluster.

The following snapshot shows a possible configuration of the DNS records referred to the domain `mydomain.com` that exports two Liqo clusters, called `liqo1` and `liqo2`:

```txt
mydomain.com.                    PTR     liqo1._liqo._tcp.mydomain.com.
                                         liqo2._liqo._tcp.mydomain.com.



liqo1._liqo._tcp.mydomain.com.   SRV     0 0 6443 apiserver1.mydomain.com.

liqo1._liqo._tcp.mydomain.com.   TXT     "id=<YourClusterIDHere>"
                                         "namespace=<YourNamespaceHere>"

apiserver1.mydomain.com.         A       <YourIPHere>
### OR
apiserver1.mydomain.com.         CNAME   <YourServerNameHere>


liqo2._liqo._tcp.mydomain.com.   SRV     0 0 6443 apiserver2.mydomain.com.

liqo2._liqo._tcp.mydomain.com.   TXT     "id=<YourClusterIDHere>"
                                         "namespace=<YourNamespaceHere>"

apiserver2.mydomain.com.         A       <YourIPHere>
### OR
apiserver2.mydomain.com.         CNAME   <YourServerNameHere>
```

where:
* PTR record: it simply lists which Liqo clusters are available (in this case two clusters, named `liqo1` and `liqo2`).
* SRV record: it specifies the network parameters needed to connect to the Kubernetes API server of each cluster. An SRV record has the following format:
    ```
    _service._proto.name. TTL class SRV priority weight port target.
   ```
   and, in this case, it means that anybody can peer with the first Liqo cluster by connecting to host `apiserver1.mydomain.com`, using the TCP protocol on port 6443, with priority and weight equal to zero (which are important only in case of multiple redundant servers). More information about SRV records are available on [Wikipedia](https://en.wikipedia.org/wiki/SRV_record).
* TXT record: this record is opaque to the DNS system and it can contain any information. Liqo uses the TXT record to store the `ClusterID` and `LiqoNamespace` parameters presented above.

Given the proper DNS configuration, the discovery process consist in at least 4 DNS queries:

1. `PTR` query to get the list of clusters registered on this domain (mydomain.com);
2. `SRV` query to get the network parameters (transport-level protocol, TCP/UDP port) required to connect to the Kubernetes API server of the selected cluster;
3. `TXT` query to get ClusterID and the LiqoNamespace of the selected Liqo instance;
4. `CNAME/A/AAAA` query to get the actual IP address of the Kubernetes API server.

With this configuration two `ForeignCluster`s will be created locally (if local cluster has different clusterID from both)

### How can I trigger this process?

This process can be triggered by telling Liqo which domain to look for. We can do that on Liqo dashboard on manually adding a resource

Create a new file (`mydomain.yaml`) with this content:

```yaml
apiVersion: discovery.liqo.io/v1
kind: SearchDomain
metadata:
  name: mydomain.com
spec:
  domain: mydomain.com
  autojoin: false
```

Then

```bash
kubectl apply -f mydomain.yaml
```

When this resource is created, changed or regularly each 30 seconds (by default), SearchDomain operator contacts DNS server specified in ClusterConfig (8.8.8.8:53 by default), loads data and creates discovered ForeignClusters.


## Manual Insertion

With manual insertion with can make aware one cluster of existence of another one, in same or different network, without needing of DNS server.

We need Liqo up and running on both clusters, then we can get foreign `ClusterID` from command line:

```bash
kubectl get configmap cluster-id -n <LiqoNamespace>
```

Copy your foreign `clusterID` inside a new `ForeignCluster` CR and fill `namespace` and `apiUrl` fields:

```yaml
apiVersion: discovery.liqo.io/v1
kind: ForeignCluster
metadata:
  name: foreign-cluster
spec:
  clusterID: <ForeignClusterID>
  join: true
  namespace: <LiqoNamespace>
  apiUrl: https://<ForeignIP>:6443
  discoveryType: Manual
  allowUntrustedCA: true
```

Then apply this file to home cluster:

```bash
kubectl apply -f foreign-cluster.yaml
```

Wait few seconds and a new node will appear on your home cluster.


## Trust Remote Clusters

We can check if a remote cluster requires server authentication before to peer it.

If a remote cluster requires authentication, `ForeignCluster` CR will be created with the `allowUntrustedCA` flag enabled in its Spec. If the remote cluster certificate is signed by a "default" root CA we are ok, else if not we have to add its root CA (provided Out-Of-Band) in our `trusted-ca-certificates` ConfigMap.

Example:
```bash
kubectl edit cm trusted-ca-certificates
```
```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: trusted-ca-certificates
data:
  remote: |
    -----BEGIN CERTIFICATE-----
    MIIBVjCB/qADAgECAgEAMAoGCCqGSM49BAMCMCMxITAfBgNVBAMMGGszcy1zZXJ2
    ...
    APKY9n4CRdSWSQ==
    -----END CERTIFICATE-----
```

When we will try the peering, https client will check that API server certificate was issued by one of trusted CAs.

__NOTE:__ when this ConfigMap is updated, the discovery component will trigger a restart to reload new CA configurations.


### Untrusted Mode

When a new Kubernetes cluster is deployed, by default, it creates a new CA that will be used to issue all needed certificates. This CA is required by a remote client that want to contact a cluster.

To allow users to use Liqo without the need of managing TLS certificates and to have a trusted CA installed on API server, we support an Untrusted Mode. With this modality a cluster that wants to contact another one can read its CA certificate in a well-known path.

![../images/discovery/untrusted.png](/images/discovery/untrusted.png)

With the untrusted mode clusters are allowed to send PeeringRequest simply downloading CA from the remote cluster.

Peering process will be automatically triggered if local cluster config has `autojoinUntrusted` flag active.


### Trusted Mode

When trusted mode is enabled our cluster does not expose our CA certificate, if a remote cluster want to join us has to trust our CA and check our identity.

![../images/discovery/trusted.png](/images/discovery/trusted.png)

With the trusted mode clusters are not allowed to send PeeringRequest if they don't authenticate the remote cluster.

API server certificate has to be issued from "default" root CAs or by CA provided out-of-band.

Peering process will be automatically triggered if local cluster config has `autojoin` flag active.
