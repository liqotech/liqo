---
title: Dynamic topology
weight: 8
---

Liqo allows you to select dynamically the clusters eligible for a specific namespace. 
If you have specified certain characteristics, all the clusters that match them will be automatically added as candidate to receive certain pods.
Similarly, if one cluster leaves the topology, the workload will be redistributed among the remaining clusters, by destroying and recreating the pods elsewhere.
If new clusters are peered or unpeered at runtime, you do not have to take care to configure the topology or terminate the offloading: Liqo ensures the convergence to the new set-up automatically. 

In this step, you will try to disable the peering with a cluster that has at least one pod inside.
This will show you that a new replacing pod is correctly rescheduled inside another available cluster.

### Disable a peering

First, you have to check on which remote clusters your pods have been scheduled:

```bash
export KUBECONFIG=$KUBECONFIG_1
kubectl get pods -n liqo-test -o wide
```

```bash
NAME                               READY   STATUS    RESTARTS   IP            NODE                                        
nginx-deployment-5c97c84f6-5p47g   1/1     Running   0          10.204.0.13   liqo-b07938e3-d241-460c-a77b-e286c0f733c7   
nginx-deployment-5c97c84f6-8h58s   1/1     Running   0          10.202.0.12   liqo-b38f5c32-a877-4f82-8bde-2fd0c5c8f862   
nginx-deployment-5c97c84f6-cf8qc   1/1     Running   0          10.204.0.14   liqo-b07938e3-d241-460c-a77b-e286c0f733c7   
```

Given that, in this case, 2 pods are scheduled in *cluster-3*, we can disable the peering with that cluster to test what happens to our pods and services when we dynamically change the topology.

{{% notice tip %}}
If all your pods have been scheduled in *cluster-2*, you have to disable that peering using the env variable `$REMOTE_CLUSTER_ID_2` instead of `$REMOTE_CLUSTER_ID_3`.
{{% /notice %}}

You can now disable the peering with the *cluster-3*.

```bash
export KUBECONFIG=$KUBECONFIG_1
kubectl patch foreignclusters $REMOTE_CLUSTER_ID_3 \
--patch '{"spec":{"outgoingPeeringEnabled":"No"}}' \
--type 'merge'
```

The remote namespace in the cluster-3 is teared down.
After a couple of seconds, you should see all the three pods scheduled in the remaining cluster (*cluster-2* in this example):

```bash
export KUBECONFIG=$KUBECONFIG_2
kubectl get pods -n liqo-test 
```

The output will be something like:

```bash
NAME                                     READY   STATUS    
nginx-deployment-5c97c84f6-8h58s-58w27   1/1     Running   
nginx-deployment-5c97c84f6-hvfb9-6tqll   1/1     Running   
nginx-deployment-5c97c84f6-rb4rz-4b5qd   1/1     Running
```

### Resume the peering 

You can now try to resume the previous peering:

```bash
export KUBECONFIG=$KUBECONFIG_1
kubectl patch foreignclusters $REMOTE_CLUSTER_ID_3 \
--patch '{"spec":{"outgoingPeeringEnabled":"Yes"}}' \
--type 'merge'
```

The namespaces created on the newly peered cluster is immediately updated and *cluster-3* is re-added to the multi-cluster topology.
A remote namespace is correctly re-created in *cluster-3*:

```bash
export KUBECONFIG=$KUBECONFIG_3
kubectl get namespace liqo-test 
```

```bash
NAME        STATUS   AGE
liqo-test   Active   40s
```

### Clean up the environment
 
To clean up your environment, you have to execute the following steps:

1. Delete the deployed pods and the service:
   ```bash
   export KUBECONFIG=$KUBECONFIG_1
   kubectl delete deployment nginx-deployment -n liqo-test
   ```
2. Delete the namespaceOffloading resource and the associated Liqo namespace:

   ```bash
   kubectl delete namespaceoffloading offloading -n liqo-test
   kubectl delete namespace liqo-test
   ```

3. Disable the two (unidirectional) peerings to get ready to [uninstall Liqo](../uninstall): 

   ```bash
   kubectl patch foreignclusters $REMOTE_CLUSTER_ID_2 \
   --patch '{"spec":{"outgoingPeeringEnabled":"No"}}' \
   --type 'merge'
   ```

   ```bash
   kubectl patch foreignclusters $REMOTE_CLUSTER_ID_3 \
   --patch '{"spec":{"outgoingPeeringEnabled":"No"}}' \
   --type 'merge'
   ```