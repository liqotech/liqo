---
title: Pre-Install 
weight: 1
---

## Introduction

Liqo can be installed on different types of clusters, either on-premise or on public cloud providers. 
To peer two clusters together, they must (1) discover each other, (2) negotiate the required parameters and (3) activate the peering.


As presented in a [dedicated section](/configuration/discovery), Liqo has several mechanisms to handle new clusters discovery (LAN, DNS, Manual). 
Despite LAN discovery is limited to very specific use-cases (when clusters are in the same broadcast domain), DNS and Manual discovery can be applied to many scenarios.
However, the clusters should satisfy few peering requirements. In particular, the configuration depends on the type of connectivity between the two clusters. 

### Peering Requirements

Liqo requires the following services to be reciprocally accessible on both clusters to be able to start the cluster peering:

* **Authentication server** (`liqo-auth`): the Liqo service used to authenticate incoming peering requests coming from other clusters. 
You should modify the values in the ``auth`` section of the [Liqo chart values](/installation/chart_values) to configure how the authentication server is exposed.
* **Kubernetes API server**: the standard Kubernetes API Server, which the (remote) Liqo instance will contact to create the required resources when the peering process starts. 
The API Server URL can be configured in the ``apiServer`` section of the [Liqo chart values](/installation/chart_values). 
By default, Liqo will use an endpoint composed of the IP of the first control plane node and the 6443 port.
In managed clusters, you have to configure those values to have Liqo working correctly.
* **Network gateway** (`liqo-gateway`): the Liqo service responsible for setting up the network connectivity between clusters. 
The Liqo Gateway is configured in the ``gateway`` section of the [Liqo chart values](/installation/chart_values).

Depending on the physical setup of your cluster, you need to properly configure, at install time, how those services are exposed and can be reached by remote clusters. 
Below we present some common scenarios that Liqo can handle. Once you identify yours, you can refer to the *table* of each section to find how to determine the right values you should specify.

The exposition parameters can be configured at installation time using the [Liqo Helm Chart](/installation/chart_values) and updated after the installation by issuing an ``helm update`` after changing them in your values.yml. 
If you need more information about Helm and how charts can be configured, you can have a look at the [Helm official documentation](https://helm.sh/docs/). 
Pay attention that changing exposition parameters may affect and break active peerings. We suggest to disable all peerings before changing the Liqo exposition configuration.

### Cloud to cloud

![](/images/scenarios/cloud-to-cloud.svg)

Two managed clusters peered together through the Internet. It is possible to have a multi-cloud setup (AKS to AKS, GKE to GKE, and AKS to GKE). 
In this scenario, the services should be exposed using Public IPs/URLs.

|           | Cluster A (Cloud) | Cluster B (Cloud) |
| --------- | ----------------- | ----------------- | 
| **Auth Server** |  LoadBalancer/ingress | LoadBalancer/Ingress |
| **API server** | Provided by the Cloud Provider| Provided by the Cloud Provider |
| **Network gateway** | LoadBalancer/Node Port | LoadBalancer/Node Port |

At least one among Cluster A and Cluster B should have the **Network Gateway** IP accessible from the other one (e.g., Public IP).

### On-premise to cloud

![](/images/scenarios/on-prem-to-cloud.svg)

On-premise cluster (K3s or K8s) exposed through the Internet peered with a Managed cluster (AKS or GKE).

|           | Cluster A (On-prem) | Cluster B (Cloud) |
| --------- | ------------------- | ----------------- |
| **Auth Server** |  LoadBalancer/Ingress | LoadBalancer/Ingress |
| **API server** | Ingress/Public IP | Provided by the Cloud Provider |
| **Network gateway** | LoadBalancer/Node Port | LoadBalancer/Node Port |

Clusters API Server should be mutually accessible, and so should be for the Auth Service.
In addition, at least one among Cluster A and Cluster B should have the **Network Gateway** IP accessible from the other one (e.g., Public IP) to establish a cluster network interconnection. 
If you configure the Auth service as Ingress, you should pay attention to disable TLS on the service or to configure your Ingress Controller to support a TLS backend. 
This last configuration will ensure an end-to-end TLS connection.

#### On-premise behind NAT to cloud

![](/images/scenarios/on-prem-nat-to-cloud.svg)

When the On-premise cluster is exposed through a NAT, the configuration slightly changes:

|           | Cluster A (On-prem behind NAT) | Cluster B (Cloud) |
| --------- | ------------------------------ | ----------------- |
| **Auth Server** |  NodePort with port-forwarding | LoadBalancer/ingress |
| **API server** | Port-forwarding | Provided |
| **Network gateway** | NodePort with port-forwarding | LoadBalancer |

In this situation, the "cloud" cluster should have the Network Gateway exposed as a **LoadBalancer**. 
A couple of port-forwardings should also be configured for the Auth Server and K8s API Server to make them accessible from Cloud B.

### Clusters in the same LAN

![](/images/scenarios/on-prem-to-on-prem.svg)

Clusters (K3s or K8s) in the same LAN may rely on the mDNS-based Liqo discovery mechanism.
The Liqo discovery mechanism based on mDNS will handle the discovery automatically. 

|           | Cluster A (On-prem) | Cluster B (On-prem) |
| --------- | ------------------- | ------------------- |
| **Auth Server** |  NodePort | NodePort |
| **API server** | Exposed | Exposed |
| **Network gateway** | NodePort | NodePort |

This configuration is provided using the standard values of the Liqo chart.
