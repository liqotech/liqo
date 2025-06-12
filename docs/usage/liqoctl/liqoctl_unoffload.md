# liqoctl unoffload

Unoffload a resource from remote clusters

## Description

### Synopsis

Unoffload a resource from remote clusters.


## liqoctl unoffload namespace

Unoffload namespaces from remote clusters

### Synopsis

Unoffload one or more namespaces from remote clusters.

This command disables the offloading of one or more namespaces, deleting all resources
reflected to remote clusters (including the namespaces themselves), and causing
all offloaded pods to be rescheduled locally.



```
liqoctl unoffload namespace name [flags]
```

### Examples


```bash
  $ liqoctl unoffload namespace foo
```

or

```bash
  $ liqoctl unoffload namespace foo bar
```

or

```bash
  $ liqoctl unoffload namespace --ns-selector 'foo=bar'
```





### Options
`--ns-selector` _string_:

>Selector (label query) to filter namespaces, supports '=', '==', and '!=' (e.g., -l key1=value1,key2=value2).

`--timeout` _duration_:

>Timeout for the unoffload operation **(default 2m0s)**


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

