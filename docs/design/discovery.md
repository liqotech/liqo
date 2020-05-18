Discovery service aims to join two clusters running Liqo. We call "client" cluster the one that needs resources and "server" cluster the one that can share resources.

### How it works

Server:

1. create and serve a ConfigMap with stored inside a kube-config file with create-only permission on `FederationRequest` resource
2. register master IP and ConfigMap URL to a mDNS service

Client:

1. send on local network mDNS query to find available servers
2. download kube-config from them
3. store these files in `ForeignCluster` CR along with their `clusterID`
4. an operator is running on `ForeignCluster` CRD and when Federate flag become true (both automatically or manually) uses stored kube-config file to create a new `FederationRequest` CR in foreign cluster

Server:

1. federation-requests admission webhook accept/reject `FederationRequest`s
2. using a `FederationRequest` we build the federation
