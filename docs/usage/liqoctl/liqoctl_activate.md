# liqoctl activate

Activate a liqo resource

## Description

### Synopsis

Activate a liqo resource


## liqoctl activate tenant

Activate a tenant cluster

### Synopsis

Activate a tenant cluster.

This command allows to activate a tenant cluster, allowing it to receive new resources.
Resources provided by existing ResourceSlices are provided again.



```
liqoctl activate tenant [flags]
```

### Examples


```bash
  $ liqoctl activate tenant my-tenant-name
```





### Options
`--cluster` _string_:

>The name of the kubeconfig cluster to use

`--context` _string_:

>The name of the kubeconfig context to use

`--kubeconfig` _string_:

>Path to the kubeconfig file to use for CLI requests

`--timeout` _duration_:

>Timeout for activate completion **(default 2m0s)**

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

