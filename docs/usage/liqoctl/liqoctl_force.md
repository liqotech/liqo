# liqoctl force

Force actions on Liqo

## Description

### Synopsis

Force actions on Liqo components and resources.

The force command allows you to override normal Liqo operations and execute
actions that might otherwise be blocked or require manual intervention.
This command provides mechanisms to forcefully manipulate Liqo resources
when standard operations are not sufficient or when immediate action is required.

Use with caution as force operations may bypass safety checks and could
potentially impact cluster stability or data consistency.



## liqoctl force unpeer

Force unpeer a cluster

### Synopsis

Force unpeer from a remote cluster.

This command forcefully terminates the peering relationship with a remote cluster,
bypassing normal unpeer procedures and safety checks. It is designed to handle
situations where the standard unpeer process fails or when the remote cluster
is unreachable or unresponsive.

The force unpeer operation will:
- Mark the ForeignCluster as permanently unreachable
- Clean up local resources associated with the peering
- Remove tenant namespaces

Use with caution as this operation cannot be undone and may leave resources
in an inconsistent state if the remote cluster is still accessible.



```
liqoctl force unpeer [flags]
```

### Examples


```bash
  $ liqoctl force unpeer <cluster-id>
```





### Options

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

