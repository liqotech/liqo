# liqoctl drain

Drain a liqo resource

## Description

### Synopsis

Drain a liqo resource


## liqoctl drain tenant

Drain a tenant cluster

### Synopsis

Drain a tenant cluster.

This command allows to drain a tenant cluster, preventing it from receiving new resources.
Resources provided by existing ResourceSlices are drained.



```
liqoctl drain tenant [flags]
```

### Examples


```bash
  $ liqoctl drain tenant my-tenant-name
```





### Options
`--cluster` _string_:

>The name of the kubeconfig cluster to use

`--context` _string_:

>The name of the kubeconfig context to use

`--kubeconfig` _string_:

>Path to the kubeconfig file to use for CLI requests

`--timeout` _duration_:

>Timeout for drain completion **(default 2m0s)**

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

