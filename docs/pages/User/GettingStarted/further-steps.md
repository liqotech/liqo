---
title: Deploy a sample application
weight: 4
---

So far, we tested Liqo with a simple nginx container, but Liqo can be used with more complex microservices.

##  Deploy a micro-service application with micro-services application

We can test Liqo using a sample microservices application provided by [Google](https://github.com/GoogleCloudPlatform/microservices-demo):

```
kubectl apply -f https://raw.githubusercontent.com/LiqoTech/microservices-demo/master/release/kubernetes-manifests.yaml -n liqo-demo
```
Now, you can play with the commands presented in the [test](test) section in order to force the scheduling of the different pods in the local/remote cluster, and see that everything works smoothly.
Your scheduler will decide for each pod which is the best destination for each pod. Each deployed pod will be exposed as 
a service and a front-end application. Liqo implements a replication model of K8s services where each micro-service deployed across clusters can reach the others seamlessly. Independently from the cluster a pod is deployed, each pod is able to use another service via traditional K8s discovery mechanisms (e.g. DNS, Environment variables).

Several other objects (i.e. configmap and secrets) inside a namesapce are copied to the remote cluster inside the "virtual twin" namespace.  

## Observe Application deployment


##  Test the application

### Node Port
In order to access the dashboard you need to first get the port on which LiqoDash is exposed, which can be done with the following command:
```
kubectl -n liqo-demo get service frontend-external 
```
Which will output:
```
Type:          NodePort
NodePort:      https  32421/TCP
```
In this case, the dashboard has been exposed to the port ``32421``
Now, you can access LiqoDash using your master node IP and the service port you just found: ``https://<MASTER_IP>:<LIQODASH_PORT>``

**NOTE: to get your master node IP, you can run ``kubectl get nodes -o wide | grep master``, and take the
``INTERNAL-IP``**

### Port-Forward
A simple way to access the dashboard with a simpler URL than the one specified above is to use ``kubectl port-forward``.
```
kubectl port-forward -n liqo service/liqo-dashboard 6443:443
```
To access LiqoDash you can go to:
```
https://localhost:6443
```