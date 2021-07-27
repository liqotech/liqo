---
title: Remote service access
weight: 7
---

If all pods run remotely, you can still access them through a standard Kubernetes service as if they were deployed locally.

### Create a new deployment

You can create a deployment with three pods and a service to expose them:

```yaml
export KUBECONFIG=$KUBECONFIG_1
cat << "EOF" | kubectl apply -f - -n liqo-test
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx-deployment
  labels:
    app: liqo-test
spec:
  replicas: 3
  selector:
    matchLabels:
      app: liqo-test
  template:
    metadata:
      labels:
        app: liqo-test
    spec:
      containers:
        - name: nginx
          image: nginxdemos/hello
          imagePullPolicy: IfNotPresent
          ports:
            - containerPort: 80
              name: web
---
apiVersion: v1
kind: Service
metadata:
  name: liqo-test-service
spec:
  ports:
    - name: web
      port: 80
      protocol: TCP
      targetPort: web
  selector:
    app: liqo-test
  type: ClusterIP
EOF
```

The three pods are scheduled remotely, but in this case, you do not know a priori inside which remote cluster they will be deployed.  
The scheduler makes the best choice according to its parameters. 
  
In the *home-cluster* you should have a situation like this:

```bash
export KUBECONFIG=$KUBECONFIG_1
kubectl get pods -n liqo-test -o wide
```

```bash
NAME                               READY   STATUS    RESTARTS   IP            NODE                                        
nginx-deployment-5c97c84f6-5p47g   1/1     Running   0          10.204.0.13   liqo-b07938e3-d241-460c-a77b-e286c0f733c7  (cluster-3) 
nginx-deployment-5c97c84f6-8h58s   1/1     Running   0          10.202.0.12   liqo-b38f5c32-a877-4f82-8bde-2fd0c5c8f862  (cluster-2) 
nginx-deployment-5c97c84f6-cf8qc   1/1     Running   0          10.204.0.14   liqo-b07938e3-d241-460c-a77b-e286c0f733c7  (cluster-3)
```
In this case, two pods run inside the *cluster-3* and just one inside the *cluster-2*. 

{{% notice tip %}}
These local pods are just "*shadow pods*". 
If you do not remember this Liqo concept, look at the [Hello World tutorial](#)
{{% /notice %}}

{{% notice note %}}
Your pods may have been scheduled differently, but at the moment, it is not relevant to know inside which of the two clusters they are. 
The only important thing is that they are remotely scheduled and ready.
{{% /notice %}}

### Test the service reachability

The three nginx pods are correctly specified as Endpoints of the local service:

```bash
kubectl describe service liqo-test-service -n liqo-test | grep "Endpoints"
```

```bash
Endpoints:    10.202.0.12:80,  10.204.0.13:80,  10.204.0.14:80
```
Liqo allows you to contact the service from the local cluster even if the endpoints are all deployed remotely.
First, you have to retrieve the IP address of the service:

```bash
export KUBECONFIG=$KUBECONFIG_1
SVC_IP=$(kubectl get service liqo-test-service -n liqo-test --template={{.spec.clusterIP}})
echo "Service IP: ${SVC_IP}"
```

After this you can fire up a pod and run curl from inside:

```bash
kubectl run --image=curlimages/curl curl -n default -it --rm --restart=Never -- curl --silent ${SVC_IP} | grep 'nginx-'
```

The hostname should alternate between the remote pods' names. 
This change confirms that Kubernetes correctly leverage all the three pods as back-ends (i.e., Endpoints) of the service.

The "liqo-test-service" is automatically replicated inside both the remote clusters:

```bash
export KUBECONFIG=$KUBECONFIG_2
kubectl get service liqo-test -n liqo-test
```

```bash
export KUBECONFIG=$KUBECONFIG_3
kubectl get service liqo-test -n liqo-test
```
So also from here is possible to contact the endpoints.

Liqo deployment topologies can immediately react to external changes. 
When you enable or disable a new peering, the topology evolves consequentially.
The next section figure out this concept of [dynamism](../dynamic_topology)
