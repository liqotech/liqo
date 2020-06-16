![Go](https://github.com/liqoTech/liqo/workflows/Go/badge.svg) 
[![Coverage Status](https://coveralls.io/repos/github/LiqoTech/liqo/badge.svg?branch=master)](https://coveralls.io/github/LiqoTech/liqo?branch=master)
[![Go Report Card](https://goreportcard.com/badge/github.com/LiqoTech/liqo)](https://goreportcard.com/report/github.com/LiqoTech/liqo)
# Liqo

Liqo is a framework to enable dynamic sharing across Kubernetes Clusters. You can run your pods on a remote cluster
seamlessly, without any modification (Kubernetes or your application). 

Differently from the [Kubernetes Federation](https://github.com/kubernetes-sigs/kubefed) and the
[Admiralty](https://admiralty.io/) project, Liqo is designed to handle dynamic, temporary
resource sharing across multi-owner clusters and targets every type of computing resources (e.g. Raspberry PI, 
Desktop PCs, Servers).

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

You have to label one node of your cluster as the gateay node. This will be used as the gateway for the inter-cluster traffic.

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

#### [K3s](k3s.io)

K3s is a minimal Kubernetes cluster which is pretty small and easy to set up. However, it does not store its configuration in the
way that traditional installers (e.g.; Kubedam) do. Therefore, it is required to know the configuration you entered for your cluster.

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

...and some others. Check out the architecture [Documentation](docs/design/architecture.md)

