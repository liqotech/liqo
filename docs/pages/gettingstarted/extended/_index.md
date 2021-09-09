---
title: Advanced tutorial
weight: 2
---

The following steps will guide you through a tour to learn how to use the core Liqo features.

* [Set up the Playground](./kind): Deploy 3 KiND (Kubernetes in Docker) clusters.
* [Install Liqo](./install): Install Liqo on all the above clusters.
* [Enable Peering](./peer): Setup a peering between the _home_ cluster and the other _remote_ clusters.
* [Selective Offloading](./select_clusters): Offload a service on a _selected_ remote cluster, using the Liqo *selective offloading*.
* [Managing hard constraints](./hard_constraints): Explain how Liqo handle the possible violation of the offloading constraints.
* [Change topology](./change_topology): Show how change which remote clusters are used for the offloading (without modifying the current peerings).
* [Access to remote services](./remote_service_access): Explain how the local cluster can contact Services whose Endpoints are running on remote clusters.
* [Dynamic topology](./dynamic_topology): Show how Liqo reach to a cluster topology update (e.g., an existing peering is turned off).
* [Uninstall Liqo](./uninstall): Uninstall Liqo from your clusters.






