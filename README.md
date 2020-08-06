<p align="center">
<img alt="Liqo Logo" src="https://doc.liqo.io/images/logo-liqo-blue.svg" />
</p>

# Liqo

![Go](https://github.com/liqoTech/liqo/workflows/Go/badge.svg) 
[![Coverage Status](https://coveralls.io/repos/github/LiqoTech/liqo/badge.svg?branch=master)](https://coveralls.io/github/LiqoTech/liqo?branch=master)
[![Go Report Card](https://goreportcard.com/badge/github.com/LiqoTech/liqo)](https://goreportcard.com/report/github.com/LiqoTech/liqo)

Liqo is a framework to enable dynamic sharing across Kubernetes Clusters. You can run your pods on a remote cluster
seamlessly, without any modification (Kubernetes or your application). 

Liqo is an open source project started at Politecnico of Turin that allows Kubernetes to seamlessly and securely share resources and services, so you can run your tasks on any other cluster available nearby.

Thanks to the support for K3s, also single machines can participate,creating dynamic, opportunistic data centers that include commodity desktop computers and laptops as well.

Liqo leverages the same highly successful “peering” model of the Internet, without any central point of control. New peering relationships can be established dynamically, whenever needed, even automatically. Cluster auto-discovery can further simplify this process.

## Features

* Dynamic discovery of clusters in LAN
* Seamless pod execution on remote cluster,
* Seamless reconciliation on remote clusters of K8s objects (i.e. configmaps, secrets, services, endpoints)


## Quickstart

Liqo can be installed via Helm. 

### Pre-requisites

* Two Kubernetes clusters
    * Supported CNIs:
      * Flannel
      * Calico 
    * K3s is also supported
* Helm 3

### Installation

The following process will install Liqo on your Cluster. This will make your cluster ready to share resources with other Liqo resources.

### Pre-requirements

You have to label one node of your cluster as the gateway node. This will be used as the gateway for the inter-cluster traffic.

```bash
kubectl label no __your__gateway__node liqonet.liqo.io/gateway=true
```

To get the list of your nodes, you can use: 

```
kubectl get no
```

#### Kubernetes

Liqo Installer should be capable to look for the cluster parameters required. 

```bash
curl https://raw.githubusercontent.com/LiqoTech/liqo/master/install.sh | bash
```

#### [K3s](https://k3s.io)

K3s is a minimal Kubernetes cluster which is pretty small and easy to set up. However, it does not store its configuration in the
way that traditional installers (e.g.; Kubeadm) do. Therefore, it is required to know the configuration you entered for your cluster.

After having exported your K3s Kubeconfig, you can install LIQO setting the following variables before launching the installer.
The following values represent the default configuration for K3s cluster, you may need to adapt them to the actual values of your cluster.

```bash
export POD_CIDR=10.42.0.0/16
export SERVICE_CIDR=10.43.0.0/16
export GATEWAY_IP=10.0.0.31
export GATEWAY_PRIVATE_IP=192.168.100.1
curl https://raw.githubusercontent.com/LiqoTech/liqo/master/install.sh | bash
```
## Architecture

Liqo relies on several components:

* *Liqo Virtual Kubelet*: Based on [Virtual Kubelet](https://github.com/virtual-kubelet/virtual-kubelet) project, the VK
 is responsible to "masquerade" a foreign Kubernetes cluster.
* *Advertisement Operator/Broadcaster*: Those components embed the logic to advertise/accept resources from partner
 clusters and spawn new virtual kubelet instances
* *Liqonet Operators*: Those operators are responsible to establish Pod-to-Pod and Pod-to-Service connection across 
partner clusters.

...and some others. Check out the architecture [Documentation](https://doc.liqo.io/architecture/)

