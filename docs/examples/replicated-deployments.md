# Multi-cluster deployments

In this tutorial you will learn how to deploy an application, and use Liqo to replicate it on multiple clusters.

![Offloading Multiple Pods Overview](/_static/images/examples/replicated-deployments/replicated-deployments.drawio.svg)

In this example you will configure a scenario composed of a *single entry point cluster* used for the deployment of the applications (called *origin cluster*) and two *destination clusters*.
The deployed application will be replicated on all *destination clusters* in order to deploy exactly **one** identical application on each destination cluster.

## Provision the playground

First, make sure that the [requirements](/examples/requirements.md) for Liqo are satisfied.

Then, let's open a terminal on your machine and launch the following script, which creates the three above-mentioned clusters with KinD and installs Liqo on all of them.

{{ env.config.html_context.generate_clone_example('replicated-deployments') }}

Export the kubeconfigs environment variables to use them in the rest of the tutorial:

```bash
export KUBECONFIG=liqo_kubeconf_europe-cloud
export KUBECONFIG_EUROPE_ROME_EDGE=liqo_kubeconf_europe-rome-edge
export KUBECONFIG_EUROPE_MILAN_EDGE=liqo_kubeconf_europe-milan-edge
```

```{admonition} Note
We suggest exporting the kubeconfig of the *origin* cluster as default (i.e., `KUBECONFIG`), since you will mainly interact with it.
```

Now you should have three clusters with Liqo running.
The setup script named them **europe-cloud**, **europe-rome-edge** and **europe-milan-edge**, and respectively configured the following cluster labels:

* *origin*: `topology.liqo.io/type=origin`
* *europe-rome-edge*: `topology.liqo.io/type=destination`
* *europe-milan-edge*: `topology.liqo.io/type=destination`

## Peer the clusters

Now, you can establish new Liqo *peerings* from *origin* to *destination* clusters:

```bash
liqoctl peer --remote-kubeconfig "$KUBECONFIG_EUROPE_ROME_EDGE" --gw-server-service-type NodePort
liqoctl peer --remote-kubeconfig "$KUBECONFIG_EUROPE_MILAN_EDGE" --gw-server-service-type NodePort
```

When the above commands return successfully, you can check the peering status by running:

```bash
kubectl get foreignclusters
```

The output should look like the following, indicating that an outgoing peering is currently active towards both the *europe-rome-edge* and the *europe-milan-edge* clusters, and that the cross-cluster network tunnels have been established:

```text
NAME                ROLE       AGE
europe-milan-edge   Provider   27s
europe-rome-edge    Provider   55s
```

Additionally, you should have two new virtual nodes in the *origin* cluster, characterized by the labels set at install-time:

```bash
kubectl get node --selector=liqo.io/type=virtual-node --show-labels
```

```text
NAME                STATUS   ROLES   AGE    VERSION   LABELS
europe-milan-edge   Ready    agent   100s   v1.30.0   [...] liqo.io/provider=kind,liqo.io/remote-cluster-id=europe-milan-edge,liqo.io/type=virtual-node,node.kubernetes.io/exclude-from-external-load-balancers=true,storage.liqo.io/available=true,topology.liqo.io/type=destination
europe-rome-edge    Ready    agent   2m     v1.30.0   [...] liqo.io/provider=kind,liqo.io/remote-cluster-id=europe-rome-edge,liqo.io/type=virtual-node,node.kubernetes.io/exclude-from-external-load-balancers=true,storage.liqo.io/available=true,topology.liqo.io/type=destination
```

```{admonition} Note
Some of the default labels were omitted for the sake of clarity.
```

## Tune namespace offloading

Now, let's pretend you want to deploy an application that needs to be scheduled on all *destination* clusters, but not in the *origin* one.
First, we create a new namespace, then enable Liqo offloading to it:

```bash
kubectl create namespace liqo-demo
```

Then, enable Liqo offloading for that namespace:

```bash
liqoctl offload namespace liqo-demo \
  --namespace-mapping-strategy EnforceSameName \
  --pod-offloading-strategy Remote \
  --selector 'topology.liqo.io/type=destination'
```

The above command configures Liqo with the following behaviour (see the dedicated [usage page](/usage/namespace-offloading.md) for additional information concerning namespace offloading configurations):

* The `liqo-demo` namespace, and the contained resources, are offloaded only to the clusters with the `topology.liqo.io/type=destination` label.
* The pods living in the `liqo-demo` namespace are scheduled only on virtual nodes.
* With the `EnforceSameName` mapping strategy, we instruct Liqo to create the offloaded namespace in the remote cluster with the same name as the local one. This is not required, but it has been done for the sake of clarity in this example.

```{admonition} Selectors
This example uses **selectors**, but they are not strictly necessary here, as all *peered* clusters have been targeted as *destination*.
**Selectors** become necessary in case you want to target a subset of *peered* clusters.
More information are available in the [offloading with policies](/examples/offloading-with-policies.md) example.
```

You can now query for the namespaces either in the *europe-rome-edge* or *europe-milan-edge* cluster to see if the remote namespace has been correctly created by Liqo:

```bash
kubectl get namespaces liqo-demo --kubeconfig="$KUBECONFIG_EUROPE_ROME_EDGE"
kubectl get namespaces liqo-demo --kubeconfig="$KUBECONFIG_EUROPE_MILAN_EDGE"
```

If everything is correct, both commands should return an output similar to the following:

```text
NAME        STATUS   AGE
liqo-demo   Active   70s
```

## Deploy applications

Now it is time to deploy the application.

In order to create a replica of the application in each *destination* cluster, you need to enforce the following conditions:

* The *deployment* resource must produce at least one *pod* for each *destination* cluster.
* Each *destination* cluster must schedule at most one *pod* on its nodes.

To obtain this result you can leverage the following features available in *kubernetes*:

* Set a number of replicas in the *deployment* which is equal to the number of *destination* clusters
* Set [**topologySpreadConstraints**](https://kubernetes.io/docs/concepts/scheduling-eviction/topology-spread-constraints/) inside the *deployment*'s template, which sets the [**maxSkew**](https://kubernetes.io/docs/concepts/scheduling-eviction/topology-spread-constraints/#spread-constraint-definition) equal to **1**.

The file `./manifests/deploy.yaml` contains an example of a *deployment* which satisfies these conditions.
Let's deploy it:

```bash
kubectl apply -f ./manifests/deploy.yaml -n liqo-demo
```

```{admonition} More replicas
If the deployment uses a number of replicas which is higher than the number of *virtual nodes*, the pods will be scheduled respecting the [**maxSkew**](https://kubernetes.io/docs/concepts/scheduling-eviction/topology-spread-constraints/#spread-constraint-definition) value, which guarantees that the difference between the maximum number of pods (scheduled on a single node) and the minimum will be **1**.
```

We can check the pod status and verify that each *destination* cluster has scheduled one *pod* on its nodes, i.e., one pod has been scheduled onto the *europe-rome-edge* cluster, and the other on *europe-milan-edge*, and they are both correctly running:

```bash
kubectl get pod -n liqo-demo -o wide
```

```text
NAME                            READY   STATUS    RESTARTS   AGE     IP            NODE                NOMINATED NODE   READINESS GATES
liqo-demo-app-777fb9fc8-bbt4d   1/1     Running   0          7m28s   10.113.0.65   liqo-europe-rome-edge   <none>           <none>
liqo-demo-app-777fb9fc8-wrjph   1/1     Running   0          7m28s   10.109.0.62   liqo-europe-milan-edge   <none>           <none>
```

## Tear down the playground

Our example is finished; now we can remove all the created resources and tear down the playground.

### Unoffload namespaces

Before starting the uninstallation process, make sure that all namespaces are unoffloaded:

```bash
liqoctl unoffload namespace liqo-demo
```

### Revoke peerings

Similarly, make sure that all the peerings are revoked:

```bash
liqoctl unpeer --remote-kubeconfig "$KUBECONFIG_EUROPE_ROME_EDGE"
liqoctl unpeer --remote-kubeconfig "$KUBECONFIG_EUROPE_MILAN_EDGE"
```

At the end of the process, the virtual nodes are removed from the local cluster.

### Uninstall Liqo

Now you can uninstall Liqo from your clusters:

```bash
liqoctl uninstall --skip-confirm
liqoctl uninstall --kubeconfig="$KUBECONFIG_EUROPE_ROME_EDGE" --skip-confirm
liqoctl uninstall --kubeconfig="$KUBECONFIG_EUROPE_MILAN_EDGE" --skip-confirm
```

```{admonition} Purge
By default the Liqo CRDs will remain in the cluster, but they can be removed with the `--purge` flag.
```

### Destroy clusters

To teardown the KinD clusters, you can issue:

```bash
kind delete cluster --name origin
kind delete cluster --name europe-rome-edge
kind delete cluster --name europe-milan-edge
```
