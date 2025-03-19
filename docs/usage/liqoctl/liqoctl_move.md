# liqoctl move

Move an object to a different cluster

## Description

### Synopsis

Move an object to a different cluster.


## liqoctl move volume

Move a Liqo-managed PVC to a different node (i.e., cluster)

### Synopsis

Move a Liqo-managed PVC to a different node (i.e., cluster).

Liqo supports the offloading of *stateful workloads* through a storage fabric
leveraging a custom storage class. PVCs associated with the Liqo storage class
eventually trigger the creation of the corresponding PV in the cluster (either
local or remote) where its first consumer (i.e., pod) is scheduled on. Locality
constraints are automatically embedded in the resulting PV, to enforce each pod
to be scheduled on the cluster where the associated storage pools are available.

This command allows to *move* a volume created in a given cluster to a different
cluster, ensuring mounting pods will then be attracted in that location. This
process leverages Restic to backup the source data and restore it into a volume
in the target cluster.

```{warning}
 only PVCs not currently mounted by any pod can
be moved to a different cluster.
```


```
liqoctl move volume [flags]
```

### Examples


```bash
  $ liqoctl move volume database01 --namespace foo --target-node worker-023
```

or

```bash
  $ liqoctl move volume database01 --namespace foo --target-node liqo-neutral-colt
      --containers-cpu-limits 1000m --containers-ram-limits 2Gi
```





### Options
`--containers-cpu-limits` _quantity_:

>The CPU limits for the Restic containers

`--containers-cpu-requests` _quantity_:

>The CPU requests for the Restic containers

`--containers-ram-limits` _quantity_:

>The RAM limits for the Restic containers

`--containers-ram-requests` _quantity_:

>The RAM requests for the Restic containers

`-n`, `--namespace` _string_:

>The namespace scope for this request

`--restic-image` _string_:

>The Restic image to use **(default "restic/restic:0.14.0")**

`--restic-server-image` _string_:

>The Restic server image to use **(default "restic/rest-server:0.11.0")**

`--target-node` _string_:

>The target node (either physical or virtual) the PVC will be moved to


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

