---
title: Scenarios 
weight: 2
---

### Introduction

Liqo can be installed either in private or local clusters. Its configuration depends on the type of connectivity between the two clusters.

### Types

Liqo relies on the following services that must be exposed outside the cluster:

* Authentication server: Liqo authentication endpoint
* API server: The API Server of Kubernetes
* VPN gateway: Liqo Network endpoint

Those services exposure are distribution independent (K8s, K3s, AKS, GKE) and can be summarized as follows:

### Examples

Below it is possible to find some common scenarios that Liqo can handle. Once you identify yours, you can go ahead to the install section to find the installation instruction for your distribution.

### Cloud to cloud

![](/images/scenarios/cloud-to-cloud.svg)

Two managed clusters peered together through the internet. It is possible to have a multi-cloud setup (AKS to AKS, GKE to GKE, and AKS to GKE).

|  | Cluster A (Cloud) | Cluster B (Cloud) |
| --------- | -------- |  ---------       |
| **Auth Server** |  LoadBalancer/ingress | LoadBalancer/ingress |
| **API server** | Provided | Provided |
| **VPN gateway** | LoadBalancer | LoadBalancer |

### On-premise to cloud

![](/images/scenarios/on-prem-to-cloud.svg)

On-premise cluster (K3s or K8s) exposed through the internet peered with a Managed cluster (AKS or GKE).

|  | Cluster A (On-prem) | Cluster B (Cloud) |
| --------- | -------- |  ---------       |
| **Auth Server** |  LoadBalancer/ingress | LoadBalancer/ingress |
| **API server** | Ingress/Public IP | Provided |
| **VPN gateway** | LoadBalancer | LoadBalancer |

### On-premise behind NAT to cloud

![](/images/scenarios/on-prem-nat-to-cloud.svg)

On-premise cluster (K3s or K8s) exposed through a NAT over the internet peered with a managed cluster (AKS or GKE).

|  | Cluster A (On-prem behind NAT) | Cluster B (Cloud) |
| --------- | -------- |  ---------       |
| **Auth Server** |  NodePort with port-forwarding | LoadBalancer/ingress |
| **API server** | Port-forwarding | Provided |
| **VPN gateway** | NodePort with port-forwarding | LoadBalancer |

### On-premise to on-premise (LAN)

![](/images/scenarios/on-prem-to-on-prem.svg)

On-premise cluster (K3s or K8s) peered with another on-premise cluster (K3s or K8s) in the same LAN.
From the discovery perspective, if the clusters you would like to connect are in the same L2 broadcast domain, the Liqo discovery mechanism based on mDNS will handle the discovery automatically. If you have your clusters in different L3 domains, you have to manually [create](/user/post-install/discovery#forging-the-foreigncluster) a *foreign_cluster* resource or rely on [DNS discovery](/user/post-install/discovery#manual-configuration).

|  | Cluster A (On-prem behind NAT) | Cluster B (Cloud) |
| --------- | -------- |  ---------       |
| **Auth Server** |  NodePort | NodePort |
| **API server** | Exposed | Exposed |
| **VPN gateway** | NodePort | NodePort |
