---
title: Exploit foreign resources
weight: 3
---

This third step allows to verify that the resulting infrastructure works correctly.
This is done by showing the deployment of a small *Hello World*  service in presence of two peered clusters (*home* and *foreign*).
This demonstrates the capability of Liqo to start a pod either in the local (*home*) or remote (*foreign*) cluster, transparently, without any change in the user experience.

## Start an Hello World pod

To schedule a Pod in a foreign cluster, first you have to set the `KUBECONFIG` variable in the home cluster, such as in the following:

```shell script
export KUBECONFIG=home-kubeconfig.yaml
```

Now you have to create a namespace where your pod will be started and label it as ```liqo.io/enabled=true```. This label will tell the Kubernetes scheduler that the namespace spans across the foreign clusters as well; hence, pods started in the above namespaces are suitable for being executed on the foreign cluster.

```
kubectl create namespace liqo-demo
kubectl label namespace liqo-demo liqo.io/enabled=true
```

Then, you can deploy the test pod in the `liqo-demo` namespace:

```
kubectl apply -f https://raw.githubusercontent.com/liqotech/liqo/master/docs/examples/hello-world.yaml -n liqo-demo
```
The `hello-world.yaml` file is a simple `nginx` service; it is composed of a pod running an `nginx` image, and a service exposing the pod to the cluster. We labeled the pod to be executed on the virtual-node just created.

{{%expand "Expand here for a more advanced explanation of what is happened under the hoods:" %}}

The complete `hello-world.yaml` file is as follows:
{{% render-code file="static/examples/hello-world.yaml" language="yaml" %}}


Differently from the traditional examples, we added an (optional) `nodeSelector` field. This tells the Kubernetes scheduler that the pod has to be started on a virtual node. Virtual nodes are like traditional Kubernetes nodes, but they represent foreign clusters and are labelled with `type: virtual-node`.

In case the `nodeSelector` is not specified, the Kubernetes scheduler will select the best hosting node based on the available resources, which can be either a node in the *home* cluster or in the *foreign* cluster.

{{% /expand%}}

Now you can check the state of your pod; the output should be similar to the one below, confirming that the pod is running on a virtual node (i.e. a node named `liqo-<...>`):

```
kubectl get pod -o wide -n liqo-demo
NAME    READY   STATUS    RESTARTS   AGE   IP           NODE                                      NOMINATED NODE   READINESS GATES
nginx   1/1     Running   0          41m   10.45.0.12   liqo-1dfa22f9-1cdd-4401-9e7a-c5342ec90059   <none>           <none>
```

## Check the pod connectivity

First, let's retrieve the IP address of the `nginx` pod:

```bash
POD_HOST=$(kubectl get pod nginx -n liqo-demo --template={{.status.podIP}})
echo $POD_HOST
```

If you have direct connectivity with the cluster (e.g. K3s) from your host, you can open a browser and directly connect to the value of `$POD_HOST`; alternatively you can also use the following `curl` command:

```
curl --verbose $POD_HOST
```

If you do not have direct connectivity, on the other hand, you can fire up a pod and run `curl` from inside:

```
kubectl run --image=curlimages/curl tester -n default -it --rm -- curl --verbose $POD_HOST
```

## Expose the pod through a Service

The above `hello-world.yaml` manifest tells Kubernetes to create also a service, which is designed to serve traffic to the previously deployed pod.
The service is a traditional [Kubernetes Service](https://kubernetes.io/docs/concepts/services-networking/service/) and can work with Liqo with no modifications.

This can be seen by inspecting the service and its endpoints:

```kubectl get endpoints -n liqo-demo```

You should see the pod offloaded to the remote cluster listed as service endpoint.

<!-- TODO: report the output of this command -->


### Check the Service connectivity

First, you have to retrieve the IP address of the service:

```
SVC_IP=$(kubectl get service -n liqo-demo --template={{.spec.clusterIP}})
echo $SVC_IP
```

If you have direct connectivity with the cluster (e.g. K3s) from your host, open a browser and connect to the value of `$SVC_IP` or use the following `curl` command:

```
curl -v $SVC_IP
```

If you do not have direct connectivity, you can fire up a pod and run a `curl` inside:

```
kubectl run --image=curlimages/curl tester -n test-liqo -ti --rm -- curl -L $SVC_IP
```

You can also connect to the service through its _service name_, which exploits the Kubernetes DNS service:

```
kubectl run --image=curlimages/curl tester -n test-liqo -ti --rm -- curl -L http://test-liqo 
```





