# liqoctl delete

Delete Liqo resources

## Description

### Synopsis

Delete Liqo resources.


## liqoctl delete configuration

Delete a Configuration

### Synopsis

Delete a Configuration.



```
liqoctl delete configuration [flags]
```

### Examples


```bash
  $ liqoctl delete configuration my-configuration
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

`-n`, `--namespace` _string_:

>The namespace scope for this request

`--skip-confirm`

>Skip the confirmation prompt (suggested for automation)

`--user` _string_:

>The name of the kubeconfig user to use

`-v`, `--verbose`

>Enable verbose logs (default false)

## liqoctl delete gatewayclient

Delete a GatewayClient

### Synopsis

Delete a GatewayClient.



```
liqoctl delete gatewayclient [flags]
```

### Examples


```bash
  $ liqoctl delete gatewayclient my-gateway-client
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

`-n`, `--namespace` _string_:

>The namespace scope for this request

`--skip-confirm`

>Skip the confirmation prompt (suggested for automation)

`--user` _string_:

>The name of the kubeconfig user to use

`-v`, `--verbose`

>Enable verbose logs (default false)

## liqoctl delete gatewayserver

Delete a GatewayServer

### Synopsis

Delete a GatewayServer.



```
liqoctl delete gatewayserver [flags]
```

### Examples


```bash
  $ liqoctl delete gatewayserver my-gateway-server
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

`-n`, `--namespace` _string_:

>The namespace scope for this request

`--skip-confirm`

>Skip the confirmation prompt (suggested for automation)

`--user` _string_:

>The name of the kubeconfig user to use

`-v`, `--verbose`

>Enable verbose logs (default false)

## liqoctl delete peering-user

Delete an existing user with the permissions to peer with this cluster

### Synopsis

elete an existing user with the permissions to peer with this cluster.

Delete a peering user, so that it will no longer be able to peer with this cluster from the cluster with the given Cluster ID.
The previous credentials will be invalidated, and cannot be used anymore, even if the user is recreated.



```
liqoctl delete peering-user [flags]
```

### Examples


```bash
  $ liqoctl delete peering-user --consumer-cluster-id=<cluster-id>
```


### Options
`--consumer-cluster-id` _clusterID_:

>The cluster ID of the cluster from which peering has been performed


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

`-n`, `--namespace` _string_:

>The namespace scope for this request

`--skip-confirm`

>Skip the confirmation prompt (suggested for automation)

`--user` _string_:

>The name of the kubeconfig user to use

`-v`, `--verbose`

>Enable verbose logs (default false)

## liqoctl delete publickey

Delete a PublicKey

### Synopsis

Delete a PublicKey.



```
liqoctl delete publickey [flags]
```

### Examples


```bash
  $ liqoctl delete publickey my-public-key
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

`-n`, `--namespace` _string_:

>The namespace scope for this request

`--skip-confirm`

>Skip the confirmation prompt (suggested for automation)

`--user` _string_:

>The name of the kubeconfig user to use

`-v`, `--verbose`

>Enable verbose logs (default false)

## liqoctl delete virtualnode

Delete a virtual node

### Synopsis

Delete a virtual node.



```
liqoctl delete virtualnode [flags]
```

### Examples


```bash
  $ liqoctl delete virtualnode my-cluster
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

`-n`, `--namespace` _string_:

>The namespace scope for this request

`--skip-confirm`

>Skip the confirmation prompt (suggested for automation)

`--user` _string_:

>The name of the kubeconfig user to use

`-v`, `--verbose`

>Enable verbose logs (default false)

