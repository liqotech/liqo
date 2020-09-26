---
title: Install Liqo
weight: 1
---

## Install steps

This procedure installs Liqo on your cluster, enabling it to share resources with other Liqo clusters.

This procedure comes in two variants:
* [Default install](#default-install): suitable if your Kubernetes cluster has been installed via `kubeadm`
* [Custom install](#custom-install): suitable if you did not use `kubeadm` to install your Kubernetes, or you are running another distribution of Kubernetes (such as [K3s](https://k3s.io/)).


### Default install

If your cluster has been installed via `kubeadm`, the Liqo Installer can automatically retrieve the parameters required by Liqo to start.
Before installing, you have to properly set the `kubeconfig` for your cluster. The Liqo installer leverages `kubectl`: by default kubectl refers to the default identity in `~/.kube/config` but you can override this configuration by exporting a `KUBECONFIG` variable.

For example:
```
export KUBECONFIG=my-kubeconfig.yaml
```

You can find more details about configuring `kubectl` in the [official documentation](https://kubernetes.io/docs/concepts/configuration/organize-cluster-access-kubeconfig/).

Now, you can install Liqo by launching:

```bash
curl -sL https://raw.githubusercontent.com/liqotech/liqo/master/install.sh | bash
```

If you want to know more about possible customizations, you can show the help message:
```bash
curl -sL https://raw.githubusercontent.com/liqotech/liqo/master/install.sh | bash -s -- --help
```

### Custom install (K3s)

If you did not use `kubeadm` to install your Kubernetes cluster, or you are running another distribution of Kubernetes (such as [K3s](https://k3s.io/)), you should explicitly define the parameters required by Liqo, by exporting the following variables **before** launching the installer:

* `POD_CIDR`: range of IP addresses for the pod network (K3s default: 10.42.0.0/16)
* `SERVICE_CIDR`: range of IP addresses for service VIPs (k3s default: 10.43.0.0/16)

Then, you can run the Liqo installer script, which will use the above settings to configure your Liqo instance.

Please remember to export your K3s `kubeconfig` before launching the script, as presented in previous section. For K3s, the kubeconfig is normally stored in `/etc/rancher/k3s/k3s.yaml`

A possible example of installation is the following (please replace the IP addresses with the ones related to your Kubernetes instance):
```bash
export KUBECONFIG=/etc/rancher/k3s/k3s.yaml
export POD_CIDR=10.42.0.0/16
export SERVICE_CIDR=10.43.0.0/16
curl -sL https://raw.githubusercontent.com/liqotech/liqo/master/install.sh | bash
```

Obviously, you should have enough privileges to read the K3s kubeconfig file.

## Peer with another cluster

In order to peer with another cluster, you need to have **two** Kubernetes clusters with Liqo enabled.
Therefore you may need to repeat the above procedure on another cluster in order to get a second Liqo instance.

Once you have two clusters ready, you can start the peering procedure, which is presented in the [next step](../peer).
