---
title: Usage 
weight: 1
---

In this short tutorial, we present a small hello world deployment in presence of a home cluster and a foreign
cluster. The objective of this tutorial is to see how Liqo is able to 

## Hello World!

To schedule a Pod from your home cluster to another, first you have to set the home KUBECONFIG to deploy 

Example:

```shell script
export KUBECONFIG=home-kubeconfig.yaml
```

Moreover, you can create a namespace and label it as ```liqo.io/enabled=true```. 

```
kubectl create ns test-liqo
kubectl label ns test-liqo liqo.io/enabled=true
```

Then, you can deploy the test pod:

```
kubectl apply -f https://raw.githubusercontent.com/LiqoTech/liqo/master/docs/examples/hello-world.yaml -n test-liqo
```

Adding the label to the namespace will allow your pods to tolerate the virtual-node taint, by making them schedulable on the virtual node.

Checking the state of your pod, you can observe that it is scheduled and running on a virtual node:

```
kubectl get po -o wide -n test
NAME    READY   STATUS    RESTARTS   AGE   IP           NODE                                      NOMINATED NODE   READINESS GATES
nginx   1/1     Running   0          41m   10.45.0.12   vk-1dfa22f9-1cdd-4401-9e7a-c5342ec90059   <none>           <none>
```
### Check pod connectivity 

If you have direct connectivity with the cluster (e.g. K3s) from your host:

```bash
POD_HOST=$(kubectl get pod nginx -n test-liqo --template={{.status.podIP}})
echo $POD_HOST
```
Open a browser and connect to the value of $POD_HOST or use a curl command

```
curl -v $POD_HOST
```

### Under the hoods

For sake of completeness, the spec of the deployed pod is the following:

```
apiVersion: v1
kind: Pod
metadata:
  name: nginx
  labels:
    app: test-liqo
spec:
  containers:
  - name: nginx
    image: nginxdemos/hello
    imagePullPolicy: IfNotPresent
    ports:
      - containerPort: 80
        name: web
  nodeSelector:
    type: virtual-node
```

It is typical nginx pod with a node selector in addition. In this case, the node selector would require the pod will be 
scheduled on a virtual node.

## Service

The previous "apply" also created a service, which is designed to serve traffic to the previously deployed pod.
The service is a traditional [Kubernetes Service](https://kubernetes.io/docs/concepts/services-networking/service/) and 
can work with Liqo with no modifications.

This can be seen by inspecting the service and its endpoints:

```kubectl get endpoints -n test-liqo```

You should see the pod offloaded to the remote cluster listed as service endpoint.

### Check Service connectivity 

If you have direct connectivity with the cluster (e.g. K3s) from your host:

```bash
SVC_IP=$(kubectl get svc -n test-liqo --template={{.spec.clusterIP}})
echo $SVC_IP
```





