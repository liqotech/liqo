---
title: Quickstart
weight: 1
---

### Introduction

Liqo enables the creation of arbitrary multi-cluster topologies, proposing a seamless multi-cluster model which dynamically allows the pods and services to be offloaded on remote clusters.

This section presents several tutorials that guide to the discovery and use of the most important features of Liqo.

#### System Requirements

Before starting to run the following tutorials, you should have installed on your system:

* [**Docker**](https://docker.io), the container runtime.
* [**Kubectl**](https://kubernetes.io/docs/tasks/tools/install-kubectl/), the command line tool for Kubernetes.
* [**Helm**](https://helm.sh/docs/intro/install/), the package manager for Kubernetes.
* **curl**, to interact with the cluster through HTTP/HTTPS. In Ubuntu it can be installed with `sudo apt update && sudo apt install -y curl`.

The above tutorials have been tested on Linux, macOS, and Windows (through Docker Desktop).

### Tutorials

* [Basic](./helloworld): a first _Hello World_-style tutorial to introduce basic Liqo features. In this tutorial, you will setup multiple clusters and establish the first peering among them.
* [Advanced](./extended): a more in-depth _Dominate the world_-style tutorial showing how to use the most important features of Liqo.
