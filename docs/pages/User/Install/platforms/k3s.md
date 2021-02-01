---
title: K3s 
weight: 3
---

## Introduction

### About K3s

[K3s](https://k3s.io) is a Kubernetes distribution packaged as a single binary. It is generally lighter than K8s: it can use sqlite3 as the default storage backend, it has no OS dependencies, etc. More information about K3s can be found on [its Github repository](https://github.com/k3s-io/k3s).

K3s is a great choice with Liqo if you want to create a group small clusters in your LAN or exposed via your home router with a scalable cloud-managed Kubernetes cluster.

K3s has really low requirements in term of memory footprint and it is suitable to be installed on small PCs/Servers.
### K3s Installation

K3s installation can be found on the official [documentation website](https://rancher.com/docs/k3s/latest/en/installation/)

## Liqo Installation

When installing LIQO on K3s, you should explicitly define the parameters required by Liqo, by exporting the following variables **before** launching the installer:

| Variable               | Default             | Description                                 |
| ---------------------- | -------             | ------------------------------------------- |
| `networkManager.config.podCIDR`             | 10.42.0.0/16        | The cluster Pod CIDR                        |
| `networkManager.config.serviceCIDR`         | 10.43.0.0/16        | The cluster Service CIDR                    |
| `discovery.config.clusterName`         |                     | Nickname for your cluster that others will see. If you don't specify one, the installer will give you a cluster name in the form "LiqoClusterX", where X is a random number |

### Install

You can then run the Liqo installer script, which will use the above settings to configure your Liqo instance.

*N.B.* Please remember to export your K3s `kubeconfig` before launching the script, as presented in previous section. For K3s, the kubeconfig is normally stored in `/etc/rancher/k3s/k3s.yaml`

#### Pre-requirements

To install Liqo, you have to install the following dependencies:

* [Helm 3](https://helm.sh/docs/intro/install/)
* [Kubectl](https://kubernetes.io/docs/tasks/tools/install-kubectl/)

### Deploy

You can install Liqo using helm 3.

Firstly, you should add the official Liqo repository to your Helm Configuration:

```bash
helm repo add liqo-helm https://helm.liqo.io/charts
```

If you are installing Liqo for the first time, you can download the default values.yaml file from the chart.

```bash
helm fetch liqo-helm/liqo --untar
less ./liqo/values.yaml
```

The most important values you can set are the following:

| Variable               | Default             | Description                                 |
| ---------------------- | -------             | ------------------------------------------- |
| networkManager.config.podCIDR         | 10.42.0.0/16        | the cluster Pod CIDR                        |
| networkManager.config.serviceCIDR         | 10.43.0.0/16        | the cluster Service CIDR                    |
| discovery.config.clusterName         |         | nickname for your cluster that will be seen by others. If you don't specify one, the installer will give you a cluster name in the form "LiqoClusterX", where X is a random number |

You can modify the ```./liqo/values.yaml``` to obtain your desired configuration and install Liqo.

```bash
helm install test liqo-helm/liqo -f ./liqo/values.yaml
```

or ALTERNATIVELY pass the desired parameters as extra-arguments:

```bash
helm install liqo liqo-helm/liqo --set clusterName="MyCluster" --set networkManager.config.podCIDR="10.42.0.0/16" --set networkManager.config.serviceCIDR="10.43.0.0/16" ...
```
### On-premise Cluster behind NAT

If your cluster is hosted on premise behind a NAT nd you would like to connect your cluster with another via the Internet,you should avoid to use ingress and use the following configuration:

| Component | Variables | Value | Notes |
| --------- | -------- | ------ | ----- |
| **Auth Server** | auth.service.type  | NodePort |
| **API server** | apiserver.ip |  | The IP/Host exposed by the NAT |
|                | apiserver.port |  | The port exposed by the NAT  |
| **VPN gateway** | gateway.service.type | NodePort |

### On-Premise to On-Premise (LAN)

If you want to connect your cluster with another K3s/K8s in the same LAN, you do not need further configuration. You can install Liqo with just specifying the correct values for the three variables mentioned above:

```
helm install liqo liqo-helm/liqo --set clusterName="MyCluster" --set networkManager.config.podCIDR="10.42.0.0/16" --set networkManager.config.serviceCIDR="10.43.0.0/16"
```

If the clusters you would like to connect are in the same L2 broadcast domain, the Liqo discovery mechanism based on mDNS will handle the discovery automatically. If you have your clusters in different L3 domains, you have to manually create [manually a *foreign_cluster* resource](/user/post-install/discovery) or rely on [DNS discovery](/user/post-install/discovery#manual-configuration).