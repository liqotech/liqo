---
title: Selective Offloading 
weight: 4
---

Imagine to have some requirements:

"*you want deployments spanning across a multi-cluster architecture but using only local resources and, the ones offered by **provider-3** managed clusters*".

Liqo can help you manage such a scenario by simply creating a *[Liqo namespace](/usage/namespace_offloading#introduction)*. 

### Create the Liqo namespace

As you may remember, a *Liqo namespace* is composed of: 

* a local Namespace.  
* a NamespaceOffloading resource containing the desired configuration.

1. Create the local namespace called "*liqo-test* ”:

```bash
export KUBECONFIG=$KUBECONFIG_1
kubectl create namespace liqo-test
```

2. Now create the NamespaceOffloading resource inside the namespace:

```yaml
cat << "EOF" | kubectl apply -f -
apiVersion: offloading.liqo.io/v1alpha1
kind: NamespaceOffloading
metadata:
  name: offloading
  namespace: liqo-test
spec:
  clusterSelector:
    nodeSelectorTerms:
      - matchExpressions:
        - key: liqo.io/provider
          operator: In
          values:
          - provider-3
EOF
```

You do not have to specify the *PodOffloadingStrategy* and *NamespaceMappingStrategy* fields at the resource creation. 
[The default values](/usage/namespace_offloading#selecting-the-namespace-mapping-strategy), enforced automatically by Liqo controllers, match the previous requirements: "*pod could be deployed both locally and remotely, and the remote namespace name is the default one*".

The clusterSelector allows you to choose target clusters for your namespace offloading.
In this case, the chosen filter is `liqo.io/provider="provider-3`.
After the resource creation, your deployment topology is ready to be used.

### Check remote namespaces presence 

You can check if the topology just built is compliant with the requirements specified in the NamespaceOffloading object.
There should be a remote namespace only inside the *cluster-3*:

```bash
export KUBECONFIG=$KUBECONFIG_3
kubectl get namespaces 
```

{{% notice note %}}
The namespace name should be "*liqo-test-yourHomeClusterID*", due to the NamespaceMappingStrategy default value:
{{% /notice %}}

```bash
NAME                                              STATUS   
liqo-test-b5de574d-a0a6-4a2a-8bc8-ac8c726862c5    Active   
```

You can export this name as an environment variable:

```bash
REMOTE_NAMESPACE=$(kubectl get namespace | grep "liqo-test" | cut -d " " -f1)
echo $REMOTE_NAMESPACE
```

The *cluster-2* should not have any remote namespace with that name:

```bash
export KUBECONFIG=$KUBECONFIG_2
kubectl get namespaces $REMOTE_NAMESPACE
```

### Analyze the namespaceOffloading status

The offloading process was successful, so the NamespaceOffloading resource should have the *OffloadingPhase* equal to Ready and a vector of RemoteNamespaceConditions for each remote cluster.
In this case, there are two single-condition vectors:

* Vector with the condition *OffloadingRequired* set to False, for the cluster without the remote namespaces (*cluster-2*).
* Vector with the condition *Ready* set to True, for the cluster with the remote namespace (*cluster-3*).

```bash
export KUBECONFIG=$KUBECONFIG_1
kubectl get namespaceoffloadings offloading -n liqo-test -o yaml
```

```yaml
status:
   offloadingPhase: Ready
   remoteNamespaceName: liqo-test-b5de574d-a0a6-4a2a-8bc8-ac8c726862c5
   remoteNamespacesConditions:
      
      b38f5c32-a877-4f82-8bde-2fd0c5c8f862:     <========== 1° vector
         - lastTransitionTime: "2021-07-31T10:00:53Z"
           message: You have not selected this cluster through ClusterSelector fields
           reason: ClusterNotSelected
           status: "False"
           type: OffloadingRequired             <========== OffloadingRequired condition
      
      b07938e3-d241-460c-a77b-e286c0f733c7:     <========== 2° vector
         - lastTransitionTime: "2021-07-31T10:01:03Z"
           message: Namespace correctly offloaded on this cluster
           reason: RemoteNamespaceCreated
           status: "True"
           type: Ready                          <========== Ready condition
```

All the possible values of RemoteNamespaceConditions are described in the [section about NamespaceOffloading status](/usage/namespace_offloading#remotenamespacesconditions)

Now that you have checked the topology, you can try to deploy a simple application inside it.
During the deployment, you can test the PodOffloadingStrategy enforcement and how the [violation of the offloading constraints](../hard_constraints) is managed.

