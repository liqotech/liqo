---
title: Namespace Offloading
weight: 3
---

### What resources should you check if there are problems?

### NamespaceOffloading resource

#### Remote namespace conditions field

If you want more detailed information about the offloading status, you can check the *remoteNamespaceConditions* field of the *namespaceOffloading* resource.
The *namespaceOffloading* is inside your local namespace.
Enter the namespace name in the following variable:

```bash
NAMESPACE=<your-namespace>
echo "Namespace name: $NAMESPACE"
```

```bash
kubectl get namespaceoffloadings offloading -n $NAMESPACE -o yaml
```

The *remoteNamespaceConditions* field is a map:

* The key is the *remote cluster-id*.
* The value is a *vector of conditions* for the namespace created inside that remote cluster.

There are two types of *conditions*:

**Ready condition**

   | Value     | Description |
   | -------   | ----------- |
   | **True**  |  The namespace is successfully created inside the remote cluster. |
   | **False** |  There was a problem during the remote namespace creation. |

**OffloadingRequired condition**

   | Value   | Description |
   | ------- | ----------- |
   | **True**  |  The remote namespace is requested to be created inside this cluster (the condition `OffloadingRequired = true` is removed when the remote namespace acquires a `Ready` condition). |
   | **False** |  The remote namespace creation inside this cluster is not required. |

{{% notice note %}}
The *remoteNamespaceCondition* syntax is the same of the standard [NamespaceCondition](https://pkg.go.dev/k8s.io/api/core/v1@v0.21.0#NamespaceCondition).
{{% /notice %}}

#### OffloadingPhase field

Another important field of the *namespaceOffloading* status is the *offloadingPhase*.
It informs you about the global offloading status.

The possible error values are:

   | Value   | Description |
   | ------- | ----------- |
   | **AllFailed**  |  There was a problem during the creation of all remote namespaces. |
   | **SomeFailed** |  There was a problem during the creation of some remote namespaces (one or more). |

To have a more detailed description of the other values that the *offloadingPhase* can assume, look at the [NamespaceOffloading Status section](#).

If instead you have selected some clusters, but the *offloadingPhase* is "**NoClusterSelected** ” then there may be an error in the specified *clusterSelector* syntax. 
In this case, the *namespaceOffloading* resource should expose an annotation with key `liqo.io/scheduling-enabled` that signals the syntax error:

```bash
kubectl get namespaceoffloadings -n $NAMESPACE -o yaml | grep "liqo.io/scheduling-enabled:" -A 1
```

The *offloadingPhase* takes a few seconds to be updated, so initially, the value may not match the expected one.

If, after a couple of seconds, the resource status is not correctly updated, then you can continue your troubleshooting.

### NamespaceMap resource

The *remoteNamespacesCondition* field seen above is a map that has "*remote cluster-id* " as its keys. 
You have to:

1. Identify the remote cluster that has problems.
2. Get its cluster-id from the *remoteNamespacesCondition*.
3. Enter the cluster-id in the following variable.
      ```bash
      REMOTE_CLUSTER_ID=<your-remote-cluster-id>
      echo "Remote cluster-id: $REMOTE_CLUSTER_ID"
      ```

For each remote cluster, there is a "*Liqo Tenant namespace*" containing several Liqo CRDs.
We are interested in the resource called *NamespaceMap*.
This resource keeps track of:

* All remote namespace creation requests (in the map “*Spec.DesiredMapping* ”).
* The actual status of the remote namespaces (in the map “*Status.CurrentMapping* ”).

Thanks to the remote cluster-id set previously, you just need to run the following commands to observe the correct NamespaceMap:

```bash
LIQO_TENANT_NAMESPACE=$(kubectl get namespace --show-labels | grep $REMOTE_CLUSTER_ID | cut -d " " -f1)
echo "Liqo tenant namespace name: $LIQO_TENANT_NAMESPACE"
NAMESPACEMAP=$(kubectl get namespacemaps -n $LIQO_TENANT_NAMESPACE | grep $REMOTE_CLUSTER_ID | cut -d " " -f1)
echo "NamespaceMap name: $NAMESPACEMAP"
kubectl get namespacemaps -n $LIQO_TENANT_NAMESPACE $NAMESPACEMAP -o yaml
```

#### DesiredMapping map

In the *Spec.DesiredMapping* map there should be an entry like this:

```yaml
spec:
  desiredMapping:
    ...
    LocalNamespaceName: remoteNamespaceName
    ...
```

If you have trouble locating the entry use the following command:

```bash
kubectl get namespacemaps -n $LIQO_TENANT_NAMESPACE $NAMESPACEMAP -o=jsonpath="{['spec.desiredMapping.$NAMESPACE']} "
```

If the result obtained is the remote namespace name, then the entry is present, and the creation request has been stored correctly.

#### CurrentMapping map

In the *Status.CurrentMapping* map there should be an entry like this:

```yaml
spec:
  currentMapping:
    ...
    LocalNamespaceName: 
      phase: CreationLoopBackoff
      remoteNamespace: remoteNamespaceName
    ...
```

If you have trouble locating the entry use the following command:

```bash
kubectl get namespacemaps -n $LIQO_TENANT_NAMESPACE $NAMESPACEMAP -o=jsonpath="{['status.currentMapping.$NAMESPACE.phase']} "
```

As there is an error, the *phase* value should be "*CreationLoopBackoff* ”. 
If this is the case, there could be two main reasons:

* The name of your local namespace is too long, and with the addition of the cluster-id it exceeds the 63 characters limit imposed by the [RFC 1123](https://datatracker.ietf.org/doc/html/rfc1123). 
* A remote namespace with that name already exists inside the remote cluster. 
  To undestrand if the remote namespace was created by Liqo controllers, check the presence of the `liqo.io/remote-namespace` annotation.
  This should have has as its value the local cluster-id:
  
  ```bash
  kubectl get namespace <your-remote-namespace> -o yaml | grep "liqo.io/remote-namespace:"
  ```
