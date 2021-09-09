---
title: Use resources available in a foreign cluster
weight: 4
---

This fourth step allows to verify that the resulting infrastructure works correctly.
This is done by showing the deployment of a small *Hello World*  service in presence of two peered clusters (*home* and *foreign*).
This demonstrates the capability of Liqo to leverage resources available in a foreign cluster, and how it can start a pod either in the local (*home*) or remote (*foreign*) cluster, transparently, without any change in the user experience.

## Start a Hello World pod

First, ensure you have configured your KUBECONFIG to point to your home cluster. Otherwise, set the `KUBECONFIG` variable to the correct value:
```shell script
export KUBECONFIG=home-kubeconfig.yaml
```

If you want to deploy an application schedulable on the Liqo node, you should create a namespace where your pod will be started and label it as ```liqo.io/enabled=true```. Indirectly, this label will tell the Kubernetes scheduler that the namespace spans across the foreign clusters as well.

```shell script
kubectl create namespace liqo-demo
kubectl label namespace liqo-demo liqo.io/enabled=true
```

Then, you can deploy a demo application in the `liqo-demo` namespace:

```shell script
kubectl apply -f https://raw.githubusercontent.com/liqotech/liqo/master/docs/examples/hello-world.yaml -n liqo-demo
```
The `hello-world.yaml` file is a simple `nginx` service; it is composed of two pods running an `nginx` image, and a service exposing the pods to the cluster; the reason for having _two_ `nginx` pods is to create a configuration in which one pod runs in the local cluster, while the other is forced to be scheduled on the remote cluster.

{{%expand "Expand here for a more advanced explanation of what happens under the hood:" %}}

The complete `hello-world.yaml` file is as follows:
{{% render-code file="static/examples/hello-world.yaml" language="yaml" %}}


Differently from the traditional examples, the above deployment introduces an *affinity* constraint. This forces Kubernetes to schedule the first pod (i.e. `nginx-local`) on a physical node, and the second one (i.e. `nginx-remote`) on a virtual node.
Virtual nodes are like traditional Kubernetes nodes, but they represent foreign clusters and are labelled with `liqo.io/type: virtual-node`.

In case the affinity constraint is not specified, the Kubernetes scheduler selects the best hosting node based on the available resources.
Hence, each pod can be scheduled either in the *home* cluster or in the *foreign* cluster.

{{% /expand%}}

Now you can check the state of your pods.
The output should be similar to the one below, confirming that one `nginx` pod is running locally, while the other on a virtual node (i.e. a node named `liqo-<...>`):

```
kubectl get pod -o wide -n liqo-demo

NAME           READY   STATUS    RESTARTS   AGE   IP              NODE
nginx-local    1/1     Running   0          76s   10.244.2.214    worker-node-1
nginx-remote   1/1     Running   0          76s   172.16.97.219   liqo-9a596a4b-591c-4ac6-8fd6-80258b4b3bf9
```

## Check the pod connectivity

Once both pods are correctly running, it is possible to check one of the abstractions introduced by Liqo.
Indeed, Liqo enables each pod to be transparently contacted by every other pod and physical node (according to the Kubernetes model), regardless of whether it is hosted by the _local_ or by the foreign cluster.

First, let's retrieve the IP address of the `nginx` pods:

```shell script
LOCAL_POD_IP=$(kubectl get pod nginx-local -n liqo-demo --template={{.status.podIP}})
REMOTE_POD_IP=$(kubectl get pod nginx-remote -n liqo-demo --template={{.status.podIP}})
echo "Local Pod IP: ${LOCAL_POD_IP} - Remote Pod IP: ${REMOTE_POD_IP}"
```

If you have direct connectivity with the home cluster from your host (e.g. you are running K3s locally), you can open a browser and directly check the connectivity to both IP addresses.
You should notice no differences when connecting to the two IP addresses, although one pod is running in the _local_ cluster and the other in the _foreign_ cluster.

Similarly, you can also use `curl` to perform the same check:
```shell script
curl ${LOCAL_POD_IP}
curl ${REMOTE_POD_IP}
```

If you do not have direct connectivity, on the other hand, you can fire up a pod and run `curl` from inside:
```shell script
kubectl run --image=curlimages/curl curl -n default -it --rm --restart=Never -- curl ${LOCAL_POD_IP}
kubectl run --image=curlimages/curl curl -n default -it --rm --restart=Never -- curl ${REMOTE_POD_IP}
```
Also in this case both commands should lead to a successful outcome (i.e. return a demo web page), regardless of whether each pod is executed locally or remotely.

## Expose the pod through a Service

The above `hello-world.yaml` manifest tells Kubernetes to create also a service, which is designed to serve traffic to the previously deployed pods.
The service is a traditional [Kubernetes Service](https://kubernetes.io/docs/concepts/services-networking/service/) and can work with Liqo with no modifications.

Indeed, inspecting the service it is possible to observe that both `nginx` pods are correctly specified as endpoints.
Nonetheless, it is worth noticing that the first endpoint (i.e. `10.244.2.214:80` in this example) refers to a pod running in the _home_ cluster, while the second one (i.e. `172.16.97.219:80`) points to a pod hosted by the _foreign_ cluster.
```shell script
kubectl describe service liqo-demo -n liqo-demo

Name:              liqo-demo
Namespace:         liqo-demo
Type:              ClusterIP
IP:                10.105.51.71
Port:              web  80/TCP
TargetPort:        web/TCP
Endpoints:         10.244.2.214:80,172.16.97.219:80
```


### Check the Service connectivity

It is now possible to contact the service: as usual, kubernetes will forward the HTTP request to one the available back-end pods.
Additionally, all traditional mechanisms still work seamlessly (e.g. DNS discovery), even though one of the pods is actually running in a _foreign_ cluster.

First, you have to retrieve the IP address of the service:
```shell script
SVC_IP=$(kubectl get service liqo-demo -n liqo-demo --template={{.spec.clusterIP}})
echo "Service IP: ${SVC_IP}"
```

If you have direct connectivity with the home cluster from your host (e.g. you are running K3s locally), you can open a browser and directly check the connectivity to the service IP address.
At the bottom of the displayed demo web-page, you should see the IP address and the hostname of the back-end that is serving the request (i.e., either the local or the remote pod).

If you try reloading the page, you can observe a difference: the hostname should alternate between `nginx-local` and `nginx-remote`. This change confirms that Kubernetes correctly leverage both pods as back-ends (i.e., endpoints) of the service.

Similarly, you can also use `curl` to perform the same verification (execute this command multiple times to contact both endpoints):
```
curl --silent ${SVC_IP} | grep 'Server'
```

If you do not have direct connectivity, on the other hand, you can fire up a pod and run `curl` from inside:
```
kubectl run --image=curlimages/curl curl -n default -it --rm --restart=Never -- curl --silent ${SVC_IP} | grep 'Server'
```
Also in this case, if you execute the command multiple times, you should observe an alternation between the local and the remote endpoint.

Finally, you can also connect to the service through its _service name_, which exploits the Kubernetes DNS service:

```
kubectl run --image=curlimages/curl curl -n default -it --rm --restart=Never -- curl --silent http://liqo-demo.liqo-demo | grep 'Server'
```

Now, you are ready to move to the [next section](../play), which plays with a more sophisticated application composed of multiple micro-services.

> **Clean-up**: If you want to delete the deployed example, just issue:
> ```
> kubectl delete -f https://raw.githubusercontent.com/LiqoTech/liqo/master/docs/examples/hello-world.yaml -n liqo-demo
> ```
