# liqoctl authenticate

Authenticate with a provider cluster

## Description

### Synopsis

Authenticate with a provider cluster.

This command allows a consumer cluster to communicate with a remote provider cluster
to obtain slices of resources from. At the end of the process, the consumer cluster will
be able to replicate ResourceSlices resources to the provider cluster, and to receive
an associated Identity to consume the provided resources.



```
liqoctl authenticate [flags]
```

### Examples


```bash
  $ liqoctl authenticate --remote-kubeconfig <provider>
```





### Options
`--cluster` _string_:

>The name of the kubeconfig cluster to use

`--context` _string_:

>The name of the kubeconfig context to use

`--in-band`

>Use in-band authentication. Use it only if required and if you know what you are doing

`--kubeconfig` _string_:

>Path to the kubeconfig file to use for CLI requests

`-n`, `--namespace` _string_:

>The namespace where Liqo is installed in **(default "liqo")**

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

`--timeout` _duration_:

>Timeout for completion **(default 2m0s)**

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

