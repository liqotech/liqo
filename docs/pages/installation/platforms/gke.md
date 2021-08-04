---
title: GKE
weight: 3
---

* [Introduction](#introduction)
* [Create a GKE Cluster](#create-a-gke-cluster)
* [Deploy Liqo](#deploy-liqo)
* [Check that Liqo is Running](#check-that-liqo-is-running)

### About GKE

Google Kubernetes Engine (GKE) is a managed Kubernetes service available on the Google Cloud. The GKE environment consists of multiple machines (specifically, Compute Engine instances) grouped to form a cluster.

### Scenarios

This guide will show you how to install Liqo on your GKE cluster. GKE clusters have by default an Internet-exposed API Service and can easily expose LoadBalancer services. As discussed in [Scenarios](/installation/pre-install) section, those latter are the requirements to have a "public-exposed" cluster, which can be accessed by other Liqo instances.

Liqo may be installed on a newly created clusters or on existing ones.

## Create a Liqo-compliant GKE Cluster

### Requirements

* a [Google Cloud account](https://cloud.google.com/?hl=it)

### Access the Google Cloud Console

The first step consists in accessing the Google Cloud [Console](https://cloud.google.com/?hl=it) in the Kubernetes Service.

![](/images/install/gke/01.png)

Clicking on the `Create` button, you can create a new cluster. In the new panel, you can select the desired name and a location for your cluster.

__NOTE__: So far, Liqo only supports Kubernetes >= 1.19.0 and clusters with a /16 pod CIDR. This parameter cannot be changed during the cluster's lifecycle and should be carefully chosen at cluster creation.

![](/images/install/gke/02.png)

#### Set the Node Pool

GKE clusters are organized in node pools. A [node pool](https://cloud.google.com/kubernetes-engine/docs/concepts/node-pools) is a "group of nodes within a cluster that all have the same configuration".

New node pools can be replaced and created during the cluster lifecycle. To be compatible with Liqo, your nodes should:

* Disable the ["Intranode Visibility"](https://cloud.google.com/kubernetes-engine/docs/how-to/intranode-visibility) feature.

> NOTE: Liqo is fully compliant with Google [Preemptible Nodes](https://cloud.google.com/kubernetes-engine/docs/how-to/preemptible-vms)

![](/images/install/gke/03.png)

Liqo does not require any other configurations to the cluster. You can click on the `Create` button.

Google Cloud will take some minutes to deploy your cluster.

## Deploy Liqo

### Installation with Helm

#### Values setting

In order to install Liqo, we need to configure some values of the Helm chart related to the accessibility of the cluster
and its internal configuration.

In particular, we have to set the following values:

| Variable               | Default | Description                                 |
| ---------------------- | ------- | ------------------------------------------- |
| `networkManager.config.podCIDR`             |  10.124.0.0/14        | The cluster Pod CIDR                        |
| `networkManager.config.serviceCIDR`         |  10.0.0.0/20       | The cluster Service CIDR                    |
| `auth.ingress.class`   | \<YOUR INGRESS CLASS\>     | The [ingress class](https://kubernetes.io/docs/concepts/services-networking/ingress/#ingress-class) to be used by the Auth Service Ingress |
| `apiServer.address`  |         | The hostname where to access the API server |
| `apiServer.port`  | 443 | the port where to access the API server     |
| `auth.ingress.host` |         | The hostname where to access the Auth Service, the one exposed with the ingress. If this parameter is not set, the service will be exposed with a [NodePort Service](https://kubernetes.io/docs/concepts/services-networking/service/#nodeport) instead of an [Ingress](https://kubernetes.io/docs/concepts/services-networking/ingress/) |
| `auth.ingress.port` | 443  | the port where to access the Auth Service   |

__NOTE__: if at install time you changed the default values make sure to set the right ones

The other values can be found in the Google Cloud Console.

The `apiServer.address` con be found in our cluster details as __Endpoint__.

![](/images/install/gke/04.png)

The `auth.ingress.host` is where the Liqo Auth Service will be reachable. If you are using an Ingress, you can set here
a hostname that you can manage. Another possible solution is to expose it as a `LoadBalancer` Service as described [below](#expose-the-auth-service-with-a-loadbalancer-service).

#### Deploy

You can install Liqo using helm 3.

Firstly, you should add the official Liqo repository to your Helm Configuration:

```bash
helm repo add liqo https://helm.liqo.io/
```

If you are installing Liqo for the first time, you can download the default values.yaml file from the chart.

```bash
helm show values liqo/liqo > ./values.yaml
```

After, modify the ```values.yaml``` as specified above to obtain the desired configuration and install Liqo.

```bash
helm install liqo liqo/liqo -f ./values.yaml -n liqo --create-namespace
```

#### Expose the Auth Service with a LoadBalancer Service

To make the Auth Service reachable without the needing of an Ingress and a Domain Name, you can change the `auth-service`
Service type from `NodePort` to `LoadBalancer` by setting the value `.auth.service.type` to `LoadBalancer`.

## Check that Liqo is Running

Wait that all Liqo pods and services are up and running

```bash
kubectl get pods -n liqo
```

```bash
kubectl get svc -n liqo
```

![](/images/install/gke/05.png)

### Access the cluster configurations

You can get the cluster configurations from the Auth Service endpoint to check that this service has been correctly deployed

```bash
curl --insecure https://34.71.59.19/ids
```

```json
{"clusterId":"0558de48-097b-4b7d-ba04-6bd2a0f9d24f","clusterName":"LiqoCluster0692","guestNamespace":"liqo"}
```

Congratulations! Liqo is now up and running on your GKE cluster; you can now peer with other Liqo instances!

### Establish a Peering

The Auth Service URL is the only required value to make this cluster peerable from the external world.

You can add a `ForeignCluster` resource in any other cluster where Liqo is installed to be able to join your cluster.

An example of this resource can be:

```yaml
apiVersion: discovery.liqo.io/v1alpha1
kind: ForeignCluster
metadata:
  name: my-gke-cluster
spec:
  authUrl: "https://34.71.59.19"
```

When the C.R. will be created the Liqo control plane will contact the URL shown in the step before with the curl command to
retrieve all the required cluster information.
