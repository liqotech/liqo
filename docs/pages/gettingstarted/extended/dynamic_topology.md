---
title: Dynamic topology
weight: 8
---

Liqo builds deployment topologies extremely dynamic. 
If you have specified certain characteristics, all the clusters that match them will be automatically added to the topology.
Similarly, if one cluster leaves the topology, the workload will be redistributed among the remaining clusters, obtaining a balanced solution.
You do not have to reconfigure the topology or terminate the offloading Liqo manages the topology reconciliation for you. 

You can try to disable the peering with a cluster that has at least one pod inside to see if it is correctly rescheduled inside another available cluster.

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

In this case, there are 2 pods scheduled inside the *cluster-3* so you can disable the peering with the *cluster-3*.

{{% notice tip %}}
If all your pods have been scheduled inside the *cluster-2*, you have to disable the peering with it using the env variable `$REMOTE_CLUSTER_ID_2` instead of `$REMOTE_CLUSTER_ID_3`.
{{% /notice %}}

You can now disable the peering with the *cluster-3*.

```bash
export KUBECONFIG=$KUBECONFIG_1
kubectl patch foreignclusters $REMOTE_CLUSTER_ID_3 \
--patch '{"spec":{"outgoingPeeringEnabled":"No"}}' \
--type 'merge'
```

The remote namespace inside the cluster-3 is destroyed, and after a couple of seconds, you should see all the three pods scheduled inside the last remaining cluster (in this case the *cluster-2*):

```bash
export KUBECONFIG=$KUBECONFIG_2
kubectl get pods -n liqo-test 
```

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

The topology is immediately updated. A remote namespace is correctly regenerated inside the *cluster-3*:

```bash
export KUBECONFIG=$KUBECONFIG_3
kubectl get namespace liqo-test 
```

### Clean up the environment
 
To clean up your environment, you have to execute the following step:

1. Delete the deployed pods and the service:
   ```bash
   export KUBECONFIG=$KUBECONFIG_1
   kubectl delete deployment nginx-deployment -n liqo-test
   ```
2. Delete the deployment topology removing the Liqo namespace:

   ```bash
   kubectl delete namespaceoffloading offloading -n liqo-test
   kubectl delete namespace liqo-test
   ```

3. Disable the two unidirectional peerings to get ready to [uninstall Liqo](../uninstall): 

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