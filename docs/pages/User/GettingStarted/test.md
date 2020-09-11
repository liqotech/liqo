---
title: Exploit foreign resources
weight: 3
---

This third step allows to verify that the resulting infrastructure works correctly.
This is done by showing the deployment of a small *Hello World*  service in presence of two peered clusters (*home* and *foreign*).
This demonstrates the capability of Liqo to start a pod either in the local (*home*) or remote (*foreign*) cluster, transparently, without any change in the user experience.

## Start an Hello World pod

First, ensure you are operating in your home cluster. Otherwise, set the `KUBECONFIG` variable to the correct value:
```shell script
export KUBECONFIG=home-kubeconfig.yaml
```

Now you have to create a namespace where your pod will be started and label it as ```liqo.io/enabled=true```. This label will tell the Kubernetes scheduler that the namespace spans across the foreign clusters as well; hence, pods started in the above namespaces are suitable for being executed on the foreign cluster.

```
kubectl create namespace liqo-demo
kubectl label namespace liqo-demo liqo.io/enabled=true
```

Then, you can deploy a demo application in the `liqo-demo` namespace:

```
kubectl apply -f https://raw.githubusercontent.com/liqotech/liqo/master/docs/examples/hello-world.yaml -n liqo-demo
```
The `hello-world.yaml` file is a simple `nginx` service; it is composed of two pods running an `nginx` image, and a service exposing the pods to the cluster.
The two `nginx` pods are configured such that one is executed in the local cluster, while the other is forced to be scheduled on the remote cluster.

{{%expand "Expand here for a more advanced explanation of what is happened under the hoods:" %}}

The complete `hello-world.yaml` file is as follows:
{{% render-code file="static/examples/hello-world.yaml" language="yaml" %}}


Differently from the traditional examples, we introduced an *affinity* constraint.
It forces Kubernetes to schedule the first pod (i.e. `nginx-local`) on a physical node, and the second one (i.e. `nginx-remote`) on a virtual node.
Virtual nodes are like traditional Kubernetes nodes, but they represent foreign clusters and are labelled with `type: virtual-node`.

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

First, let's retrieve the IP address of the `nginx` pods:

```bash
LOCAL_POD_IP=$(kubectl get pod nginx-local -n liqo-demo --template={{.status.podIP}})
REMOTE_POD_IP=$(kubectl get pod nginx-remote -n liqo-demo --template={{.status.podIP}})
echo "Local Pod IP: ${LOCAL_POD_IP} - Remote Pod IP: ${REMOTE_POD_IP}"
```

If you have direct connectivity with the home cluster from your host (e.g. you are running K3s locally),
you can open a browser and directly check the connectivity to both IP addresses.
Similarly, you can also use `curl`:
```
curl ${LOCAL_POD_IP}
curl ${REMOTE_POD_IP}
```

If you do not have direct connectivity, on the other hand, you can fire up a pod and run `curl` from inside:
```
kubectl run --image=curlimages/curl curl -n default -it --rm --restart=Never -- curl ${LOCAL_POD_IP}
kubectl run --image=curlimages/curl curl -n default -it --rm --restart=Never -- curl ${REMOTE_POD_IP}
```

## Expose the pod through a Service

The above `hello-world.yaml` manifest tells Kubernetes to create also a service, which is designed to serve traffic to the previously deployed pods.
The service is a traditional [Kubernetes Service](https://kubernetes.io/docs/concepts/services-networking/service/) and can work with Liqo with no modifications.

Indeed, inspecting the service it is possible to observe that both `nginx` pods are correctly specified as endpoints.
```
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

First, you have to retrieve the IP address of the service:
```
SVC_IP=$(kubectl get service liqo-demo -n liqo-demo --template={{.spec.clusterIP}})
echo "Service IP: ${SVC_IP}"
```

If you have direct connectivity with the home cluster from your host (e.g. you are running K3s locally),
you can open a browser and directly check the connectivity to the service IP address.
Similarly, you can also use `curl`:
```
curl ${SVC_IP}
```

If you do not have direct connectivity, on the other hand, you can fire up a pod and run `curl` from inside:
```
kubectl run --image=curlimages/curl curl -n default -it --rm --restart=Never -- curl ${SVC_IP}
```

You can also connect to the service through its _service name_, which exploits the Kubernetes DNS service:

```
kubectl run --image=curlimages/curl curl -n default -it --rm --restart=Never -- curl http://liqo-demo.liqo-demo
```

Now, you are ready to move to the [next section](../further-steps). to deploy a more complex demo application.

> **Clean-up**: If you want to delete the deployed example, just issue:
> ```
> kubectl delete -f https://raw.githubusercontent.com/LiqoTech/liqo/master/docs/examples/hello-world.yaml -n test-liqo
> ```