---
title: Advertisement Operator
---

## Overview
The advertisement operator is the module that receives Advertisement CRs and creates the virtual nodes with the announced resources.
Doing so, the remote clusters (emulated by the virtual nodes) are taken into account by the scheduler, which can offload
the jobs it receives on them.

### Features

The operator dynamically watches the `ClusterConfig` CR to know the maximum number of Advertisements that can be accepted and the acceptance policy.
Check [cluster configuration](/user/configure/cluster-config#advertisement-configuration) to get more details about `ClusterConfig` management.
If the configured policy is `AutoAcceptMax`, the behavior is the following:
 - if `MaxAcceptableAdvertisements` increases, check if there are refused Advertisements: if so, they are accepted until
   the new maximum is reached.
 - if MaxAcceptableAdvertisements decreases, Advertisements already accepted are left and from now on the new Advertisements that arrive will follow the new policy.

### Future work
* Implement manual acceptance of Advertisement: at the moment the `ManualAccept` policy is not implemented.
* Implement more complex policies, for example blacklist/whitelist some foreign clusters.
* Gracefully delete the virtual-kubelet when the Advertisement is deleted.

  The Virtual-Kubelet has an ownerReference to the Advertisement, so when this is deleted, the Virtual-Kubelet is deleted as well.
  However, the deletion order is: Advertisement deleted -> Virtual-Kubelet deleted -> Virtual node deleted.
  The desired deletion order is the opposite: in fact, by first deleting the virtual node, the Virtual-Kubelet will reflect the deletion of
  all offloaded resources on the foreign cluster, leaving a clean namespace in it.
  
* Recreate the Virtual-Kubelet if it is unexpectedly deleted.

  If an Accepted Advertisement exists in the cluster, the linked Virtual-Kubelet should exist as well.
  If the Deployment of the Virtual-Kubelet is deleted in some way, it should be recreated by the Advertisement Operator.
  

## Architecture and workflow

![](/images/advertisement-protocol/controller-workflow.png)

1. An `Advertisement` is created by the foreign cluster.
2. Apply configuration read from the `ClusterConfig` CR to accept/refuse the `Advertisement`.
3. If the `Advertisement` is accepted, wait for the network modules to set a possible PodCIDR remapping.
4. When everything has been set up, create the Virtual-Kubelet deployment, giving it the `Secret`, created by the foreign cluster,
   with the permissions to create resources on it.
5. The Virtual-Kubelet creates a virtual node linked with the foreign cluster:
   when a pod is scheduled on the virtual node, it will be sent to the foreign cluster.