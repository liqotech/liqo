---
title: Change topology
weight: 6
---

Liqo allows you to change your deployment topology without any effort. 
Only two straightforward steps are required:

1. Remove the old configuration in the *Liqo namespace*, so the old NamespaceOffloading resource.
2. Create the new resource with a new configuration.

### Remove the old resource

First, you can remove the old resource:

```bash
export KUBECONFIG=$KUBECONFIG_1
kubectl delete namespaceoffloadings offloading -n liqo-test  
```

{{% notice warning %}}
Deleting the NamespaceOffloading object, you will delete all the remote namespaces previously created and their content.
You have to be sure that everything you deployed remotely is no longer needed.
{{% /notice %}}

You can check that the remote namespace is correctly deleted from the *cluster-3*:

```bash
export KUBECONFIG=$KUBECONFIG_3
kubectl get namespaces 
```

### Create the new resource

Now you can imagine creating a new configuration:

* All remote clusters selected. 
* Pods could be deployed only remotely.
* The remote namespaces must have the same name as the local one.

The NamespaceOffloading resource will be like this:

```yaml
export KUBECONFIG=$KUBECONFIG_1
cat << "EOF" | kubectl apply -f - -n liqo-test
apiVersion: offloading.liqo.io/v1alpha1
kind: NamespaceOffloading
metadata:
  name: offloading
  namespace: liqo-test
spec:
  namespaceMappingStrategy: EnforceSameName
  podOffloadingStrategy: Remote  
EOF
```

It is not necessary to specify a ClusterSelector field to select all available clusters. 
This is the standard behavior of its [default value](/usage/namespace_offloading/#selecting-the-remote-clusters)

There should be two remote namespaces: one inside the *cluster-2* and the other inside *cluster-3*:

```bash
export KUBECONFIG=$KUBECONFIG_2
kubectl get namespaces liqo-test
```
```bash
export KUBECONFIG=$KUBECONFIG_3
kubectl get namespaces liqo-test
```

Once the new topology has been created, you can find out [how to contact remote pods from the home-cluster](../remote_service_access)
