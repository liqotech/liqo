---
title: Deploy a complex application
weight: 4
---

So far, you tested Liqo with a simple `nginx` application, but Liqo can be used with more complex micro-services.

##  Deploy a micro-service application with micro-services application

For a complete demo of the capabilities of Liqo, we can play with a micro-services application provided by [Google](https://github.com/GoogleCloudPlatform/microservices-demo), which include multiple cooperating services:

```
kubectl apply -f https://raw.githubusercontent.com/liqotech/microservices-demo/master/release/kubernetes-manifests.yaml -n liqo-demo
```

By default, Kubernetes schedules each pod either in the local or in the remote cluster, optimizing each deployment based on the available resources.
However, you can play with *affinity* constraints as presented in the [*exploit foreign resources*](../test) section to force the scheduling of each component in a specific location, and see that everything continues to work smoothly.

Each demo component is exposed as a service and accessed by other components.
However, given that nobody knows, a priori, where each service will be deployed (either locally or in the remote cluster), Liqo _replicates_ all Kubernetes services across both clusters, although the corresponding pod may be running only in one location.
Hence, each micro-service deployed across clusters can reach the others seamlessly: independently from the cluster a pod is deployed in, each pod is able to contact other services and leverage the traditional Kubernetes discovery mechanisms (e.g. DNS, Environment variables).

Additionally, several other objects (e.g. `configmap` and `secrets`) inside a namespace are replicated in the remote cluster, within the "virtual twin" namespace, thus, ensuring that complex applications can work seamlessly across clusters.

## Observe the application deployment

Once the demo application manifest is applied, you can observe the creation of the different pods:

```
watch kubectl get pods -n liqo-demo
```

At steady state, you should see an output similar to the following.
Yet, different pods may be hosted by either the local (whose name start with _worker_ in the example below) or remote cluster (whose name start with _liqo_ in the example below), depending on the scheduling decisions.
```
NAME                                     READY   STATUS    RESTARTS   AGE   IP               NODE
adservice-5c9c7c997f-gmmdx               1/1     Running   0          12m   10.244.2.56      worker-node-2
cartservice-6d99678dd6-db6ns             1/1     Running   0          13m   172.16.97.199    liqo-9a596a4b-591c-4ac6-8fd6-80258b4b3bf9
checkoutservice-779cb9bfdf-h48tg         1/1     Running   0          13m   172.16.97.201    liqo-9a596a4b-591c-4ac6-8fd6-80258b4b3bf9
currencyservice-5db6c7d559-gb7ln         1/1     Running   0          12m   172.16.226.110   liqo-9a596a4b-591c-4ac6-8fd6-80258b4b3bf9
emailservice-5c47dc87bf-zzz4z            1/1     Running   0          13m   10.244.2.235     worker-node-2
frontend-5fcb8cdcdc-vvq4m                1/1     Running   0          13m   172.16.97.207    liqo-9a596a4b-591c-4ac6-8fd6-80258b4b3bf9
loadgenerator-79bff5bd57-t7976           1/1     Running   0          12m   172.16.97.208    liqo-9a596a4b-591c-4ac6-8fd6-80258b4b3bf9
paymentservice-6564cb7fb9-vn7pn          1/1     Running   0          13m   172.16.97.197    liqo-9a596a4b-591c-4ac6-8fd6-80258b4b3bf9
productcatalogservice-5db9444549-cxjlb   1/1     Running   0          13m   172.16.226.87    liqo-9a596a4b-591c-4ac6-8fd6-80258b4b3bf9
recommendationservice-78dd87ff95-2s8ks   1/1     Running   0          13m   10.244.2.241     worker-node-2
redis-cart-57bd646894-9x4cd              1/1     Running   0          12m   172.16.226.120   liqo-9a596a4b-591c-4ac6-8fd6-80258b4b3bf9
shippingservice-f47755f97-5jcpm          1/1     Running   0          12m   10.244.4.169     worker-node-1
```

## Access the demo application

Once the deployment is completed, you can start using the demo application and verify that everything works correctly even if its components are distributed across multiple Kubernetes clusters.
By default, the frontend web-page is exposed through a `LoadBalancer` service, which can be inspected using:
```bash
kubectl get service -n liqo-demo frontend-external
```

You should see an output similar to the following, indicating that you can access the demo application at `http://xxx.yyy.zzz.www`:
```
NAME                TYPE           CLUSTER-IP    EXTERNAL-IP       PORT(S)        AGE
frontend-external   LoadBalancer   10.96.8.220   xxx.yyy.zzz.www   80:32233/TCP   15m
```

In case the `EXTERNAL-IP` field is characterized by a `<pending>` value, on the other hand, you are probably using a bare metal cluster (i.e. not managed by an IaaS platform such as GCP, AWS, Azure, ...) and lacking a Load Balancer implementation.

Still, you have two alternatives to access the demo application:
1. Leverage the `NodePort` (e.g. 32233 in the previous example), which can be accessed using the IP address of any of the physical servers of your home cluster;
2. Leverage `kubectl port-forward` to forward the requests from your local machine (i.e. `http://localhost:8080`) to the frontend service:
   ```bash
   kubectl port-forward -n liqo-demo service/frontend-external 8080:80
   ```

> **Clean-up**: If you want to delete the deployed example, just issue:
> ```bash
> kubectl delete -f https://github.com/liqotech/microservices-demo/blob/master/release/kubernetes-manifests.yaml -n liqo-demo
> ```
