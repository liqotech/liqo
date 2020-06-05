![Go](https://github.com/liqoTech/liqo/workflows/Go/badge.svg)

# Liqo

Liqo is a framework to enable dynamic sharing across Kubernetes Clusters.

## Installation

Liqo can be installed via Helm. 

```
cd deployments/liqo_chart/
helm dep up
helm install -n liqo liqo ./ -f values.yaml
```

## Components

Liqo relies on several components:

- *K8s-to-K8s Virtual Kubelet*: Based on [Virtual Kubelet](https://github.com/virtual-kubelet/virtual-kubelet) project, the VK is responsible to "masquerade" a foreign Kubernetes cluster.
- *Advertisement Operator/Broadcaster*: Those components embeds the logic to advertise/discover resources from partner clusters and spawn new virtual kubelet instances
- *Liqonet Operators*: Those operators are responsible to establish Pod-to-Pod connection across partner clusters.

