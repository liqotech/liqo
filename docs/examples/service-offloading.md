# Offloading a Service

In this tutorial you will learn how to create a multi-cluster Service and how to consume it from each connected cluster.

Specifically, you will deploy an application in a first cluster (*London*) and then offload the corresponding Service and transparently consume it from a second cluster (*New York*).

## Provision the playground

First, check that you are compliant with the [requirements](/examples/requirements.md).

Then, let's open a terminal on your machine and launch the following script, which creates the two above-mentioned clusters with KinD and installs Liqo on them.
Each cluster is made by a single combined control-plane + worker node.

{{ env.config.html_context.generate_clone_example('service-offloading') }}

Export the kubeconfigs environment variables to use them in the rest of the tutorial:

```bash
export KUBECONFIG="$PWD/liqo_kubeconf_london"
export KUBECONFIG_NEWYORK="$PWD/liqo_kubeconf_newyork"
```

```{admonition} Note
We suggest exporting the kubeconfig of the first cluster as default (i.e., `KUBECONFIG`), since it will be the entry point of the virtual cluster and you will mainly interact with it.
```

At this point, you should have two clusters with Liqo installed on them.
The setup script named them **london** and **newyork**.

## Peer the clusters

Once Liqo is installed in your clusters, you can establish new *peerings*:

```bash
liqoctl peer --remote-kubeconfig "$KUBECONFIG_NEWYORK" --gw-server-service-type NodePort
```

When the above command returns successfully, you can check the peering status by running:

```bash
kubectl get foreignclusters
```

The output should look like the following, indicating that an outgoing peering is currently active towards the *New York* cluster:

```text
NAME      ROLE       AGE
newyork   Provider   41s
```

## Offload a service

Now, let's deploy a simple application composed of a *Deployment* and a *Service* in the *London* cluster.

First, you should create a hosting namespace in the *London* cluster:

```bash
kubectl create namespace liqo-demo
```

Then, deploy the application in the *London* cluster:

```bash
kubectl apply -f manifests/app.yaml -n liqo-demo
```

At this point, you should have an HTTP application serving JSON data through a Service, and running in the *London* cluster (i.e., locally).
If you look at the *New York* cluster, you will not see the application yet.

To make it visible, you need to enable the Liqo offloading of the Services in the desired namespace to the *New York* cluster:

```bash
liqoctl offload namespace liqo-demo \
    --namespace-mapping-strategy EnforceSameName \
    --pod-offloading-strategy Local
```

This command enables the offloading of the Services in the *London* cluster to the *New York* cluster and sets:

* With the `EnforceSameName` mapping strategy, we instruct Liqo to create the offloaded namespace in the remote cluster with the same name as the local one. This is particularly useful when you want to consume the Services in the remote cluster using the Kubernetes DNS service discovery (i.e. with `svc-name.namespace-name.svc.cluster.local`).
* The pod offloading strategy to *Local*, which means that the pods running in this namespace will be kept local and not scheduled on virtual nodes (i.e., no pod is offloaded to remote clusters).

Refer to the dedicated [usage page](/usage/namespace-offloading.md) for additional information concerning namespace offloading configurations.

Some seconds later, you should see that the *Service* has been replicated by the [resource reflection process](FeatureResourceReflection), and is now available in the *New York* cluster:

```bash
kubectl get services --namespace liqo-demo --kubeconfig "$KUBECONFIG_NEWYORK"
```

```text
NAME              TYPE        CLUSTER-IP      EXTERNAL-IP   PORT(S)    AGE
flights-service   ClusterIP   10.81.139.132   <none>        7999/TCP   14s
```

The Service is characterized by a different *ClusterIP* address in the two clusters, since each cluster handles it independently.
Additionally, you can also check that there is no application pod running in the *New York* cluster:

```bash
kubectl get pods --namespace liqo-demo --kubeconfig "$KUBECONFIG_NEWYORK"
```

```text
No resources found in liqo-demo namespace.
```

### Consume the service

Let's now consume the Service from both clusters from a different pod (e.g., a temporary shell).

Starting from the *London* cluster:

```bash
kubectl run consumer -it --image=curlimages/curl --rm --restart=Never \
    -- curl -s -H 'accept: application/json' http://flights-service.liqo-demo:7999/schedule
```

A similar result is obtained executing the same command in a shell running in the *New York* cluster, although the backend pod is effectively running in the *London* cluster:

```bash
kubectl run consumer -it --image=curlimages/curl --rm --restart=Never \
    --kubeconfig $KUBECONFIG_NEWYORK \
    -- curl -s -H 'accept: application/json' http://flights-service.liqo-demo:7999/schedule
```

This quick example demonstrated how Liqo can **upgrade *ClusterIP* Services to multi-cluster Services**, allowing your local pods to transparently serve traffic originating from remote clusters with no additional configuration neither in the local cluster and/or applications nor in the remote ones.

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
liqoctl unpeer --remote-kubeconfig "$KUBECONFIG_NEWYORK" --skip-confirm
```

At the end of the process, the virtual node is removed from the local cluster.

### Uninstall Liqo

Now you can uninstall Liqo from your clusters with *liqoctl*:

```bash
liqoctl uninstall
liqoctl uninstall --kubeconfig="$KUBECONFIG_NEWYORK"
```

```{admonition} Purge
By default the Liqo CRDs will remain in the cluster, but they can be removed with the `--purge` flag:

```bash
liqoctl uninstall --purge
liqoctl uninstall --kubeconfig="$KUBECONFIG_NEWYORK" --purge
```

### Destroy clusters

To teardown the KinD clusters, you can issue:

```bash
kind delete cluster --name london
kind delete cluster --name newyork
```
