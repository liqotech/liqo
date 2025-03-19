# liqoctl network

Manage liqo networking

## Description

### Synopsis

Manage liqo networking.


## liqoctl network connect

Connect two clusters using liqo networking

### Synopsis

Connect two clusters using liqo networking.

This command creates the Gateways to connect the two clusters.
Run this command after inizialiting the network using the *network init* command.


```
liqoctl network connect [flags]
```

### Options
`--disable-sharing-keys`

>Disable the sharing of public keys between the two clusters

`--gw-client-address` _string_:

>Define the address used by the gateway client to connect to the gateway server. This value overrides the one automatically retrieved by Liqo and it is useful when the server is not directly reachable (e.g. the server is behind a NAT)

`--gw-client-port` _int32_:

>Define the port used by the gateway client to connect to the gateway server. This value overrides the one automatically retrieved by Liqo and it is useful when the server is not directly reachable (e.g. the server is behind a NAT)

`--gw-client-template-name` _string_:

>Name of the Gateway Client template **(default "wireguard-client")**

`--gw-client-template-namespace` _string_:

>Namespace of the Gateway Client template

`--gw-client-type` _string_:

>Type of Gateway Client. Leave empty to use default Liqo implementation of WireGuard **(default "networking.liqo.io/v1beta1/wggatewayclienttemplates")**

`--gw-server-service-loadbalancerip` _string_:

>Force LoadBalancer IP of the Gateway Server service. Leave empty to use the one provided by the LoadBalancer provider

`--gw-server-service-nodeport` _int32_:

>Force the NodePort of the Gateway Server service. Leave empty to let Kubernetes allocate a random NodePort

`--gw-server-service-port` _int32_:

>Port of the Gateway Server service. Default: 51840 **(default 51840)**

`--gw-server-service-type` _string_:

>Service type of the Gateway Server service. Default: LoadBalancer. Note: use ClusterIP only if you know what you are doing and you have a proper network configuration **(default "LoadBalancer")**

`--gw-server-template-name` _string_:

>Name of the Gateway Server template **(default "wireguard-server")**

`--gw-server-template-namespace` _string_:

>Namespace of the Gateway Server template

`--gw-server-type` _string_:

>Type of Gateway Server. Leave empty to use default Liqo implementation of WireGuard **(default "networking.liqo.io/v1beta1/wggatewayservertemplates")**

`--mtu` _int_:

>MTU of the Gateway server and client. Default: 1340 **(default 1340)**


### Global options

`--cluster` _string_:

>The name of the kubeconfig cluster to use

`--context` _string_:

>The name of the kubeconfig context to use

`--global-annotations` _stringToString_:

>Global annotations to be added to all created resources (key=value)

`--global-labels` _stringToString_:

>Global labels to be added to all created resources (key=value)

`--kubeconfig` _string_:

>Path to the kubeconfig file to use for CLI requests

`--liqo-namespace` _string_:

>The namespace where Liqo is installed in **(default "liqo")**

`-n`, `--namespace` _string_:

>The namespace scope for this request

`--remote-cluster` _string_:

>The name of the kubeconfig cluster to use (in the remote cluster)

`--remote-context` _string_:

>The name of the kubeconfig context to use (in the remote cluster)

`--remote-kubeconfig` _string_:

>Path to the kubeconfig file to use for CLI requests (in the remote cluster)

`--remote-liqo-namespace` _string_:

>The namespace where Liqo is installed in (in the remote cluster) **(default "liqo")**

`--remote-namespace` _string_:

>The namespace scope for this request (in the remote cluster)

`--remote-user` _string_:

>The name of the kubeconfig user to use (in the remote cluster)

`--skip-confirm`

>Skip the confirmation prompt (suggested for automation)

`--skip-validation`

>Skip the validation

`--timeout` _duration_:

>Timeout for completion **(default 2m0s)**

`--user` _string_:

>The name of the kubeconfig user to use

`-v`, `--verbose`

>Enable verbose logs (default false)

`--wait`

>Wait for completion

## liqoctl network disconnect

Disconnect two clusters keeping the network configuration

### Synopsis

Disconnect two clusters keeping the network configuration.

It deletes the Gateways, but keeps the network configurations generated with the *network init* command.
Useful when a user wants to disconnect the clusters keeping the same IP mapping.


```
liqoctl network disconnect [flags]
```

### Options

### Global options

`--cluster` _string_:

>The name of the kubeconfig cluster to use

`--context` _string_:

>The name of the kubeconfig context to use

`--global-annotations` _stringToString_:

>Global annotations to be added to all created resources (key=value)

`--global-labels` _stringToString_:

>Global labels to be added to all created resources (key=value)

`--kubeconfig` _string_:

>Path to the kubeconfig file to use for CLI requests

`--liqo-namespace` _string_:

>The namespace where Liqo is installed in **(default "liqo")**

`-n`, `--namespace` _string_:

>The namespace scope for this request

`--remote-cluster` _string_:

>The name of the kubeconfig cluster to use (in the remote cluster)

`--remote-context` _string_:

>The name of the kubeconfig context to use (in the remote cluster)

`--remote-kubeconfig` _string_:

>Path to the kubeconfig file to use for CLI requests (in the remote cluster)

`--remote-liqo-namespace` _string_:

>The namespace where Liqo is installed in (in the remote cluster) **(default "liqo")**

`--remote-namespace` _string_:

>The namespace scope for this request (in the remote cluster)

`--remote-user` _string_:

>The name of the kubeconfig user to use (in the remote cluster)

`--skip-confirm`

>Skip the confirmation prompt (suggested for automation)

`--skip-validation`

>Skip the validation

`--timeout` _duration_:

>Timeout for completion **(default 2m0s)**

`--user` _string_:

>The name of the kubeconfig user to use

`-v`, `--verbose`

>Enable verbose logs (default false)

`--wait`

>Wait for completion

## liqoctl network reset

Tear down liqo networking between two clusters (disconnect and remove network configurations)

### Synopsis

Tear down all liqo networking between two clusters.

It disconnects the two clusters and remove network configurations generated with the *network init* command.


```
liqoctl network reset [flags]
```

### Options

### Global options

`--cluster` _string_:

>The name of the kubeconfig cluster to use

`--context` _string_:

>The name of the kubeconfig context to use

`--global-annotations` _stringToString_:

>Global annotations to be added to all created resources (key=value)

`--global-labels` _stringToString_:

>Global labels to be added to all created resources (key=value)

`--kubeconfig` _string_:

>Path to the kubeconfig file to use for CLI requests

`--liqo-namespace` _string_:

>The namespace where Liqo is installed in **(default "liqo")**

`-n`, `--namespace` _string_:

>The namespace scope for this request

`--remote-cluster` _string_:

>The name of the kubeconfig cluster to use (in the remote cluster)

`--remote-context` _string_:

>The name of the kubeconfig context to use (in the remote cluster)

`--remote-kubeconfig` _string_:

>Path to the kubeconfig file to use for CLI requests (in the remote cluster)

`--remote-liqo-namespace` _string_:

>The namespace where Liqo is installed in (in the remote cluster) **(default "liqo")**

`--remote-namespace` _string_:

>The namespace scope for this request (in the remote cluster)

`--remote-user` _string_:

>The name of the kubeconfig user to use (in the remote cluster)

`--skip-confirm`

>Skip the confirmation prompt (suggested for automation)

`--skip-validation`

>Skip the validation

`--timeout` _duration_:

>Timeout for completion **(default 2m0s)**

`--user` _string_:

>The name of the kubeconfig user to use

`-v`, `--verbose`

>Enable verbose logs (default false)

`--wait`

>Wait for completion

