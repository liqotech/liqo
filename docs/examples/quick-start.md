# Quick Start

This tutorial aims at presenting how to install Liqo and practicing with its most notable capabilities.
You will learn how to create a *virtual cluster* by peering two Kubernetes clusters and how to deploy a simple application on it.

## Provision the playground

First, check that you are compliant with the [requirements](/examples/requirements.md).

Then, let's open a terminal on your machine and launch the following script, which creates a pair of clusters with KinD.
Each cluster is made by two nodes (one for the control plane and one as a simple worker):

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
You can export the appropriate environment variables leveraged for the rest of the tutorial (i.e., `KUBECONFIG` and `KUBECONFIG_MILAN`), and referring to their location, through the following:

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

* **rome**: the *local* cluster, where you will deploy and control the applications.
* **milan**: the *remote* cluster, where part of your workloads will be offloaded to.

You can install Liqo on the *Rome* cluster by launching:

```bash
liqoctl install kind --cluster-name rome
```

This command will generate the suitable configuration for your KinD cluster and then install Liqo.

Similarly, you can install Liqo on the *Milan* cluster by launching:

```bash
liqoctl install kind --cluster-name milan --kubeconfig "$KUBECONFIG_MILAN"
```

On both clusters, you should see the following output:

```text
 INFO  Kubernetes clients successfully initialized
 INFO  Installer initialized
 INFO  Cluster configuration correctly retrieved
 INFO  Installation parameters correctly generated
 INFO  All Set! You can now proceed establishing a peering (liqoctl peer --help for more information)
```

And the Liqo pods should be up and running:

```bash
kubectl get pods -n liqo
```

```text
NAME                                       READY   STATUS    RESTARTS   AGE
liqo-auth-74c795d84c-x2p6h                 1/1     Running   0          2m8s
liqo-controller-manager-6c688c777f-4lv9d   1/1     Running   0          2m8s
liqo-crd-replicator-6c64df5457-bq4tv       1/1     Running   0          2m8s
liqo-gateway-78cf7bb86b-pkdpt              1/1     Running   0          2m8s
liqo-metric-agent-5667b979c7-snmdg         1/1     Running   0          2m8s
liqo-network-manager-5b5cdcfcf7-scvd9      1/1     Running   0          2m8s
liqo-proxy-6674dd7bbd-kr2ls                1/1     Running   0          2m8s
liqo-route-7wsrx                           1/1     Running   0          2m8s
liqo-route-sz75m                           1/1     Running   0          2m8s
```

In addition, you can check the installation status, and the main Liqo configuration parameters, using:

```bash
liqoctl status
```

## Peer two clusters

Once Liqo is installed in your clusters, you can establish new *peerings*.
In this example, since the two API Servers are mutually reachable, you will use the [out-of-band peering approach](FeaturesPeeringOutOfBandControlPlane).

First, get the *peer command* from the *Milan* cluster:

```bash
liqoctl generate peer-command --kubeconfig "$KUBECONFIG_MILAN"
```

Second, copy and paste the command in the *Rome* cluster:

```bash
liqoctl peer out-of-band milan --auth-url [redacted] --cluster-id [redacted] --auth-token [redacted]
```

Now, the Liqo control plane in the *Rome* cluster will contact the provided authentication endpoint providing the token to the *Milan* cluster to get its Kubernetes identity.

You can check the peering status by running:

```bash
kubectl get foreignclusters
```

The output should look like the following, indicating that the cross-cluster network tunnel has been established, and an outgoing peering is currently active (i.e., the *Rome* cluster can offload workloads to the *Milan* one, but not vice versa):

```text
NAME    TYPE        OUTGOING PEERING   INCOMING PEERING   NETWORKING    AUTHENTICATION   AGE
milan   OutOfBand   Established        None               Established   Established      12s
```

At the same time, you should see a virtual node (`liqo-milan`) in addition to your physical nodes:

```bash
kubectl get nodes
```

```text
NAME                 STATUS   ROLES                  AGE     VERSION
liqo-milan           Ready    agent                  14s     v1.23.6
rome-control-plane   Ready    control-plane,master   7m56s   v1.23.6
rome-worker          Ready    <none>                 7m25s   v1.23.6
```

In addition, you can check the peering status, and retrieve more advanced information, using:

```bash
liqoctl status peer milan
```

## Leverage remote resources

Now, you can deploy a standard Kubernetes application in a multi-cluster environment as you would do in a single cluster scenario (i.e. no modification is required).

(ExamplesStartHelloWorldApplication)=

### Start a hello world application

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
One pods is running in the local cluster, while the other is forced to be scheduled on the remote cluster.

```{admonition} Info
Differently from the traditional examples, the above deployment introduces an *affinity* constraint. This forces Kubernetes to schedule the first pod (i.e. `nginx-local`) on a physical node and the second (i.e. `nginx-remote`) on a virtual node.
Virtual nodes are like traditional Kubernetes nodes, but they represent remote clusters and have the `liqo.io/type: virtual-node` label.

When the affinity constraint is not specified, the Kubernetes scheduler selects the best hosting node based on the available resources.
Hence, each pod can be scheduled either in the *local* cluster or in the *remote* cluster.
```

Now you can check the status of the pods.
The output should be similar to the one below, confirming that one `nginx` pod is running locally; while the other is hosted by the virtual node (i.e., `liqo-milan`).

```bash
kubectl get pod -n liqo-demo -o wide
```

And the output should look like this:

```text
NAME           READY   STATUS    RESTARTS   AGE   IP            NODE          NOMINATED NODE   READINESS GATES
nginx-local    1/1     Running   0          94s   10.200.1.10   rome-worker   <none>           <none>
nginx-remote   1/1     Running   0          94s   10.202.1.9    liqo-milan    <none>           <none>
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

The above `hello-world.yaml` manifest additionally creates a Service which is designed to serve traffic to the previously deployed pods.
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
IP:                10.93.84.150
IPs:               10.93.84.150
Port:              web  80/TCP
TargetPort:        web/TCP
Endpoints:         10.200.1.10:80,10.202.1.9:80
Session Affinity:  None
Events:            <none>
```

#### Check the Service connectivity

It is now possible to contact the Service: as usual, Kubernetes will forward the HTTP request to one of the available back-end pods.
Additionally, all traditional mechanisms still work seamlessly (e.g. DNS discovery), even though one of the pods is actually running in a *remote* cluster.

You can fire up a pod and run `curl` from inside the cluster:

```bash
kubectl run --image=curlimages/curl curl -n default -it --rm --restart=Never -- \
    curl --silent liqo-demo.liqo-demo.svc.cluster.local | grep 'Server'
```

```{admonition} Note
Executing the previous command multiple times, you will observe that part of the requests are answered by the pod running in the *local* cluster, and in part by that in the *remote* cluster (i.e., the Server value changes).
```

## Play with a microservice application

It is very common in a cloud-based environment to deploy microservices applications composed of many pods interacting among each other.
This pattern is transparently supported by Liqo and the virtual cluster abstraction.

You can play with a [microservices application](https://github.com/GoogleCloudPlatform/microservices-demo) provided by Google, which includes multiple cooperating Services leveraging different networking protocols:

```bash
kubectl apply -k ./manifests/demo-application -n liqo-demo
```

By default, Kubernetes schedules each pod either in the local or in the remote cluster, optimizing each deployment based on the available resources.
However, you can play with *affinity* constraints to force Kubernetes to schedule of each component in a specific location, and see that everything continues to work smoothly.
Specifically, the manifest above forces the frontend component to be executed in the *local* cluster, as this is required to enable *port-forwarding*, which is leveraged below.

Each demo component is exposed as a Service and accessed by other components.
However, given that nobody knows, a priori, where each Service will be deployed (either locally or in the remote cluster), Liqo [*replicates*](FeatureResourceReflection) all Kubernetes Services across both clusters, although the corresponding pod may be running only in one location.
Hence, each microservice deployed across clusters can reach the others seamlessly: independently of the cluster a pod is deployed in, each pod can contact other Services and leverage the traditional Kubernetes discovery mechanisms (e.g., DNS discovery and environment variables).

Additionally, several other objects (e.g. `ConfigMaps` and `Secrets`) inside a namespace are replicated in the remote cluster within the *twin namespace*, thus, ensuring that complex applications can work seamlessly across clusters.

### Observe the application deployment

Once the demo application manifest is applied, you can observe the creation of the different pods:

```bash
watch kubectl get pods -n liqo-demo -o wide
```

At steady-state, you should see an output similar to the following.
Different pods may be hosted by either the local nodes (*rome-worker* in the example below) or remote cluster (*liqo-milan* in the example below), depending on the scheduling decisions.

```text
NAME                                     READY   STATUS    RESTARTS   AGE     IP            NODE          NOMINATED NODE   READINESS GATES
adservice-66f6b5c6fd-w95th               1/1     Running   0          2m23s   10.202.1.19   liqo-milan    <none>           <none>
cartservice-76dc758684-wd5px             1/1     Running   0          2m23s   10.202.1.15   liqo-milan    <none>           <none>
checkoutservice-85b74f746f-lm7gh         1/1     Running   0          2m24s   10.202.1.11   liqo-milan    <none>           <none>
currencyservice-64775746dd-k85z6         1/1     Running   0          2m23s   10.202.1.16   liqo-milan    <none>           <none>
emailservice-58f8b4f854-9mx7g            1/1     Running   0          2m24s   10.202.1.10   liqo-milan    <none>           <none>
frontend-7b648dcb8f-gfbh2                1/1     Running   0          2m23s   10.200.1.15   rome-worker   <none>           <none>
nginx-local                              1/1     Running   0          6m5s    10.200.1.10   rome-worker   <none>           <none>
nginx-remote                             1/1     Running   0          6m5s    10.202.1.9    liqo-milan    <none>           <none>
paymentservice-5dd7bb5855-ssbhf          1/1     Running   0          2m23s   10.202.1.14   liqo-milan    <none>           <none>
productcatalogservice-587c8dbf7d-nmw77   1/1     Running   0          2m23s   10.202.1.13   liqo-milan    <none>           <none>
recommendationservice-6cd468f4d4-h8k2t   1/1     Running   0          2m24s   10.202.1.12   liqo-milan    <none>           <none>
redis-cart-78746d49dc-rkqrn              1/1     Running   0          2m23s   10.202.1.18   liqo-milan    <none>           <none>
shippingservice-59c7b7458d-sqb9x         1/1     Running   0          2m23s   10.202.1.17   liqo-milan    <none>           <none>
```

### Access the demo application

Once the deployment is up and running, you can start using the demo application and verify that everything works correctly, even if its components are distributed across multiple Kubernetes clusters.

By default, the frontend web-page is exposed through a `LoadBalancer` Service, which can be inspected using:

```bash
kubectl get service -n liqo-demo frontend-external
```

Leverage `kubectl port-forward` to forward the requests from your local machine (i.e., `http://localhost:8080`) to the frontend Service:

```bash
kubectl port-forward -n liqo-demo service/frontend-external 8080:80
```

Open the [http://localhost:8080](http://localhost:8080) page in your browser and enjoy the demo application.

## Tear down the playground

### Unoffload namespaces

Before starting the uninstallation process, make sure that all namespaces are unoffloaded:

```bash
liqoctl unoffload namespace liqo-demo
```

Every pod that was offloaded to a remote cluster is going to be rescheduled onto the local cluster.

### Revoke peerings

Similarly, make sure that all the peerings are revoked:

```bash
liqoctl unpeer out-of-band milan
```

At the end of the process, the virtual node is removed from the local cluster.

### Uninstall Liqo

Now you can uninstall Liqo from your clusters with *liqoctl*:

```bash
liqoctl uninstall
liqoctl uninstall --kubeconfig="$KUBECONFIG_MILAN"
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
