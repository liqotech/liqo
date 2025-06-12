# liqoctl offload

Offload a resource to remote clusters

## Description

### Synopsis

Offload a resource to remote clusters.


## liqoctl offload namespace

Offload namespaces to remote clusters

### Synopsis

Offload one or more namespaces to remote clusters.

Once a given namespace is selected for offloading, Liqo extends it across the
cluster boundaries, through the the automatic creation of twin namespaces in the
subset of selected remote clusters. Remote namespaces host the actual pods
offloaded in the corresponding cluster, as well as the additional resources
(i.e., Services, EndpointSlices, Ingresses, ConfigMaps, Secrets, PVCs and PVs)
propagated by the resource reflection process.

Namespace offloading can be tuned in terms of:
* Clusters: select the target clusters through virtual node labels.
* Pod offloading: whether pods should be scheduled on physical nodes only,
  virtual nodes only, or both. Forcing all pods to be scheduled locally enables
  the consumption of services from remote clusters.
* Naming: whether remote namespaces have the same name or a suffix is added to
  prevent conflicts.

Besides the direct offloading of a namespace, this command also provides the
possibility to generate and output the underlying NamespaceOffloading
resource, that can later be applied through automation tools.



```
liqoctl offload namespace name [flags]
```

### Examples


```bash
  $ liqoctl offload namespace foo
```

or

```bash
  $ liqoctl offload namespace foo bar
```

or

```bash
  $ liqoctl offload namespace --ns-selector 'foo=bar'
```

or

```bash
  $ liqoctl offload namespace foo --pod-offloading-strategy Remote --namespace-mapping-strategy EnforceSameName
```

or (cluster labels in logical AND)

```bash
  $ liqoctl offload namespace foo --namespace-mapping-strategy EnforceSameName \
      --selector 'region in (europe,us-west), !staging'
```

or (cluster labels in logical OR)

```bash
  $ liqoctl offload namespace foo --namespace-mapping-strategy EnforceSameName \
      --selector 'region in (europe,us-west)' --selector '!staging'
```

or (output the NamespaceOffloading resource as a yaml manifest, without applying it)

```bash
  $ liqoctl offload namespace foo --output yaml
```





### Options
`--namespace-mapping-strategy` _string_:

>The naming strategy adopted for the creation of remote namespaces, among DefaultName, EnforceSameName and SelectedName **(default "DefaultName")**

`--ns-selector` _string_:

>Selector (label query) to filter namespaces, supports '=', '==', and '!=' (e.g., -l key1=value1,key2=value2).

`-o`, `--output` _string_:

>Output the resulting NamespaceOffloading resource, instead of applying it. Supported formats: json, yaml

`--pod-offloading-strategy` _string_:

>The constraints regarding pods scheduling in this namespace, among Local, Remote and LocalAndRemote **(default "LocalAndRemote")**

`--remote-namespace-name` _string_:

>The name of the remote namespace, required when using the SelectedName NamespaceMappingStrategy. Otherwise, it is ignored

`-l`, `--selector` _stringArray_:

>The selector to filter the target clusters. Can be specified multiple times, defining alternative requirements (i.e., in logical OR)

`--timeout` _duration_:

>The timeout for the offloading process **(default 20s)**


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

