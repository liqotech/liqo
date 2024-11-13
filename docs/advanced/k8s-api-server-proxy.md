# Kubernetes API Server Proxy

The Kubernetes API Server Proxy is a Liqo component that allows you to expose the Kubernetes API Server of a cluster to a remote cluster through the Liqo network fabric.

This feature is **internally** used by the [in-band peering](UsagePeeringInBand) to establish a peering relationship between two clusters, which does not publicly expose the Kubernetes API Server.

```{warning}
If you just need to peer two clusters without publicly exposing the Kubernetes API server, you can use the [in-band peering](UsagePeeringInBand).
```

The Kubernetes API Server Proxy is an HTTP server that accepts [HTTP Connect](https://developer.mozilla.org/en-US/docs/Web/HTTP/Methods/CONNECT) requests and forwards them to the Kubernetes API Server of the local cluster.
It just proxy the requests to the API server and it has no permission on the local cluster.
This means that, as usual, all the requesters must authenticate with the Kubernetes API Server to access the resources.

By default the Kubernetes API Server Proxy is deployed as a Kubernetes deployment in the `liqo` namespace and it is exposed by a service, whose address is remapped by an `IP` CRD located in the `liqo` namespace.
This allows another peer cluster to reach the Kubernetes API server of the peer even when this is not directly reachable, by passing through the **gateways tunnel** between the clusters, using the remapped IP.

You can get this IP address by running the following command:

```bash
kubectl get ip -n liqo api-server-proxy -o wide
```

```text
NAME               LOCAL IP        AGE    REMAPPED IPS
api-server-proxy   10.95.245.141   167m   {"wild-frog":"10.70.0.2"}
```

Note that **the IP retrieved with the command above might not be used as is in a remote cluster, you may need to remap it** to a different IP address according to the remote cluster network configuration, as explained in the [external IP remapping](ExternalIPRemappingConnectToExternalHost) page.

For example, let's suppose that a cluster `cl01` wants to reach the Kubernetes API Server Proxy of another cluster `cl02`.

Looking at the `Configuration` resource, we might see that, for example, the `REMAPPED_EXT_CIDR` is *10.81.0.0/16*, which means that the requests directed to that network will be redirected to cluster `cl01` and remmapped to the `cl02` external CIDR.
Therefore, if the `REMAPPED_IP` of the `api-server-proxy` in `cl02` is *10.70.0.3*, the final IP to be used in `cl01` to reach the Kubernetes API Server Proxy will be *10.81.0.3*.
