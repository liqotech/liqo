---
title: Scheduling Constraints 
weight: 5
---

In the last section, we enabled the NamespaceOffloading logic to extend a namespace across multiple clusters.
As we discussed, the constraints enforced in the NamespaceOffloading define the extreme "borders" that constrain the pod scheduling on the cluster matching the features specified in the *clusterSelector* of the NamespaceOfflloading resource.
In a nutshell, Liqo prevents the pod scheduling over clusters not selected in the cluster-selector.
In this section, you can test this mechanism with the help of a simple deployment.

### Enforce a constraints violation

At the beginning of the previous section, we described a scenario with the constraints specified for the multi-cluster topology:

"*deploy pods both locally and remotely but use only remote cluster managed by the **provider-3***".

Let us deploy a couple of pods in namespace `liqo-test`. To force the scheduling on the different nodes, we use a custom *NodeSelectorTerm* term.
More precisely, for the second pod, the *NodeSelectorTerm* forces the "*pod-provider-2*" scheduling inside a cluster managed by *provider-2*.

```yaml
cat << "EOF" | kubectl apply -f - -n liqo-test
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

{{% notice note %}}
For the sake of clarity, we use pods instead of deployments, but the behavior would be the same.
{{% /notice %}}

The expected result is that the first pod will be correctly scheduled on the node labeled *liqo.io/provider=provider-3* and the other one will remain in a "pending" state, since it does not match the namespace-wide constraint.

You can check the actual behavior by typing:

```
kubectl get pod -n liqo-test
```
### Clean the environment

You can now delete the deployed pods: 

```bash
kubectl delete pod pod-provider-2 -n liqo-test
kubectl delete pod pod-provider-3 -n liqo-test
```

In the next section, you will experiment with the simplicity of [changing the deployment topology](../change_topology) without deleting the local namespace.



