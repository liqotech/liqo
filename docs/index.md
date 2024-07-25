---
myst:
    substitutions:
        github_badge: "[![GitHub stars](https://img.shields.io/github/stars/liqotech/liqo.svg?logo=github)](https://github.com/liqotech/liqo/stargazers/)"
        license_badge: "[![GitHub license](https://img.shields.io/github/license/liqotech/liqo.svg)](https://github.com/liqotech/liqo/blob/master/LICENSE)"
        slack_badge: "[![Join slack](https://img.shields.io/badge/slack-liqo.io-blueviolet?logo=slack)](https://liqo-io.slack.com/join/shared_invite/zt-h20212gg-g24YvN6MKiD9bacFeqZttQ)"
        twitter_badge: "[![Twitter follow](https://img.shields.io/twitter/follow/liqo_io?style=flat&color=ff69b4&logo=twitter)](https://twitter.com/liqo_io)"
        documentation_badge: "[![Documentation status](https://readthedocs.org/projects/liqo/badge)](https://doc.liqo.io)"

---

<!-- markdownlint-disable first-line-h1 -->
<!-- Badges above the Liqo logo -->
```{cssclass} flex items-center justify-center gap-2
{{ github_badge }} {{ license_badge }} {{ slack_badge }} {{ twitter_badge }} {{ documentation_badge }}
```

<!-- Liqo logo image -->
````{cssclass} custom-logo-image
```{figure} _static/images/common/liqo-logo-blue.svg
:width: 400px
:align: center
:alt: Liqo
```
````

# What is Liqo?

Liqo is an open-source project that enables **dynamic and seamless Kubernetes multi-cluster topologies**, supporting heterogeneous on-premise, cloud and edge infrastructures.

## What does it provide?

````{grid} 1 1 1 2
:padding: 0
:gutter: 2

```{grid-item-card}
:class-header: sd-fs-5 sd-pt-3
:link: features/peering
:link-type: doc

{fa}`link;1.5rem` Peering
^^^

Automatic peer-to-peer establishment of **resource and service consumption relationships** between independent and heterogeneous clusters.
No need to worry about complex VPN configurations and certification authorities: everything is transparently **self-negotiated** for you.
```

```{grid-item-card}
:class-header: sd-fs-5 sd-pt-3
:link: features/offloading
:link-type: doc

{fa}`paper-plane;1.5rem` Offloading
^^^

Seamless **workloads offloading** to remote clusters, without requiring any modification to Kubernetes or the applications themselves.
**Multi-cluster is made native and transparent**: collapse an entire remote cluster to a **virtual node** compliant with the standard Kubernetes approaches and tools.
```

```{grid-item-card}
:class-header: sd-fs-5 sd-pt-3
:link: features/network-fabric
:link-type: doc

{fa}`globe;1.5rem` Network Fabric
^^^

A transparent **network fabric**, enabling multi-cluster **pod-to-pod** and **pod-to-service** connectivity, regardless of the underlying configurations and CNI plugins.
Natively **access the services** exported by remote clusters, and spread interconnected application components across multiple infrastructures, with all cross-cluster traffic flowing through **secured network tunnels**.
```

```{grid-item-card}
:class-header: sd-fs-5 sd-pt-3
:link: features/storage-fabric
:link-type: doc

{fa}`archive;1.5rem` Storage Fabric
^^^

A native **storage fabric**, supporting the remote execution of **stateful workloads** according to the **data gravity** approach.
Seamlessly extend standard (e.g., database) **high availability deployment techniques** to the multi-cluster scenarios, for **increased guarantees**.
All without the complexity of managing multiple independent cluster and application replicas.
```
````

## What to explore next?

````{grid} 1 1 1 2
:padding: 0
:gutter: 2

```{grid-item-card}
:class-header: sd-fs-5 sd-pt-3
:class-footer: sd-text-center sd-font-italic custom-fs-7
:link: examples/quick-start
:link-type: doc

{fa}`bolt;1.5rem` Quick start
^^^

New to Liqo? Would you like to know more?
Here you can find everything needed to set up an testing evironment, install Liqo and experiment its functionality with an "Hello World!" example.
```

```{grid-item-card}
:class-header: sd-fs-5 sd-pt-3
:class-footer: sd-text-center sd-font-italic custom-fs-7
:link: installation/requirements
:link-type: doc

{fa}`cloud-download;1.5rem` Installation
^^^

Ready to give Liqo a try?
Learn about installation and connectivity **requirements**, discover how to download and install **liqoctl**, the CLI tool to streamline the installation and management of Liqo, and explore the **customization options**, based on the target environment characteristics.
```

```{grid-item-card}
:class-header: sd-fs-5 sd-pt-3
:class-footer: sd-text-center sd-font-italic custom-fs-7
:link: examples/requirements
:link-type: doc

{fa}`search;1.5rem` Examples
^^^

Would you like to quickly join the fray and experiment with Liqo?
Set up your playground and check out the **getting started examples**, which will guide you through a scenario-driven tour of the most notable features of Liqo.
Discover how to **offload** (a subset of) your workloads, **access services** provided by remote clusters, **expose** multi-cluster applications, and more.
```

```{grid-item-card}
:class-header: sd-fs-5 sd-pt-3
:class-footer: sd-text-center sd-font-italic custom-fs-7
:link: usage/peer
:link-type: doc

{fa}`cogs;1.5rem` Usage
^^^

Do you want to make a step further and discover all the Liqo configuration options?
These guides get you covered!
Find out how to **establish and configure a peering** between two clusters, as well as how to enable and customize **namespace offloading**.
Explore the details about which and how native resources are **reflected** to remote clusters, and learn more about the support for **stateful applications**.
```
````
