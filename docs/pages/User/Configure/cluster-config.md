---
title: Cluster configuration
weight: 1
---

Liqo installer automatically installs a default configuration in your cluster. You can find this configuration in `deployments/liqo_chart/templates/clusterconfig.yaml`.
The configuration can be modified through `kubectl` or the Liqo dashboard.
There are three main sections you can configure:
* [**AdvertisementConfig**](#advertisement-configuration): defines the configuration for the advertisement protocol
* [**DiscoveryConfig**](#discovery-configuration): defines the configuration for the discovery protocol
* [**NetworkConfig**](#network-configuration): defines the configuration for the network modules

## Advertisement configuration

In this section you can configure your cluster behaviour regarding the Advertisement broadcasting and acceptance,
and the parameters for the [keepalive check](#keepalive-check):
* **OutgoingConfig** defines the behaviour for the creation of the Advertisement for other clusters.
  - `enableBroadcaster` flag allows you to enable/disable the broadcasting of your Advertisement to the foreign clusters your cluster knows
  - `resourceSharingPercentage` defines the percentage of your cluster resources that you will share with other clusters
* **IngoingConfig** defines the behaviour for the acceptance of Advertisements from other clusters.
  - `maxAcceptableAdvertisement` defines the maximum number of Advertisements that can be accepted over time
  - `acceptPolicy` defines the policy to accept or refuse a new Advertisement from a foreign cluster. The possible policies are:
    - `AutoAcceptMax`: every Advertisement is automatically checked considering the configured maximum;
    AutoAcceptAll policy can be achieved by setting MaxAcceptableAdvertisement to 1000000, a symbolic value representing infinite; AutoRefuseAll can be achieved by setting MaxAcceptableAdvertisement to 0
    - `ManualAccept`: every Advertisement needs to be manually accepted or refused; this mode is not implemented yet.

### Keepalive check

After establishing a sharing with a foreign cluster (i.e. you have received an Advertisement and are using that cluster resources), a keepalive mechanism starts,
in order to know if the foreign cluster is reachable or not. In the AdvertisementConfig you can configure:
* `KeepaliveThreshold`: the number of failed attempts to contact the foreign cluster your cluster will tolerate before deleting it.
* `KeepaliveRetryTime`: the time between an attempt and the next one.

## Discovery configuration

## Network configuration
