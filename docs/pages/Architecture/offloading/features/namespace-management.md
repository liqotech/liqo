---
title: Namespace management
weight: 2
---

### Overview

The Virtual kubelet maps each local namespace with offloaded Pods to a remote namespace with a one-to-one correspondence,
to ensure isolation between namespaced resources. Hence, the elected namespaces are mapped to the foreign ones, and the 
resource reflection in that specific namespace starts.

> This documentation section is a work in progress