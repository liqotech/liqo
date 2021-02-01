---
title: Computing
weight: 2
---

## Overview

The computing resource sharing in Liqo is implemented by exploiting our implementation of the
[Virtual Kubelet (VK)](https://github.com/virtual-kubelet/virtual-kubelet) project, a component that, through a custom
kubelet implementation, is in charge of masquerading a remote cluster by means of a local (virtual) node. This local
node pretends to have available resources and to effectively handle the pods scheduled on it, but actually it acts as a
proxy towards a remote Kubernetes cluster. The virtual kubelet is also in charge of handling lifecycle for services, 
endpointslices, configmaps, and secrets across the two clusters. 

### Scheduling behavior

By default, the Kubernetes scheduler selects the node with the highest amount of free resources.
Given that the virtual node summarizes all the resources shared by a given foreign cluster (no matter how many remote physical nodes are involved), is very likely that the above node will be perceived as *fatter* than any physical node available locally. Hence, very likely, new pods will be scheduled on that node.

However, in general, you cannot know which node (either local, or in the foreign cluster) will be selected: it simply depends on the amount of available resources.

To schedule a pod on a given cluster, you have to follow one of the options below.
