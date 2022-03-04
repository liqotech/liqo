---
title: Advanced installation options
weight: 3
---

Liqoctl is the swiss-knife CLI tool to install and manage Liqo clusters.

You can find how to install Liqo in the [Installation section](/installation/install).
In this section, you can find how to use some `liqoctl install` advanced features.

## Unstable releases

By default, `liqoctl install` installs the last stable version of Liqo.

However, if you want to try an unstable release you can just specify it using the --version flag:

```
liqoctl install kind --version v0.3.1-alpha.1
```

It is suggested to use liqoctl of the same version you are installing. You can download liqoctl through the [release page](https://github.com/liqotech/liqo/releases).

## Generate chart values

Under the hood, liqoctl uses [Helm 3](https://helm.sh/) to configure and install the Liqo chart available on the official repository.

However, if you prefer to directly use Helm you can use  `liqoctl install` just to "compile" the `values.yaml` of the Liqo helm chart for your cluster and then install it using an explicit helm command.

```bash
liqoctl install k3s --only-output-values
```

By default, you will find the values.yaml file in the current directory.

You can then install Liqo by typing:

```bash
helm repo add liqo https://helm.liqo.io/ # if the repository was not already present
helm install liqo/liqo -n liqo -f values.yaml
```

## Install from a local chart

If you need to install a custom version from a local version, you can use the `--chart-path` and `--version` option like the following example:

```
git clone https://github.com/liqotech/liqo.git
cd liqo
git checkout $YOUR_COMMIT
go run ./cmd/liqoctl install kind --chart-path ./deployments/liqo/ --version "$(git log --format="%H" -n 1)"
```

In this command, you are selecting the version tagged with the current commit as version and the `./deployments/liqo` as chart path.
