# liqoctl get

Get Liqo resources

## Description

### Synopsis

Get Liqo resources.


## liqoctl get kubeconfig

Get a kubeconfig

### Synopsis

Get a Kubeconfig of an Identity of a remote cluster.



```
liqoctl get kubeconfig [flags]
```

### Examples


```bash
  $ liqoctl get kubeconfig my-identity-name --remote-cluster-id remote-cluster-id
```


### Options
`--remote-cluster-id` _clusterID_:

>The cluster ID of the remote cluster


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

## liqoctl get nonce

Get a nonce

### Synopsis

Get a Nonce.

The Nonce secret is used to authenticate the remote cluster to the local cluster.



```
liqoctl get nonce [flags]
```

### Examples


```bash
  $ liqoctl get nonce --remote-cluster-id remote-cluster-id
```


### Options
`--remote-cluster-id` _clusterID_:

>The cluster ID of the remote cluster


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

