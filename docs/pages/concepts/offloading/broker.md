---
title: Broker
weight: 4
---

### Overview

The **broker** is a component that facilitates resource sharing in a scenario with multiple cloud providers. Depending on the model it can act as a resource catalog or a real-time aggregator of resource advertisements.

For the purposes of this document, "provider" and "consumer" refer to entities (single users, institutions, etc.) that respectively offer computing resources or have a demand for resources.

#### Resource catalog

A **resource catalog** is a type of broker that simply collects resource advertisements from different cloud providers. The consumer will use this information to choose a provider, and then peer directly with the provider; no further interaction with the broker is needed.

#### Transparent broker

A **transparent broker** collects resource advertisements from different cloud providers, and acts as an intermediary in the peering and offloading process. After choosing a provider, the consumer will peer with the broker and offload deployments to it; the broker is in charge of offloading these in turn to the chosen provider.

#### Opaque broker

An **opaque broker** collects resource advertisements from different cloud providers and aggregates them (typically as a simple sum), exposing to the consumer a global view of the providers' resources. Unlike in a transparent broker, the consumer is not aware of individual providers or advertisements and thus is not responsible for choosing a provider, shifting this responsibility to the opaque broker.

To create an opaque broker, simply use `liqoctl install --broker` when installing Liqo, or set `controllerManager.pod.extraArgs` to `['--broker-mode=true']` in `values.yml` if you're doing a Helm install. Note that a Helm install allows you to configure the aggregation policies, namely `controllerManager.config.resourceSharingPercentage` will allow you to apply a factor to the sum of resources.

You will then need to add an peering towards each provider that you want to expose to the user. Depending on your setup it may happen automatically or you may need to paste the output of `liqoctl generate-add-command` from the provider to the broker; refer to [the user guide](/concepts/peering/_index.md) for more information. Likewise, users will need to peer with you.

Once the broker are peered with consumers you are ready to accept workloads. From the point of view of the consumer, the process is no different from [offloading](/concepts/offloading/_index.md) to any other cluster: just label the namespace with `liqo.io/enabled=true` or create a `NamespaceOffloading` resource. In fact, is is also a standard offloading from the point of view of the broker: you will find that a `NamespaceOffloading` is automatically created for each namespace that the consumer offloads on the broker. This means that you can use the standard tools to debug offloaded workloads:
 - Use `kubectl describe foreigncluster <name>` to understand why a provider is not reachable or accepting pods;
 - Use `kubectl describe namespaceoffloading -n <namespace> offloading` to understand the labels for selecting providers and which providers are selected.

Pod scheduling is done dynamically, as well as resource updates. This means that if providers go down or have a change in resources available, the broker will automatically reschedule pods with no intervention from the broker operator or the consumer.