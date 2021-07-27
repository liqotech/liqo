---
title: Hard constraints
weight: 5
---

The constraints enforced in the NamespaceOffloading resource must always be respected.
Liqo can detect attempts to breach them, and react preventing the pod scheduling.
You can test it with a simple deployment.

### Enforce a constraints violation

The constraints specified for the topology were:

"*deploy pods both locally and remotely but use only remote cluster managed by the **provider-3***".

You can check their enforcement deploying two pods:

* The "*pod-provider-3*" compliant with the requirements.
* The "*pod-provider-2*" in contrast with them.
 
{{% notice note %}}
For the sake of clarity, we use pods instead of deployments, but the behavior would be the same.
{{% /notice %}}

```yaml
cat << "EOF" | kubectl apply -f - -n liqo-namespace
apiVersion: v1
kind: Pod
metadata:
  name: pod-provider-3
  labels:
    app: provider-3
spec:
  containers:
    - name: nginx
      image: nginxdemos/hello
      imagePullPolicy: IfNotPresent
      ports:
        - containerPort: 80
          name: web
  affinity:
    nodeAffinity:
      requiredDuringSchedulingIgnoredDuringExecution:
        nodeSelectorTerms:
          - matchExpressions:
              - key: liqo.io/provider
                operator: In
                values:
                  - provider-3
---
apiVersion: v1
kind: Pod
metadata:
  name: pod-provider-2
  labels:
    app: provider-2
spec:
  containers:
    - name: nginx
      image: nginxdemos/hello
      imagePullPolicy: IfNotPresent
      ports:
        - containerPort: 80
          name: web
  affinity:
    nodeAffinity:
      requiredDuringSchedulingIgnoredDuringExecution:
        nodeSelectorTerms:
          - matchExpressions:
              - key: liqo.io/provider
                operator: In
                values:
                  - provider-2
EOF
```

The NodeSelectorTerm forces the "*pod-provider-2*" scheduling inside a cluster managed by *provider-2*.
The Liqo webhook denies this scheduling enforcing on the pod the NamespaceOffloading constraints.
These are the resulting NodeSelectorTerms:

```bash
kubectl get pod -n liqo-test pod-provider-2 -o yaml | grep "spec:" -A 22
```

```yaml
spec:
  affinity:
    nodeAffinity:
      requiredDuringSchedulingIgnoredDuringExecution:
        nodeSelectorTerms:
          - matchExpressions:
              - key: liqo.io/provider
                operator: In
                values:
                  - provider-3
              - key: liqo.io/provider
                operator: In
                values:
                  - provider-2
          - matchExpressions:
              - key: liqo.io/type
                operator: NotIn
                values:
                  - virtual-node
              - key: liqo.io/provider
                operator: In
                values:
                  - provider-2
```

* The first MatchExpression is always False: it is impossible to have two different values for the same label. 
  The cluster could not be managed both by provider-2 and provider-3.

* The second one is usually False: the pod could be scheduled locally if a node of your cluster exposes the label `liqo.io/provider=provider-2`. 
  This scenario would be rather unusual since this label takes on the meaning of a "*cluster label*" rather than a standard Kubernetes node label.

So at the most, the pod could be scheduled locally because the admin has left this possibility specifying the requirements.
The important thing is that pods could never be scheduled inside a remote cluster that is not allowed.

You can check that this pod is *pending*:

```bash
kubectl get pod -n liqo-test pod-provider-2
```
```bash
NAME             READY   STATUS    
pod-provider-2   0/1     Pending   
```
  
The "*pod-provider-3*" is compliant to the offloading constraints, so it should be correctly scheduled inside the *cluster-3*: 

```bash
export KUBECONFIG=$KUBECONFIG_3
kubectl get pods -n $REMOTE_NAMESPACE
```

```bash
NAME                         READY   STATUS       
pod-provider-3-v4lk5         1/1     Running             
```

### Clean the environment

You can now delete the deployed pods: 

```bash
kubectl delete pod pod-provider-2 -n liqo-test
kubectl delete pod pod-provider-3 -n liqo-test
```

In the next section, you will experiment with the simplicity of [changing the deployment topology](../change_topology) without deleting the local namespace.



