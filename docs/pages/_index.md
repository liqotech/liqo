---
title: Welcome to Liqo Documentation
chapter: true
---

# Welcome to Liqo Documentation

![](/images/logo-liqo-blue.svg)

Liqo is an open source project started at Politecnico of Turin that allows Kubernetes to seamlessly and securely share
resources and services, so you can run your tasks on any other cluster available nearby.

Thanks to the support for K3s, also single machines can participate,creating dynamic, opportunistic data centers that 
include commodity desktop computers and laptops as well.

Liqo leverages the same highly successful “peering” model of the Internet, without any central point of control. 
New peering relationships can be established dynamically, whenever needed, even automatically. 
Cluster auto-discovery can further simplify this process.

![](/images/home/architecture.png)

Sharing and peering operations are strictly enforced by policies: each cluster retains full control of its infrastructure, 
deciding what to share, how much, with whom. For security we leverage all the features available in Kubernetes, such as 
Role-Based Access Control (RBAC), Pod Security Policies (PSP), hardened Container Runtimes Interfaces (CRI) implementations.

With Liqo, there is no disruption neither in the common Kubernetes administration tasks nor from the user perspective 
because everything happens as your cluster gets bigger.

[Get started!](user/install)