# Quick Start

This tutorial aims to present how to install Liqo and practice with its most notable capabilities.
You will learn how to create a *virtual cluster* by peering two Kubernetes clusters and how to deploy a simple application on it.

## Provision the playground

First, check that you are compliant with the [requirements](/examples/requirements.md) and install all the necessary tools, including [liqoctl](/installation/liqoctl.md), which will be used in this guide to install Liqo and to create the pairing between the clusters.

Then, let's open a terminal on your machine and launch the following script, which creates a pair of clusters with KinD.
Each cluster is made of two nodes (one for the control plane and one as a simple worker):

{{ env.config.html_context.generate_clone_example('quick-start') }}

### Explore the playground

You can inspect the deployed clusters by typing:

```bash
kind get clusters
```

You should see a couple of entries:

```text
milan
rome
```

This means that two KinD clusters are deployed and running on your host.

Then, you can simply inspect the status of the clusters.
To do so, you can export the `KUBECONFIG` variable to specify the identity file for *kubectl* and *liqoctl*, and then contact the cluster.

By default, the kubeconfigs of the two clusters are stored in the current directory (`./liqo_kubeconf_rome`, `./liqo_kubeconf_milan`).
You can export the appropriate environment variables leveraged for the rest of the tutorial (i.e., `KUBECONFIG` and `KUBECONFIG_MILAN`), and refer to their location, through the following:

```bash
export KUBECONFIG="$PWD/liqo_kubeconf_rome"
export KUBECONFIG_MILAN="$PWD/liqo_kubeconf_milan"
```

```{admonition} Note
We suggest exporting the kubeconfig of the first cluster as default (i.e., `KUBECONFIG`), since it will be the entry point of the virtual cluster and you will mainly interact with it.
```

On the first cluster, you can get the available pods by merely typing:

```bash
kubectl get pods -A
```

Similarly, on the second cluster, you can observe the pods in execution:

```bash
kubectl get pods -A --kubeconfig "$KUBECONFIG_MILAN"
```

If the above commands return each an output similar to the following, your clusters are up and ready.

```text
NAMESPACE            NAME                                         READY   STATUS    RESTARTS   AGE
kube-system          coredns-558bd4d5db-9vdr9                     1/1     Running   0          3m58s
kube-system          coredns-558bd4d5db-tzdxg                     1/1     Running   0          3m58s
kube-system          etcd-rome-control-plane                      1/1     Running   0          4m10s
kube-system          kindnet-fcspl                                1/1     Running   0          3m58s
kube-system          kindnet-q6qkm                                1/1     Running   0          3m42s
kube-system          kube-apiserver-rome-control-plane            1/1     Running   0          4m10s
kube-system          kube-controller-manager-rome-control-plane   1/1     Running   0          4m11s
kube-system          kube-proxy-2c9bl                             1/1     Running   0          3m42s
kube-system          kube-proxy-7nngv                             1/1     Running   0          3m58s
kube-system          kube-scheduler-rome-control-plane            1/1     Running   0          4m11s
local-path-storage   local-path-provisioner-85494db59d-skd55      1/1     Running   0          3m58s
```

## Install Liqo

You will now install Liqo on both clusters, using the following characterizing names:

* **rome**: the *local* (**consumer**) cluster, where you will deploy and control the applications.
* **milan**: the *remote* (**provider**) cluster, where part of your workloads will be offloaded to.

You can install Liqo on the *Rome* cluster by launching:

```bash
liqoctl install kind --cluster-id rome
```

This command will generate the suitable configuration for your KinD cluster and then install Liqo.

Similarly, you can install Liqo on the *Milan* cluster by launching:

```bash
liqoctl install kind --cluster-id milan --kubeconfig "$KUBECONFIG_MILAN"
```

On both clusters, you should see the following output:

```text
 INFO  Installer initialized
 INFO  Cluster configuration correctly retrieved
 INFO  Installation parameters correctly generated
 INFO  All Set! You can now proceed establishing a peering (liqoctl peer --help for more information)
 INFO  Make sure to use the same version of Liqo on all remote clusters
```

And the Liqo pods should be up and running:

```bash
kubectl get pods -n liqo
```

```text
NAME                                       READY   STATUS    RESTARTS   AGE
liqo-controller-manager-6888ccc645-hnxrl   1/1     Running   0          8m15s
liqo-crd-replicator-5f5b448bd-ldz67        1/1     Running   0          8m15s
liqo-fabric-28tb5                          1/1     Running   0          8m15s
liqo-fabric-dvjgk                          1/1     Running   0          8m15s
liqo-ipam-658dcbcb66-z4vhq                 1/1     Running   0          8m15s
liqo-metric-agent-597c5dbcfd-bzg2t         1/1     Running   0          8m15s
liqo-proxy-599958d9b8-6fzfc                1/1     Running   0          8m15s
liqo-webhook-8fbd8c664-pxrfh               1/1     Running   0          8m15s
```

At this point, it is possible to check status and info about the current Liqo instance, runnning:

```bash
liqoctl info
```

```text
─ Local installation info ────────────────────────────────────────────────────────
  Cluster ID:     milan
  Version:        v1.0.0-rc.2
  K8s API server: https://172.19.0.10:6443
  Cluster labels
      liqo.io/provider: kind
──────────────────────────────────────────────────────────────────────────────────
─ Installation health ────────────────────────────────────────────────────────────
  ✔    Liqo is healthy
──────────────────────────────────────────────────────────────────────────────────
─ Active peerings ────────────────────────────────────────────────────────────────

──────────────────────────────────────────────────────────────────────────────────
```

## Peer two clusters

Once Liqo is installed in your clusters, you can establish new *peerings*.
In this example, we will leverage the *liqoctl peer* command to peer the two clusters.
This approach requires the user to have access to the `kubeconfig` of both clusters.

Let's issue the peering command from the consumer cluster, which is Rome in this case.
The `--remote-kubeconfig` flag is used to specify the `kubeconfig` of the remote provider cluster, which is Milan in this case. Moreover, as no load balancer is configured in the clusters, we will set `--gw-server-service-type` to `NodePort` to use a port of the nodes of the clusters to expose the Liqo gateway.

```bash
liqoctl peer --remote-kubeconfig "$KUBECONFIG_MILAN" --gw-server-service-type NodePort
```

The peering should be completed successfully after a few seconds (if the Docker images are already cached in the involved clusters) or minutes (when the Docker images have to be downloaded).
The output should look like this:

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
 INFO   (remote) Gateway pod gw-rome is ready
 INFO   (remote) Gateway server Service created successfully
 INFO   (local) Gateway client correctly set up
 INFO   (local) Gateway pod gw-milan is ready
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

You can check the peering status by running:

```bash
liqoctl info
```

Where in the output you should be able to see that a new peer appeared in the "Active peerings" section:

```text
─ Local installation info ────────────────────────────────────────────────────────
  Cluster ID:     rome
  Version:        v1.0.0-rc.2
  K8s API server: https://172.19.0.9:6443
  Cluster labels
      liqo.io/provider: kind
──────────────────────────────────────────────────────────────────────────────────
─ Installation health ────────────────────────────────────────────────────────────
  ✔    Liqo is healthy
──────────────────────────────────────────────────────────────────────────────────
─ Active peerings ────────────────────────────────────────────────────────────────
  milan
      Role:                  Provider
      Networking status:     Healthy
      Authentication status: Healthy
      Offloading status:     Healthy
──────────────────────────────────────────────────────────────────────────────────
```

````{admonition} Tip
To get additional info about the specific peering you can run:

```bash
liqoctl info peer milan
```
````

Additionally, you should be able to see a new CR describing the relationship with the foreign cluster:

```bash
kubectl get foreignclusters
```

```text
NAME    ROLE       AGE
milan   Provider   52s
```

Moreover, you should be able to see a new virtual node (`milan`) among the list of nodes in the cluster:

```bash
kubectl get nodes
```

```text
NAME                 STATUS   ROLES           AGE     VERSION
rome-control-plane   Ready    control-plane   8m33s   v1.30.0
rome-worker          Ready    <none>          8m13s   v1.30.0
milan                Ready    agent           44s     v1.30.0
```

## Leverage remote resources

Now, you can deploy a standard Kubernetes application in a multi-cluster environment as you would do in a single-cluster scenario (i.e. no modification is required).

(ExamplesStartHelloWorldApplication)=

### Start a "Hello World" application

If you want to deploy an application that is scheduled onto Liqo virtual nodes, you should first create a namespace where your pod will be started.
Then tell Liqo to make this namespace eligible for the pod offloading.

```bash
kubectl create namespace liqo-demo
liqoctl offload namespace liqo-demo
```

The `liqoctl offload namespace` command enables Liqo to offload the namespace to the remote cluster.
Since no further configuration is provided, Liqo will add a suffix to the namespace name to make it unique on the remote cluster (see the dedicated [usage page](/usage/namespace-offloading.md) for additional information concerning namespace offloading configurations).

```{admonition} Note
The virtual nodes have a [taint](https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration/) that prevents the pods from being scheduled on them.
The Liqo webhook will add the toleration for this taint to the pods created in the liqo-enabled namespaces.
```

Then, you can deploy a demo application in the `liqo-demo` namespace of the local cluster:

```bash
kubectl apply -f ./manifests/hello-world.yaml -n liqo-demo
```

The `hello-world.yaml` file represents a simple `nginx` service.
It contains two pods running an `nginx` image and a Service exposing the pods to the cluster.
One pod is running in the local cluster, while the other is forced to be scheduled on the remote cluster.

```{admonition} Info
Differently from the traditional examples, the above deployment introduces an *affinity* constraint. This forces Kubernetes to schedule the first pod (i.e. `nginx-local`) on a physical node and the second (i.e. `nginx-remote`) on a virtual node.
Virtual nodes are like traditional Kubernetes nodes, but they represent remote clusters and have the `liqo.io/type: virtual-node` label.

When the affinity constraint is not specified, the Kubernetes scheduler selects the best hosting node based on the available resources.
Hence, each pod can be scheduled either in the *local* cluster or in the *remote* cluster.
```

Now you can check the status of the pods.
The output should be similar to the one below, confirming that one `nginx` pod is running locally; while the other is hosted by the virtual node (i.e., `milan`).

```bash
kubectl get pod -n liqo-demo -o wide
```

The output should look like this:

```text
NAME           READY   STATUS    RESTARTS   AGE   IP            NODE          NOMINATED NODE   READINESS GATES
nginx-local    1/1     Running   0          10s   10.200.1.11   rome-worker   <none>           <none>
nginx-remote   1/1     Running   0          9s    10.202.1.10   milan         <none>           <none>
```

#### Check the pod connectivity

Once both pods are correctly running, it is possible to check one of the abstractions introduced by Liqo.
Indeed, Liqo enables each pod to be transparently contacted by every other pod and physical node (according to the Kubernetes model), regardless of whether it is hosted by the *local* or by the *remote* cluster.

First, let's retrieve the IP address of the `nginx` pods:

```bash
LOCAL_POD_IP=$(kubectl get pod nginx-local -n liqo-demo --template={{.status.podIP}})
REMOTE_POD_IP=$(kubectl get pod nginx-remote -n liqo-demo --template={{.status.podIP}})
echo "Local Pod IP: ${LOCAL_POD_IP} - Remote Pod IP: ${REMOTE_POD_IP}"
```

You can fire up a pod and run `curl` from inside the cluster:

```bash
kubectl run --image=curlimages/curl curl -n default -it --rm --restart=Never -- curl ${LOCAL_POD_IP}
kubectl run --image=curlimages/curl curl -n default -it --rm --restart=Never -- curl ${REMOTE_POD_IP}
```

Both commands should lead to a successful outcome (i.e., return a demo web page), regardless of whether each pod is executed locally or remotely.

### Expose the pods through a Service

The above `hello-world.yaml` manifest additionally creates a Service that is designed to serve traffic to the previously deployed pods.
This is a traditional [Kubernetes Service](https://kubernetes.io/docs/concepts/services-networking/service/) and can work with Liqo with no modifications.

Indeed, inspecting the Service, it is possible to observe that both `nginx` pods are correctly specified as endpoints.
Nonetheless, it is worth noticing that the first endpoint (i.e. `10.200.1.10:80` in this example) refers to a pod running in the *local* cluster, while the second one (i.e. `10.202.1.9:80`) points to a pod hosted by the *remote* cluster.

```bash
kubectl describe service liqo-demo -n liqo-demo
```

```text
Name:              liqo-demo
Namespace:         liqo-demo
Labels:            <none>
Annotations:       <none>
Selector:          app=liqo-demo
Type:              ClusterIP
IP Family Policy:  SingleStack
IP Families:       IPv4
IP:                10.94.41.143
IPs:               10.94.41.143
Port:              web  80/TCP
TargetPort:        web/TCP
Endpoints:         10.200.1.11:80,10.202.1.10:80
Session Affinity:  None
Events:
  Type    Reason                Age                From                     Message
  ----    ------                ----               ----                     -------
  Normal  SuccessfulReflection  51s (x2 over 51s)  liqo-service-reflection  Successfully reflected object to cluster "milan"
```

#### Check the Service connectivity

It is now possible to contact the Service: as usual, Kubernetes will forward the HTTP request to one of the available back-end pods.
Additionally, all traditional mechanisms still work seamlessly (e.g. DNS discovery), even though one of the pods is actually running in a *remote* cluster.

You can fire up a pod and run `curl` from inside the cluster:

```bash
kubectl run --image=curlimages/curl curl -n default -it --rm --restart=Never -- \
    curl --silent liqo-demo.liqo-demo.svc.cluster.local
```

```{admonition} Note
Executing the previous command multiple times, the requests will be answered by both, the pod running in the *local* cluster, and in part by the one running in the *remote* cluster.
```

## Play with a microservice application

It is very common in a cloud-based environment to deploy microservices applications composed of many pods interacting with each other.
This pattern is transparently supported by Liqo and the virtual cluster abstraction.

You can play with a [microservices application](https://github.com/GoogleCloudPlatform/microservices-demo) provided by Google, which includes multiple cooperating Services leveraging different networking protocols:

```bash
kubectl apply -k ./manifests/demo-application -n liqo-demo
```

By default, Kubernetes schedules each pod either in the local or in the remote cluster, optimizing each deployment based on the available resources.
However, you can play with *affinity* constraints to force Kubernetes to schedule each component in a specific location, and see that everything continues to work smoothly. That's because Liqo takes care of the inter-pod communication across the clusters in the offloaded namespace, and of the [*replication*](FeatureResourceReflection) of the `Service` resources, so that each pod can contact each others, reach the Services and leverage the traditional Kubernetes discovery mechanisms (e.g., DNS discovery and environment variables).

Additionally, several other objects (e.g. `ConfigMaps` and `Secrets`) inside a namespace are replicated in the remote cluster within the *twin namespace*, thus, ensuring that complex applications can work seamlessly across clusters.

### Observe the application deployment

Once the demo application manifest is applied, you can observe the creation of the different pods:

```bash
watch kubectl get pods -n liqo-demo -o wide
```

At steady-state, you should see an output similar to the following.
Pods may be hosted by either the local nodes (*rome-worker* in the example below) or the remote cluster (*milan* in the example below), depending on the decisions of the Kubernetes scheduler.

```text
NAME                                     READY   STATUS    RESTARTS        AGE     IP            NODE          NOMINATED NODE   READINESS GATES
adservice-84cdf76d7d-6s8pq               1/1     Running   0               5m1s    10.202.1.11   milan         <none>           <none>
cartservice-5c9c9c7b4-w49gr              1/1     Running   0               5m1s    10.202.1.12   milan         <none>           <none>
checkoutservice-6cb9bb8cd8-5w2ht         1/1     Running   0               5m1s    10.202.1.13   milan         <none>           <none>
currencyservice-7d4bd86676-5b5rq         1/1     Running   0               5m1s    10.202.1.14   milan         <none>           <none>
emailservice-c9b45cdb-6zjrk              1/1     Running   0               5m1s    10.202.1.15   milan         <none>           <none>
frontend-58b9b98d84-hg4xz                1/1     Running   0               5m1s    10.200.1.13   rome-worker   <none>           <none>
loadgenerator-5f8cd58cd4-wvqqq           1/1     Running   0               5m1s    10.202.1.16   milan         <none>           <none>
nginx-local                              1/1     Running   0               7m35s   10.200.1.11   rome-worker   <none>           <none>
nginx-remote                             1/1     Running   0               7m34s   10.202.1.10   milan         <none>           <none>
paymentservice-69558cf7bb-v4zjw          1/1     Running   0               5m      10.202.1.17   milan         <none>           <none>
productcatalogservice-55c58b57cb-k8mfq   1/1     Running   0               5m      10.202.1.18   milan         <none>           <none>
recommendationservice-55cd66cf64-6fz9w   1/1     Running   0               5m      10.202.1.19   milan         <none>           <none>
redis-cart-5d45978b94-wjd97              1/1     Running   0               5m      10.202.1.20   milan         <none>           <none>
shippingservice-5df47fc86-f867j          1/1     Running   0               4m59s   10.202.1.21   milan         <none>           <none>
```

### Access the demo application

Once the deployment is up and running, you can start using the demo application and verify that everything works correctly, even if its components are distributed across multiple Kubernetes clusters.

By default, the frontend web page is exposed through a `LoadBalancer` Service, which can be inspected using:

```bash
kubectl get service -n liqo-demo frontend-external
```

Leverage `kubectl port-forward` to forward the requests from your local machine (i.e., `http://localhost:8080`) to the frontend Service:

```bash
kubectl port-forward -n liqo-demo service/frontend-external 8080:80
```

Open the [http://localhost:8080](http://localhost:8080) page in your browser and enjoy the demo application.

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
liqoctl unpeer --remote-kubeconfig "$KUBECONFIG_MILAN"
```

At the end of the process, the virtual node is removed from the local cluster.

### Uninstall Liqo

Now you can uninstall Liqo from your clusters with *liqoctl*:

```bash
liqoctl uninstall --skip-confirm
liqoctl uninstall --kubeconfig="$KUBECONFIG_MILAN" --skip-confirm
```

```{admonition} Purge
By default the Liqo CRDs will remain in the cluster, but they can be removed with the `--purge` flag:

```bash
liqoctl uninstall --purge
liqoctl uninstall --purge --kubeconfig="$KUBECONFIG_MILAN"
```

### Destroy clusters

To teardown the KinD clusters, you can issue:

```bash
kind delete cluster --name rome
kind delete cluster --name milan
```
