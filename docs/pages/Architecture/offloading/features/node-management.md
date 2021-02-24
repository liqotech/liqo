---
title: Node management
weight: 1
---

### Overview

The first duty of the Virtual Kubelet is creating the virtual node resource and reconciling its status by taking 
information from two different resources:
* `Advertisement`: node capacity and allocatable.
* `TunnelEndpoint`: inter-connectivity parameters.

> This documentation section is a work in progress