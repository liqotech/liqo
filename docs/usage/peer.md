# Peer two Clusters

This section describes the procedure to **establish a peering** with a remote provider cluster.

## Overview

The peering process leverages **[liqoctl](/installation/liqoctl.md)** to interact with the clusters, abstracting the creation and update of the appropriate custom resources.
To this end, it:

- creates the networking fabric to securely enable the communication between the two clusters,
- authenticates the consumer with the provider
- ask the provider for a certain amount of resource to schedule the workloads
- creates a VirtualNode on the consumer to schedule workloads and consume provider resources.

As a result, a new `ForeignCluster` resource is created, which **represents a remote cluster** and is univocally identified by its identity (i.e., cluster ID).
Additionally, its status reports a **summary of the current peering status**, including the role (i.e, *consumer* and/or *provider*), the associated authentication and networking endpoints, and the peering conditions for each module (i.e., whether the networking/authentication/offloading is established, and the status of its associated resources).

The following sections present the respective procedures to **peer a local cluster A** (i.e., the *consumer*), with a **remote cluster B** (i.e., the *provider*).
At the end of the process, a new **virtual node** is created in the consumer, abstracting the resources shared by the provider, and enabling seamless **pod offloading** to the remote cluster.
Additional details are also provided to enable the reverse peering direction, hence achieving a **bidirectional peering**, allowing both clusters to offload a part of their workloads to the other.

All examples leverage two different *contexts* to refer to *consumer* and *provider* clusters, respectively named `consumer` and `provider`.

```{admonition} Note
*liqoctl* displays a *kubectl* compatible behavior concerning Kubernetes API access, hence supporting the `KUBECONFIG` environment variable, as well as all the standard flags, including `--kubeconfig` and `--context`.
Ensure you selected the correct target cluster before issuing *liqoctl* commands (as you would do with *kubectl*).
```

## Requirements

The peering command requires the user to provide the kubeconfig of **both** *consumer* and *provider* clusters, as it will apply resources on both clusters.
To perform a peering without having access to both clusters, you need to manually apply on your cluster the resources and exchange with the remote cluster all the resources needed over out-of-band mediums (refer to the [individual guides](../advanced/manual-peering.md) describing the procedure for each module).

## Performed steps

The peering command enables all 3 liqo modules and performs the following steps:

1. **enables networking**.
Exchanges network configurations and creates the two **gateways** (one acting as _server_ and located in the provider cluster, another acting as _client_ in the consumer cluster) to let the two clusters communicate over a secure tunnel.  
The location of the client/server gateway can be customized when creating the peering using the `--gw-server-service-location` flag in `liqoctl`.  
2. **enables authentication**.
Authenticates the consumer with the provider.
In this step, the consumer obtains an `Identity` (*kubeconfig*) to replicate resources to the provider cluster.
3. **enables offloading**.
The consumer creates and replicates to the provider a `ResourceSlice`, to ask and obtain an `Identity` to consume a fixed amount of resources from the provider.
At the end of the process, the consumer obtains a **virtual node** to schedule workloads.

The command is intended to be a wrapper to simplify the peering process.
You can configure and fine-tune each module separately using the individual commands:

1. [`liqoctl network` (networking module)](/advanced/peering/inter-cluster-network.md)

2. [`liqoctl authenticate` (authentication module)](/advanced/peering/inter-cluster-authentication.md)

3. [`liqoctl create resourceslice` or `liqoctl create virtualnode` (offloading module)](/advanced/peering/offloading-in-depth.md)

For the majority and the cases the `liqoctl peer` is enough.
However, **to know the best strategy for each case and the requirements of each approach, check the [peering strategies guide](/advanced/peering-strategies.md)**.

### Establish a peering connection

To set up a peering with a *provider* cluster, `liqoctl` requires a `kubeconfig` with the necessary permissions to create a few resources on the provider cluster, which are used to establish the peering connection.
More details about the specific permissions required in this phase are presented in a dedicated [section](#get-a-kubeconfig-with-the-minimum-permissions-to-establish-a-peering) of the documentation, which shows how this `kubeconfig` limits actions on the other resources of the provider cluster.

To establish a peering connection between two clusters, ensure that you are operating in the *consumer* cluster, then issue the *liqoctl peer* command:

```bash
liqoctl peer \
  --kubeconfig=$CONSUMER_KUBECONFIG_PATH \
  --remote-kubeconfig $PROVIDER_KUBECONFIG_PATH
```

```{warning}
The establishment of a peering with a remote cluster leveraging a **different version of Liqo**, net of patch releases, is currently **not supported**, and could lead to unexpected results.
```

You should see the following output:

```text
 INFO   (local) Network configuration correctly retrieved
 INFO   (remote) Network configuration correctly retrieved
 INFO   (local) Network configuration correctly set up
 INFO   (remote) Network configuration correctly set up
 INFO   (local) Configuration applied successfully
 INFO   (remote) Configuration applied successfully
 INFO   (local) Network correctly initialized
 INFO   (remote) Network correctly initialized
 INFO   (remote) Gateway server correctly set up
 INFO   (remote) Gateway pod gw-cluster1 is ready
 INFO   (remote) Gateway server Service created successfully
 INFO   (local) Gateway client correctly set up
 INFO   (local) Gateway pod gw-cluster2 is ready
 INFO   (remote) Gateway server Secret created successfully
 INFO   (local) Public key correctly created
 INFO   (local) Gateway client Secret created successfully
 INFO   (remote) Public key correctly created
 INFO   (remote) Connection created successfully
 INFO   (local) Connection created successfully
 INFO   (local) Connection is established
 INFO   (remote) Connection is established
 INFO   (local) Tenant namespace correctly ensured
 INFO   (remote) Tenant namespace correctly ensured
 INFO   (remote) Nonce secret ensured
 INFO   (remote) Nonce generated successfully
 INFO   (remote) Nonce retrieved
 INFO   (local) Signed nonce secret ensured
 INFO   (local) Nonce is signed
 INFO   (local) Signed nonce retrieved
 INFO   (local) Tenant correctly generated
 INFO   (remote) Tenant correctly applied on provider cluster
 INFO   (remote) Tenant status is filled
 INFO   (remote) Identity correctly generated
 INFO   (local) Identity correctly applied on consumer cluster
 INFO   (local) Identity status is filled
 INFO   (local) ResourceSlice created
 INFO   (local) ResourceSlice authentication: Accepted
 INFO   (local) ResourceSlice resources: Accepted
```

(UsagePeeringInBand)=

### In-Band

If you can't make the Kubernetes API Server of the **provider** cluster reachable from the **consumer**, you can leverage on **in-band** peering.
You can enable this by setting the `--in-band` flag in the `liqoctl peer` command, which automatically configure all the features needed for this mechanism to work (i.e., the API server proxy and the IP remapping).

```{admonition} Note
For this feature to work, the Liqo **networking module** must be enabled.
```

## Results

The command configures the above-described modules.
It waits for the different modules to be set correctly (this might require a few seconds, depending on the download time of the required images).
At the end, a new *ForeignCluster* resource will appear on both clusters, containing the status of the peering.

The *ForeignCluster* resource can be inspected through *kubectl*:

```bash
kubectl --kubeconfig $CONSUMER_KUBECONFIG_PATH get foreignclusters
```

If the peering process is completed successfully, you should observe an output similar to the following.
In the consumer cluster:

```{code-block} text
:caption: "Cluster consumer"
NAME       ROLE       AGE
cl-provider   Provider   110s
```

In the provider cluster:

```{code-block} text
:caption: "Cluster provider"
NAME       ROLE       AGE
cl-consumer   Consumer   3m16s
```

At the same time, a new *virtual node* has been created in the *consumer* cluster.
Specifically:

```bash
kubectl --kubeconfig $CONSUMER_KUBECONFIG_PATH get nodes -l liqo.io/type=virtual-node
```

Should return an output similar to the following:

```text
NAME       STATUS   ROLES   AGE     VERSION
provider   Ready    agent   4m53s   v1.29.1
```

```{admonition} Note
The name of the `ForeignCluster` resources, as well as that of the *virtual node*, reflects the cluster IDs specified for the two clusters.
```

### Check status of peerings

Via `liqoctl` it is possible to check status and info about the active peerings:

- To get some **brief info** about the health of the active peerings:

  ```bash
  liqoctl info
  ```

- To get **detailed info** about peerings:

  ```bash
  liqoctl info peer
  ```

By specifing one of more cluster IDs, you can get status and info of one or more peers.
For example to get the status of the peerings with clusters `cl01` and `cl02`:

```bash
liqoctl info peer cl01 cl02
```

By default the output is presented in a human-readable form.
However, to simplify automate retrieval of the data, via the `-o` option it is possible to format the output in **JSON or YAML format**.
Moreover via the `--get field.subfield` argument, each field of the reports can be individually retrieved.

For example:

```{code-block} bash
:caption: Get a complete dump of peerings in JSON format
liqoctl info peer -o json
```

```{code-block} bash
:caption: Get the amount of resources shared with peer `cl01`
liqoctl info peer cl01 --get authentication.resourceslices
```

## Defining the amount of resources to share

By default, the `liqoctl peer` command requests the provider a default amount of resources, defined at installation time, which are:

- cpu: "4"
- memory: "8Gi"
- pods: "110"
- ephemeral-storage: "20Gi"

```{admonition} Tip
These defaults [can be changed](../installation/install.md#customization-options) by customizing the value of the `offloading.defaultNodeResources` option.
```

To define different values for a specific peering, you can provide the `--cpu`, `--memory`, and `--pods` arguments to the `liqoctl peer` command:

```bash
liqoctl peer \
  --kubeconfig=$CONSUMER_KUBECONFIG_PATH \
  --remote-kubeconfig=$PROVIDER_KUBECONFIG_PATH \
  --cpu=2 \
  --memory=2Gi
```

```{warning}
To make sure the consumer cluster does not exceed the quota of shared resources, the offloaded pods need to be created with the resources `limits` set.

Check the [resource enforcement section](../advanced/peering/offloading-in-depth.md#resource-enforcement) to know more about how to configure Liqo to enforce the quota of resources that the provider has granted to the consumer cluster.
```

### Defining custom resources

If you would ask for resources other than CPUs and memory, such as GPUs, you will need to create the peering connection without requesting resources, which can be done by providing the `--create-resource-slice=false` option:

```bash
liqoctl peer \
  --kubeconfig=$CONSUMER_KUBECONFIG_PATH \
  --remote-kubeconfig=$PROVIDER_KUBECONFIG_PATH \
  --create-resource-slice=false
```

The command above creates a peering connection, but **the provider cluster is currently not sharing resources**.

**To request resources** from a provider cluster, **you will need to create a `ResourceSlice`**, where you specify the amount of resources you would like the provider to share with the consumer, including your custom resources.

You can check [here to learn how to create a `ResourceSlice`](../advanced/peering/offloading-in-depth.md#create-resourceslice).

## Bidirectional peering

Once the peering from the *consumer* to the *provider* has been established, the reverse direction (i.e., leading to a bidirectional peering) can be enabled through the same procedure.

```bash
liqoctl --kubeconfig $PROVIDER_KUBECONFIG_PATH peer --remote-kubeconfig $CONSUMER_KUBECONFIG_PATH
```

## Tear down

A peering can be disabled by leveraging the symmetric `liqoctl unpeer` command, causing the local virtual node (abstracting the remote cluster) to be destroyed, and all offloaded workloads to be rescheduled:

```bash
liqoctl --kubeconfig $CONSUMER_KUBECONFIG_PATH unpeer --remote-kubeconfig $PROVIDER_KUBECONFIG_PATH
```

The reverse peering direction, if any, is preserved, and the remote cluster can continue offloading workloads to its virtual node representing the local cluster.
Hence, the specular command shall be executed on the opposite clusters to completely tear down a bidirectional peering.

```bash
liqoctl --kubeconfig $PROVIDER_KUBECONFIG_PATH unpeer --remote-kubeconfig $CONSUMER_KUBECONFIG_PATH
```

## Get a Kubeconfig with the minimum permissions to establish a peering

The setup of a peering with a *provider* cluster requires the creation of a `kubeconfig` tailored with the proper set of permissions to minimize security risks, namely, to allow the creation of the peering connection with the provider cluster, while preventing any other actions on the other resources on that cluster.

The [liqoctl](../installation/liqoctl.md) CLI tool provides utility functions to manage the permissions of users able to create only a peering connection with the current cluster.

```{danger}
liqoctl is designed to support administrators that needs to connect their clusters with Liqo.
Even though **the generated kubeconfig** has the minimum permissions to create a peering connection, it **must never be shared with third parties**, as it still grants some access to the provider cluster.
**In that cases, avoid using liqoctl and refer to the [advanced peering](../advanced/manual-peering.md)** guide for a secure setup.
```

**From the *provider* cluster**, you can run the following command to generate a *kubeconfig*:

```bash
liqoctl generate peering-user \
  --kubeconfig $PROVIDER_KUBECONFIG_PATH \
  --consumer-cluster-id $CONSUMER_CLUSTER_ID > $CONSUMER_KUBECONFIG_PATH
```

```{warning}
Remember that the generated *kubeconfig* is not saved anywhere by Liqo.
Hence, it would be your responsibility to keep it safely stored.
If you lose it, you will need to delete and recreate it.
```

This command will create a *kubeconfig* with **the minimum permissions to create and destroy a peering with the current cluster** from a cluster with ID `$CONSUMER_CLUSTER_ID`.
These permissions are limited to the _tenant namespace_ related to the peering with that specific cluster and some resources in the _Liqo_ namespace.

For a detailed list of the granted permissions:

- **tenant namespace**, refer to {{ env.config.html_context.generate_link_to_repo('this link', 'deployments/liqo/files/liqo-peering-user-tenant-ns-ClusterRole.yaml') }}
- **Liqo namespace**, refer to {{ env.config.html_context.generate_link_to_repo('this link', 'deployments/liqo/files/liqo-peering-user-liqo-ns-Role.yaml') }}

````{admonition} Note
**You are allowed to have a single peering user for each consumer cluster**, so you will not be able to create a new kubeconfig for the same consumer cluster unless you delete the previous one.

To delete a peering user for the consumer cluster with ID `$CONSUMER_CLUSTER_ID`, run:

```bash
liqoctl delete peering-user \
  --consumer-cluster-id $CONSUMER_CLUSTER_ID
```

**Once you delete a peering user, its kubeconfig will not be valid anymore, even though a new peering user for the same cluster is created.**
````
