# Discovery 

## Scenarios

* Exploring Neighborhood (e.g. LAN) for resources
* DNS Discovery
* Manual Insertion

# Neighborhood discovery

service aims to join two clusters running Liqo. We call "client" cluster the one that needs resources and "server" 
cluster the one that can share resources.

Discovery service allows two clusters to know each other, ask for resources and begin exchanging Advertisements.
The protocol is described by the following steps:
1. each cluster creates and manages a ConfigMap containing a kubeconfig file with create-only permission on `FederationRequest` resources
2. each cluster registers its master IP and ConfigMap URL to a mDNS service
3. the requesting cluster sends on local network a mDNS query to find available servers
4. when someone replies, the requesting cluster downloads its exposed kubeconfig
5. the client cluster stores this information in `ForeignCluster` CR along with their `clusterID`
6. when the `Join` flag in the `ForeignCluster` CR becomes true (either automatically or manually), 
an operator is triggered and uses the stored kubeconfig to create a new `FederationRequest` CR on the _foreign cluster_.
 `FederationRequest` creation process includes the creation of new kubeconfig with management permission on `Advertisement` CRs
7. on the server cluster, an admission webhook accept/reject `FederationRequest`s
8. the `FederationRequest` is used to start the sharing of resources

