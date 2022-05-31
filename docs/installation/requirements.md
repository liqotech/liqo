# Requirements

This page presents an overview of the main requirements, both in terms of **resources** and **network connectivity**, to use Liqo and successfully establish peerings with remote clusters.

## Overview

Typically, Liqo requires very **limited resources** (i.e., in terms of CPU, RAM, and network bandwidth) for the control plane execution, and it is compatible with both standard clusters and more **resource constrained devices** (e.g., Raspberry Pi), leveraging K3s as Kubernetes distribution.

The exact numbers depend on the **number of established peerings and offloaded pods**, as well as on the **cluster size** and whether it is leveraged in testing or production scenarios.
As a rule of thumb, the Liqo control plane as a whole, executed on a two-node KinD cluster, peered with a remote cluster, and while offloading 100 pods, conservatively demands for less than:

* Half a CPU core (only during transient periods, while CPU consumption is practically negligible in all the other instants).
* 200 MB of RAM (this metric increases the more pods are offloaded to remote clusters).
* 5 Mbps of cross-cluster control plane traffic (only during transient periods). Data plane traffic, instead, depends on the applications and their placements across the clusters.

A thorough analysis of the Liqo performance compared to vanilla Kubernetes, including the characterization of the resources consumed by Liqo, is presented in a [dedicated blog post](https://medium.com/the-liqo-blog/benchmarking-liqo-kubernetes-multi-cluster-performance-d77942d7f67c).

## Connectivity

As detailed in the [peering section](/features/peering), Liqo supports two alternative peering approaches, each characterized by **different requirements in terms of network connectivity** (i.e., mutually reachable endpoints).
Specifically, the establishment of an [**out-of-band control plane peering**](FeaturesPeeringOutOfBandControlPlane) necessitates **three separated traffic flows** (hence, exposed endpoints), while the [**in-band control plane peering**](FeaturesPeeringInBandControlPlane) approach relaxes this requirement to a **single endpoint**, as all control plane traffic is tunneled inside the cross-cluster VPN.

```{admonition} Note
The two peering approaches are **non-mutually exclusive**.
In other words, a single cluster can leverage different approaches toward different remote clusters, in case all connectivity requirements are fullfilled.
```

(InstallationRequirementsOutOfBandControlPlane)=

### Out-of-band control plane peering

In order to successfully establish an out-of-band control plane peering with a remote cluster, the following three services need to be **reciprocally accessible** on both clusters (i.e., in terms of IP address/port):

* **Authentication service** (`liqo-auth`): the Liqo service used to authenticate incoming peering requests coming from other clusters.
* **Network gateway** (`liqo-gateway`): the Liqo service responsible for the setup of the cross-cluster VPN tunnels.
* **Kubernetes API server**: the standard Kubernetes API Server, that is used by the (remote) Liqo instance to create the resources required to start the peering process, and perform workload offloading.

*Reciprocally accessible* means that a first cluster must be able to connect to the *<IP/port>* of the above services exposed on the second cluster, and vice versa (i.e., from second cluster to the first); some exceptions that refer to the network gateway are detailed in the following of this page.
This implies also that any network device (**NAT**, **firewall**, etc.) sitting in the path between the two clusters must be configured to **enable direct connectivity** toward the above services, as presented in the [network firewalls](RequirementsConnectivityFirewall) section.

The tuple *<IP/port>* exported by the Liqo services (i.e., `liqo-auth`, `liqo-gateway`) depends on the Liqo configuration, chosen at installation time, which may depend on the physical setup of your cluster and the characteristics of your service.

**Authentication Service**: when you install Liqo, you can choose to expose the authentication service through a *LoadBalancer* service, a *NodePort* service, or an *Ingress* (the latter allows the service to be exposed as *ClusterIP*).
This choice depends (1) on your necessities, (2) on the cluster configuration (e.g., a *NodePort* cannot be used if your nodes have private IP addresses, hence cannot be reached from the Internet), and (3) whether the above primitives (e.g., the *Ingress Controller*) are available in your cluster.

**Network Gateway**: the same applies also for the network gateway, except that it cannot be exported through an *Ingress*.
In fact, while the authentication service uses a standard HTTP/REST interface, the network gateway is the termination of a UDP-based network tunnel; hence only *LoadBalancer* and *NodePort* services are supported.

```{admonition} Note
Liqo supports scenarios in which only one of the two network gateway is publicly reachable from the remote cluster (i.e., in terms of *<IP/port>* tuple), although communication must be allowed by possible firewalls sitting in the path.
```

By default, *liqoctl* exposes both the authentication service and the network gateway through a **dedicated *LoadBalancer* service**, falling back to a *NodePort* for simpler setups (i.e., KinD and K3s).
However, more advanced configurations can be achieved by configuring the proper [Helm chart parameters](https://github.com/liqotech/liqo/tree/master/deployments/liqo), either directly or customizing the installation process [through *liqoctl*](InstallCustomization).

An overview of the overall connectivity requirements to establish out-of-band control plane peerings in Liqo is shown in the figure below.

![Out-of-band peering network requirements](/_static/images/installation/requirements/out-of-band.drawio.svg)

#### Additional considerations

The choice of the way you expose Liqo services to remote cluster may not be trivial in some cases.
Here, we list some additional notes you should consider in your choice:

* **NodePort service**: although a *NodePort* service can be used to expose the authentication service and the network gateway, often the IP addresses of the nodes are configured with private IP addresses, hence not being suitable for connections originated from the Internet.
This happens rather often in production clusters, and on public clusters as well.
* **Ingress controller**: in case the authentication service is exposed through an *Ingress*, you should remember that, by default, the authentication service uses the TLS protocol.
Hence, either you configure your *Ingress Controller* to connect to backend services with TLS as well, or you disable TLS on the authentication service.

Finally, in some cases clusters may reside behind a NAT.
Liqo transparently supports scenarios with **one cluster behind NAT** and the other publicly reachable.
Yet, in such situations, we suggest leveraging the in-band peering, as it simplifies the overall configuration.

(InstallationRequirementsInBandControlPlane)=

### In-band control plane peering

The establishment of an in-band control plane peering with a remote cluster requires only that the **network gateways are *mutually* reachable**, since all the Liqo control plane traffic is then configured to flow inside the VPN tunnel.
All considerations presented above and referring to the exposition of the network gateway apply also in this case.

Given the connectivity requirements are a subset, this solution is compatible with the configurations enabling the out-of-band peering approach.
Additionally, it:

* Supports scenarios characterized by a **non publicly accessible Kubernetes API Server**.
* Allows to expose the authentication service as a *ClusterIP* service, reducing the number of externally exposed services.
* Enables setups with one cluster **behind NAT**, since the VPN tunnel can be established successfully even in case only one of the two network gateways is publicly reachable from the other cluster.

An overview of the overall connectivity requirements to establish in-band peerings in Liqo is shown in the figure below.

![In-band peering network requirements](/_static/images/installation/requirements/in-band.drawio.svg)

```{warning}
Due to current limitations, the establishment of an in-band peering may not complete successfully in case the authentication service is exposed through an Ingress, delegating to it TLS termination (i.e., when TLS is disabled on the authentication service).
```

(RequirementsConnectivityFirewall)=

### Network firewalls

In some cases, especially on production setups, additional network limitations are present, such as firewalls that may impair network connectivity, which must be considered in order to enable Liqo peering.

Depending on your configuration and the selected peering approach, you may have to configure existing firewalls to enable remote clusters to contact either the `liqo-gateway` only or all the three endpoints (i.e., `liqo-auth`, `liqo-gateway` and Kubernetes API server) that need to be publicly accessible in the peering phase.

To know the network parameters (i.e., <IP/port>) used by `liqo-auth` and `liqo-gateway`, you can use standard Kubernetes commands (e.g., `kubectl get services -n liqo`), while the <IP/port> tuple used by your Kubernetes API server is the one written in the `kubeconfig` file.

Remember that the Kubernetes API server and authentication service use the HTTPS protocol (over TCP); vice versa, the network gateway uses the [WireGuard](https://www.wireguard.com/) protocol over UDP.
