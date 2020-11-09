<p align="center">
<img alt="Liqo Logo" src="https://doc.liqo.io/images/logo-liqo-blue.svg" />
</p>

# Liqo

![Go](https://github.com/liqotech/liqo/workflows/Go/badge.svg) 
[![Coverage Status](https://coveralls.io/repos/github/liqotech/liqo/badge.svg?branch=master)](https://coveralls.io/github/liqotech/liqo?branch=master)
[![Go Report Card](https://goreportcard.com/badge/github.com/liqotech/liqo)](https://goreportcard.com/report/github.com/liqotech/liqo)
![Docker Pulls](https://img.shields.io/docker/pulls/liqo/virtual-kubelet?label=Liqo%20vkubelet%20pulls)
[![FOSSA Status](https://app.fossa.com/api/projects/git%2Bgithub.com%2Fliqotech%2Fliqo.svg?type=shield)](https://app.fossa.com/projects/git%2Bgithub.com%2Fliqotech%2Fliqo?ref=badge_shield)
[<img src="https://img.shields.io/badge/slack-liqo.io-yellow">](https://liqo-io.slack.com) 
[![Twitter](https://img.shields.io/twitter/url/https/twitter.com/liqo_io.svg?style=social&label=Follow%20%40liqo_io)](https://twitter.com/liqo_io)

Great for:
* Dynamic resource sharing across clusters, with decentralized governance
* Seamless multi-cluster management
* Edge clusters orchestration

Liqo is in an alpha version and not yet production-ready.

Community Meeting: Monday 18.30 CET, 9.30 PST [here](https://meet.google.com/dyr-ieso-smu)

# What is Liqo?

Liqo is a framework to enable dynamic sharing across Kubernetes Clusters. You can run your pods on a remote cluster
seamlessly, without any modification (Kubernetes or your application). 

Liqo is an open source project started at Politecnico of Turin that allows Kubernetes to seamlessly and securely share resources and services, so you can run your tasks on any other cluster available nearby.

Thanks to the support for K3s, also single machines can participate, creating dynamic, opportunistic data centers that include commodity desktop computers and laptops as well.

Liqo leverages the same highly successful “peering” model of the Internet, without any central point of control. New peering relationships can be established dynamically, whenever needed, even automatically. Cluster auto-discovery can further simplify this process.

## Quickstart

Create two [KinD](https://kind.sigs.k8s.io/) clusters via our script or bring your own clusters.  
N.B. Using our cluster, Docker has to be installed on your test machine and user should have permission to issue commands.  
N.B. Scripts are also compatible with `Windows` if you use `Mingw` (provided by default with `git bash`)

```bash
source <(curl -L https://get.liqo.io/clusters.sh)
```

Install Liqo on both clusters:

```bash
export KUBECONFIG=$KUBECONFIG_1
curl -L https://get.liqo.io | bash -s
export KUBECONFIG=$KUBECONFIG_2
curl -L https://get.liqo.io | bash -s
```

Wait that all containers are up and running (around 30 seconds). When a new virtual-kubelet pops out, a new node modeling the remote cluster is present and ready to receive pods. Check it out with:

```bash
kubectl get nodes -o wide
```

Let's use those resources. Deploy the [Google microservice Shop](https://github.com/liqotech/microservices-demo/blob/master/release/kubernetes-manifests.yaml) application via: 

```bash
kubectl create namespace demo-liqo
kubectl label namespace demo-liqo liqo.io/enabled=true
kubectl apply -f https://get.liqo.io/app.yaml -n demo-liqo
```

You can observe that:

* Your application is correctly working by exposing the application frontend port and later connecting with a browser to [localhost:8000](localhost:8000). To expose the pod port:
```bash
  kubectl port-forward -n demo-liqo service/frontend 8080:80
```
* Your application is transparently deployed across two different clusters:
```bash
  kubectl get pods -n demo-liqo -o wide  
``` 

To get more information about how to install Liqo in your own clusters and configure it, you can check out the [Liqo Documentation](https://doc.liqo.io/user/).

## Architecture

Liqo relies on several components:

* *Liqo Virtual Kubelet*: Based on [Virtual Kubelet](https://github.com/virtual-kubelet/virtual-kubelet) project, the VK
 is responsible to "masquerade" a foreign Kubernetes cluster.
* *Advertisement Operator/Broadcaster*: Those components embed the logic to advertise/accept resources from partner
 clusters and spawn new virtual kubelet instances
* *Liqonet Operators*: Those operators are responsible to establish Pod-to-Pod and Pod-to-Service connection across 
partner clusters.

...and some others. Check out the architecture [documentation](https://doc.liqo.io/architecture/)


## License
[![FOSSA Status](https://app.fossa.com/api/projects/git%2Bgithub.com%2Fliqotech%2Fliqo.svg?type=large)](https://app.fossa.com/projects/git%2Bgithub.com%2Fliqotech%2Fliqo?ref=badge_large)