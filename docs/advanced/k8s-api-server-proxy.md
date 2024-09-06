# Kubernetes API Server Proxy

The Kubernetes API Server Proxy is a Liqo component that allows you to expose the Kubernetes API Server of a cluster to a remote cluster through the Liqo network fabric.
This feature is useful when you want to establish a peering relationship between two clusters, but you cannot expose the Kubernetes API Server to the public.

The Kubernetes API Server Proxy is an Envoy HTTP server that accepts [HTTP Connect](https://developer.mozilla.org/en-US/docs/Web/HTTP/Methods/CONNECT) requests and forwards them to the Kubernetes API Server of the local cluster.
It has no permission on the local cluster, and the requesters must authenticate with the Kubernetes API Server to access the resources.

The Kubernetes API Server Proxy is deployed as a Kubernetes deployment in the `liqo` namespace.

By default, its service is remapped by an `IP` CRD in the `liqo` namespace.
You can get its IP address by running the following command:

```bash
kubectl get ip -n liqo api-server-proxy -o wide
```

```text
NAME               LOCAL IP        AGE    REMAPPED IPS
api-server-proxy   10.95.245.141   167m   {"wild-frog":"10.70.0.2"}
```

Note that in order to use it in a remote cluster, you may need to remap it to a different IP address according to the remote cluster network configuration as explained in the [external IP remapping](ExternalIPRemappingConnectToExternalHost) page.

For example, if the `REMAPPED_EXT_CIDR` is *10.81.0.0/16* and the `REMAPPED_IP` is *10.70.0.3* the final IP will be *10.81.0.3*.
