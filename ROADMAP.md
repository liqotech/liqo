# Roadmap

The roadmap for the  next release (v0.11, expected February 2024) is the following:

* [Feature] Complete re-design of the network fabric, involving a new communication model _inside_ the cluster (i.e., pod-based communication instead of host-based services; geneve instead of vxlan) and a new, simplified model for the intra-cluster connectivity (still based on Wireguard, but more robust and open to other technologies).
* [Feature] CRD-based network modularity. All the network components can be individually controlled with new CRDs, hence enabling the different components of Liqo to be turned on/off and configured in a more granular way.
