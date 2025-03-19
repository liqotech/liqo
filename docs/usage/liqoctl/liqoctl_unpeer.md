# liqoctl unpeer

Disable a peering towards a remote provider cluster

## Description

### Synopsis

Disable a peering towards a remote provider cluster.

Depending on the approach adopted to initially establish the peering towards a
remote cluster, the corresponding unpeer command performs the symmetrical
operations to tear the peering down.

This command disables a peering towards a remote provider cluster, causing
virtual nodes and associated resourceslices to be destroyed, and all
offloaded workloads to be rescheduled. The Identity and Tenant are respectively
removed from the consumer and provider clusters, and the networking between the
two clusters is destroyed.

The reverse peering, if any, is preserved, and the remote cluster can continue
offloading workloads to its virtual node representing the local cluster.



```
liqoctl unpeer [flags]
```

### Examples


```bash
  $ liqoctl unpeer --remote-kubeconfig <provider>
```





### Options
`--cluster` _string_:

>The name of the kubeconfig cluster to use

`--context` _string_:

>The name of the kubeconfig context to use

`--delete-namespaces`

>Delete the tenant namespace after unpeering

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

>Timeout for unpeering completion **(default 2m0s)**

`--user` _string_:

>The name of the kubeconfig user to use

`-v`, `--verbose`

>Enable verbose logs (default false)

`--wait`

>Wait for resource to be deleted before returning **(default true)**


### Global options

`--global-annotations` _stringToString_:

>Global annotations to be added to all created resources (key=value)

`--global-labels` _stringToString_:

>Global labels to be added to all created resources (key=value)

`--skip-confirm`

>Skip the confirmation prompt (suggested for automation)

