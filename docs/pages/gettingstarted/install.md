---
title: Install Liqo
weight: 2
---

### Simple Installation (One-liner)

Before installing Liqo, you have to set the right `kubeconfig` for your cluster properly. The Liqo installer leverages `kubectl`: by default kubectl refers to the default identity in `~/.kube/config` but you can override this configuration by exporting a `KUBECONFIG` variable.

For the clusters, we just deployed in the [previous step](../kind), you can type:

```bash
export KUBECONFIG=./liqo_kubeconf_1
```

You can find more details about configuring `kubectl` in the [official documentation](https://kubernetes.io/docs/concepts/configuration/organize-cluster-access-kubeconfig/).

Now, you can install Liqo by launching:

```bash
curl -sL https://get.liqo.io | bash
```

If you want to know more about possible customizations, you can show the help message:
```bash
curl -sL https://get.liqo.io | bash -s -- --help
```

#### Install the second cluster

Similarly, as done on the first cluster, you can deploy Liqo on the second cluster:

```
export KUBECONFIG=./liqo_kubeconf_2
curl -sL https://get.liqo.io | bash
```

## Enable cluster peering

Now, you have two clusters with Liqo enabled.
Once you have two clusters ready, you can start the [peering procedure](../peer).