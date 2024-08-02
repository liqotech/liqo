# Roadmap

The next release of Liqo will include many breaking changes, due to the necessity to bring more modularity to the project, a cleaner and more granular control (e.g., public APIs to control the most important features of Liqo), and other features.
Hence, the Liqo team has decided to release the most breaking updates within a single release, hence limiting the disruption (and potentially painful migration strategies) to the current adopters.
Hence, the original plans to release v0.11 in Feb 2024 have been rescheduled to Jun 2024, with the following main features:

* [Feature] Complete re-design of the network fabric, involving a new communication model _inside_ the cluster (i.e., pod-based communication instead of host-based services; geneve instead of vxlan) and a new, simplified model for the intra-cluster connectivity (still based on Wireguard, but more robust and open to other technologies).
* [Feature] CRD-based network modularity. All the network components can be individually controlled with new CRDs, hence enabling the different components of Liqo to be turned on/off and configured in a more granular way.
