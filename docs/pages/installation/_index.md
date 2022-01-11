---
title: Installation
weight: 2
---

The default installation of Liqo is rather simple; however, Liqo offers extended customization capabilities that can be useful for production clusters and that may require a deeper understanding of the configuration options.

* [Connectivity requirements](connect-requirements): what you need to know to connect and peer to a Liqo cluster.
* [Installation](install): how to install Liqo.
* [Advanced configuration](install-advanced): advanced configuration options.
* [Helm Chart values](chart_values): detailed description of all the configuration options.

{{% notice info %}}
If you are using Calico on your cluster, __YOU MUST READ__ the [Calico configuration section](calico-configuration) before installing Liqo, otherwise you may end up breaking your set-up.
{{% /notice %}}

After you have successfully installed Liqo, you may:

* [Configure Liqo](/configuration): tune security properties, the automatic discovery of new clusters and other system parameters.
* [Use Liqo](/usage): orchestrate your applications across multiple clusters.
