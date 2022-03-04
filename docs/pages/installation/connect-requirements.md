---
title: Connectivity requirements
weight: 1
---

## Introduction

Liqo has several ways to discover new clusters (i.e., LAN, DNS, Manual), as detailed in the [Discovery mechanisms page](/configuration/discovery).
Peering is the next step; however, peering requires that clusters satisfy some (minimal) _network_ requirements, whose configuration depends (1) on the type of connectivity between the two clusters and (2) on the configuration chosen during the Liqo installation phase.

### Connectivity requirements

In order to successfully establish a peering with another cluster, the following services need to be _reciprocally accessible_ on both clusters (i.e., in terms of IP address/port):

* **Authentication service** (`liqo-auth`): Liqo service used to authenticate incoming peering requests coming from other clusters.
* **Network gateway service** (`liqo-gateway`): Liqo service responsible for setting up the network connectivity (i.e., VPN-like) between clusters.
* **Kubernetes API server**: standard Kubernetes API Server, that is used by the (remote) Liqo instance to create the resources required to start the peering process.

_Reciprocally accessible_ means that a first cluster must be able to connect to the <IP/port> of the above services exposed on the second cluster, and vice versa (from second cluster to the first); some exceptions that refer to the Network gateway are detailed in the following of this page.
This implies also that any network device (**NAT**, **firewall**, etc.) sitting in the path between the two clusters must be configured to enable direct connectivity toward the above services, as presented in the [Network firewalls](#network-firewalls) section.

The tuple <IP/port> exported by the Liqo services (i.e., `liqo-auth`, `liqo-gateway`) depends on the Liqo configuration, chosen at Liqo installation time, which may depend on the physical setup of your cluster and the characteristics of your service.

**Auth Service**: when you install Liqo, you can choose to expose this service through a LoadBalancer service, a NodePort service, or an Ingress controller (the latter requires the service to be exposed as ClusterIP).
This choice depends (1) on your necessities, (2) on the cluster configuration (e.g., the NodePort cannot be used if your nodes have private IP addresses, hence cannot be reached from the Internet), and (3) whether the above primitives (e.g., the Ingress controller) are available in your cluster.
For instance, if you play with a "toy" cluster such as one created with KIND, you may have neither a LoadBalancer service, nor an ingress controller, hence you may be forced to use a NodePort service.

**Network Gateway**: the same applies also for the Network Gateway, except that it cannot be exported through an Ingress. In fact, while the Auth service uses a standard HTTP/REST interface, the Network gateway is the termination of a network tunnel; hence only LoadBalancer and NodePort services are supported.

We suggest installing Liqo through the [liqoctl install](/installation/install) command; this tool analyzes the characteristics of your cluster and makes reasonable choices for you.
However, more advanced configuration can be achieved by setting the proper parameters in the [Liqo Helm Chart](/installation/chart_values) before launching the Liqo installation, as detailed in the [advanced installation option page](/installation/install-advanced).

In particular, advanced connectivity requirements may require the explicit setting of the following parameters:

* **Authentication service** (`liqo-auth`): Configured in the ``auth`` section of the [Liqo chart values](/installation/chart_values).
* **Network gateway service** (`liqo-gateway`): Configured in the ``gateway`` section of the [Liqo chart values](/installation/chart_values).
* **Kubernetes API server**: Configured in the ``apiServer`` section of the [Liqo chart values](/installation/chart_values).
Although in most cases _liqoctl_ can derive the IP/port used by your API server, it is possible to manually override the above coordinates in the Liqo configuration.
Remember also that this is not a Liqo service, but the control plane of your Kubernetes cluster. Hence, this value cannot be used to _update_ the endpoint of the API server, but simply to tell Liqo which is the <IP/port> used by that endpoint.

The above values can also be updated after installation by changing the values in your `values.yml`, then issuing the ``helm update`` command.

Pay attention that changing exposition parameters may affect (hence, break!) active peerings. We suggest disabling all peerings before updating the Liqo exposition config.

An overview of the overall connectivity requirements of Liqo is shown in the figure below.

![](/images/scenarios/connect-requirements.svg)

### Connectivity workflow during cluster peering

In order to successfully establish a peering, Liqo undergoes through the following steps:

1) Cluster A connects to the Auth Service of cluster B to authenticate itself and for the initial exchange of parameters. This step requires also that Cluster B connects to the Auth Service of cluster A to complete this phase.
2) Once done, Cluster A connects to the API Server of cluster B to configure the proper resources to establish a peering. This step requires also that Cluster B connects to the API server of cluster A to complete this phase.
3) Once done, Cluster A connects to the Network Gateway of cluster B to establish a direct tunnel (i.e., VPN-like) for all the traffic between the two clusters.

Although in some cases the step (3) may be completed successfully even if a single Network Gateway is publicly reachable (Liqo will try to establish the connection from A to B; if this fails, it forces the connection starting from B to A), we strongly suggest to publicly expose both Network Gateways in order to avoid problems in more advanced deployment options (e.g., a peering between A and B-C, followed by an automatic direct peering between B-C as a shortcut).

### Examples of possible Liqo peering scenarios

Liqo can be installed on different types of clusters, either on-premise or on public cloud providers.
Therefore, a large number of peering scenarios are possible; among the most common options, we can cite:

* public cloud (e.g., AKS, EKS, GKE) to public cloud
* public cloud to on-premise
* on-premise to on-premise
* on-LAN to on-LAN (mainly for testing purposes)

In some cases, especially on production setups, additional network limitations are present, such as firewalls that may impair network connectivity, which must be considered in order to enable Liqo peering.

Finally, in some cases clusters may reside behind a NAT, which may also introduce additional limitations, and it may require the configuration of some port-forwarding rules.

#### Additional considerations

The choice of the way you expose Liqo services to remote cluster may not be trivial in some cases.
Here we list some additional notes you should consider in your choice:

* **NodePort service**: although a NodePort service can be used to expose the Auth service and the Network gateway, often the IP addresses of the nodes are configured with private IP addresses, hence not being suitable for connections originated from the Internet. This happens rather often in production clusters, and on public clusters as well.
* **Ingress controller**: in case the Auth service is exposed through an Ingress, you should remember that, by default, the Auth service uses the TLS protocol. Hence, either you configure your Ingress controller to connect to backend services with TLS as well, or you disable TLS on the Auth Service.
* **API Server behind NAT**: in case the cluster is behind NAT, the IP address/port of the API server (i.e., `apiServer.address` in `values.yaml`) must be set as the _public_ <IP/port> tuple.
* **Auth service behind NAT**: in case your cluster is behind NAT, the `liqoctl add cluster` command used by the remote cluster to peer with you must be adapted to use the _public_ <IP/port> tuple of the Auth service.

Finally, currently Liqo supports only scenarios in which _one_ of the cluster is **behind NAT**; network connectivity cannot be established in case both clusters are behind NAT. This limitation should be addressed in future releases.

### Clusters in the same LAN

This represents a very particular scenario supported by Liqo, in which clusters (K3s or K8s) in the same LAN can rely on the mDNS-based discovery mechanism implemented by Liqo to identify all the other clusters present on the same LAN segment.
This enables easy cluster setup (and peering) e.g., in case of rapid prototyping.

![](/images/scenarios/on-prem-to-on-prem.svg)

Although other alternatives are possible, we suggest the easiest configuration in this case, which relies on the following options:

|           | Cluster A (On-LAN)  | Cluster B (On-LAN) |
| --------- | ------------------- | ------------------- |
| **Auth service** |  NodePort | NodePort |
| **Network gateway** | NodePort | NodePort |
| **API server** | Exposed | Exposed |

### Network firewalls

Once Liqo has been installed, you may have to configure existing firewalls to enable remote clusters to contact the three endpoints (`liqo-auth`, `liqo-gateway`, API server) that need to be publicly accessible in the peering phase.

To know the network parameters (i.e., <IP/port>) used by `liqo-auth` and `liqo-gateway`, you can use standard Kubernetes commands (e.g., `kubectl get services -n liqo`), while you should know the <IP/port> used by your API server since it is written in your `kubeconfig` file.

For example, you can _open_ three ports (actually, three couples <IP/port>) to enable the Liqo peering; you can also specify more stringent rules (e.g., involving also IP source addresses) in order to limit the risks into your own network.

Remember that API server and Auth Service, by default, use the HTTPS protocol (over TCP); vice versa, the Network Gateway uses the Wireguard protocol over UDP.
