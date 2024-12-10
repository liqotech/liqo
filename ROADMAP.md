# Roadmap

tl;dr: Liqo **1.0.0 GA** to be released around the beginning of March 2025

The next release of Liqo (`v1.0.0`) will include many breaking changes, due to the necessity to bring more modularity and flexibility to the project, a cleaner and more granular control (e.g., fully declarative APIs to control the most important features of Liqo), and other features.
Even more important, the Liqo team believes that the project is now mature and stable enough to be safely deployed in production environments, and to consolidate most of its APIs that were defined as `alpha` in the past. This is reflected in the change in the numbering of the Liqo version, now `v1.0.0`.
Therefore, the Liqo team has decided to publish incremental release candidates for this version, allowing early adopters to discover and test the new features in advance, hence potentially reducing the disruption (and painful migration strategies) to the current adopters.
Liqo `v1.0.0` (GA) is planned to be released around the beginning of March 2025.

The most relevant features for this release are the following:

- [Feature] Liqo is now structured in three core modules (*network*, *authentication*, and *offloading*) that are totally independent and can be individually configured and used (e.g. you can enable offloading or resource reflection without necessarily setting up the network connectivity between the clusters, which can be provided by other third-party tools).

- [Feature] Fully declarative APIs to configure and control Liqo.
This approach allows users to support gitops and automation use cases, as Liqo can be completely configured via *CRs*, without necessarily relying on *liqoctl* (e.g., perform peering declaratively).

- [Feature] Network module: complete re-design of the network fabric, involving a new communication model *inside* the cluster (i.e., intra-cluster traffic now flows inside the CNI thanks to *node-to-gateway* `geneve` tunnels instead of a *node-to-node* `vxlan` overlay) and a new, simplified model for the inter-cluster connectivity (still based on `wireguard`, but more robust and open to other technologies).

- [Feature] Authentication module: the peering authentication between clusters is now fully declarative, simpler (it does not require exposing a dedicated auth service anymore as well as no API server exposition is necessary from the consumer side), and more robust overall (e.g. fixing a broken peer is easier and fast).
Moreover, the new `ResourceSlice` CR allows a more granular and flexible control of the resources requested to cluster providers.

- [Feature] Offloading module: it is now possible to have multiple *virtual nodes* targeting the same remote provider cluster.
This allows, for example, to split the resources of bigger clusters across multiple virtual nodes, or to have nodes with specific resources (e.g. GPUs) or characteristics (e.g. specific architectures).
It can also be used to share huge resource pools with another cluster while keeping the virtual nodes size quite small, avoiding a "black hole" effect during scheduling.

There are many more news and features, that will be presented in dedicated blog posts.
