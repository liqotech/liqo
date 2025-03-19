# liqoctl version

Print the liqo CLI version and the deployed Liqo version

## Description

### Synopsis

Print the liqo CLI version and the deployed Liqo version.

The CLI version is embedded in the binary and directly displayed. The deployed
Liqo version version is determined based on the installed chart version.



```
liqoctl version [flags]
```

### Examples


```bash
  $ liqoctl version
```





### Options
`--client`

>Show client version only (no server required) (default false)

`-n`, `--namespace` _string_:

>The namespace where Liqo is installed in **(default "liqo")**


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

