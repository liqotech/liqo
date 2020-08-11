---
title: Getting Started 
weight: 1
---

## Installation

The following process will install Liqo on your local cluster. This will make your cluster ready to share resources with other Liqo clusters.

The Liqo Installer can retrieve automatically the cluster parameters required by Liqo to start.
After having properly configured the `kubeconfig` for your cluster, you can install Liqo by launching: 

```bash
curl https://raw.githubusercontent.com/LiqoTech/liqo/master/install.sh | bash
```

This would normally work "out of the box" if your cluster has been installed via **Kubeadm**.
If you used another installer or another distribution (such as K3s), you can override them by exporting the following variables before launching the installer:

* *POD_CIDR*:  range of IP addresses for the pod network
* *SERVICE_CIDR*: range of IP address for service VIPs
* *GATEWAY_IP*: public IP of the node targeted for cluster interconnection
* *GATEWAY_PRIVATE_IP*: private IP of the tunnel for interconnected clusters. This IP can be chosen randomly but they
have to be unique for each cluster which share resources.

A possible example of installation:
```bash
export POD_CIDR=10.32.0.0/16
export SERVICE_CIDR=10.10.0.0/16
export GATEWAY_IP=10.0.0.23
export GATEWAY_PRIVATE_IP=192.168.100.2
curl https://raw.githubusercontent.com/LiqoTech/liqo/master/install.sh | bash
```

#### [Example: K3s](k3s.io)

K3s is a minimal Kubernetes distribution with reduced resource consumption and it is easy to set up.
However, since it stores its configuration in a different way compared to traditional installers (e.g.; `kubeadm`), you have to use a slightly different procedure to setup Liqo, which requires some manual steps.

After having exported your K3s `kubeconfig`, you can install LIQO setting the following variables before launching the installer. The following values represent the default configuration for K3s cluster, you may need to adapt them to the current values of your cluster.

```bash
export POD_CIDR=10.42.0.0/16
export SERVICE_CIDR=10.43.0.0/16
export GATEWAY_IP=10.0.0.31
export GATEWAY_PRIVATE_IP=192.168.100.1
curl https://raw.githubusercontent.com/LiqoTech/liqo/master/install.sh | bash
```

### Join another cluster

After having performed the same installation on another cluster. You have to let them peer [together](./peering).

