# liqoctl uninstall

Uninstall Liqo from the selected cluster

## Description

### Synopsis

Uninstall Liqo from the selected cluster.

This command wraps the Helm command to uninstall Liqo from the selected cluster,
optionally removing all the associated CRDs (i.e., with the --purge flag).

```{warning}
 due to current limitations, the uninstallation process might hang in
case peerings are still established, or namespaces are selected for offloading.
It is necessary to unpeer all clusters and unoffload all namespaces in advance.
```


```
liqoctl uninstall [flags]
```

### Examples


```bash
  $ liqoctl uninstall
```

or

```bash
  $ liqoctl uninstall --purge
```





### Options
`-n`, `--namespace` _string_:

>The namespace where Liqo is installed in **(default "liqo")**

`--purge`

>Whether to purge all Liqo CRDs from the cluster (default false)

`--timeout` _duration_:

>The timeout for the completion of the uninstallation process **(default 10m0s)**


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

`--skip-confirm`

>Skip the confirmation prompt (suggested for automation)

`--user` _string_:

>The name of the kubeconfig user to use

`-v`, `--verbose`

>Enable verbose logs (default false)

