---
title: Liqo Namespace Model 
weight: 2
---

### Overview

The Virtual kubelet maps each local namespace with offloaded Pods to a remote namespace with a one-to-one correspondence, to ensure isolation between namespaced resources. Hence, the elected namespaces are mapped to the foreign ones, and the resource reflection in that specific namespace starts.

<<<<<<< HEAD:docs/pages/concepts/offloading/features/namespace-management.md
{{% notice note %}}
This documentation section is a work in progress
{{% /notice %}}
=======
### Namespace Offloading Mechanism

<<<<<<< HEAD:docs/pages/concepts/offloading/features/namespace-management.md
The Liqo webhook ensures that the constraints specified in the configuration are always respected. Your application is never offloaded inside an unselected cluster, you have always the full control of where
your pods are deployed and who can reach them.
>>>>>>> 3a49c80f... Refactor Documentation Structure:docs/pages/Architecture/offloading/features/namespace-management.md
=======
The Liqo webhook ensures that the constraints specified in the configuration are always respected. Your application is never offloaded inside an unselected cluster, you have always the full control of where your pods are deployed and who can reach them.
>>>>>>> 68ea6073... Restructure Docs:docs/pages/Concepts/offloading/features/namespace-management.md
