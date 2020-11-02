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
