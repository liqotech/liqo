<p align="center">
<img alt="Liqo Logo" src="https://doc.liqo.io/images/logo-liqo-blue.svg" />
</p>

# Liqo

![Go](https://github.com/liqoTech/liqo/workflows/Go/badge.svg) 
[![Coverage Status](https://coveralls.io/repos/github/LiqoTech/liqo/badge.svg?branch=master)](https://coveralls.io/github/LiqoTech/liqo?branch=master)
[![Go Report Card](https://goreportcard.com/badge/github.com/LiqoTech/liqo)](https://goreportcard.com/report/github.com/LiqoTech/liqo)
![Docker Pulls](https://img.shields.io/docker/pulls/liqo/virtual-kubelet?label=Liqo%20vkubelet%20pulls)
[![FOSSA Status](https://app.fossa.com/api/projects/git%2Bgithub.com%2FLiqoTech%2Fliqo.svg?type=shield)](https://app.fossa.com/projects/git%2Bgithub.com%2FLiqoTech%2Fliqo?ref=badge_shield)

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
The parameters of the home cluster required by Liqo to start should be automatically discovered by the Liqo installer 
(through proper kubeadm calls).

```bash
curl https://raw.githubusercontent.com/LiqoTech/liqo/master/install.sh | bash
```

For more details about [Liqo installation](https://doc.liqo.io/user/gettingstarted/install)

## Architecture

Liqo relies on several components:

* *Liqo Virtual Kubelet*: Based on [Virtual Kubelet](https://github.com/virtual-kubelet/virtual-kubelet) project, the VK
 is responsible to "masquerade" a foreign Kubernetes cluster.
* *Advertisement Operator/Broadcaster*: Those components embed the logic to advertise/accept resources from partner
 clusters and spawn new virtual kubelet instances
* *Liqonet Operators*: Those operators are responsible to establish Pod-to-Pod and Pod-to-Service connection across 
partner clusters.

...and some others. Check out the architecture [Documentation](https://doc.liqo.io/architecture/)


## License
[![FOSSA Status](https://app.fossa.com/api/projects/git%2Bgithub.com%2FLiqoTech%2Fliqo.svg?type=large)](https://app.fossa.com/projects/git%2Bgithub.com%2FLiqoTech%2Fliqo?ref=badge_large)