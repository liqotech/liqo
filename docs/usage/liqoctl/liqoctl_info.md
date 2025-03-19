# liqoctl info

Show info about the current Liqo instance

## Description

### Synopsis

Show info about the current Liqo instance.

Liqoctl provides a set of commands to verify the status of the Liqo control
plane, its configuration, as well as the characteristics of the currently
active peerings, and reports the outcome in human-readable or
machine-readable format (either JSON or YAML).
Additionally, via '--get', it allows to retrieve each single field of the reports
using a query in dot notation (e.g. '--get field.subfield')

This command shows information about the local cluster and checks the presence
and the sanity of the Liqo namespace and the Liqo pods and some brief info about
the active peerings and their status.



```
liqoctl info [flags]
```

### Examples


```bash
  $ liqoctl info
  $ liqoctl info --namespace liqo-system
```

show the output in YAML format

```bash
  $ liqoctl info -o yaml
```

get a specific field

```bash
  $ liqoctl info --get clusterid
  $ liqoctl info --get network.podcidr
```





### Options
`-g`, `--get` _string_:

>Path to the desired subfield in dot notation. Each part of the path corresponds to a key of the output structure

`-n`, `--namespace` _string_:

>The namespace where Liqo is installed in **(default "liqo")**

`-o`, `--output` _string_:

>Output format. Supported formats: json, yaml

`-v`, `--verbose`

>Make info more verbose


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

## liqoctl info peer

Show additional info about peered clusters

### Synopsis

Show additional info about peered clusters.

Liqoctl provides a set of commands to verify the status of the Liqo control
plane, its configuration, as well as the characteristics of the currently
active peerings, and reports the outcome in human-readable or
machine-readable format (either JSON or YAML).
Additionally, via '--get', it allows to retrieve each single field of the reports
using a query in dot notation (e.g. '--get field.subfield')

This command shows additional information about the peered clusters, the status
of the modules and the amount of shared resources.



```
liqoctl info peer [flags]
```

### Examples


```bash
  $ liqoctl info peer
```

or

```bash
  $ liqoctl info peer cluster1
```

or

```bash
  $ liqoctl info peer cluster1 cluster2
```

or

```bash
  $ liqoctl info peer cluster1 cluster2 --namespace liqo-system
```

show the output in YAML format

```bash
  $ liqoctl info peer -o yaml
```

get a specific field

```bash
  $ liqoctl info peer cluster1 cluster2 --get cluster2.network.cidr
```

when a single cluster is specified, the cluster ID at the beginning of the query can be omitted

```bash
  $ liqoctl info peer cluster1 --get network.cidr
```





### Options

### Global options

`--cluster` _string_:

>The name of the kubeconfig cluster to use

`--context` _string_:

>The name of the kubeconfig context to use

`-g`, `--get` _string_:

>Path to the desired subfield in dot notation. Each part of the path corresponds to a key of the output structure

`--global-annotations` _stringToString_:

>Global annotations to be added to all created resources (key=value)

`--global-labels` _stringToString_:

>Global labels to be added to all created resources (key=value)

`--kubeconfig` _string_:

>Path to the kubeconfig file to use for CLI requests

`-n`, `--namespace` _string_:

>The namespace where Liqo is installed in **(default "liqo")**

`-o`, `--output` _string_:

>Output format. Supported formats: json, yaml

`--skip-confirm`

>Skip the confirmation prompt (suggested for automation)

`--user` _string_:

>The name of the kubeconfig user to use

`-v`, `--verbose`

>Make info more verbose

