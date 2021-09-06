---
title: Quickstart
weight: 1
---

### Introduction

Liqo enables the creation of arbitrary multi-cluster topologies, proposing a seamless multi-cluster model which dynamically allows the pods and services to offload on remote clusters.

This section presents several tutorials that guide the discovery and use of the key Liqo features.

#### System Requirements

Before starting to run the following tutorials, you should have installed the following software on your system:

* [**Docker**](https://docker.io), the container runtime.
* [**Kubectl**](https://kubernetes.io/docs/tasks/tools/install-kubectl/), the command-line tool for Kubernetes.
* [**Helm**](https://helm.sh/docs/intro/install/), the package manager for Kubernetes.
* **curl**, to interact with the cluster through HTTP/HTTPS. In Ubuntu systems, you can install it via a simple `sudo apt update && sudo apt install -y curl`.

In addition, you should install the [**liqoctl**](/installation#liqoctl) command-line tool for Liqo. 

The following tutorials were tested on Linux, macOS, and Windows (WSL2 and Docker Desktop).

### Tutorials

* [Basic](./helloworld): a first _Hello World_-style tutorial introducing the basic Liqo features. In this tutorial, you will set up multiple clusters and establish the first peering among them.
* [Advanced](./extended): a more in-depth _Dominate the world_-style tutorial showing how to profit from Liqo advanced features.
