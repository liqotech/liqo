# Resource Sharing



## Virtual kubelet

Based on the wonderful [Virtual Kubelet (VK)](https://github.com/virtual-kubelet/virtual-kubelet) project, the Liqo VK 
is responsible to "masquerade" a foreign Kubernetes cluster.

The Liqo VK is responsible to:

* Reconcile pods across different clusters
* Nats namespace across foreign clusters
* Creates the node resource and keep it posted to the corresponding advertisement
* Reflect service and endpoints to make services accessible on foreign cluster
* Reflect configmap and secrets to make configuration accessible on remote clusters

### Cluster Natting

At start time, the virtual kubelet fetches from the etcd a CR of kind `namespacenattingtable` (or creates it if it 
doesn't exist) that contains the natting table of the namespaces for the given virtual node, i.e., the translation 
between local namespaces and remote namespaces. 
Every time a new entry is added in this natting table, a new reflection routine for that namespace is triggered; this 
routine implies:


### Remote Reflection

The remote reflection keeps synchronized many resources, among which:
  * `service`
  * `endpoints`
  * `configmap`
  * `secret` 


### Remote pod-watcher
The remote pod-watcher is a routine that listens for all the events related to a remotely offloaded pod in a given 
translated namespace; this is needed to reconcile the remote status with the local one, such that the local cluster 
always knows in which state each offloaded pod is. There are some remote status transitions that trigger the 
`providerFailed` status in the local pod instance: `providerFailed` means that the local status cannot be correctly 
updated because of an unrecognized remote status transition. We need to deeper investigate for understanding when and 
why this status is triggered and to avoid it as much as possible.
The currently known reasons that trigger this status are:
* deletion of an offloaded pod from the remote cluster