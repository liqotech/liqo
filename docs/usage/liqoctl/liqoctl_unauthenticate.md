# liqoctl unauthenticate

Unauthenticate a pair of consumer and provider clusters

## Description

### Synopsis

Unauthenticate a pair of consumer and provider clusters.

This command deletes all authentication resources on both consumer and provider clusters.
In the consumer cluster, it deletes the control plane Identity.
In the provider cluster, it deletes the Tenant.
The execution is prevented if any ResourceSlice or VirtualNode associated with the provider cluster is found.



```
liqoctl unauthenticate [flags]
```

### Examples


```bash
  $ liqoctl unauthenticate --remote-kubeconfig <provider>
```





### Options
`--cluster` _string_:

>The name of the kubeconfig cluster to use

`--context` _string_:

>The name of the kubeconfig context to use

`--kubeconfig` _string_:

>Path to the kubeconfig file to use for CLI requests

`-n`, `--namespace` _string_:

>The namespace where Liqo is installed in **(default "liqo")**

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

`--wait`

>Wait for the unauthentication to complete **(default true)**


### Global options

`--global-annotations` _stringToString_:

>Global annotations to be added to all created resources (key=value)

`--global-labels` _stringToString_:

>Global labels to be added to all created resources (key=value)

`--skip-confirm`

>Skip the confirmation prompt (suggested for automation)

