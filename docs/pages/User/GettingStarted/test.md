---
title: Exploit foreign resources 
weight: 3
---

This third step allows to verify that the resulting infrastructure works correctly.
This is done by showing the deployment of a small *Hello World*  service in presence of two peered clusters (*home* and *foreign*).
This demonstrates the capability of Liqo to start a pod either in the local (*home*) or remote (*foreign*) cluster, transparently, without any change in the user experience.

## Start Hello World pod

To schedule a Pod in a foreign cluster, first you have to set the `KUBECONFIG` variable in the home cluster, such as in the following:

```shell script
export KUBECONFIG=home-kubeconfig.yaml
```

Now you have to create a namespace where your pod will be started and label it as ```liqo.io/enabled=true```. This label will tell the Kubernetes scheduler that the namespace spans across the foreign clusters as well; hence, pods started in the above namespaces are suitable for being executed on the foreign cluster.

```
kubectl create ns test-liqo
kubectl label ns test-liqo liqo.io/enabled=true
```

Then, you can deploy the test pod in the `test-liqo` namespace:

```
kubectl apply -f https://raw.githubusercontent.com/LiqoTech/liqo/master/docs/examples/hello-world.yaml -n test-liqo
```
The `hello-world.yaml` file is a simple `nginx` service; it is composed by a simple pod running a nginx image, and a service exposing the pod to the cluster. We labeled the pod to be executed on the virtual-node just created.

{{%expand "Expand here for a more advanced explanation of what is happened under the hoods:" %}}

where your `hello-world.yaml` looks the following:
{{% render-code file="static/examples/hello-world.yaml" language="yaml" %}}


Differently from traditional examples, we added an (optional) `nodeSelector` field. This latter tells the Kubernetes scheduler that the pod has to be started on a virtual node. Virtual nodes are like traditional Kubernetes nodes, but they are labelled with `type: virtual-node`.

In case the above tag is missing, the Kubernetes scheduler will select the best hosting node based on the available resources, which can be either a node in the *home* cluster or in the *foreign* cluster.

{{% /expand%}}

Now you can check the state of your pod; the output confirms that the pod is running on a virtual node (i.e. a node whose name that starts with `vk`, i.e. *virtual kubelet*):

```
kubectl get po -o wide -n test
NAME    READY   STATUS    RESTARTS   AGE   IP           NODE                                      NOMINATED NODE   READINESS GATES
nginx   1/1     Running   0          41m   10.45.0.12   liqo-1dfa22f9-1cdd-4401-9e7a-c5342ec90059   <none>           <none>
```

## Check pod connectivity 

If you have direct connectivity with the cluster (e.g. K3s) from your host:

```bash
POD_HOST=$(kubectl get pod nginx -n test-liqo --template={{.status.podIP}})
echo $POD_HOST
```
Open a browser and connect to the value of `$POD_HOST` or use the following `curl` command:

```
curl -v $POD_HOST
```

If you have not direct connectivity, you can fire up a pod and run a curl inside:

```
kubectl run --image=curlimages/curl tester -n default -ti --rm -- curl -L $POD_HOST
```

## Service

The above `hello-world.yaml` tells Kubernetes to create also a service, which is designed to serve traffic to the previously deployed pod.
The service is a traditional [Kubernetes Service](https://kubernetes.io/docs/concepts/services-networking/service/) and can work with Liqo with no modifications.

This can be seen by inspecting the service and its endpoints:

```kubectl get endpoints -n test-liqo```

You should see the pod offloaded to the remote cluster listed as service endpoint.

<!-- TODO: report the output of this command -->


### Check Service connectivity 

If you have direct connectivity with the cluster (e.g. K3s) from your host:

```bash
SVC_IP=$(kubectl get svc -n test-liqo --template={{.spec.clusterIP}})
echo $SVC_IP
```

<!-- TODO:  add a 'curl' command or something like that that shows that the pod returns the expected page. Show that the command is exactly the same in either cases (i.e., when the pod is local, when the pod is remote) -->






