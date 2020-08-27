---
title: Cluster configuration
weight: 1
---

Liqo installer automatically installs a default configuration in your cluster. You can find this configuration in `deployments/liqo_chart/templates/clusterconfig.yaml`.
The configuration can be modified through kubectl or the Liqo dashboard.
There are 3 main sections you can configure:
* [**AdvertisementConfig**](#advertisement-configuration): defines the configuration for the advertisement protocol
* [**DiscoveryConfig**](#discovery-configuration): defines the configuration for the discovery protocol
* [**NetworkConfig**](#network-configuration): defines the configuration for the network modules

## Advertisement configuration

In this section you can configure your cluster behaviour regarding the Advertisement broadcasting and acceptance:
* **OutgoingConfig** defines the behaviour for the creation of the Advertisement for other clusters.
  - `enableBroadcaster` flag allows you to enable/disable the broadcasting of your Advertisement to the foreign clusters your cluster knows
  - `resourceSharingPercentage` defines the percentage of your cluster resources that you will share with other clusters
* **IngoingConfig** defines the behaviour for the acceptance of Advertisements from other clusters.
  - `maxAcceptableAdvertisement` defines the maximum number of Advertisements that can be accepted.
  - `acceptPolicy` defines the policy to accept or refuse a new Advertisement from a foreign cluster. The possible policies are:
    - `AutoAcceptWthinMaximum`: every Advertisement is automatically checked considering the configured maximum.
    - `ManualAccept`: every Advertisement needs to be manually accepted or refused; this mode is not implemented yet.
 
## Discovery configuration

## Network configuration
