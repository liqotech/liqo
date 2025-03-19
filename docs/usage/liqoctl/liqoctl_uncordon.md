# liqoctl uncordon

Uncordon a liqo resource

## Description

### Synopsis

Uncordon a liqo resource


## liqoctl uncordon resourceslice

Uncordon a ResourceSlice

### Synopsis

Uncordon a ResourceSlice.

This command allows to uncordon a ResourceSlice, allowing it to receive and accept new resources.
Resources provided by existing ResourceSlices can be accepted again.



```
liqoctl uncordon resourceslice [flags]
```

### Examples


```bash
  $ liqoctl uncordon resourceslice my-rs-name
```





### Options
`--cluster` _string_:

>The name of the kubeconfig cluster to use

`--context` _string_:

>The name of the kubeconfig context to use

`--kubeconfig` _string_:

>Path to the kubeconfig file to use for CLI requests

`--remote-cluster-id` _clusterID_:

>ClusterID of the ResourceSlice to uncordon

`--timeout` _duration_:

>Timeout for uncordon completion **(default 2m0s)**

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

## liqoctl uncordon tenant

Uncordon a tenant cluster

### Synopsis

Uncordon a tenant cluster.

This command allows to uncordon a tenant cluster, allowing it to receive and accept new resources.
Resources provided by existing ResourceSlices can be accepted again.



```
liqoctl uncordon tenant [flags]
```

### Examples


```bash
  $ liqoctl uncordon tenant my-tenant-name
```





### Options
`--cluster` _string_:

>The name of the kubeconfig cluster to use

`--context` _string_:

>The name of the kubeconfig context to use

`--kubeconfig` _string_:

>Path to the kubeconfig file to use for CLI requests

`--timeout` _duration_:

>Timeout for uncordon completion **(default 2m0s)**

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

