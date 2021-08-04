---
title: Installation 
weight: 2
---

### Pre-Installation

Liqo can be used with different topologies and scenarios. This impacts several installation parameters you will configure (e.g., API Server, Authentication). The [scenarios page](./pre-install) presents some common patterns used to expose and interconnect clusters.

### Installation

Liqo can be installed on existing clusters or deployed to newly created clusters.

| K8s platforms/Service                                         | Supported                      | Notes                                  |
| ------------------------------------------------------------- | ------------------------------ | -------------------------------------- |
| [Azure Kubernetes Service (AKS)](./platforms/aks)             | Yes                            |                                        |
| [Google Kubernetes Engine (GKE)](./platforms/gke)             | Yes                            |                                        |
| [K3s](./platforms/k3s)                                        | Yes                            |                                        |
| [Kubernetes with Kubeadm](./platforms/k8s)                    | Yes                            |                                        |
| [AWS Elastic Kubernetes Service (EKS)](./platforms/k8s)       | In Progress                    |                                        |

#### Peer your clusters

When you have installed Liqo on your clusters, you can decide to peer them to offload your applications on a different cluster as documented in [usage section](/usage).