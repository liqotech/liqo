![Go](https://github.com/liqoTech/liqo/workflows/Go/badge.svg) 
[![Coverage Status](https://coveralls.io/repos/github/LiqoTech/liqo/badge.svg?branch=master)](https://coveralls.io/github/LiqoTech/liqo?branch=master)

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

### 
We will refer to cluster1 and cluster2.
To start playing with Liqo you will require to create two kubeconfig. A script to properly generate a kubeconfig is 
available [here](https://gist.github.com/innovia/fbba8259042f71db98ea8d4ad19bd708).

Among the others possible values, you have to set up the following parameters in 
[values.yaml](./deployments/liqo_chart/values.yaml):

```bash
cd deployments/liqo_chart/
helm dep up
export kubeconfig=cluster1 # the name of your kubeconfig
helm install -n liqo liqo ./ -f values-c1.yaml
export kubeconfig=cluster2 # the name of your kubeconfig
helm install -n liqo liqo ./ -f values-c2.yaml
```

## Architecture

Liqo relies on several components:

* *Liqo Virtual Kubelet*: Based on [Virtual Kubelet](https://github.com/virtual-kubelet/virtual-kubelet) project, the VK
 is responsible to "masquerade" a foreign Kubernetes cluster.
* *Advertisement Operator/Broadcaster*: Those components embed the logic to advertise/accept resources from partner
 clusters and spawn new virtual kubelet instances
* *Liqonet Operators*: Those operators are responsible to establish Pod-to-Pod and Pod-to-Service connection across 
partner clusters.

...and some others. Check out the architecture [Documentation](docs/architecture.md)

