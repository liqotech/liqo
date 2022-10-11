# Peer two Clusters

This section describes the procedure to **establish a peering** with a remote cluster, using one of the two alternative approaches featured by Liqo.
You can refer to the [dedicated features section](FeaturesPeeringApproaches) for a high-level presentation of their characteristics, and the associated trade-offs.

```{warning}
The establishment of a peering with a remote cluster leveraging a **different version of Liqo**, net of patch releases, is currently **not supported**, and could lead to unexpected results.
```

## Overview

The peering process leverages **[liqoctl](/installation/liqoctl.md)** to interact with the clusters, abstracting the creation and update of the appropriate custom resources.
To this end, the most important one is the ***ForeignCluster*** resource, which **represents a remote cluster**, including its identity, the associated authentication endpoint, and the desired peering state (i.e., whether it should be established, and in which directions).
Additionally, its status reports a **summary of the current peering status**, detailing whether the different phases (e.g., authentication, network establishment, resource negotiation, ...) correctly succeeded.

The following sections present the respective procedures to **peer a local cluster A** (i.e., the *consumer*), with a **remote cluster B** (i.e., the *provider*).
At the end of the process, a new **virtual node** is created in the consumer, abstracting the resources shared by the provider, and enabling seamless **pod offloading** to the remote cluster.
Additional details are also provided to enable the reverse peering direction, hence achieving a **bidirectional peering**, allowing both clusters to offload a part of their workloads to the other.

By default, Liqo shares a configurable percentage of the currently available resources of the **provider** cluster with **consumers**.
You can change this behavior by using a custom [resource plugin](https://github.com/liqotech/liqo-resource-plugins).

All examples leverage two different *contexts* to refer to *consumer* and *provider* clusters, respectively named `consumer` and `provider`.

```{admonition} Note
*liqoctl* displays a *kubectl* compatible behavior concerning Kubernetes API access, hence supporting the `KUBECONFIG` environment variable, as well as all the standard flags, including `--kubeconfig` and `--context`.
Ensure you selected the correct target cluster before issuing *liqoctl* commands (as you would do with *kubectl*).
```

## Out-of-band control plane

Briefly, the procedure to establish an [out-of-band control plane peering](FeaturesPeeringOutOfBandControlPlane) consists of a first step performed on the *provider*, to **retrieve the set of information** required (i.e., authentication endpoint and token, cluster ID, ...), followed by the creation, on the *consumer*, of the necessary resources to **start the actual peering**.
The remainder of the process, including identity retrieval, resource negotiation and network tunnel establishment is **performed automatically** by Liqo, through a mutual exchange of information and negotiation between the two clusters involved.

### Information retrieval

To proceed, ensure that you are operating in the *provider* cluster, and then issue the *liqoctl generate peer-command* command:

```bash
liqoctl --context=provider generate peer-command
```

This retrieves the information concerning the *provider* cluster (i.e., authentication endpoint and token, cluster ID, ...) and generates a command that can be executed on a *different* cluster (i.e., the *consumer*) to establish an out-of-band outgoing peering towards the *provider* cluster.

An example of the resulting command is the following:

```bash
liqoctl peer out-of-band <cluster-name> --auth-url <auth-url> \
    --cluster-id <cluster-id> --auth-token <auth-token>
```

### Peering establishment

Once obtained the peering command, it is possible to execute it in the *consumer* cluster, to kick off the peering process.

```{warning}
Pay attention to operate in the correct cluster, possibly adding the appropriate flags to the generated command (e.g., `--context=consumer`).
```

```bash
liqoctl --context=consumer peer out-of-band <cluster-name> --auth-url <auth-url> \
    --cluster-id <cluster-id> --auth-token <auth-token>
```

The above command configures the appropriate authentication token, and then creates a new *ForeignCluster* resource in the *consumer cluster*.
Finally, it waits for the different peering phases to complete (this might require a few seconds, depending on the download time of the Liqo virtual kubelet image).

The *ForeignCluster* resource can be inspected through *kubectl*:

```bash
kubectl --context=consumer get foreignclusters
```

If the peering process completed successfully, you should observe an output similar to the following, indicating that the cross-cluster network tunnel has been established, and an outgoing peering is currently active (i.e., the *consumer* cluster can offload workloads to the *provider* one, but not vice versa):

```text
NAME       TYPE        OUTGOING PEERING   INCOMING PEERING   NETWORKING    AUTHENTICATION
provider   OutOfBand   Established        None               Established   Established
```

At the same time, a new *virtual node* should have been created in the *consumer* cluster.
Specifically:

```bash
kubectl --context=consumer get nodes -l liqo.io/type=virtual-node
```

Should return an output similar to the following:

```text
NAME            STATUS   ROLES   AGE    VERSION
liqo-provider   Ready    agent   179m   v1.23.4
```

In addition, you can check the peering status, and retrieve more advanced information, using:

```bash
liqoctl status peer provider
```

```{admonition} Note
The name of the *ForeignCluster* resource, as well as that of the *virtual node*, reflects the cluster name specified with the *liqoctl peer out-of-band* command.
```

### Bidirectional peering

Once the peering from the *consumer* to the *provider* has been established, the reverse direction (i.e., leading to a bidirectional peering) can be enabled through a simpler command, since the *ForeignCluster* resource is already present:

```bash
liqoctl --context=provider peer consumer
```

### Tear down

An out-of-band peering can be disabled leveraging the symmetric *liqoctl unpeer* command, causing the local virtual node (abstracting the remote cluster) to be destroyed, and all offloaded workloads to be rescheduled:

```bash
liqoctl --context=consumer unpeer out-of-band
```

```{admonition} Note
The reverse peering direction, if any, is preserved, and the remote cluster can continue offloading workloads to its virtual
node representing the local cluster.
In this case, the command *emits a warning*, and it does not proceed deleting the *ForeignCluster* resource.
Hence, the same command shall be executed on both clusters to completely tear down a bidirectional peering.
```

In case only one peering direction shall be teared down, while preserving the opposite, it is suggested to leverage the appropriate *liqoctl unpeer* command to disable the outgoing peering (e.g., on the *provider* cluster):

```bash
liqoctl --context=provider unpeer consumer
```

(UsagePeerInBand)=

## In-band control plane

Briefly, the procedure to establish an [in-band control plane peering](FeaturesPeeringInBandControlPlane) consists of a first step performed by *liqoctl*, which interacts alternatively with both clusters to **establish the cross-cluster VPN tunnel**, exchange the **authentication tokens** and configure the Liqo control plane traffic to flow inside the VPN.
The remainder of the process, including identity retrieval and resource negotiation, is **performed automatically** by Liqo, through a mutual exchange of information and negotiation between the two clusters involved.

```{admonition} Note
The host used to issue the *liqoctl peer in-band* command must have **concurrent access to both clusters** (i.e., *consumer* and *provider*) while carrying out the in-band control plane peering process.
To this end, these subcommands feature a parallel set of flags concerning Kubernetes API access to the remote cluster, in the form `--remote-<flag>` (e.g., `--remote-kubeconfig`, `--remote-context`).
```

<!-- markdownlint-disable-next-line no-duplicate-heading -->
### Peering establishment

The in-band control plane peering process can be started leveraging a single *liqoctl* command:

```bash
liqoctl peer in-band --context=consumer --remote-context=provider
```

The above command outputs a set of information concerning the different operations performed on the two clusters.
Notably, it exchanges the appropriate authentication tokens, establishes the cross-cluster VPN tunnel, and then creates a new *ForeignCluster* resource in *both clusters*.
Finally, it waits for the different peering phases to complete (this might require a few seconds, depending on the download time of the Liqo virtual kubelet image).

The *ForeignCluster* resource can be inspected through *kubectl* (e.g., on the *consumer*):

```bash
kubectl --context=consumer get foreignclusters
```

If the peering process completed successfully, you should observe an output similar to the following, indicating that the cross-cluster network tunnel has been established, and an outgoing peering is currently active (i.e., the *consumer* cluster can offload workloads to the *provider* one, but not vice versa):

```text
NAME       TYPE     OUTGOING PEERING   INCOMING PEERING   NETWORKING    AUTHENTICATION
provider   InBand   Established        None               Established   Established
```

At the same time, a new *virtual node* should have been created in the *consumer* cluster.
Specifically:

```bash
kubectl --context=consumer get nodes -l liqo.io/type=virtual-node
```

Should return an output similar to the following:

```text
NAME             STATUS   ROLES   AGE    VERSION
liqo-provider    Ready    agent   179m   v1.23.4
```

In addition, you can check the peering status, and retrieve more advanced information, using:

```bash
liqoctl status peer provider
```

```{admonition} Note
The name of the *ForeignCluster* resource, as well as that of the *virtual node*, reflects the cluster name specified by the remote cluster administrators at install time.
```

<!-- markdownlint-disable-next-line no-duplicate-heading -->
### Bidirectional peering

A bidirectional in-band peering can be established adding the `--bidirectional` flag to the *liqoctl peer* command invocation:

```bash
liqoctl peer in-band --context=consumer --remote-context=provider --bidirectional
```

```{admonition} Note
The *liqoctl peer in-band* command is idempotent, and can be re-executed without side effects to enable a bidirectional peering.
```

Alternatively, the reverse peering can be also activated executing the following on the *provider* cluster:

```bash
liqoctl --context=provider peer consumer
```

<!-- markdownlint-disable-next-line no-duplicate-heading -->
### Tear down

An in-band peering can be disabled leveraging the symmetric *liqoctl unpeer* command, causing both virtual nodes (if present) to be destroyed, all offloaded workloads
to be rescheduled, and finally tearing down the cross-cluster VPN tunnel:

```bash
liqoctl unpeer in-band --context=consumer --remote-context=provider
```

In case only one peering direction shall be teared down, while preserving the opposite, it is possible to leverage the appropriate *liqoctl unpeer* command to disable the outgoing peering (e.g., on the *provider* cluster):

```bash
liqoctl --context=provider unpeer consumer
```
