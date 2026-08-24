# liqoctl test

Launch E2E tests

## Description

### Synopsis

Launch E2E tests


## liqoctl test network

Launch E2E tests for the network

### Synopsis

Launch network E2E tests.

```{warning}
 to run the tests you need to have kyverno installed on every cluster https://kyverno.io/docs/installation/methods/ .
```
This command allows to launch E2E tests, which are used to check the network functionalities between the clusters.
The command needs to be run on the cluster that will act as the consumer,
and it requires the kubeconfig of the remote cluster that will act as the providers.
The consumer cluster must be peered with the providers, previously using "liqoctl peer".




```
liqoctl test network [flags]
```

### Examples


```bash
  $ liqoctl test network --remote-kubeconfigs $HOME/.kube/config2,$HOME/.kube/config3
```

or

```bash
  $ liqoctl test network --remote-kubeconfigs $HOME/.kube/config2,$HOME/.kube/config3 --basic
```

or

```bash
  $ liqoctl test network --remote-kubeconfigs $HOME/.kube/config2,$HOME/.kube/config3 --np-nodes all --np-ext --pod-np
```

or

```bash
  $ liqoctl test network --remote-kubeconfigs $HOME/.kube/config2,$HOME/.kube/config3 --ip
```

or

```bash
  $ liqoctl test network --remote-kubeconfigs $HOME/.kube/config2,$HOME/.kube/config3 --lb
```





### Options
`--basic`

>Run only pod-to-pod checks

`--info`

>Print information about the network configurations of the clusters

`--ip`

>Enable IP remapping for the tests

`--lb`

>Enable curl from external to loadbalancer service

`--np-ext`

>Enable curl from external to nodeport service

`--np-nodes` _string_:

>Select nodes type for NodePort tests. Possible values: all, workers, control-planes **(default "all")**

`--pod-np`

>Enable curl from pod to nodeport service

`-p`, `--remote-kubeconfigs` _strings_:

>A list of kubeconfigs for remote provider clusters

`--rm`

>Remove namespace after the test


### Global options

`--cluster` _string_:

>The name of the kubeconfig cluster to use

`--context` _string_:

>The name of the kubeconfig context to use

`--fail-fast`

>Stop the test as soon as an error is encountered

`--global-annotations` _stringToString_:

>Global annotations to be added to all created resources (key=value)

`--global-labels` _stringToString_:

>Global labels to be added to all created resources (key=value)

`--kubeconfig` _string_:

>Path to the kubeconfig file to use for CLI requests

`--skip-confirm`

>Skip the confirmation prompt (suggested for automation)

`--timeout` _duration_:

>Timeout for the test **(default 5m0s)**

`--user` _string_:

>The name of the kubeconfig user to use

`-v`, `--verbose`

>Verbose output

