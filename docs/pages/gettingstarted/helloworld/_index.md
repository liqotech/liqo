---
title: Basic tutorial
weight: 1
---

## Basic tutorial

This tutorial aims at presenting how to install Liqo and practice with its most notable capabilities. It is is organized in the following steps:

* [Provision your playground](./kind): Deploy a couple of Kubernetes in Docker (KiND) clusters to play with Liqo
* [Install Liqo](./install): Install Liqo on a first cluster (*home* cluster).
* [Peer to a foreign cluster](./peer): establish a peering with a second Liqo cluster (*foreign* cluster).
* [Leverage foreign resources](./test): start an *Hello World* application to verify that the two peered clusters can actually share resources correctly and that you are able to run a pod in a foreign cluster.
* [Play with a micro-service application](./play): play with a more structured application that includes multiple micro-services, to demonstrate the advanced capabilities of Liqo.
* [Uninstall Liqo](./uninstall): uninstall Liqo from your cluster.
