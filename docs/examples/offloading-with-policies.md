# Offloading with Policies

This tutorial aims to guide you through a tour to learn how to use the core Liqo features.
You will learn how to tune namespace offloading, and specify the target clusters through the [cluster selector](UsageOffloadingClusterSelector) concept.

More specifically, you will configure a scenario composed of a *single entry point cluster* leveraged for the deployment of the applications (i.e., the *Venice* cluster, located in *north* Italy) and two *worker clusters* characterized by different geographical regions (i.e., the *Florence* and *Naples* clusters, respectively located in *center* and *south* Italy).
Then, you will offload a given namespace (and the applications contained therein) to a subset of the worker clusters (i.e., only to the *Naples* cluster), while allowing pods to be also scheduled on the local cluster (i.e., the *Venice* one).

## Provision the playground

First, check that you are compliant with the [requirements](/examples/requirements.md).

Then, let's open a terminal on your machine and launch the following script, which creates the three above-mentioned clusters with KinD and installs Liqo on all of them.
Each cluster is made by a single combined control-plane + worker node.

{{ env.config.html_context.generate_clone_example('offloading-with-policies') }}

Export the kubeconfigs environment variables to use them in the rest of the tutorial:

```bash
export KUBECONFIG="$PWD/liqo_kubeconf_venice"
export KUBECONFIG_FLORENCE="$PWD/liqo_kubeconf_florence"
export KUBECONFIG_NAPLES="$PWD/liqo_kubeconf_naples"
```

```{admonition} Note
We suggest exporting the kubeconfig of the first cluster as default (i.e., `KUBECONFIG`), since it will be the entry point of the virtual cluster and you will mainly interact with it.
```

At this point, you should have three clusters with Liqo installed on them.
The setup script named them **venice**, **florence** and **naples**, and respectively configured the following cluster labels:

* *venice*: `topology.liqo.io/region=north`
* *florence*: `topology.liqo.io/region=center`
* *naples*: `topology.liqo.io/region=south`

You can check that the clusters are correctly labeled through:

```bash
liqoctl status
liqoctl --kubeconfig $KUBECONFIG_FLORENCE status
liqoctl --kubeconfig $KUBECONFIG_NAPLES status
```

These labels will be propagated to the virtual nodes corresponding to each cluster.
In this way, you can easily identify the clusters through their characterizing labels, and define the appropriate scheduling policies.

## Peer the clusters

Once Liqo is installed in your clusters, you can establish new *peerings*.
In this example, since the two API Servers are mutually reachable, you will use the [out-of-band peering approach](FeaturesPeeringOutOfBandControlPlane).

To implement the desired scenario, let's first retrieve the *peer command* from the *Florence* and *Naples* clusters:

```bash
PEER_FLORENCE=$(liqoctl generate peer-command --only-command --kubeconfig $KUBECONFIG_FLORENCE)
PEER_NAPLES=$(liqoctl generate peer-command --only-command --kubeconfig $KUBECONFIG_NAPLES)
```

Then, establish the peerings from the *Venice* cluster:

```bash
echo "$PEER_FLORENCE" | bash
echo "$PEER_NAPLES" | bash
```

When the above commands return successfully, you can check the peering status by running:

```bash
kubectl get foreignclusters
```

The output should look like the following, indicating that an outgoing peering is currently active towards both the *Florence* and the *Naples* clusters, as well as the cross-cluster network tunnels have been established:

```text
NAME       TYPE        OUTGOING PEERING   INCOMING PEERING   NETWORKING    AUTHENTICATION   AGE
florence   OutOfBand   Established        None               Established   Established      111s
naples     OutOfBand   Established        None               Established   Established      98s
```

Additionally, you should have two new virtual nodes in the *Venice* cluster, characterized by the install-time provided labels:

```bash
kubectl get node --selector=liqo.io/type=virtual-node --show-labels
```

```text
NAME            STATUS   ROLES   AGE     VERSION   LABELS
liqo-florence   Ready    agent   2m30s   v1.23.6   liqo.io/remote-cluster-id=4858caa2-7848-4aab-a6b4-6984389bac56,liqo.io/type=virtual-node,topology.liqo.io/region=center
liqo-naples     Ready    agent   2m29s   v1.23.6   liqo.io/remote-cluster-id=53d11339-fc85-4741-a912-a3eda781a69e,liqo.io/type=virtual-node,topology.liqo.io/region=south
```

```{admonition} Note
Some of the default labels were omitted for the sake of clarity.
```

## Tune namespace offloading

Now, let's suppose you want to deploy an application that needs to be scheduled in the *north* and in the *south* region, but not in the *center* one.
This constraint needs to be respected at the infrastructural level: the dev team does not need to be aware of required affinities and/or node selectors, nor it should be able to bypass them.

First, you should create a new namespace in the *Venice* cluster, which will host the application:

```bash
kubectl create namespace liqo-demo
```

Then, enable Liqo offloading for that namespace:

```bash
liqoctl offload namespace liqo-demo \
  --namespace-mapping-strategy EnforceSameName \
  --pod-offloading-strategy LocalAndRemote \
  --selector 'topology.liqo.io/region=south'
```

The above command configures the following aspects (see the dedicated [usage page](/usage/namespace-offloading.md) for additional information concerning namespace offloading configurations):

* the `liqo-demo` namespace is replicated with the same name in the other clusters.
* the `liqo-demo` namespace, and the contained resources, are offloaded only to the clusters with the `topology.liqo.io/region=south` label.
* the pods living in the `liqo-demo` namespace are free to be scheduled onto both physical and virtual nodes.

The *NamespaceOffloading* resource created by *liqoctl* in the `liqo-demo` namespace exposes the status of the offloading process, including a global *OffloadingPhase*, which is expected to be `Ready`, and a list of *RemoteNamespaceConditions*, one for each remote cluster.

In this case:

* the *Florence* cluster has not been selected to offload the namespace `liqo-demo`, since it does not match the cluster selector;
* the *Naples* cluster has been selected to offload the namespace `liqo-demo`, and the namespace has been correctly created.

```bash
kubectl get namespaceoffloadings offloading -n liqo-demo -o yaml
```

```yaml
...
status:
  offloadingPhase: Ready
  remoteNamespaceName: liqo-demo
  remoteNamespacesConditions:
    florence-539f8b:
    - lastTransitionTime: "2022-05-05T15:08:33Z"
      message: You have not selected this cluster through ClusterSelector fields
      reason: ClusterNotSelected
      status: "False"
      type: OffloadingRequired
    naples-7881be:
    - lastTransitionTime: "2022-05-05T15:08:43Z"
      message: The remote cluster has been selected through the ClusterSelector field
      reason: ClusterSelected
      status: "True"
      type: OffloadingRequired
    - lastTransitionTime: "2022-05-05T15:08:43Z"
      message: Namespace correctly offloaded on this cluster
      reason: RemoteNamespaceCreated
      status: "True"
      type: Ready
```

Indeed, if you query for the namespaces in the *Naples* cluster, you should see the following output, confirming that the remote namespace has been correctly created by Liqo:

```bash
kubectl get namespaces liqo-demo --kubeconfig="$KUBECONFIG_NAPLES"
```

```text
NAME        STATUS   AGE
liqo-demo   Active   70s
```

Instead, the same command executed in the *Florence* cluster should return an error, as the namespace has not been replicated:

```bash
kubectl get namespaces liqo-demo --kubeconfig="$KUBECONFIG_FLORENCE"
```

```text
Error from server (NotFound): namespaces "liqo-demo" not found
```

## Deploy applications

All constraints specified during namespace offloading are automatically enforced by Liqo, and merged with other pod-level specifications.

To verify this, you can now create two deployments in the `liqo-demo` namespace, characterized by additional *NodeAffinity* constraints.
More precisely, one (`app-south`) is forced to be scheduled onto the virtual node representing the *Naples* cluster, while the other (`app-center`) is forced onto the *Florence* virtual cluster (which is incompatible with the namespace-level constraints).

```bash
kubectl apply -f ./manifests/deploy.yaml -n liqo-demo
```

Checking the pod status, it is possible to verify that one has been scheduled onto the *Naples* cluster, and it is correctly running, while the other remained *Pending* due to conflicting requirements (i.e., no node is available to satisfy all its constraints).

```bash
kubectl get pod -n liqo-demo -o wide
```

```text
NAME                          READY   STATUS    RESTARTS   AGE     IP            NODE          NOMINATED NODE   READINESS GATES
app-center-6f6bd7547b-dh4m4   0/1     Pending   0          2m30s   <none>        <none>        <none>           <none>
app-south-85b9b99c4d-stwhw    1/1     Running   0          2m30s   10.204.0.12   liqo-naples   <none>           <none>
```

```{admonition} Note
You can remove the conflicting node affinity from the `app-center` deployment, and check that the generated pod gets scheduled onto either the *Venice* (i.e., locally) or the *Naples* cluster, as constrained by the namespace offloading configuration.
```

## Tear down the playground

Our example is finished; now we can remove all the created resources and tear down the playground.

### Unoffload namespaces

Before starting the uninstallation process, make sure that all namespaces are unoffloaded:

```bash
liqoctl unoffload namespace liqo-demo
```

Every pod that was offloaded to a remote cluster is going to be rescheduled onto the local cluster.

### Revoke peerings

Similarly, make sure that all the peerings are revoked:

```bash
liqoctl unpeer out-of-band florence
liqoctl unpeer out-of-band naples
```

At the end of the process, the virtual nodes are removed from the local cluster.

### Uninstall Liqo

Now you can uninstall Liqo from your clusters with *liqoctl*:

```bash
liqoctl uninstall
liqoctl uninstall --kubeconfig="$KUBECONFIG_FLORENCE"
liqoctl uninstall --kubeconfig="$KUBECONFIG_NAPLES"
```

```{admonition} Purge
By default the Liqo CRDs will remain in the cluster, but they can be removed with the `--purge` flag:

```bash
liqoctl uninstall --purge
liqoctl uninstall --kubeconfig="$KUBECONFIG_FLORENCE" --purge
liqoctl uninstall --kubeconfig="$KUBECONFIG_NAPLES" --purge
```

### Destroy clusters

To teardown the KinD clusters, you can issue:

```bash
kind delete cluster --name venice
kind delete cluster --name florence
kind delete cluster --name naples
```
