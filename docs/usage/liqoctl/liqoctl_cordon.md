# liqoctl cordon

Cordon a liqo resource

## Description

### Synopsis

Cordon a liqo resource


## liqoctl cordon resourceslice

Cordon a ResourceSlice

### Synopsis

Cordon a ResourceSlice.

This command allows to cordon a ResourceSlice, preventing it from receiving new resources.
Resources provided by existing ResourceSlices are left untouched, while new ones are denied.



```
liqoctl cordon resourceslice [flags]
```

### Examples


```bash
  $ liqoctl cordon resourceslice my-rs-name
```





### Options
`--cluster` _string_:

>The name of the kubeconfig cluster to use

`--context` _string_:

>The name of the kubeconfig context to use

`--kubeconfig` _string_:

>Path to the kubeconfig file to use for CLI requests

`--remote-cluster-id` _clusterID_:

>ClusterID of the ResourceSlice to cordon

`--timeout` _duration_:

>Timeout for cordon completion **(default 2m0s)**

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

## liqoctl cordon tenant

Cordon a tenant cluster

### Synopsis

Cordon a tenant cluster.

This command allows to cordon a tenant cluster, preventing it from receiving new resources.
Resources provided by existing ResourceSlices are left untouched, while new ResourceSlices
are denied.



```
liqoctl cordon tenant [flags]
```

### Examples


```bash
  $ liqoctl cordon tenant my-tenant-name
```





### Options
`--cluster` _string_:

>The name of the kubeconfig cluster to use

`--context` _string_:

>The name of the kubeconfig context to use

`--kubeconfig` _string_:

>Path to the kubeconfig file to use for CLI requests

`--timeout` _duration_:

>Timeout for cordon completion **(default 2m0s)**

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

