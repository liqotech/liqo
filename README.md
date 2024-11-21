<!-- markdownlint-disable first-line-h1 -->
<p align="center">
  <a href="https://github.com/liqotech/liqo/actions/workflows/codeql.yml"><img src="https://github.com/liqotech/liqo/actions/workflows/codeql.yml/badge.svg" alt="Integration Pipeline Status"></a>
  <a href="https://goreportcard.com/report/github.com/liqotech/liqo"><img src=https://goreportcard.com/badge/github.com/liqotech/liqo></a>
  <a href="https://docs.liqo.io/en/stable" alt="Liqo's Documentation"><img src="https://readthedocs.org/projects/liqo/badge/"></a>
  <a href="https://liqo-io.slack.com/join/shared_invite/zt-h20212gg-g24YvN6MKiD9bacFeqZttQ"><img src=https://img.shields.io/badge/slack-liqo.io-blueviolet?logo=slack></a>
  <a href="https://twitter.com/liqo_io"><img src=https://img.shields.io/twitter/follow/liqo_io?style=flat&color=ff69b4&logo=twitter></a>

  <br />
  <a href="https://docs.liqo.io/en/stable/installation/?provider=GKE"><img src=https://img.shields.io/badge/Google%20GKE-supported-green></a>
  <a href="https://docs.liqo.io/en/stable/installation/?provider=AKS" ><img src=https://img.shields.io/badge/Azure%20AKS-supported-green></a>
  <a href="https://docs.liqo.io/en/stable/installation/?provider=EKS"><img src=https://img.shields.io/badge/Amazon%20EKS-supported-green></a>
  <a href="https://docs.liqo.io/en/stable/installation/?provider=OpenShift%20Container%20Platform%20(OCP)"><img src=https://img.shields.io/badge/Openshift-supported-green></a>
  <br />
  <br />
  <br />

  <a href="https://github.com/liqotech/liqo">
    <img alt="Liqo Logo" src="docs/_static/images/common/liqo-logo-blue.svg" height="80">
  </a>
  <br />

  <h3 align="center">Enable dynamic and seamless Kubernetes multi-cluster topologies</h3>
  <br />
</p>

<p align="center">
    <a href="https://docs.liqo.io/"><strong>Explore the docs »</strong></a>
    <br />
    <br />
    <a href="https://www.youtube.com/channel/UCYbWJMfwy3P6xT4JI_K84xw">View Videos</a>
    ·
    <a href="https://github.com/liqotech/liqo/issues/new?assignees=&labels=&template=bug_report.md&title=">Report Bug</a>
    ·
    <a href="https://github.com/liqotech/liqo/issues/new?assignees=&labels=enhancement&template=feature_request.md&title=%5BFeature%5D">Request Feature</a>
</p>

## What is Liqo?

Liqo is an open-source project that enables dynamic and seamless Kubernetes multi-cluster topologies, supporting heterogeneous on-premise, cloud and edge infrastructures.

## What does it provide?

* **Peering**: automatic peer-to-peer establishment of resource and service consumption relationships between independent and heterogeneous clusters.
  No need to worry about complex VPN configurations and certification authorities: everything is transparently self-negotiated for you.
* **Offloading**: seamless workloads offloading to remote clusters, without requiring any modification to Kubernetes or the applications themselves.
  Multi-cluster is made native and transparent: collapse an entire remote cluster to a virtual node compliant with the standard Kubernetes approaches and tools.
* **Network fabric**: transparent multi-cluster pod-to-pod and pod-to-service connectivity, regardless of the underlying configurations and CNI plugins.
  Natively access the services exported by remote clusters, and spread interconnected application components across multiple infrastructures, with all cross-cluster traffic flowing through secured network tunnels.
* **Storage fabric**: support for remote execution of stateful workloads, according to the data gravity approach.
  Seamlessly extend standard (e.g., database) high availability deployment techniques to the multi-cluster scenarios, for increased guarantees.
  All without the complexity of managing multiple independent cluster and application replicas.

## Quick start

Would you like to quickly join the fray and experiment with Liqo?
Set up your playground and check out the getting started examples, which will guide you through a scenario-driven tour of the most notable features of Liqo:

* [Quick Start](https://docs.liqo.io/en/stable/examples/quick-start.html): grasp a quick overview of what Liqo can do.
* [Offloading with Policies](https://docs.liqo.io/en/stable/examples/offloading-with-policies.html): discover how to tune namespace offloading, and how to use policies to select which clusters may host each workload.
* [Offloading a Service](https://docs.liqo.io/en/stable/examples/service-offloading.html): learn how to create a multi-cluster service, and how to consume it from each connected cluster.
* [Stateful Applications](https://docs.liqo.io/en/stable/examples/stateful-applications.html): find out how to deploy a database across a multi-cluster environment, leveraging the Liqo storage fabric.
* [Global Ingress](https://docs.liqo.io/en/stable/examples/global-ingress.html): discover how route external traffic to multi-cluster applications through a global ingress and automatic DNS configurations.
* [Replicated Deployments](https://docs.liqo.io/en/stable/examples/replicated-deployments.html): learn how to deploy an application by replicating it on multiple remote clusters.
* [Provision with Terraform](https://docs.liqo.io/en/stable/examples/provision-with-terraform.html): explore Liqo Terraform provider capabilities.

### Going Further

Got curious?
Check out the [documentation website](https://docs.liqo.io) for an in-depth overview of the Liqo features, to discover how to install Liqo on your clusters, as well as to find out about the different usage and configuration options.

## Roadmap

Want to know about the features to come? Check out the [project roadmap](ROADMAP.md) for more information.

## Contributing

All contributors are warmly welcome. If you want to become a new contributor, we are so happy! Just, before doing it, read the tips and guidelines presented in the [dedicated documentation page](https://docs.liqo.io/en/stable/contributing/contributing.html).

## Community

To get involved with the Liqo community, join the [Slack workspace](https://liqo-io.slack.com/join/shared_invite/zt-h20212gg-g24YvN6MKiD9bacFeqZttQ).

|:bell: Community Meeting|
|------------------|
|Liqo holds community meetings to discuss directions and options with the community. Please refer to the Liqo Slack workspace to see the date/time of the next meeting.|

## License

This project includes code from the [Virtual Kubelet project](https://github.com/virtual-kubelet/virtual-kubelet), licensed under the Apache 2.0 license.

Liqo is distributed under the Apache-2.0 License. See [License](LICENSE) for more information.

<p align="center">
Liqo is a project kicked off at Polytechnic of Turin (Italy) and actively maintained with :heart: by all the Liqoers.
</p>
