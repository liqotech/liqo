---
title: Discovery
weight: 2
---

When a first (home) cluster would like to peer to a second (foreign) cluster, it needs to know the required network parameters
(e.g., the IP address of the Kubernetes API server, etc).
Liqo simplifies this process by defining three ways of discovering a remote cluster:

* **LAN Discovery**: automatic discovery of other **neighboring clusters**, reachable on the same local area network of your cluster. This looks similar to the automatic discovery of WiFi hotspots available nearby and it is particularly suitable when your cluster is composed of a single node (e.g., using [K3s](https://k3s.io)).
* **DNS Discovery**: automatic discovery of the parameters associated with **remote clusters** through a proper set of DNS queries (e.g., *which are the Liqo parameters if I want to peer with domain `foo.bar`?*). This looks similar to the discovery of voice-over-IP SIP servers and it is mostly oriented to big organizations that have already adopted Liqo in production.
* **Manual Discovery**: when the other methods are not available, Liqo allows the manual insertion of the parameters required for the peering. This method is particularly appropriate for testing purposes, when you want to connect to a remote cluster and the DNS discovery is not yet available.

## Required Parameters

We need some parameters to contact and to connect to a remote cluster:

* `ClusterID`: cluster's unique identifier, it is created by Liqo during the first run
* `LiqoNamespace`: namespace where the Liqo components are deployed and they expect to find the resources required to work. If you installed Liqo with the [provided script](../../gettingstarted/install/) the namespace should be `liqo`
* `IP` and `Port` of the Kubernetes API Server

<!-- TODO As discussed in the weekly call on 18/08, not clear why we need to specify ClusterID, instead of allowing the system to discover that parameter automatically -->

<!-- TODO Alex, please help me here. Most of the following text doesn't look appropriate for this section, which is about "advanced config". It looks you should be a developer to understand most of the following. Should we move this text into another place? -->


## Peering Process

1. Each cluster grants create-only permissions on `FederationRequest` resources to unauthenticated user.
2. When the `Join` flag in the `ForeignCluster` CR becomes true (either automatically or manually),
   an operator is triggered and creates a new `FederationRequest` CR in the _foreign cluster_.
   The `FederationRequest` creation process includes the creation of new kubeconfig with management permissions on
   `Advertisement` CRs.
3. In the foreign cluster, an admission webhook accepts/rejects `FederationRequest`s.
4. The `FederationRequest` is used to start the sharing of resources.


## Neighbor discovery (on LAN)

<!-- TODO Alex, should we move this into the 'architecture' section? -->

Discovery service allows two clusters to know each other, ask for resources and begin exchanging `Advertisements`.
The protocol is described by the following steps:

1. Each cluster registers its master IP, its ClusterID and the namespace where Liqo is deployed to a mDNS service
2. The requesting cluster sends on local network a mDNS query to find available foreigns
3. When someone replies, the requesting cluster gets the required data from the mDNS server
4. The home cluster stores this information in a `ForeignCluster` CR, along with their `clusterID`

Exchanged DNS packets are analogous to the ones exchanged in DNS discovery with exception of PTR record.
In mDNS discovery list of all clusters will be the ones that replies on multicast query on `local.` domain.
(See following section)

## DNS Discovery

Each DNS domain (e.g., `foo.bar`) can export one or more Liqo clusters, which can then be automatically discovered though proper DNS queries. Hence, new peerings can be configured specifying the domain name of the target clusters only.

The DNS discovery mechanism exploits a mixture of `PTR`, `SRV`, and `TXT` DNS records (in addition to traditional `A/AAAA/CNAME` ones) and is very similar to the service discovery defined by other services, such as the Session Initiation Protocol (SIP) for discovering voice-over-IP servers.

This mechanism is mostly oriented to big organizations that have already adopted Liqo in production and that would like to simplify the peering with a remote cluster.

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

Given the proper DNS configuration, the discovery process consists in at least four DNS queries:

1. `PTR` query to get the list of clusters registered on this domain (mydomain.com);
2. `SRV` query to get the network parameters (transport-level protocol, TCP/UDP port) required to connect to the Kubernetes API server of the selected cluster;
3. `TXT` query to get ClusterID and the LiqoNamespace of the selected Liqo instance;
4. `CNAME/A/AAAA` query to get the actual IP address of the Kubernetes API server.

With this configuration two `ForeignCluster`s will be created locally (if local cluster has different clusterID from both)

### How can I trigger this process?

This process can be triggered by telling Liqo which domain to look for. This operation can be performed through the Liqo dashboard or manually adding a `SearchDomain` resource.

Create a new file (`mydomain.yaml`) with this content:

```yaml
apiVersion: discovery.liqo.io/v1alpha1
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

When this resource is created, changed or regularly every 30 seconds (by default), the SearchDomain operator contacts the DNS server specified in the ClusterConfig (8.8.8.8:53 by default), loads the data and creates the discovered ForeignClusters.


## Manual Insertion

Through the manual insertion procedure it is possible to make one cluster aware of the existence of another one, either in the same or in a different network, without the need for a DNS server.

We need Liqo up and running on both clusters, then we can get the foreign `ClusterID` through:

```bash
kubectl get configmap cluster-id -n <LiqoNamespace>
```

Copy your foreign `clusterID` inside a new `ForeignCluster` CR and fill the `namespace` and `apiUrl` fields:

```yaml
apiVersion: discovery.liqo.io/v1alpha1
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

Then apply this file to the home cluster:

```bash
kubectl apply -f foreign-cluster.yaml
```

Wait few seconds and a new node will appear on your home cluster.

<!--
## Trust Remote Clusters

#### The Problem

In Liqo all communications between two clusters are on HTTPS protocol. so, how can we know who is the cluster that we are peering with? It can expose known IP and clusterId, but everyone else can set it and steal our offloaded jobs... So we will use TLS server authentication.
What is the problem? By default, Kubernetes clusters expose API server with a self-signed certificate that, again, does not provide us a way to trust the remote cluster.
We can add another certificate to API server issued by a trusted Certification Authority (CA) or making this self-signed CA as trusted in home cluster, in this way the peering will be authenticated.
Liqo supports both authenticate and unauthenticated peering, in environments controlled and safe authentication can be unnecessary (i.e. at your home), but in environments public and unsafe trusted mode is strongly recommended.

If a remote cluster requires authentication, `ForeignCluster` CR will be created with the `allowUntrustedCA` flag disabled in its Spec. If the remote cluster certificate is signed by a "default" root CA we are ok, otherwise we have to add its root CA (provided Out-Of-Band) in our `trusted-ca-certificates` ConfigMap.


### Trusted Mode

When trusted mode is enabled our cluster does not expose our CA certificate, if a remote cluster want to join us has to trust our CA and check our identity.

![../images/discovery/trusted.png](/images/discovery/trusted.png)

With the trusted mode clusters are not allowed to send PeeringRequest if they don't authenticate the remote cluster.

API server certificate has to be issued from "default" root CAs or by CA provided out-of-band.

Peering process will be automatically triggered if local cluster config has `autojoin` flag active.

#### Trust public CA

If your API server exposes a certificate issued by a public CA, your identity will be automatically checked and there is no further actions needed

#### Add trusted CA

If your API server exposes a certificate issued by your own CA, you have to add this CA as trusted in the remote cluster. To do that you have to add your CA certificate in a ConfigMap:
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

__NOTE:__ when this ConfigMap is updated, the discovery component will trigger a restart to reload new CA configurations.


### Untrusted Mode

When a new Kubernetes cluster is deployed, by default, it creates a new self-signed CA that is be used to issue all certificates. This CA needs to be trusted by each remote client that wants to contact the cluster.

To allow the users to use Liqo without requiring to manage TLS certificates and have a trusted CA installed in the API server, we support an Untrusted Mode. With this modality a cluster that wants to contact another one can read its CA certificate in a well-known path.

![../images/discovery/untrusted.png](/images/discovery/untrusted.png)

With the untrusted mode clusters are allowed to send PeeringRequest simply downloading CA from the remote cluster.

Peering process will be automatically triggered if local cluster config has `autojoinUntrusted` flag active.
-->
