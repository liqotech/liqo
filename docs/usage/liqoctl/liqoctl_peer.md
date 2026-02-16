# liqoctl peer

Enable a peering towards a remote cluster

## Description

### Synopsis

Enable a peering towards a remote provider cluster.

In Liqo, a *peering* is a unidirectional resource and service consumption
relationship between two Kubernetes clusters, with one cluster (i.e., the
consumer) granted the capability to offload tasks in a remote cluster (i.e., the
provider), but not vice versa. Bidirectional peerings can be achieved through
their combination. The same cluster can play the role of provider and consumer
in multiple peerings.

This commands enables a peering towards a remote provider cluster, performing
the following operations:
- [optional] ensure networking between the two clusters
- ensure authentication between the two clusters (Identity in consumer cluster,
  Tenant in provider cluster)
- [optional] create ResourceSlice in consumer cluster and wait for it to be
  accepted by the provider cluster
- [optional] create VirtualNode in consumer cluster



```
liqoctl peer [flags]
```

### Examples


```bash
  $ liqoctl peer --remote-kubeconfig <provider>
  $ liqoctl peer --remote-kubeconfig <provider> --gw-server-service-type NodePort
  $ liqoctl peer --remote-kubeconfig <provider> --cpu 2 --memory 4Gi --pods 10
  $ liqoctl peer --remote-kubeconfig <provider> --cpu 2 --memory 4Gi --pods 10 --resource nvidia.com/gpu=2
  $ liqoctl peer --remote-kubeconfig <provider> --create-resource-slice false
  $ liqoctl peer --remote-kubeconfig <provider> --create-virtual-node false
```





### Options
`--cluster` _string_:

>The name of the kubeconfig cluster to use

`--context` _string_:

>The name of the kubeconfig context to use

`--cpu` _string_:

>The amount of CPU requested for the VirtualNode

`--create-resource-slice`

>Create a ResourceSlice for the peering **(default true)**

`--create-virtual-node`

>Create a VirtualNode for the peering **(default true)**

`--gw-client-address` _string_:

>Define the address used by the gateway client to connect to the gateway server. This value overrides the one automatically retrieved by Liqo and it is useful when the server is not directly reachable (e.g. the server is behind a NAT)

`--gw-client-port` _int32_:

>Define the port used by the gateway client to connect to the gateway server. This value overrides the one automatically retrieved by Liqo and it is useful when the server is not directly reachable (e.g. the server is behind a NAT)

`--gw-server-service-loadbalancerip` _string_:

>IP of the LoadBalancer for the Gateway Server service

`--gw-server-service-location` _string_:

>Location of the service to expose the Gateway Server ("Consumer" or "Provider"). Default: "Provider" **(default "Provider")**

`--gw-server-service-nodeport` _int32_:

>Force the NodePort of the Gateway Server service. Leave empty to let Kubernetes allocate a random NodePort

`--gw-server-service-port` _int32_:

>Port of the Gateway Server service. Default: 51840 **(default 51840)**

`--gw-server-service-type` _string_:

>Service type of the Gateway Server service. Default: LoadBalancer. Note: use ClusterIP only if you know what you are doing and you have a proper network configuration **(default "LoadBalancer")**

`--in-band`

>Use in-band authentication. Use it only if required and if you know what you are doing

`--kubeconfig` _string_:

>Path to the kubeconfig file to use for CLI requests

`--memory` _string_:

>The amount of memory requested for the VirtualNode

`--mtu` _int_:

>MTU of the Gateway server and client. Default: 1340 **(default 1340)**

`-n`, `--namespace` _string_:

>The namespace where Liqo is installed in **(default "liqo")**

`--networking-disabled`

>Disable networking between the two clusters

`--pods` _string_:

>The amount of pods requested for the VirtualNode

`--proxy-url` _string_:

>The URL of the proxy to use for the communication with the remote cluster

`--remote-cluster` _string_:

>The name of the kubeconfig cluster to use (in the remote cluster)

`--remote-context` _string_:

>The name of the kubeconfig context to use (in the remote cluster)

`--remote-kubeconfig` _string_:

>Path to the kubeconfig file to use for CLI requests (in the remote cluster)

`--remote-namespace` _string_:

>The namespace where Liqo is installed in (in the remote cluster) **(default "liqo")**

`--remote-user` _string_:

>The name of the kubeconfig user to use (in the remote cluster)

`--resource` _stringToString_:

>Other resources requested for the VirtualNode (e.g., '--resource=nvidia.com/gpu=2')

`--resource-slice-class` _string_:

>The class of the ResourceSlice **(default "default")**

`--skip-validation`

>Skip the validation

`--timeout` _duration_:

>Timeout for peering completion **(default 10m0s)**

`--user` _string_:

>The name of the kubeconfig user to use

`-v`, `--verbose`

>Enable verbose logs (default false)


### Global options

`--global-annotations` _stringToString_:

>Global annotations to be added to all created resources (key=value)

`--global-labels` _stringToString_:

>Global labels to be added to all created resources (key=value)

`--skip-confirm`

>Skip the confirmation prompt (suggested for automation)

