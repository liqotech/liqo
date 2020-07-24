---
title: "Discovery"
---

This service aims to make aware two clusters running Liqo of each other existence and, eventually, make them peer. 
We call "home" cluster the one that needs resources and "foreign" cluster the one that can share resources.

### Scenarios

When a first (home) cluster would like to peer to a second (foreign) cluster, it needs to know of the required parameters
(e.g., cluster IP address, etc). This process can be achieved in three ways:

* **LAN Discovery**: Automatic discover of other neighboring clusters, reachable on the same LAN the home cluster is attached to
* **DNS Discovery**: Automatic discover of the parameters associated to remote clusters, through a proper DNS query
* **Manual Insertion**: Manual insertion of the parameters required for the peering

#### Required Parameters

In `ForeignCluster` CR there are required parameters to connect to a foreign cluster

* `ClusterID`: cluster's unique identifier, it is created by Liqo during first run
* `LiqoNamespace`: namespace where Liqo components are deployed and they expect to find resources required to work
* `IP` and `Port` where API Server is running

#### Peering Process

1. each cluster allow create-only permission on `FederationRequest` resources to unathenticated user.
2. when the `Join` flag in the `ForeignCluster` CR becomes true (either automatically or manually), 
   an operator is triggered and creates a new `FederationRequest` CR on the _foreign cluster_.
   `FederationRequest` creation process includes the creation of new kubeconfig with management permission on
   `Advertisement` CRs.
3. on the foreign cluster, an admission webhook accept/reject `FederationRequest`s.
4. the `FederationRequest` is used to start the sharing of resources.

## Neighborhood discovery

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

### SearchDomain CRD

This resource contain the domain where clusters that we want to find are registered

```yaml
apiVersion: discovery.liqo.io/v1
kind: SearchDomain
metadata:
  name: test.mydomain.com
spec:
  domain: test.mydomain.com
  autojoin: false
```

When this resource is created, changed or regularly each 30 seconds (by default), SearchDomain operator contacts DNS server specified in ClusterConfig (8.8.8.8:53 by default), loads data and creates discovered ForeignClusters

On DNS server there will be the following DNS records:
```txt
test.mydomain.com.			PTR	myliqo1._liqo._tcp.test.mydomain.com.
						myliqo2._liqo._tcp.test.mydomain.com.



myliqo1._liqo._tcp.test.mydomain.com.	SRV	0 0 6443 cluster1.test.mydomain.com.

myliqo1._liqo._tcp.test.mydomain.com.	TXT	"id=<YourClusterIDHere>"
						"namespace=<YourNamespaceHere>"

cluster1.test.mydomain.com.		A	<YourIPHere>



myliqo2._liqo._tcp.test.mydomain.com.	SRV	0 0 6443 cluster2.test.mydomain.com.

myliqo2._liqo._tcp.test.mydomain.com.	TXT	"id=<YourClusterIDHere>"
						"namespace=<YourNamespaceHere>"

cluster2.test.mydomain.com.		A	<YourIPHere>
```

Discovery process consist on 4 DNS queries:

1. PTR query to get the list of clusters registered on this domain (test.mydomain.com)
2. SRV query to get port and nameserver for each retrieved cluster
3. TXT query to get ClusterID and the namespace where Liqo was deployed on that cluster
4. A query to get actual IP address where to contact API Server

With this configuration two `ForeignCluster`s will be created locally (if local cluster has different clusterID from both)

## Manual Insertion

With manual insertion with can make aware one cluster of existence of another one, in same or different network, without needing of DNS server

We need Liqo up and running on both clusters, then we can get foreign cluster id from command line:

```bash
kubectl get configmap cluster-id -n <LiqoNamespace>
```

Copy your foreign `clusterID` inside a new `ForeignCluster` CR and fill `namespace` and `apiUrl` fields

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

Wait few seconds and a new node will appear on your home cluster

## Trust Remote Clusters

We can check if a remote cluster requires server authentication before to peer it

If a remote cluster requires authentication, `ForeignCluster` CR will be created with the `allowUntrustedCA` flag enabled in its Spec. If the remote cluster certificate is signed by a "default" root CA we are ok, else if not we have to add its root CA (provided Out-Of-Band) in our `trusted-ca-certificates` ConfigMap

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

When we will try the peering, https client will check that API server certificate was issued by one of trusted CAs

__NOTE:__ when this ConfigMap is updated, the discovery component will trigger a restart to reload new CA configurations

### Untrusted Mode

![../images/discovery/untrusted.png](/images/discovery/untrusted.png)

With the untrusted mode clusters are allowed to send PeeringRequest simply downloading CA from remote cluster

Peering process will be automatically triggered if local cluster config has `autojoinUntrusted` flag active

### Trusted Mode

![../images/discovery/trusted.png](/images/discovery/trusted.png)

With the trusted mode clusters are not allowed to send PeeringRequest if they don't authenticate the remote cluster

API server certificate has to be issued from "default" root CAs or by CA provided out-of-band

Peering process will be automatically triggered if local cluster config has `autojoin` flag active
