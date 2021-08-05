---
title: Namespace Offloading
weight: 3
---

### What resources should you check if there are problems?

#### NamespaceOffloading resource
### Remote namespace conditions

If you want more detailed information about the offloading status, you can check the **remoteNamespaceConditions**
inside the NamespaceOffloading resource:

```bash
   kubectl get namespaceoffloading offloading -n test-namespace -o yaml
```

The **remoteNamespaceConditions** field is a map which has as its key the ***remote cluster-id*** and as its value
a ***vector of conditions for the namespace*** created inside that remote cluster. There are two types of conditions:

#### Ready field

   | Value   | Description |
   | ------- | ----------- |
   | **True**  |  The remote namespace is successfully created. |
   | **False** |  There was a problems during the remote namespace creation. |

#### OffloadingRequired field

   | Value   | Description |
   | ------- | ----------- |
   | **True**  |  The creation of a remote namespace inside this cluster is required (the condition ***OffloadingRequired = true*** is removed when the remote namespace acquires a ***Ready*** condition). |
   | **False** |  The creation of a remote namespace inside this cluster is not required. |

> __NOTE__: The **RemoteNamespaceCondition** syntax is the same of the standard [NamespaceCondition](https://pkg.go.dev/k8s.io/api/core/v1@v0.21.0#NamespaceCondition).

A **NamespaceOffloading** CRD is created in your local namespace (“***Liqo namespace***”).
Enter the Liqo namespace name in the following variable:

```bash
   NAMESPACE=<your-namespace>
   echo "Liqo namespace name: $NAMESPACE"
```
The resource status provides information about possible problems.

```bash
  kubectl get namespaceoffloading offloading -n $NAMESPACE -o yaml
```
There are 2 main status fields:

| Field                          | Description |
| --------------                 | ----------- |
| **offloadingPhase**            | Warns you if there is a problem during remote namespaces creation. (Error values: “***SomeFailed*** ”, “***AllFailed*** ”) |
| **remoteNamespacesConditions** | Shows you on which clusters there was a problem during namespace creation and why. (Condition **Ready** = “***False***”) |

Look at the [NamespaceOffloading Status section](#) to observe in detail the values 
that these 2 fields can assume.

If you have selected clusters, but the **offloadingPhase** is "***NoClusterSelected*** ” then there may be an error
in the specified **clusterSelector** syntax. In this case the NamespaceOffloading should have an annotation with 
the key "***liqo.io/scheduling-enabled*** ”, which signals the syntax error:

```bash
   kubectl get namespaceoffloadings.offloading.liqo.io -n $NAMESPACE -o yaml | grep "liqo.io/scheduling-enabled:" -A 1
```

The **offloadingPhase** takes a few seconds to be updated, so initially the value may not match the expected one.

If the resource status is not correctly updated, then you can continue your troubleshooting.

#### NamespaceMap resource

The **remoteNamespacesCondition** field seen above is a map that has remote cluster-id as its keys. You have to:

1. Identify the remote cluster that has problems.
2. Get its cluster-id from the **remoteNamespacesCondition**.
3. Enter the cluster-id in the following variable. 
  ```bash
    REMOTE_CLUSTER_ID=<your-remote-cluster-id>
    echo "Remote cluster-id: $REMOTE_CLUSTER_ID"
  ```

For each remote cluster there is a "***Liqo Tenant namespace***" containing several Liqo CRDs.
We are interested in the resource called **NamespaceMap**.
This resource keeps track of all remote namespace creation requests (map “***Spec.DesiredMapping*** ”),
and the actual status of the remote namespaces (map “***Status.CurrentMapping*** ”).
Thanks to the remote cluster-id set previously, you just need to run the following commands 
to observe the correct NamespaceMap:

```bash
  LIQO_TENANT_NAMESPACE=$(kubectl get namespace --show-labels | grep $REMOTE_CLUSTER_ID | cut -d " " -f1)
  echo "Liqo tenant namespace name: $LIQO_TENANT_NAMESPACE"
  NAMESPACEMAP=$(kubectl get namespacemaps.virtualkubelet.liqo.io -n $LIQO_TENANT_NAMESPACE | grep $REMOTE_CLUSTER_ID | cut -d " " -f1)
  echo "NamespaceMap name: $NAMESPACEMAP"
  kubectl get namespacemaps.virtualkubelet.liqo.io -n $LIQO_TENANT_NAMESPACE $NAMESPACEMAP -o yaml
```

In the **Spec.DesiredMapping** map there should be an entry that has as its key the name of the local namespace
and as its value the name chosen for the remote namespace.
If you have trouble locating the entry use the following command:

```bash
  kubectl get namespacemaps.virtualkubelet.liqo.io -n $LIQO_TENANT_NAMESPACE $NAMESPACEMAP -o=jsonpath="{['spec.desiredMapping.$NAMESPACE']} "
```

If the result obtained is the remote namespace name then the entry is present, and the creation 
request has been stored correctly.

In the **Status.CurrentMapping** map there should be an entry that has as its key the name of the local namespace
and as its value the name and the status of the remote namespace.
As there is an error, the status value should be "***CreationLoopBackoff*** ”: 

```bash
  kubectl get namespacemaps.virtualkubelet.liqo.io -n $LIQO_TENANT_NAMESPACE $NAMESPACEMAP -o=jsonpath="{['status.currentMapping.$NAMESPACE.phase']} "
```

If the result obtained is actually "***CreationLoopBackoff*** ” there could be two main reasons:

- The name of your local namespace is too long and with the addition of the cluster-id it exceeds the 
  63 characters limit imposed by the [RFC 1123](https://datatracker.ietf.org/doc/html/rfc1123)., 
  to have more explanations see [here](#).
- A remote namespace with that name already exists inside the remote cluster. If the remote namespace was created 
  by Liqo it will have an annotation with key "***liqo.io/remote-namespace***" and as its value the local cluster-id:
  
  ```bash
    kubectl get namespace <your-remote-namespace> -o yaml | grep "liqo.io/remote-namespace:"
  ```
