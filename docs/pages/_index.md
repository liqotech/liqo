---
title: Liqo Docs
#chapter: true
---

![Liqo logo](/images/logo-liqo-blue.svg)

# Documentation

Liqo allows Kubernetes to seamlessly and securely share resources and services, enabling to run tasks on any other
cluster available nearby.

## Why Liqo

As Kubernetes gains adoption, clusters start to be everywhere: on private data-centers, on the cloud, at the edge of the network and so on. With Liqo, your applications and services can leverage those resources, by creating dynamic and opportunistic peerings of clusters.


Liqo is completely open source, and designed to be network plugin (CNI) and Kubernetes-distribution agnostic. Liqo does not require any modification to your Kubernetes cluster to work.

## What Liqo provides

* Automatic discovery of available clusters with Liqo installed
* Dynamic peering and resource sharing
* Support for inter-cluster connectivity with P2P parameter negotiation
* Transparent Multi-cluster pod offloading and service reconciliation
* Pod-to-pod and pod-to-service connectivity across the clusters, disregarding the installed CNI

[Let's get started!](gettingstarted)
