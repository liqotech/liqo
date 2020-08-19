---
title: Installing Liqo
weight: 1
---

## Install steps

This procedure installs Liqo on your cluster, enabling it to share resources with other Liqo clusters.

This procedure comes in two variants:
* [Default install](#default-install): suitable if your Kubernetes cluster has been installed via `kubeadm`
* [Custom install](#custom-install): suitable if you did not use `kubeadm` to install your Kubernetes, or you are running another distribution of Kubernetes (such as [K3s](https://k3s.io/)).


### Default install

If your cluster has been installed via `kubeadm`, the Liqo Installer can automatically retrieve the parameters required by Liqo to start.
After having properly configured the `kubeconfig` for your cluster, you can install Liqo by launching: 

```bash
curl https://raw.githubusercontent.com/LiqoTech/liqo/master/install.sh | bash
```

<!-- TODO: please specify what do you have to do to 'configure the kubeconfig', which does not look obvious to me. -->


### Custom install

If you did not use `kubeadm` to install your Kubernetes, or you are running another distribution of Kubernetes (such as [K3s](https://k3s.io/)), you should explicitly define the parameters required by Liqo by exporting the following variables **before** launching the installer:

* `POD_CIDR`: range of IP addresses for the pod network
* `SERVICE_CIDR`: range of IP addresses for service VIPs
* `GATEWAY_IP`: public IP address of the node that will be used as a gateway for all the traffic toward the foreign cluster

<!-- TODO: please be more specific about which IP addresses you have to tell for POD and SERVICE CIDR: are those the one configured on your local cluster? In this case, can you please make an example about how to get them in K3s? -->

Then, you can run the Liqo installer script, which will use the above settings to configure your Liqo instance.

Please remember to export your K3s `kubeconfig` before launching the script.

<!-- TODO: please specify what do you have to do to 'export the kubeconfig', which does not look obvious to me. -->

A possible example of installation is the following (please replace the IP addresses with the ones related to your Kubernetes instance):
```bash
export POD_CIDR=10.32.0.0/16
export SERVICE_CIDR=10.10.0.0/16
export GATEWAY_IP=10.0.0.23
curl https://raw.githubusercontent.com/LiqoTech/liqo/master/install.sh | bash
```

## Peer with another cluster

In order to peer with another cluster, you need to have **two** Kubernetes clusters with Liqo enabled.
Therefore you may need to repeat the above procedure on another cluster in order to get a second Liqo instance.

Once you have two clusters ready, you can start the peering procedure, which is presented in the [next step](../peer).

