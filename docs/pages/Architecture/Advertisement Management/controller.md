---
title: Advertisement Operator
---

## Overview
The advertisement operator is the module that receives Advertisement CRs and creates the virtual nodes with the announced resources. 
Doing so, the remote clusters (emulated by the virtual nodes) are taken into account by the scheduler, which can offload
the jobs it receives on them.

### Features
* Dynamic management of Advertisement acceptance/refusal

The operator watches `ClusterConfig` CR to know the maximum number of Advertisements that can be accepted. 
If the `AutoAccept` flag is set to true, the behaviour is the following:
 - if MaxAcceptableAdvertisements increases, check if there are refused Advertisements: if so, they are accepted until
   the new maximum is reached.
 - if MaxAcceptableAdvertisements decreases, Advertisements already accepted are left and from now on the new Advertisements that arrive will follow the new policy.

### Limitations
* Manual acceptance of Advertisement
* More complex policies (e.g. blacklist/whitelist some foreign clusters)
* Graceful deletion of virtual-kubelet when Advertisement is deleted
* Recreation of virtual-kubelet if it is unexpectedly deleted

## Architecture and workflow

![](/images/advertisement-protocol/controller-workflow.png)

1. An `Advertisement` is created by the foreign cluster
2. Apply configuration read from `ClusterConfig` CR to accept/refuse the `Advertisement`
3. If the `Advertisement` is accepted, wait for network modules to set a possible PodCIDR remapping
4. When everything has been set up, create the Virtual-Kubelet deployment, giving it the `Secret`, created by the foreign cluster,
   with the permissions to create resources on it
5. The Virtual-Kubelet creates a virtual node linked with the foreign cluster:
   when a pod is scheduled on the virtual node, it will be sent to the foreign cluster