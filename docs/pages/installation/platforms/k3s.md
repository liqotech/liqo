---
title: K3s 
weight: 5
---

## About K3s

[K3s](https://k3s.io) is a Kubernetes distribution packaged as a single binary. It is generally lighter than K8s: it can use sqlite3 as the default storage backend, it has no OS dependencies, etc. More information about K3s can be found on [its Github repository](https://github.com/k3s-io/k3s).

K3s is an excellent choice with Liqo if you want to create a group of small clusters in your LAN or exposed via your home router with a scalable cloud-managed Kubernetes cluster.

K3s has low requirements in memory footprint and is suitable to be installed on small PCs/Servers.

### K3s Installation

K3s installation can be found on the official [documentation website](https://rancher.com/docs/k3s/latest/en/installation/)

## Liqo Installation

> *N.B.* Please remember to export your K3s `kubeconfig` before installing Liqo, as presented in previous section. For K3s, the kubeconfig is stored in `/etc/rancher/k3s/k3s.yaml`

### Pre-requirements

To install Liqo, you have to install the following dependencies:

* [Helm 3](https://helm.sh/docs/intro/install/)
* [Kubectl](https://kubernetes.io/docs/tasks/tools/install-kubectl/)

### Set-up

You can install Liqo using Helm 3.

Firstly, you should add the official Liqo repository to your Helm Configuration:

```bash
helm repo add liqo https://helm.liqo.io/
```

If you are installing Liqo for the first time, you can download the default ```values.yaml``` file from the chart.

```bash
helm show values liqo/liqo > ./values.yaml
```

The most important values you can set are the following:

| Variable               | Default             | Description                                 |
| ---------------------- | -------             | ------------------------------------------- |
| networkManager.config.podCIDR         | 10.42.0.0/16        | The cluster Pod CIDR                        |
| networkManager.config.serviceCIDR         | 10.43.0.0/16        | The cluster Service CIDR                    |
| discovery.config.clusterName         |         | Nickname for your cluster that will be seen by others. If you don't specify one, the installer will give you a cluster name in the form "LiqoClusterX", where X is a random number |

#### On-Premise to On-Premise with direct connectivity

If you want to connect your cluster with another K3s/K8s in the same LAN, you do not need further configuration. You can install Liqo with just specifying the correct values for the three variables mentioned above:

```
helm install liqo liqo/liqo -n liqo --create-namespace --set clusterName="MyCluster" --set networkManager.config.podCIDR="10.42.0.0/16" --set networkManager.config.serviceCIDR="10.43.0.0/16"
```

If the clusters you would like to connect are in the same L2 broadcast domain, the Liqo discovery mechanism based on mDNS will handle the discovery automatically. If you have your clusters in different L3 domains, you have to manually create [a *foreign_cluster* resource](/configuration/discovery) or rely on [DNS discovery](/configuration/discovery#manual-configuration).

#### On-premise Cluster behind NAT

If your cluster is hosted on-premise behind a NAT and you would like to connect your cluster with another via the Internet; you should avoid to use ingress and use the following configuration:

| Component | Variables | Value |
| --------- | -------- | ------ |
| **Auth Server** | auth.service.type  | NodePort |
| **API server** | apiserver.ip |  | The IP/Host exposed by the NAT |
|                | apiserver.port |  | The port exposed by the NAT  |
| **VPN gateway** | gateway.service.type | NodePort |

#### Helm Install

You can modify the ```./values.yaml``` to obtain your desired configuration and install Liqo.

```bash
helm install liqo liqo/liqo -f ./values.yaml -n liqo --create-namespace
```

or ALTERNATIVELY pass the desired parameters as extra-arguments:

```bash
helm install liqo liqo/liqo -n liqo --create-namespace --set clusterName="MyCluster" --set networkManager.config.podCIDR="10.42.0.0/16" --set networkManager.config.serviceCIDR="10.43.0.0/16" ...
```